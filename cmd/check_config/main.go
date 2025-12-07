package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"shopify-demo/app"
)

func main() {
	// Kiểm tra environment variables
	shopDomain := os.Getenv("SHOPIFY_SHOP_DOMAIN")
	accessToken := os.Getenv("SHOPIFY_API_SECRET")

	if shopDomain == "" || accessToken == "" {
		log.Fatal("SHOPIFY_SHOP_DOMAIN and SHOPIFY_API_SECRET must be set in environment variables")
	}

	fmt.Println("=== Checking Shopify App Configuration ===\n")
	fmt.Printf("Shop Domain: %s\n", shopDomain)
	fmt.Printf("Access Token: %s...%s\n\n", accessToken[:10], accessToken[len(accessToken)-10:])

	// 1. Check shop info
	fmt.Println("--- 1. Shop Information ---")
	shopInfo, err := getShopInfo()
	if err != nil {
		log.Printf("Error getting shop info: %v\n", err)
	} else {
		fmt.Printf("Shop Name: %s\n", shopInfo["name"])
		fmt.Printf("Shop Email: %s\n", shopInfo["email"])
		fmt.Printf("Shop ID: %s\n", shopInfo["id"])
	}

	// 2. Check current app (if available)
	fmt.Println("\n--- 2. Current App Information ---")
	appInfo, err := getCurrentApp()
	if err != nil {
		fmt.Printf("Note: Cannot get app info (may require different API): %v\n", err)
	} else {
		fmt.Printf("App Info: %+v\n", appInfo)
	}

	// 3. Test query order with fulfillmentOrders
	fmt.Println("\n--- 3. Testing Order Query with FulfillmentOrders ---")
	if len(os.Args) > 1 {
		orderID := os.Args[1]
		fmt.Printf("Querying Order: %s\n", orderID)
		testOrderQuery(orderID)
	} else {
		fmt.Println("No order ID provided. Usage: go run cmd/check_config/main.go <order_id>")
	}

	// 4. Test direct fulfillmentOrders query
	fmt.Println("\n--- 4. Testing Direct FulfillmentOrders Query ---")
	testFulfillmentOrdersQuery()
}

func getShopInfo() (map[string]interface{}, error) {
	const query = `
		query {
			shop {
				id
				name
				email
				myshopifyDomain
			}
		}`

	variables := map[string]interface{}{}
	resp, err := app.CallAdminGraphQL(query, variables)
	if err != nil {
		return nil, err
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response structure")
	}

	shop, ok := data["shop"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("shop not found in response")
	}

	return shop, nil
}

func getCurrentApp() (map[string]interface{}, error) {
	const query = `
		query {
			currentAppInstallation {
				id
				launchUrl
			}
		}`

	variables := map[string]interface{}{}
	resp, err := app.CallAdminGraphQL(query, variables)
	if err != nil {
		return nil, err
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response structure")
	}

	app, ok := data["currentAppInstallation"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("currentAppInstallation not found")
	}

	return app, nil
}

func testOrderQuery(orderID string) {
	const query = `
		query GetOrderWithFulfillmentOrders($id: ID!) {
			order(id: $id) {
				id
				name
				email
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

	// Pretty print response
	jsonData, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println("Response:")
	fmt.Println(string(jsonData))

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		fmt.Println("❌ No data in response")
		return
	}

	order, ok := data["order"].(map[string]interface{})
	if !ok {
		fmt.Println("❌ Order not found in response")
		return
	}

	fmt.Printf("\n✓ Order found: %s\n", order["name"])

	fulfillmentOrders, ok := order["fulfillmentOrders"].(map[string]interface{})
	if !ok {
		fmt.Println("⚠ fulfillmentOrders field missing or null")
		fmt.Println("   This might indicate:")
		fmt.Println("   - Access scope issue (need read_merchant_managed_fulfillment_orders)")
		fmt.Println("   - Order routing not complete yet")
		fmt.Println("   - Order has no items requiring fulfillment")
		return
	}

	edges, ok := fulfillmentOrders["edges"].([]interface{})
	if !ok {
		fmt.Println("⚠ fulfillmentOrders.edges is not an array")
		return
	}

	fmt.Printf("✓ Found %d FulfillmentOrder(s)\n", len(edges))
}

func testFulfillmentOrdersQuery() {
	const query = `
		query {
			fulfillmentOrders(first: 5) {
				edges {
					node {
						id
						status
						order {
							id
							name
						}
					}
				}
			}
		}`

	variables := map[string]interface{}{}
	resp, err := app.CallAdminGraphQL(query, variables)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		fmt.Println("   This might indicate:")
		fmt.Println("   - Missing access scope: read_merchant_managed_fulfillment_orders")
		fmt.Println("   - Missing access scope: read_assigned_fulfillment_orders")
		return
	}

	jsonData, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println("Response:")
	fmt.Println(string(jsonData))

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		fmt.Println("❌ No data in response")
		return
	}

	fulfillmentOrders, ok := data["fulfillmentOrders"].(map[string]interface{})
	if !ok {
		fmt.Println("⚠ fulfillmentOrders query not available")
		return
	}

	edges, ok := fulfillmentOrders["edges"].([]interface{})
	if !ok {
		fmt.Println("⚠ No edges found")
		return
	}

	fmt.Printf("✓ Found %d FulfillmentOrder(s) in shop\n", len(edges))
}

