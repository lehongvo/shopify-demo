package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"shopify-demo/app"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run cmd/check_fulfillments/main.go <order_id>")
	}

	orderID := os.Args[1]
	fmt.Printf("Checking Fulfillments for Order: %s\n\n", orderID)

	// Method 1: REST API
	fmt.Println("=== Method 1: REST API ===")
	checkFulfillmentOrdersREST(orderID)

	// Method 2: GraphQL API
	fmt.Println("\n=== Method 2: GraphQL API ===")
	checkFulfillmentOrdersGraphQL(orderID)
}

// checkFulfillmentOrdersREST checks FulfillmentOrders using REST API
func checkFulfillmentOrdersREST(orderID string) {
	shopDomain := os.Getenv("SHOPIFY_SHOP_DOMAIN")
	accessToken := os.Getenv("SHOPIFY_API_SECRET")

	if shopDomain == "" || accessToken == "" {
		log.Fatal("SHOPIFY_SHOP_DOMAIN and SHOPIFY_API_SECRET must be set")
	}

	apiVersion := "2025-10"

	// Extract numeric ID from GID if needed
	numericOrderID := orderID
	if len(orderID) > 20 {
		// Extract numeric part from GID format: gid://shopify/Order/6593477968112
		parts := orderID
		if len(parts) > 0 {
			// Try to extract the numeric ID
			var numID string
			for i := len(parts) - 1; i >= 0; i-- {
				if parts[i] >= '0' && parts[i] <= '9' {
					numID = string(parts[i]) + numID
				} else if len(numID) > 0 {
					break
				}
			}
			if len(numID) > 0 {
				numericOrderID = numID
			}
		}
	}

	fmt.Printf("Using numeric Order ID: %s\n", numericOrderID)

	// Try different REST API endpoints
	endpoints := []string{
		fmt.Sprintf("https://%s/admin/api/%s/orders/%s/fulfillment_orders.json", shopDomain, apiVersion, numericOrderID),
		fmt.Sprintf("https://%s/admin/api/%s/fulfillment_orders.json?order_id=%s", shopDomain, apiVersion, numericOrderID),
		fmt.Sprintf("https://%s/admin/api/%s/fulfillment_orders.json", shopDomain, apiVersion),
	}

	for i, endpoint := range endpoints {
		fmt.Printf("\n[%d] Trying endpoint: %s\n", i+1, endpoint)

		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			fmt.Printf("   ❌ Error creating request: %v\n", err)
			continue
		}

		req.Header.Set("X-Shopify-Access-Token", accessToken)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("   ❌ Error executing request: %v\n", err)
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("   ❌ Error reading response: %v\n", err)
			continue
		}

		fmt.Printf("   Status: %d\n", resp.StatusCode)

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("   Response: %s\n", string(body))
			continue
		}

		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			fmt.Printf("   ❌ Error parsing JSON: %v\n", err)
			fmt.Printf("   Raw response: %s\n", string(body))
			continue
		}

		jsonData, _ := json.MarshalIndent(result, "", "  ")
		fmt.Printf("   Response:\n%s\n", string(jsonData))

		// Check for fulfillment_orders
		if fulfillmentOrders, ok := result["fulfillment_orders"].([]interface{}); ok {
			fmt.Printf("\n   ✓ Found %d FulfillmentOrder(s) via REST API\n", len(fulfillmentOrders))
			if len(fulfillmentOrders) > 0 {
				for j, fo := range fulfillmentOrders {
					if foMap, ok := fo.(map[string]interface{}); ok {
						// Format ID properly (handle float64 from JSON)
						foID := formatID(foMap["id"])
						status := foMap["status"]
						requestStatus := foMap["request_status"]
						
						fmt.Printf("\n      [%d] FulfillmentOrder:\n", j+1)
						fmt.Printf("         ID: %s\n", foID)
						fmt.Printf("         Status: %v\n", status)
						fmt.Printf("         Request Status: %v\n", requestStatus)
						
						if assignedLocation, ok := foMap["assigned_location"].(map[string]interface{}); ok {
							locationID := formatID(assignedLocation["location_id"])
							fmt.Printf("         Location: %v (ID: %s)\n", assignedLocation["name"], locationID)
						}
						
						if lineItems, ok := foMap["line_items"].([]interface{}); ok {
							fmt.Printf("         Line Items: %d\n", len(lineItems))
							for k, li := range lineItems {
								if liMap, ok := li.(map[string]interface{}); ok {
									lineItemID := formatID(liMap["line_item_id"])
									fmt.Printf("            [%d] LineItem ID: %s, Quantity: %v\n", 
										k+1, lineItemID, liMap["quantity"])
								}
							}
						}
						
						if supportedActions, ok := foMap["supported_actions"].([]interface{}); ok {
							fmt.Printf("         Supported Actions: %v\n", supportedActions)
						}
					}
				}
				return // Success, stop trying other endpoints
			}
		} else if fulfillmentOrders, ok := result["fulfillment_orders"].(map[string]interface{}); ok {
			// Sometimes it's wrapped differently
			fmt.Printf("   Response structure: %+v\n", fulfillmentOrders)
		} else {
			fmt.Printf("   ⚠ No fulfillment_orders field found in response\n")
		}
	}

	fmt.Println("\n   ⚠ REST API endpoints did not return FulfillmentOrders")
	fmt.Println("   Note: REST API may not fully support fulfillment_orders endpoint")
	fmt.Println("   Use GraphQL API instead (see Method 2 below)")
}

