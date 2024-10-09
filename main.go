package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Initialize the database, create the table if it doesn't exist
func initDb(pool *pgxpool.Pool) error {
	createTableQuery := `
    CREATE TABLE IF NOT EXISTS pool_usage (
        id SERIAL PRIMARY KEY,
        timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        percentage INT NOT NULL
    );`

	_, err := pool.Exec(context.Background(), createTableQuery)
	if err != nil {
		return fmt.Errorf("error creating table: %v", err)
	}
	return nil
}

// Fetch pool usage from the website
func fetchPoolUsage() (int, error) {
	url := "https://www.lazdynubaseinas.eu/"

	// Send an HTTP GET request
	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("error fetching the URL: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading the response body: %v", err)
	}

	// Convert the response body to a string
	htmlContent := string(body)

	// Use a regular expression to find the usage percentage
	re := regexp.MustCompile(`Šiuo metu esantis Lazdynų baseino ir sporto klubo užimtumas: <span style="font-size:\d+\.\d+rem;">(\d+)%</span>`)
	matches := re.FindStringSubmatch(htmlContent)
	if len(matches) > 1 {
		// Convert the matched percentage to an integer
		usage, err := strconv.Atoi(matches[1])
		if err != nil {
			return 0, fmt.Errorf("error converting percentage: %v", err)
		}
		return usage, nil
	}
	return 0, fmt.Errorf("could not find usage percentage")
}

// Save the pool usage to the database
func saveToDatabase(pool *pgxpool.Pool, poolUsage int) error {
	// Insert the data into the table
	_, err := pool.Exec(context.Background(),
		"INSERT INTO pool_usage (timestamp, percentage) VALUES ($1, $2)",
		time.Now(), poolUsage)
	if err != nil {
		return fmt.Errorf("error inserting data into database: %v", err)
	}

	return nil
}

func main() {
	// Get the database connection parameters from environment variables
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Println("DATABASE_URL environment variable is not set")
		return
	}

	// Create a connection pool
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		fmt.Printf("Unable to parse DATABASE_URL: %v\n", err)
		return
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		fmt.Printf("Unable to create connection pool: %v\n", err)
		return
	}
	defer pool.Close()

	// Initialize the database
	err = initDb(pool)
	if err != nil {
		fmt.Println("Error initializing the database:", err)
		return
	}

	// Fetch the pool usage
	usage, err := fetchPoolUsage()
	if err != nil {
		fmt.Println("Error fetching pool usage:", err)
		return
	}
	fmt.Printf("Current swimming pool usage: %d%%\n", usage)

	// Save the result to the database
	err = saveToDatabase(pool, usage)
	if err != nil {
		fmt.Println("Error saving to database:", err)
	} else {
		fmt.Println("Data successfully saved to the database.")
	}
}
