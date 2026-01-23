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

	// Kiểm tra order number argument
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run cmd/get_delivery_method/main.go <order_number>\nExample: go run cmd/get_delivery_method/main.go 2291")
	}

	orderNumber := os.Args[1]
	fmt.Printf("=== Getting Delivery Method for Order #%s ===\n\n", orderNumber)

	// Query order by name (order number)
	// Shopify order name format is usually #2291, #2292, etc.
	orderName := "#" + orderNumber
	
	// GraphQL query để lấy delivery method từ order
	const query = `
		query GetOrderDeliveryMethod($query: String!) {
			orders(first: 1, query: $query) {
				edges {
					node {
						id
						name
						legacyResourceId
						displayFulfillmentStatus
						shippingLine {
							title
							code
							source
							carrierIdentifier
							deliveryCategory
						}
						fulfillmentOrders(first: 10) {
							edges {
								node {
									id
									status
									deliveryMethod {
										id
										methodType
									}
									assignedLocation {
										location {
											id
											name
										}
									}
								}
							}
						}
					}
				}
			}
		}`

	variables := map[string]interface{}{
		"query": fmt.Sprintf("name:%s", orderName),
	}

	resp, err := app.CallAdminGraphQL(query, variables)
	if err != nil {
		log.Fatalf("Error calling Shopify API: %v", err)
	}

	// Pretty print full response
	jsonData, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println("=== Full Response ===")
	fmt.Println(string(jsonData))

	// Parse response
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		log.Fatal("No data in response")
	}

	orders, ok := data["orders"].(map[string]interface{})
	if !ok {
		log.Fatal("Orders not found in response")
	}

	edges, ok := orders["edges"].([]interface{})
	if !ok || len(edges) == 0 {
		log.Fatalf("Order #%s not found", orderNumber)
	}

	// Lấy order đầu tiên (chỉ có 1 order)
	edgeMap, ok := edges[0].(map[string]interface{})
	if !ok {
		log.Fatal("Invalid order data")
	}

	node, ok := edgeMap["node"].(map[string]interface{})
	if !ok {
		log.Fatal("Invalid order node")
	}

	fmt.Println("\n=== Order Information ===")
	fmt.Printf("Order ID: %v\n", node["id"])
	fmt.Printf("Order Name: %v\n", node["name"])
	fmt.Printf("Order Number: %v\n", node["legacyResourceId"])
	fmt.Printf("Fulfillment Status: %v\n", node["displayFulfillmentStatus"])

	// Lấy Shipping Line (Delivery Method từ shipping)
	fmt.Println("\n=== Delivery Method (Shipping Line) ===")
	if shippingLine, ok := node["shippingLine"].(map[string]interface{}); ok && shippingLine != nil {
		if title, ok := shippingLine["title"].(string); ok {
			fmt.Printf("✓ Delivery Method: %s\n", title)
		}
		if code, ok := shippingLine["code"].(string); ok && code != "" {
			fmt.Printf("  Code: %s\n", code)
		}
		if source, ok := shippingLine["source"].(string); ok && source != "" {
			fmt.Printf("  Source: %s\n", source)
		}
		if carrier, ok := shippingLine["carrierIdentifier"].(string); ok && carrier != "" {
			fmt.Printf("  Carrier: %s\n", carrier)
		}
		if category, ok := shippingLine["deliveryCategory"].(string); ok && category != "" {
			fmt.Printf("  Category: %s\n", category)
		}
	} else {
		fmt.Println("⚠ No shipping line found (might be a pickup/local delivery order)")
	}

	// Lấy Fulfillment Orders Delivery Method
	fmt.Println("\n=== Fulfillment Orders Delivery Method ===")
	if fulfillmentOrders, ok := node["fulfillmentOrders"].(map[string]interface{}); ok {
		if foEdges, ok := fulfillmentOrders["edges"].([]interface{}); ok {
			if len(foEdges) > 0 {
				fmt.Printf("Found %d fulfillment order(s):\n\n", len(foEdges))
				for i, foEdge := range foEdges {
					if foEdgeMap, ok := foEdge.(map[string]interface{}); ok {
						if foNode, ok := foEdgeMap["node"].(map[string]interface{}); ok {
							fmt.Printf("Fulfillment Order #%d:\n", i+1)
							fmt.Printf("  ID: %v\n", foNode["id"])
							fmt.Printf("  Status: %v\n", foNode["status"])
							
							// Delivery Method
							if deliveryMethod, ok := foNode["deliveryMethod"].(map[string]interface{}); ok && deliveryMethod != nil {
								if methodType, ok := deliveryMethod["methodType"].(string); ok {
									fmt.Printf("  ✓ Delivery Method Type: %s\n", methodType)
								}
								if methodID, ok := deliveryMethod["id"].(string); ok && methodID != "" {
									fmt.Printf("  Method ID: %s\n", methodID)
								}
							} else {
								fmt.Println("  ⚠ No delivery method specified")
							}
							
							// Location
							if assignedLoc, ok := foNode["assignedLocation"].(map[string]interface{}); ok && assignedLoc != nil {
								if location, ok := assignedLoc["location"].(map[string]interface{}); ok && location != nil {
									if locName, ok := location["name"].(string); ok {
										fmt.Printf("  Location: %s\n", locName)
									}
								}
							}
							fmt.Println()
						}
					}
				}
			} else {
				fmt.Println("⚠ No fulfillment orders found")
			}
		}
	}

	// Summary
	fmt.Println("\n=== Summary ===")
	deliveryMethod := "N/A"
	if shippingLine, ok := node["shippingLine"].(map[string]interface{}); ok && shippingLine != nil {
		if title, ok := shippingLine["title"].(string); ok {
			deliveryMethod = title
		}
	}
	fmt.Printf("Order #%s Delivery Method: %s\n", orderNumber, deliveryMethod)
}
