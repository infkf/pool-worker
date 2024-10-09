package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Function to send a message to a Telegram bot
func sendTelegramMessage(botToken, chatID, message string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	reqBody := fmt.Sprintf("chat_id=%s&text=%s", chatID, message)

	resp, err := http.Post(url, "application/x-www-form-urlencoded", strings.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to send message: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API responded with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

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
	// Get the database connection parameters and Telegram bot credentials from environment variables
	dbURL := os.Getenv("DATABASE_URL")
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	if dbURL == "" || botToken == "" || chatID == "" {
		log.Fatal("DATABASE_URL, TELEGRAM_BOT_TOKEN, and TELEGRAM_CHAT_ID environment variables must be set")
	}

	// Create a connection pool
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatalf("Unable to parse DATABASE_URL: %v\n", err)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Fatalf("Unable to create connection pool: %v\n", err)
	}
	defer pool.Close()

	// Initialize the database
	err = initDb(pool)
	if err != nil {
		log.Fatalf("Error initializing the database: %v\n", err)
	}

	// Fetch the pool usage
	usage, err := fetchPoolUsage()
	if err != nil {
		log.Printf("Error fetching pool usage: %v\n", err)
		return
	}
	log.Printf("Current swimming pool usage: %d%%\n", usage)

	// Save the result to the database
	err = saveToDatabase(pool, usage)
	if err != nil {
		log.Printf("Error saving to database: %v\n", err)
	} else {
		log.Println("Data successfully saved to the database.")
	}

	// Send the result to the Telegram bot
	message := fmt.Sprintf("Current swimming pool usage is %d%%", usage)
	err = sendTelegramMessage(botToken, chatID, message)
	if err != nil {
		log.Printf("Error sending message to Telegram: %v\n", err)
	} else {
		log.Println("Message successfully sent to Telegram.")
	}
}
