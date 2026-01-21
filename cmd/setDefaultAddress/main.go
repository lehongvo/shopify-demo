package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"shopify-demo/app"
)

func main() {
	// Accept address ID only
	// Example: go run cmd/setDefaultAddress/main.go 10146295447792
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run cmd/setDefaultAddress/main.go <address_id>")
	}

	addressIDInput := os.Args[1]
	fmt.Printf("Setting default address: %s\n\n", addressIDInput)

	// Convert address ID to GID format if needed
	addressGID := normalizeAddressID(addressIDInput)
	fmt.Printf("Address GID: %s\n", addressGID)

	// Find customer from address
	customerID, err := findCustomerFromAddress(addressGID)
	if err != nil {
		log.Fatalf("Error finding customer from address: %v", err)
	}
	if customerID == "" {
		log.Fatal("Customer not found for this address")
	}

	fmt.Printf("Customer ID: %s\n\n", customerID)

	// Set default address
	err = setDefaultAddress(customerID, addressGID)
	if err != nil {
		log.Fatalf("Error setting default address: %v", err)
	}

	fmt.Println("✓ Default address set successfully!")
}

// normalizeAddressID converts address ID to GID format
// Input can be: "10146295447792" or "gid://shopify/MailingAddress/10146295447792?model_name=CustomerAddress"
func normalizeAddressID(addressID string) string {
	// If already a GID, return as is
	if strings.HasPrefix(addressID, "gid://") {
		return addressID
	}

	// Extract numeric ID if GID format with query params
	if strings.Contains(addressID, "?") {
		parts := strings.Split(addressID, "?")
		addressID = parts[0]
	}
	if strings.Contains(addressID, "/") {
		parts := strings.Split(addressID, "/")
		addressID = parts[len(parts)-1]
	}

	// Convert to GID format
	return fmt.Sprintf("gid://shopify/MailingAddress/%s?model_name=CustomerAddress", addressID)
}

