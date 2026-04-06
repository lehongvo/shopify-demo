package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"shopify-demo/app"
)

func main() {
	shopDomain := os.Getenv("SHOPIFY_SHOP_DOMAIN")
	accessToken := os.Getenv("SHOPIFY_API_SECRET")

	if shopDomain == "" || accessToken == "" {
		log.Fatal("SHOPIFY_SHOP_DOMAIN and SHOPIFY_API_SECRET must be set")
	}

	const query = `
		query {
			currentAppInstallation {
				id
				launchUrl
				activeSubscriptions {
					id
					status
				}
				accessScopes {
					handle
					description
				}
			}
		}`

	resp, err := app.CallAdminGraphQL(query, map[string]interface{}{})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		log.Fatal("No data in response")
	}

	appInstallation, ok := data["currentAppInstallation"].(map[string]interface{})
	if !ok {
		log.Fatal("currentAppInstallation not found - App might not be installed properly")
	}

	// Kiểm tra write_orders scope
	hasWriteOrders := false
	var allScopes []string
	
	if scopes, ok := appInstallation["accessScopes"].([]interface{}); ok {
		for _, scope := range scopes {
			if scopeMap, ok := scope.(map[string]interface{}); ok {
				handle := scopeMap["handle"].(string)
				allScopes = append(allScopes, handle)
				if handle == "write_orders" {
					hasWriteOrders = true
				}
			}
		}
	}

	result := map[string]interface{}{
		"shop_domain":         shopDomain,
		"app_id":              appInstallation["id"],
		"has_write_orders":    hasWriteOrders,
		"total_scopes":        len(allScopes),
		"all_scopes":          allScopes,
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(jsonData))

	fmt.Println("\n=== Phân tích ===")
	if hasWriteOrders {
		fmt.Println("✅ App CÓ scope write_orders")
		fmt.Println("❌ NHƯNG merchant chưa approve scope này")
		fmt.Println("\n🔧 Cách fix:")
		fmt.Println("1. Vào: https://" + shopDomain + "/admin/settings/apps")
		fmt.Println("2. Tìm app của bạn")
		fmt.Println("3. Click 'Review' hoặc 'Approve' để cấp quyền mới")
		fmt.Println("\nHoặc:")
		fmt.Println("- Uninstall app")
		fmt.Println("- Reinstall lại")
		fmt.Println("- Approve tất cả permissions khi cài")
	} else {
		fmt.Println("❌ App KHÔNG có scope write_orders")
		fmt.Println("\n🔧 Cần thêm scope write_orders vào app configuration")
	}
}
