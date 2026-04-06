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

	fmt.Println("=== Testing Scope Approval Status ===\n")

	// Test 1: Check current scopes
	fmt.Println("Step 1: Checking current approved scopes...")
	currentScopes := getCurrentScopes()
	fmt.Printf("✓ Found %d approved scopes\n\n", len(currentScopes))

	// Test 2: Try to create an order (requires write_orders)
	fmt.Println("Step 2: Testing write_orders scope...")
	testWriteOrders(currentScopes)

	// Test 3: Try to modify inventory (requires write_inventory)
	fmt.Println("\nStep 3: Testing write_inventory scope...")
	testWriteInventory(currentScopes)

	// Summary
	fmt.Println("\n=== How to Reproduce 'Requires Merchant Approval' Error ===")
	fmt.Println("\n1. Go to Shopify Partner Dashboard: https://partners.shopify.com")
	fmt.Println("2. Select your app")
	fmt.Println("3. Go to Configuration > App setup")
	fmt.Println("4. Add a NEW scope (e.g., write_shipping, write_price_rules)")
	fmt.Println("5. SAVE but DO NOT reinstall app on test store")
	fmt.Println("6. Try to use API with new scope → Will get 'requires merchant approval' error")
	
	fmt.Println("\n=== How to Fix ===")
	fmt.Println("\n✓ Option 1: Go to https://" + shopDomain + "/admin/settings/apps")
	fmt.Println("  - Find your app")
	fmt.Println("  - Click 'Review' or 'Update permissions'")
	fmt.Println("  - Approve the new scopes")
	
	fmt.Println("\n✓ Option 2: Reinstall the app")
	fmt.Println("  - Uninstall app")
	fmt.Println("  - Reinstall app")
	fmt.Println("  - Approve all permissions during installation")
}

func getCurrentScopes() []string {
	const query = `
		query {
			currentAppInstallation {
				accessScopes {
					handle
				}
			}
		}`

	resp, err := app.CallAdminGraphQL(query, map[string]interface{}{})
	if err != nil {
		log.Printf("Error getting scopes: %v", err)
		return []string{}
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return []string{}
	}

	appInstallation, ok := data["currentAppInstallation"].(map[string]interface{})
	if !ok {
		return []string{}
	}

	var scopes []string
	if scopeList, ok := appInstallation["accessScopes"].([]interface{}); ok {
		for _, scope := range scopeList {
			if scopeMap, ok := scope.(map[string]interface{}); ok {
				if handle, ok := scopeMap["handle"].(string); ok {
					scopes = append(scopes, handle)
				}
			}
		}
	}

	return scopes
}

func testWriteOrders(currentScopes []string) {
	hasScope := contains(currentScopes, "write_orders")
	
	if hasScope {
		fmt.Println("✓ write_orders scope is APPROVED")
		fmt.Println("  → Can create/modify orders")
	} else {
		fmt.Println("✗ write_orders scope is NOT approved")
		fmt.Println("  → Will get 'requires merchant approval' error when creating orders")
	}
}

func testWriteInventory(currentScopes []string) {
	hasScope := contains(currentScopes, "write_inventory")
	
	if hasScope {
		fmt.Println("✓ write_inventory scope is APPROVED")
		fmt.Println("  → Can modify inventory")
	} else {
		fmt.Println("✗ write_inventory scope is NOT approved")
		fmt.Println("  → Will get 'requires merchant approval' error when modifying inventory")
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Additional: Show detailed scope status
func printDetailedStatus() {
	result := map[string]interface{}{
		"note": "To reproduce the error, add a new scope in Partner Dashboard but don't approve it on the store",
		"steps": []string{
			"1. Add new scope in Partner Dashboard",
			"2. Don't reinstall app",
			"3. Try to use API with new scope",
			"4. Error: 'requires merchant approval'",
		},
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println("\n" + string(jsonData))
}
