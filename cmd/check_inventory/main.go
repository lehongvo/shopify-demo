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
		log.Fatal("Usage: go run cmd/check_inventory/main.go <variant_id>")
	}

	variantID := os.Args[1]
	fmt.Printf("Checking Inventory for Variant: %s\n\n", variantID)

	const query = `
		query GetVariantInventory($id: ID!) {
			productVariant(id: $id) {
				id
				title
				product {
					id
					title
				}
				inventoryItem {
					id
					tracked
					inventoryLevels(first: 10) {
						edges {
							node {
								id
								quantities(names: ["available"]) {
									name
									quantity
								}
								location {
									id
									name
								}
							}
						}
					}
				}
			}
		}`

	variables := map[string]interface{}{
		"id": variantID,
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

	variant, ok := data["productVariant"].(map[string]interface{})
	if !ok {
		log.Fatal("Variant not found")
	}

	fmt.Println("\n=== Variant Information ===")
	if product, ok := variant["product"].(map[string]interface{}); ok {
		fmt.Printf("Product: %s\n", product["title"])
	}
	fmt.Printf("Variant: %s\n", variant["title"])

	if inventoryItem, ok := variant["inventoryItem"].(map[string]interface{}); ok {
		fmt.Printf("\nInventory Item ID: %s\n", inventoryItem["id"])
		fmt.Printf("Tracked: %v\n", inventoryItem["tracked"])

		if inventoryLevels, ok := inventoryItem["inventoryLevels"].(map[string]interface{}); ok {
			if edges, ok := inventoryLevels["edges"].([]interface{}); ok {
				fmt.Printf("\n=== Inventory Levels (%d location(s)) ===\n", len(edges))
				if len(edges) == 0 {
					fmt.Println("⚠ No inventory levels found!")
					fmt.Println("This might be why FulfillmentOrders are not created.")
					fmt.Println("Products need inventory at locations to create FulfillmentOrders.")
				} else {
					for i, edge := range edges {
						if edgeMap, ok := edge.(map[string]interface{}); ok {
							if node, ok := edgeMap["node"].(map[string]interface{}); ok {
								if location, ok := node["location"].(map[string]interface{}); ok {
									fmt.Printf("\n[%d] Location: %s\n", i+1, location["name"])
									if quantities, ok := node["quantities"].([]interface{}); ok {
										for _, qty := range quantities {
											if qtyMap, ok := qty.(map[string]interface{}); ok {
												fmt.Printf("     %s: %v\n", qtyMap["name"], qtyMap["quantity"])
											}
										}
									}
									fmt.Printf("     Location ID: %s\n", location["id"])
								}
							}
						}
					}
				}
			}
		}
	}
}

