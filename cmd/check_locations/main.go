package main

import (
	"encoding/json"
	"fmt"
	"log"

	"shopify-demo/app"
)

func main() {
	fmt.Println("=== Checking Shopify Locations ===\n")

	const query = `
		query {
			locations(first: 20) {
				edges {
					node {
						id
						name
						isActive
						isPrimary
						fulfillmentService {
							id
							serviceName
						}
					}
				}
			}
		}`

	variables := map[string]interface{}{}
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

	locationsData, ok := data["locations"].(map[string]interface{})
	if !ok {
		log.Fatal("Locations not found")
	}

	edges, ok := locationsData["edges"].([]interface{})
	if !ok {
		log.Fatal("No edges found")
	}

	fmt.Printf("\n=== Found %d Location(s) ===\n\n", len(edges))
	for i, edge := range edges {
		if edgeMap, ok := edge.(map[string]interface{}); ok {
			if node, ok := edgeMap["node"].(map[string]interface{}); ok {
				fmt.Printf("[%d] %s\n", i+1, node["name"])
				fmt.Printf("     ID: %s\n", node["id"])
				fmt.Printf("     Is Active: %v\n", node["isActive"])
				fmt.Printf("     Is Primary: %v\n", node["isPrimary"])
				if fulfillmentService, ok := node["fulfillmentService"].(map[string]interface{}); ok && fulfillmentService["id"] != nil {
					fmt.Printf("     Fulfillment Service: %s\n", fulfillmentService["serviceName"])
				} else {
					fmt.Printf("     Fulfillment Service: Merchant Managed\n")
				}
				fmt.Println()
			}
		}
	}

	if len(edges) == 0 {
		fmt.Println("⚠ No locations found!")
		fmt.Println("This might be why FulfillmentOrders are not being created.")
		fmt.Println("Orders need at least one active location to create FulfillmentOrders.")
	}
}

