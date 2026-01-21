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
	// Accept customer query (name/email) or customer ID as argument
	// Example: go run cmd/unsetDefaultAddress/unsetDefeultAddress.go "Vo Le"
	// Or: go run cmd/unsetDefaultAddress/unsetDefeultAddress.go "gid://shopify/Customer/123456"
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run cmd/unsetDefaultAddress/unsetDefeultAddress.go <customer_query_or_id>")
	}

	customerQuery := os.Args[1]
	fmt.Printf("Getting customer addresses for: %s\n\n", customerQuery)

	// First, try to find the customer
	customerID, err := findCustomer(customerQuery)
	if err != nil {
		log.Fatalf("Error finding customer: %v", err)
	}

	if customerID == "" {
		log.Fatal("Customer not found")
	}

	fmt.Printf("Found customer ID: %s\n\n", customerID)

	// Get customer addresses and default address ID
	addresses, defaultAddressID, err := getCustomerAddresses(customerID)
	if err != nil {
		log.Fatalf("Error getting customer addresses: %v", err)
	}

	// Display addresses
	fmt.Println("=== Customer Addresses ===")
	if len(addresses) == 0 {
		fmt.Println("No addresses found for this customer")
		return
	}

	for i, addr := range addresses {
		fmt.Printf("\n[Address %d]\n", i+1)
		// Check if this is the default address
		if addrID, ok := addr["id"].(string); ok && addrID == defaultAddressID {
			fmt.Println("⭐ Default Address")
		}
		if firstName, ok := addr["firstName"].(string); ok && firstName != "" {
			fmt.Printf("Name: %s", firstName)
			if lastName, ok := addr["lastName"].(string); ok && lastName != "" {
				fmt.Printf(" %s\n", lastName)
			} else {
				fmt.Println()
			}
		}
		if addr1, ok := addr["address1"].(string); ok && addr1 != "" {
			fmt.Printf("Address: %s\n", addr1)
		}
		if addr2, ok := addr["address2"].(string); ok && addr2 != "" {
			fmt.Printf("         %s\n", addr2)
		}
		if city, ok := addr["city"].(string); ok && city != "" {
			fmt.Printf("City: %s", city)
			if province, ok := addr["province"].(string); ok && province != "" {
				fmt.Printf(", %s", province)
			}
			if zip, ok := addr["zip"].(string); ok && zip != "" {
				fmt.Printf(" %s", zip)
			}
			fmt.Println()
		}
		if country, ok := addr["country"].(string); ok && country != "" {
			fmt.Printf("Country: %s\n", country)
		}
		if phone, ok := addr["phone"].(string); ok && phone != "" {
			fmt.Printf("Phone: %s\n", phone)
		}
		if id, ok := addr["id"].(string); ok {
			fmt.Printf("Address ID: %s\n", id)
		}
	}

	// Print full JSON response
	fmt.Println("\n=== Full JSON Response ===")
	jsonData, _ := json.MarshalIndent(addresses, "", "  ")
	fmt.Println(string(jsonData))
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

// getCustomerAddresses gets all addresses for a customer and returns the default address ID
func getCustomerAddresses(customerID string) ([]map[string]interface{}, string, error) {
	const query = `
		query GetCustomerAddresses($id: ID!) {
			customer(id: $id) {
				id
				firstName
				lastName
				email
				defaultAddress {
					id
				}
				addresses {
					id
					firstName
					lastName
					address1
					address2
					city
					province
					country
					zip
					phone
				}
			}
		}`

	variables := map[string]interface{}{
		"id": customerID,
	}

	resp, err := app.CallAdminGraphQL(query, variables)
	if err != nil {
		return nil, "", fmt.Errorf("failed to call API: %w", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return nil, "", fmt.Errorf("no data in response")
	}

	customer, ok := data["customer"].(map[string]interface{})
	if !ok {
		return nil, "", fmt.Errorf("customer not found in response")
	}

	// Get default address ID
	var defaultAddressID string
	if defaultAddr, ok := customer["defaultAddress"].(map[string]interface{}); ok {
		if id, ok := defaultAddr["id"].(string); ok {
			defaultAddressID = id
		}
	}

	addressesData, ok := customer["addresses"].([]interface{})
	if !ok {
		return []map[string]interface{}{}, defaultAddressID, nil
	}

	var addresses []map[string]interface{}
	for _, addr := range addressesData {
		addrMap, ok := addr.(map[string]interface{})
		if !ok {
			continue
		}

		addresses = append(addresses, addrMap)
	}

	return addresses, defaultAddressID, nil
}
