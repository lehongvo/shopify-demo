package main

import (
	"fmt"
	"log"
	"strings"

	"shopify-demo/app"
)

func main() {
	fmt.Println("=== Getting All Shopify API Permissions (Access Scopes) ===")
	fmt.Println()

	// Method 1: Get current app installation scopes
	fmt.Println("--- Method 1: Current App Installation Scopes ---")
	getCurrentAppScopes()

	// Method 2: Get all available scopes from documentation/API
	fmt.Println("\n--- Method 2: All Available Scopes (from API) ---")
	getAllAvailableScopes()
}

func getCurrentAppScopes() {
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
		log.Printf("Error: %v\n", err)
		return
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		fmt.Println("No data in response")
		return
	}

	appInstallation, ok := data["currentAppInstallation"].(map[string]interface{})
	if !ok {
		fmt.Println("currentAppInstallation not found")
		return
	}

	if scopes, ok := appInstallation["accessScopes"].([]interface{}); ok {
		fmt.Printf("Total Scopes: %d\n\n", len(scopes))
		
		// Group by category
		scopesByCategory := make(map[string][]map[string]interface{})
		
		for _, scope := range scopes {
			if scopeMap, ok := scope.(map[string]interface{}); ok {
				handle := scopeMap["handle"].(string)
				category := getCategoryFromHandle(handle)
				scopesByCategory[category] = append(scopesByCategory[category], scopeMap)
			}
		}
		
		// Print by category
		categories := []string{"Orders", "Products", "Customers", "Inventory", "Fulfillment", "Draft Orders", "Other"}
		for _, category := range categories {
			if scopes, ok := scopesByCategory[category]; ok {
				fmt.Printf("📦 %s:\n", category)
				for _, scope := range scopes {
					fmt.Printf("   - %s\n", scope["handle"])
					fmt.Printf("     %s\n\n", scope["description"])
				}
			}
		}
		
		// Print remaining categories
		for category, scopes := range scopesByCategory {
			found := false
			for _, c := range categories {
				if c == category {
					found = true
					break
				}
			}
			if !found {
				fmt.Printf("📦 %s:\n", category)
				for _, scope := range scopes {
					fmt.Printf("   - %s\n", scope["handle"])
					fmt.Printf("     %s\n\n", scope["description"])
				}
			}
		}
	}
}

