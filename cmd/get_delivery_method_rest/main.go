package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// RestOrder represents Shopify Order from REST API
type RestOrder struct {
	ID                   int64         `json:"id"`
	Name                 string        `json:"name"`
	OrderNumber          int           `json:"order_number"`
	FulfillmentStatus    string        `json:"fulfillment_status"`
	FinancialStatus      string        `json:"financial_status"`
	ShippingLines        []ShippingLine `json:"shipping_lines"`
	Email                string        `json:"email"`
	TotalPrice           string        `json:"total_price"`
	Currency             string        `json:"currency"`
}

// ShippingLine represents shipping line (delivery method)
type ShippingLine struct {
	ID                          int64   `json:"id"`
	Title                       string  `json:"title"`
	Price                       string  `json:"price"`
	Code                        string  `json:"code"`
	Source                      string  `json:"source"`
	Phone                       string  `json:"phone"`
	RequestedFulfillmentServiceID string `json:"requested_fulfillment_service_id"`
	DeliveryCategory            string  `json:"delivery_category"`
	CarrierIdentifier           string  `json:"carrier_identifier"`
	DiscountedPrice             string  `json:"discounted_price"`
}

// OrdersResponse represents REST API response
type OrdersResponse struct {
	Orders []RestOrder `json:"orders"`
}

func main() {
	// Kiểm tra environment variables
	shopDomain := os.Getenv("SHOPIFY_SHOP_DOMAIN")
	accessToken := os.Getenv("SHOPIFY_API_SECRET")

	if shopDomain == "" || accessToken == "" {
		log.Fatal("SHOPIFY_SHOP_DOMAIN and SHOPIFY_API_SECRET must be set in environment variables")
	}

	// Kiểm tra order number argument
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run cmd/get_delivery_method/main_rest.go <order_number>\nExample: go run cmd/get_delivery_method/main_rest.go 2291")
	}

	orderNumber := os.Args[1]

	apiVersion := "2025-01"
	url := fmt.Sprintf("https://%s/admin/api/%s/orders.json?name=%s&status=any", shopDomain, apiVersion, orderNumber)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("X-Shopify-Access-Token", accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var ordersResp OrdersResponse
	err = json.Unmarshal(body, &ordersResp)
	if err != nil {
		log.Fatalf("Failed to parse response: %v", err)
	}

	if len(ordersResp.Orders) == 0 {
		log.Fatalf("Order #%s not found", orderNumber)
	}

	order := ordersResp.Orders[0]
	deliveryMethod := "N/A"
	if len(order.ShippingLines) > 0 {
		deliveryMethod = order.ShippingLines[0].Title
	}

	// Log một lần duy nhất
	result := map[string]interface{}{
		"order_number":    orderNumber,
		"order_id":        order.ID,
		"delivery_method": deliveryMethod,
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(jsonData))
}
