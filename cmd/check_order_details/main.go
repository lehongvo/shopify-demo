package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"shopify-demo/app"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run cmd/check_order_details/main.go <order_id>")
	}

	orderID := os.Args[1]
	fmt.Printf("Checking Order Details: %s\n\n", orderID)

	const query = `
		query GetOrderDetails($id: ID!) {
			order(id: $id) {
				id
				name
				email
				displayFinancialStatus
				displayFulfillmentStatus
				lineItems(first: 10) {
					edges {
						node {
							id
							title
							quantity
							variant {
								id
								title
							}
						}
					}
				}
				fulfillmentOrders(first: 10) {
					edges {
						node {
							id
							status
							requestStatus
						}
					}
				}
			}
		}`

	variables := map[string]interface{}{
		"id": orderID,
	}

	resp, err := app.CallAdminGraphQL(query, variables)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	jsonData, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println("Full Response:")
	fmt.Println(string(jsonData))

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		log.Fatal("No data in response")
	}

	order, ok := data["order"].(map[string]interface{})
	if !ok {
		log.Fatal("Order not found")
	}

	fmt.Println("\n=== Order Summary ===")
	fmt.Printf("Order Name: %s\n", order["name"])
	fmt.Printf("Email: %s\n", order["email"])
	fmt.Printf("Financial Status: %s\n", order["displayFinancialStatus"])
	fmt.Printf("Fulfillment Status: %s\n", order["displayFulfillmentStatus"])

	// Check line items
	if lineItemsData, ok := order["lineItems"].(map[string]interface{}); ok {
		if edges, ok := lineItemsData["edges"].([]interface{}); ok {
			fmt.Printf("\nLine Items: %d\n", len(edges))
			for i, edge := range edges {
				if edgeMap, ok := edge.(map[string]interface{}); ok {
					if node, ok := edgeMap["node"].(map[string]interface{}); ok {
						fmt.Printf("\n  [%d] %s\n", i+1, node["title"])
						fmt.Printf("      Quantity: %v\n", node["quantity"])
						if variant, ok := node["variant"].(map[string]interface{}); ok {
							fmt.Printf("      Variant: %s\n", variant["title"])
						}
					}
				}
			}
		}
	}

	// Check fulfillment orders
	if foData, ok := order["fulfillmentOrders"].(map[string]interface{}); ok {
		if edges, ok := foData["edges"].([]interface{}); ok {
			fmt.Printf("\nFulfillment Orders: %d\n", len(edges))
			if len(edges) == 0 {
				fmt.Println("\n⚠ No FulfillmentOrders found!")
				fmt.Println("Possible reasons:")
				fmt.Println("  1. Order routing not complete (wait a few minutes)")
				fmt.Println("  2. All items are digital (requiresShipping = false)")
				fmt.Println("  3. No inventory locations configured")
			}
		}
	}
}

