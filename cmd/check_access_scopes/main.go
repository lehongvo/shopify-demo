package main

import (
	"encoding/json"
	"fmt"
	"log"

	"shopify-demo/app"
)

func main() {
	fmt.Println("=== Checking API Access Scopes ===\n")

	const query = `
		query {
			currentAppInstallation {
				id
				launchUrl
				accessScopes {
					handle
					description
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

	appInstallation, ok := data["currentAppInstallation"].(map[string]interface{})
	if !ok {
		log.Fatal("currentAppInstallation not found")
	}

	fmt.Println("\n=== Access Scopes ===")
	if scopes, ok := appInstallation["accessScopes"].([]interface{}); ok {
		fmt.Printf("Total Scopes: %d\n\n", len(scopes))
		
		fulfillmentScopes := []string{
			"read_merchant_managed_fulfillment_orders",
			"write_merchant_managed_fulfillment_orders",
			"read_assigned_fulfillment_orders",
			"write_assigned_fulfillment_orders",
		}
		
		hasFulfillmentScopes := false
		for _, scope := range scopes {
			if scopeMap, ok := scope.(map[string]interface{}); ok {
				handle := scopeMap["handle"].(string)
				description := scopeMap["description"].(string)
				fmt.Printf("- %s\n  %s\n\n", handle, description)
				
				for _, fs := range fulfillmentScopes {
					if handle == fs {
						hasFulfillmentScopes = true
					}
				}
			}
		}
		
		fmt.Println("\n---")
		if hasFulfillmentScopes {
			fmt.Println("✓ Fulfillment order scopes found")
		} else {
			fmt.Println("⚠ Missing fulfillment order scopes!")
			fmt.Println("Required scopes:")
			for _, fs := range fulfillmentScopes {
				fmt.Printf("  - %s\n", fs)
			}
		}
	}
}





