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

	// GraphQL query để lấy tất cả access scopes/permissions
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
		log.Fatalf("Error calling Shopify API: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		log.Fatal("No data in response")
	}

	appInstallation, ok := data["currentAppInstallation"].(map[string]interface{})
	if !ok {
		log.Fatal("currentAppInstallation not found")
	}

	accessScopes, ok := appInstallation["accessScopes"].([]interface{})
	if !ok {
		log.Fatal("accessScopes not found")
	}

	// Chuyển đổi sang format đơn giản
	var permissions []map[string]string
	for _, scope := range accessScopes {
		if scopeMap, ok := scope.(map[string]interface{}); ok {
			permission := map[string]string{
				"handle":      fmt.Sprintf("%v", scopeMap["handle"]),
				"description": fmt.Sprintf("%v", scopeMap["description"]),
			}
			permissions = append(permissions, permission)
		}
	}

	// Log một lần duy nhất
	result := map[string]interface{}{
		"total_permissions": len(permissions),
		"permissions":       permissions,
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(jsonData))
}