func getAllAvailableScopes() {
	// List of all known Shopify API scopes
	allScopes := map[string][]ScopeInfo{
		"Orders": {
			{Handle: "read_orders", Description: "Read orders, transactions, and fulfillments"},
			{Handle: "write_orders", Description: "Modify orders, transactions, and fulfillments"},
			{Handle: "read_all_orders", Description: "Read all orders"},
		},
		"Fulfillment Orders": {
			{Handle: "read_merchant_managed_fulfillment_orders", Description: "Read fulfillment orders assigned to merchant-managed locations"},
			{Handle: "write_merchant_managed_fulfillment_orders", Description: "Modify fulfillment orders assigned to merchant-managed locations"},
			{Handle: "read_assigned_fulfillment_orders", Description: "Read fulfillment orders assigned to fulfillment service locations"},
			{Handle: "write_assigned_fulfillment_orders", Description: "Modify fulfillment orders assigned to fulfillment service locations"},
			{Handle: "read_third_party_fulfillment_orders", Description: "Read fulfillment orders assigned to third-party fulfillment service locations"},
			{Handle: "write_third_party_fulfillment_orders", Description: "Modify fulfillment orders assigned to third-party fulfillment service locations"},
			{Handle: "read_marketplace_fulfillment_orders", Description: "Read marketplace fulfillment orders"},
			{Handle: "write_marketplace_fulfillment_orders", Description: "Modify marketplace fulfillment orders"},
		},
		"Products": {
			{Handle: "read_products", Description: "Read products, variants, and collections"},
			{Handle: "write_products", Description: "Modify products, variants, and collections"},
		},
		"Customers": {
			{Handle: "read_customers", Description: "Read customer details and customer groups"},
			{Handle: "write_customers", Description: "Modify customer details and customer groups"},
		},
		"Draft Orders": {
			{Handle: "read_draft_orders", Description: "Read draft orders"},
			{Handle: "write_draft_orders", Description: "Modify draft orders (required for draftOrderCreate and draftOrderComplete)"},
		},
		"Inventory": {
			{Handle: "read_inventory", Description: "Read inventory"},
			{Handle: "write_inventory", Description: "Modify inventory"},
		},
		"Locations": {
			{Handle: "read_locations", Description: "Read locations"},
		},
		"Fulfillments": {
			{Handle: "read_fulfillments", Description: "Read fulfillment services"},
			{Handle: "write_fulfillments", Description: "Modify fulfillment services"},
		},
		"Analytics": {
			{Handle: "read_analytics", Description: "View store metrics"},
		},
		"Apps": {
			{Handle: "read_apps", Description: "View or manage apps"},
		},
		"Gift Cards": {
			{Handle: "read_gift_cards", Description: "Read gift cards"},
			{Handle: "write_gift_cards", Description: "Modify gift cards"},
		},
		"Price Rules": {
			{Handle: "read_price_rules", Description: "Read price rules"},
			{Handle: "write_price_rules", Description: "Modify price rules"},
		},
		"Markets": {
			{Handle: "read_markets", Description: "Read access for Shopify Markets API"},
		},
		"Checkouts": {
			{Handle: "unauthenticated_read_checkouts", Description: "Read checkouts"},
			{Handle: "unauthenticated_write_checkouts", Description: "Modify checkouts"},
		},
	}

	fmt.Println("All Available Shopify API Scopes:")
	fmt.Println()
	
	// Get current scopes to mark which ones are enabled
	currentScopes := getCurrentScopesList()
	
	for category, scopes := range allScopes {
		fmt.Printf("📦 %s:\n", category)
		for _, scope := range scopes {
			enabled := ""
			if contains(currentScopes, scope.Handle) {
				enabled = " ✅ (ENABLED)"
			}
			fmt.Printf("   - %s%s\n", scope.Handle, enabled)
			fmt.Printf("     %s\n\n", scope.Description)
		}
	}
	
	fmt.Println("\n--- Summary ---")
	fmt.Printf("Total available scopes: %d\n", countTotalScopes(allScopes))
	fmt.Printf("Currently enabled scopes: %d\n", len(currentScopes))
}

type ScopeInfo struct {
	Handle      string
	Description string
}

func getCategoryFromHandle(handle string) string {
	if strings.Contains(handle, "order") {
		if strings.Contains(handle, "draft") {
			return "Draft Orders"
		}
		return "Orders"
	}
	if strings.Contains(handle, "fulfillment") {
		return "Fulfillment"
	}
	if strings.Contains(handle, "product") {
		return "Products"
	}
	if strings.Contains(handle, "customer") {
		return "Customers"
	}
	if strings.Contains(handle, "inventory") {
		return "Inventory"
	}
	if strings.Contains(handle, "location") {
		return "Locations"
	}
	if strings.Contains(handle, "analytics") {
		return "Analytics"
	}
	if strings.Contains(handle, "app") {
		return "Apps"
	}
	if strings.Contains(handle, "gift") {
		return "Gift Cards"
	}
	if strings.Contains(handle, "price") {
		return "Price Rules"
	}
	if strings.Contains(handle, "market") {
		return "Markets"
	}
	if strings.Contains(handle, "checkout") {
		return "Checkouts"
	}
	return "Other"
}

func getCurrentScopesList() []string {
	const query = `
		query {
			currentAppInstallation {
				accessScopes {
					handle
				}
			}
		}`

	variables := map[string]interface{}{}
	resp, err := app.CallAdminGraphQL(query, variables)
	if err != nil {
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

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func countTotalScopes(allScopes map[string][]ScopeInfo) int {
	count := 0
	for _, scopes := range allScopes {
		count += len(scopes)
	}
	return count
}