// findCustomer searches for a customer by query (name/email) or returns the ID if it's already a GID
func findCustomer(query string) (string, error) {
	// If query is already a GID, return it
	if strings.HasPrefix(query, "gid://") {
		return query, nil
	}

	const queryStr = `
		query SearchCustomer($query: String!) {
			customers(first: 1, query: $query) {
				edges {
					node {
						id
						firstName
						lastName
						email
					}
				}
			}
		}`

	variables := map[string]interface{}{
		"query": query,
	}

	resp, err := app.CallAdminGraphQL(queryStr, variables)
	if err != nil {
		return "", fmt.Errorf("failed to call API: %w", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("no data in response")
	}

	customers, ok := data["customers"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("customers not found in response")
	}

	edges, ok := customers["edges"].([]interface{})
	if !ok || len(edges) == 0 {
		return "", nil
	}

	edge, ok := edges[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid edge structure")
	}

	node, ok := edge["node"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid node structure")
	}

	customerID, ok := node["id"].(string)
	if !ok {
		return "", fmt.Errorf("customer ID not found")
	}

	// Print customer info
	if firstName, ok := node["firstName"].(string); ok {
		fmt.Printf("Customer: %s", firstName)
		if lastName, ok := node["lastName"].(string); ok {
			fmt.Printf(" %s", lastName)
		}
		fmt.Println()
	}
	if email, ok := node["email"].(string); ok && email != "" {
		fmt.Printf("Email: %s\n", email)
	}

	return customerID, nil
}

// findCustomerFromAddress searches for customer by checking their addresses
// This is less efficient - better to provide customer query/ID directly
func findCustomerFromAddress(addressGID string) (string, error) {
	// Extract numeric address ID for comparison
	addressIDNum := extractNumericID(addressGID)
	if addressIDNum == "" {
		return "", fmt.Errorf("invalid address ID format")
	}

	// Search customers and check their addresses
	// We'll search in batches
	const query = `
		query SearchCustomersWithAddresses($query: String!) {
			customers(first: 50, query: $query) {
				edges {
					node {
						id
						firstName
						lastName
						email
						addresses {
							id
						}
					}
				}
				pageInfo {
					hasNextPage
					endCursor
				}
			}
		}`

	// Search all customers (empty query returns all)
	variables := map[string]interface{}{
		"query": "",
	}

	resp, err := app.CallAdminGraphQL(query, variables)
	if err != nil {
		return "", fmt.Errorf("failed to call API: %w", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("no data in response")
	}

	customers, ok := data["customers"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("customers not found in response")
	}

	edges, ok := customers["edges"].([]interface{})
	if !ok {
		return "", fmt.Errorf("no customers found")
	}

	// Check each customer's addresses
	for _, edge := range edges {
		edgeMap, ok := edge.(map[string]interface{})
		if !ok {
			continue
		}

		node, ok := edgeMap["node"].(map[string]interface{})
		if !ok {
			continue
		}

		customerID, ok := node["id"].(string)
		if !ok {
			continue
		}

		addresses, ok := node["addresses"].([]interface{})
		if !ok {
			continue
		}

		// Check if any address matches
		for _, addr := range addresses {
			addrMap, ok := addr.(map[string]interface{})
			if !ok {
				continue
			}

			addrID, ok := addrMap["id"].(string)
			if !ok {
				continue
			}

			// Compare numeric IDs
			if extractNumericID(addrID) == addressIDNum {
				// Found the customer!
				if firstName, ok := node["firstName"].(string); ok {
					fmt.Printf("Customer: %s", firstName)
					if lastName, ok := node["lastName"].(string); ok {
						fmt.Printf(" %s", lastName)
					}
					fmt.Println()
				}
				if email, ok := node["email"].(string); ok && email != "" {
					fmt.Printf("Email: %s\n", email)
				}
				return customerID, nil
			}
		}
	}

	return "", fmt.Errorf("customer not found for this address")
}

// extractNumericID extracts numeric ID from GID format
func extractNumericID(gid string) string {
	// Remove query params if any
	if strings.Contains(gid, "?") {
		parts := strings.Split(gid, "?")
		gid = parts[0]
	}

	// Extract last numeric part
	parts := strings.Split(gid, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return ""
}

// setDefaultAddress sets the default address for a customer
func setDefaultAddress(customerID, addressGID string) error {
	const mutation = `
		mutation CustomerUpdateDefaultAddress($customerId: ID!, $addressId: ID!) {
			customerUpdateDefaultAddress(customerId: $customerId, addressId: $addressId) {
				customer {
					id
					defaultAddress {
						id
						address1
						address2
						city
						province
						country
						zip
						phone
					}
				}
				userErrors {
					field
					message
				}
			}
		}`

	variables := map[string]interface{}{
		"customerId": customerID,
		"addressId":  addressGID,
	}

	resp, err := app.CallAdminGraphQL(mutation, variables)
	if err != nil {
		return fmt.Errorf("failed to call API: %w", err)
	}

	// Check for GraphQL errors
	if errors, ok := resp["errors"].([]interface{}); ok && len(errors) > 0 {
		errorMsgs := []string{}
		for _, err := range errors {
			if errMap, ok := err.(map[string]interface{}); ok {
				if msg, ok := errMap["message"].(string); ok {
					errorMsgs = append(errorMsgs, msg)
				}
			}
		}
		return fmt.Errorf("GraphQL errors: %v", errorMsgs)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("no data in response")
	}

	customerUpdate, ok := data["customerUpdateDefaultAddress"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("customerUpdateDefaultAddress not found in response")
	}

	// Check for user errors
	if userErrors, ok := customerUpdate["userErrors"].([]interface{}); ok && len(userErrors) > 0 {
		errorMsgs := []string{}
		for _, err := range userErrors {
			if errMap, ok := err.(map[string]interface{}); ok {
				if msg, ok := errMap["message"].(string); ok {
					errorMsgs = append(errorMsgs, msg)
				}
			}
		}
		return fmt.Errorf("user errors: %v", errorMsgs)
	}

	customer, ok := customerUpdate["customer"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("customer not found in response")
	}

	defaultAddr, ok := customer["defaultAddress"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("defaultAddress not found in response")
	}

	// Print updated default address info
	fmt.Println("\n=== Updated Default Address ===")
	if id, ok := defaultAddr["id"].(string); ok {
		fmt.Printf("Address ID: %s\n", id)
	}
	if addr1, ok := defaultAddr["address1"].(string); ok && addr1 != "" {
		fmt.Printf("Address: %s\n", addr1)
	}
	if addr2, ok := defaultAddr["address2"].(string); ok && addr2 != "" {
		fmt.Printf("         %s\n", addr2)
	}
	if city, ok := defaultAddr["city"].(string); ok && city != "" {
		fmt.Printf("City: %s", city)
		if province, ok := defaultAddr["province"].(string); ok && province != "" {
			fmt.Printf(", %s", province)
		}
		if zip, ok := defaultAddr["zip"].(string); ok && zip != "" {
			fmt.Printf(" %s", zip)
		}
		fmt.Println()
	}
	if country, ok := defaultAddr["country"].(string); ok && country != "" {
		fmt.Printf("Country: %s\n", country)
	}
	if phone, ok := defaultAddr["phone"].(string); ok && phone != "" {
		fmt.Printf("Phone: %s\n", phone)
	}

	// Print full JSON response
	fmt.Println("\n=== Full JSON Response ===")
	jsonData, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println(string(jsonData))

	return nil
}
