package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"shopify-demo/app"
)

func main() {
	// Accept address ID only
	// Example: go run cmd/unsetDefaultAddressById/main.go 10146295447792
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run cmd/unsetDefaultAddressById/main.go <address_id>")
	}

	addressIDInput := os.Args[1]
	fmt.Printf("Unsetting default address: %s\n\n", addressIDInput)

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

	// Check if address is default
	isDefault, err := checkIfDefaultAddress(customerID, addressGID)
	if err != nil {
		log.Fatalf("Error checking default address: %v", err)
	}

	if !isDefault {
		fmt.Println("❌ Address is not the default address. Cannot unset.")
		return
	}

	// Get all addresses to find another address
	addresses, err := getCustomerAddresses(customerID)
	if err != nil {
		log.Fatalf("Error getting customer addresses: %v", err)
	}

	// Check if this is the only address
	addressCount := len(addresses)

	if addressCount == 1 {
		fmt.Println("❌ Cannot unset default address: This is the only address. Customer must have at least one default address.")
		return
	}

	// Shopify doesn't support unset default without setting another address as default
	// To "unset" this address's default status, we need to set another address as default
	// Find another address to set as default (silently, without logging)
	addressIDNum := extractNumericID(addressGID)
	var otherAddressID string
	for _, addr := range addresses {
		addrID, ok := addr["id"].(string)
		if !ok {
			continue
		}
		if extractNumericID(addrID) != addressIDNum {
			otherAddressID = addrID
			break
		}
	}

	if otherAddressID == "" {
		fmt.Println("❌ Cannot unset default address: No other address found.")
		return
	}

	// Set another address as default (this automatically unsets the current default)
	// We do this silently without logging to achieve "unset" behavior
	err = setDefaultAddress(customerID, otherAddressID)
	if err != nil {
		log.Fatalf("Error unsetting default address: %v", err)
	}

	fmt.Println("✓ Default address unset successfully!")
}

// normalizeAddressID converts address ID to GID format
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

// findCustomerFromAddress searches for customer by checking their addresses
func findCustomerFromAddress(addressGID string) (string, error) {
	// Extract numeric address ID for comparison
	addressIDNum := extractNumericID(addressGID)
	if addressIDNum == "" {
		return "", fmt.Errorf("invalid address ID format")
	}

	// Search customers and check their addresses
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

// checkIfDefaultAddress checks if the given address is the default address
func checkIfDefaultAddress(customerID, addressGID string) (bool, error) {
	const query = `
		query GetCustomerDefaultAddress($id: ID!) {
			customer(id: $id) {
				id
				defaultAddress {
					id
				}
			}
		}`

	variables := map[string]interface{}{
		"id": customerID,
	}

	resp, err := app.CallAdminGraphQL(query, variables)
	if err != nil {
		return false, fmt.Errorf("failed to call API: %w", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("no data in response")
	}

	customer, ok := data["customer"].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("customer not found in response")
	}

	// Check if there's a default address
	defaultAddr, ok := customer["defaultAddress"].(map[string]interface{})
	if !ok {
		// No default address
		return false, nil
	}

	defaultAddrID, ok := defaultAddr["id"].(string)
	if !ok {
		return false, nil
	}

	// Compare numeric IDs for accurate comparison
	addressIDNum := extractNumericID(addressGID)
	defaultIDNum := extractNumericID(defaultAddrID)
	
	return addressIDNum == defaultIDNum, nil
}

// getCustomerAddresses gets all addresses for a customer
func getCustomerAddresses(customerID string) ([]map[string]interface{}, error) {
	const query = `
		query GetCustomerAddresses($id: ID!) {
			customer(id: $id) {
				id
				addresses {
					id
				}
			}
		}`

	variables := map[string]interface{}{
		"id": customerID,
	}

	resp, err := app.CallAdminGraphQL(query, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to call API: %w", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no data in response")
	}

	customer, ok := data["customer"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("customer not found in response")
	}

	addressesData, ok := customer["addresses"].([]interface{})
	if !ok {
		return []map[string]interface{}{}, nil
	}

	var addresses []map[string]interface{}
	for _, addr := range addressesData {
		addrMap, ok := addr.(map[string]interface{})
		if !ok {
			continue
		}

		addresses = append(addresses, addrMap)
	}

	return addresses, nil
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

	return nil
}

// deleteAddress deletes the address
func deleteAddress(customerID, addressGID string) error {
	const mutation = `
		mutation CustomerAddressDelete($customerId: ID!, $addressId: ID!) {
			customerAddressDelete(customerId: $customerId, addressId: $addressId) {
				deletedAddressId
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

	customerDelete, ok := data["customerAddressDelete"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("customerAddressDelete not found in response")
	}

	// Check for user errors
	if userErrors, ok := customerDelete["userErrors"].([]interface{}); ok && len(userErrors) > 0 {
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

	return nil
}
