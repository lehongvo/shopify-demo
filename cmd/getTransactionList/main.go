package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// Transaction represents a Shopify transaction payload (minimal fields).
type Transaction struct {
	ID              int64   `json:"id"`
	OrderID         int64   `json:"order_id"`
	Kind            string  `json:"kind"`
	Status          string  `json:"status"`
	Amount          string  `json:"amount"`
	Gateway         string  `json:"gateway"`
	Source          string  `json:"source"`
	ProcessedAt     string  `json:"processed_at"`
	PaymentID       int64   `json:"payment_id,omitempty"`
	Authorization   string  `json:"authorization,omitempty"`
	Currency        string  `json:"currency,omitempty"`
	PaymentDetails  any     `json:"payment_details,omitempty"`
	Test            bool    `json:"test,omitempty"`
	Receipt         any     `json:"receipt,omitempty"`
	Message         string  `json:"message,omitempty"`
	ParentID        *int64  `json:"parent_id,omitempty"`
	DeviceID        *int64  `json:"device_id,omitempty"`
	TotalUnsettled  *string `json:"total_unsettled_set,omitempty"`
	UserID          *int64  `json:"user_id,omitempty"`
	AdminGraphqlAPI string  `json:"admin_graphql_api_id,omitempty"`
}

type transactionsResponse struct {
	Transactions []Transaction `json:"transactions"`
}

func main() {
	shopDomain := os.Getenv("SHOPIFY_SHOP_DOMAIN")
	accessToken := os.Getenv("SHOPIFY_API_SECRET")
	if shopDomain == "" || accessToken == "" {
		log.Fatal("SHOPIFY_SHOP_DOMAIN and SHOPIFY_API_SECRET must be set in environment variables")
	}

	orderID := "5725042999448" // default from request
	if len(os.Args) > 1 && os.Args[1] != "" {
		orderID = os.Args[1]
	}

	apiVersion := "2025-01"
	url := fmt.Sprintf("https://%s/admin/api/%s/orders/%s/transactions.json", shopDomain, apiVersion, orderID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("X-Shopify-Access-Token", accessToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("request failed: status %d", resp.StatusCode)
	}

	var tr transactionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		log.Fatalf("failed to decode response: %v", err)
	}

	fmt.Printf("✓ Transactions for order %s (count: %d)\n", orderID, len(tr.Transactions))
	for i, t := range tr.Transactions {
		fmt.Printf("%d) id=%d kind=%s status=%s amount=%s gateway=%s processed_at=%s\n",
			i+1, t.ID, t.Kind, t.Status, t.Amount, t.Gateway, t.ProcessedAt)
	}
}
