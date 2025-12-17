package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"shopify-demo/app"
)

func main() {
	// Load environment variables
	if err := loadEnv(); err != nil {
		fmt.Printf("Error loading environment: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=" + strings.Repeat("=", 60))
	fmt.Println("Testing REST API draft_orders.json with tax_lines")
	fmt.Println("=" + strings.Repeat("=", 60))
	fmt.Println()

	// Run the test
	if err := app.TestCreateDraftOrderWithTaxLines(); err != nil {
		fmt.Printf("\n❌ Test failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("Test completed")
	fmt.Println("=" + strings.Repeat("=", 60))
}

func loadEnv() error {
	// Try to load from .env file
	envFile := ".env"
	if _, err := os.Stat(envFile); err == nil {
		file, err := os.Open(envFile)
		if err != nil {
			return err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				// Remove quotes if present
				value = strings.Trim(value, "\"'")
				os.Setenv(key, value)
			}
		}
		return scanner.Err()
	}
	return nil
}

