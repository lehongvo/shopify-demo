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
			shop {
				id
				name
				currencyCode
				enabledPresentmentCurrencies
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

	shop, ok := data["shop"].(map[string]interface{})
	if !ok {
		log.Fatal("Shop not found")
	}

	result := map[string]interface{}{
		"shop_name":                    shop["name"],
		"default_currency":             shop["currencyCode"],
		"enabled_currencies":           shop["enabledPresentmentCurrencies"],
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(jsonData))

	fmt.Println("\n=== Hướng dẫn ===")
	fmt.Printf("✓ Currency mặc định của store: %s\n", shop["currencyCode"])
	fmt.Println("✓ Sử dụng currency này khi tạo order")
	fmt.Println("\nHoặc enable thêm currencies tại:")
	fmt.Printf("https://%s/admin/settings/payments\n", shopDomain)
}
