package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"shopify-demo/app"
)

func main() {
	// Kiểm tra environment variables
	shopDomain := os.Getenv("SHOPIFY_SHOP_DOMAIN")
	accessToken := os.Getenv("SHOPIFY_API_SECRET")

	if shopDomain == "" || accessToken == "" {
		log.Fatal("SHOPIFY_SHOP_DOMAIN and SHOPIFY_API_SECRET must be set in environment variables")
	}

	// Lấy order ID từ command line argument hoặc hardcode để test
	orderID := os.Args[1]
	if orderID == "" {
		log.Fatal("Please provide order ID as argument: go run cmd/test_fulfillment_orders/main.go <order_id>")
	}

	fmt.Println("Querying FulfillmentOrders for order:", orderID)
	fmt.Println("Shop Domain:", shopDomain)
	fmt.Println("---")

	// Thử query nhiều lần với delay
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			fmt.Printf("\nRetry #%d (waiting 2 seconds...)\n", i+1)
			time.Sleep(2 * time.Second)
		}

		fulfillmentOrders, err := app.GetFulfillmentOrders(orderID)
		if err != nil {
			log.Printf("Error querying FulfillmentOrders: %v\n", err)
			continue
		}

		if len(fulfillmentOrders) > 0 {
			fmt.Printf("\n✓ Found %d FulfillmentOrder(s)!\n\n", len(fulfillmentOrders))
			for i, fo := range fulfillmentOrders {
				fmt.Printf("FulfillmentOrder #%d:\n", i+1)
				fmt.Printf("  ID: %s\n", fo.ID)
				fmt.Printf("  Status: %s\n", fo.Status)
				fmt.Printf("  Request Status: %s\n", fo.RequestStatus)
				fmt.Printf("  Assigned Location ID: %s\n", fo.AssignedLocationID)
				fmt.Printf("  Line Items Count: %d\n", len(fo.LineItems))
				for j, li := range fo.LineItems {
					fmt.Printf("    [%d] FulfillmentOrderLineItem ID: %s\n", j+1, li.ID)
					fmt.Printf("         LineItem ID: %s\n", li.LineItemID)
					fmt.Printf("         Quantity: %d\n", li.Quantity)
				}
				fmt.Println()
			}
			return
		}

		fmt.Println("No FulfillmentOrders found yet...")
	}

	fmt.Println("\n---")
	fmt.Println("FulfillmentOrders not found after", maxRetries, "retries.")
	fmt.Println("This might mean:")
	fmt.Println("  1. Order routing is still processing")
	fmt.Println("  2. Order doesn't have items that need fulfillment")
	fmt.Println("  3. Access scopes might not include fulfillment orders")
}