// checkFulfillmentOrdersGraphQL checks FulfillmentOrders using GraphQL API
func checkFulfillmentOrdersGraphQL(orderID string) {

	const query = `
		query GetOrderFulfillments($id: ID!) {
			order(id: $id) {
				id
				name
				fulfillments(first: 10) {
					id
					status
					trackingInfo {
						number
						company
						url
					}
					fulfillmentLineItems(first: 10) {
						edges {
							node {
								lineItem {
									id
									title
								}
								quantity
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
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	jsonData, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println("Full Response:")
	fmt.Println(string(jsonData))

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		fmt.Println("❌ No data in response")
		return
	}

	order, ok := data["order"].(map[string]interface{})
	if !ok {
		fmt.Println("❌ Order not found")
		return
	}

	fmt.Println("\n=== Order Summary ===")
	fmt.Printf("Order: %s\n\n", order["name"])

	// Check fulfillments
	if fulfillments, ok := order["fulfillments"].([]interface{}); ok {
		fmt.Printf("Fulfillments: %d\n", len(fulfillments))
		if len(fulfillments) > 0 {
			fmt.Println("\n⚠ Order has been fulfilled automatically!")
			fmt.Println("This might be why FulfillmentOrders are not visible.")
			fmt.Println("FulfillmentOrders may have been consumed by auto-fulfillment.")
		}
	}

	// Check fulfillment orders
	if foData, ok := order["fulfillmentOrders"].(map[string]interface{}); ok {
		if edges, ok := foData["edges"].([]interface{}); ok {
			fmt.Printf("\nFulfillmentOrders: %d\n", len(edges))
			if len(edges) == 0 {
				fmt.Println("\n⚠ No FulfillmentOrders found")
			} else {
				fmt.Println("\n✓ FulfillmentOrders found via GraphQL:")
				for i, edge := range edges {
					if edgeMap, ok := edge.(map[string]interface{}); ok {
						if node, ok := edgeMap["node"].(map[string]interface{}); ok {
							fmt.Printf("   [%d] ID: %v, Status: %v, RequestStatus: %v\n",
								i+1, node["id"], node["status"], node["requestStatus"])
						}
					}
				}
			}
		}
	}
}

// formatID formats numeric ID from JSON (handles float64) to string
func formatID(id interface{}) string {
	switch v := id.(type) {
	case float64:
		// JSON numbers are parsed as float64, convert to int64 then string
		return fmt.Sprintf("%.0f", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case int:
		return fmt.Sprintf("%d", v)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", id)
	}
}

