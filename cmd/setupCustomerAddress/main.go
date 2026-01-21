package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"shopify-demo/app"
)

type AddressInput struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Address1  string `json:"address1,omitempty"`
	Address2  string `json:"address2,omitempty"`
	City      string `json:"city"`
	Province  string `json:"province,omitempty"`
	Country   string `json:"country"`
	Zip       string `json:"zip"`
	Phone     string `json:"phone,omitempty"`
}

func main() {
	// Accept customer query/ID as argument
	// Example: go run cmd/setupCustomerAddress/main.go "Vo Le"
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run cmd/setupCustomerAddress/main.go <customer_query_or_id>")
	}

	customerQuery := os.Args[1]
	fmt.Printf("Setting up addresses for customer: %s\n\n", customerQuery)

	// Find customer
	customerID, err := findCustomer(customerQuery)
	if err != nil {
		log.Fatalf("Error finding customer: %v", err)
	}
	if customerID == "" {
		log.Fatal("Customer not found")
	}

	fmt.Printf("Customer ID: %s\n\n", customerID)

	// Define addresses to create (based on previous data)
	addresses := []AddressInput{
		{
			FirstName: "Vo",
			LastName:  "Le",
			Address1:  "HaNoi",
			Address2:  "Vinh",
			City:      "Seattle",
			Province:  "",
			Country:   "Vietnam",
			Zip:       "100000",
			Phone:     "+84364999641",
		},
		{
			FirstName: "Vo",
			LastName:  "Le",
			Address1:  "",
			Address2:  "",
			City:      "Seattle",
			Province:  "Washington",
			Country:   "United States",
			Zip:       "100000",
			Phone:     "+84364999643",
		},
	}

	// Create addresses
	var createdAddressIDs []string
	for i, addr := range addresses {
		fmt.Printf("Creating address %d/%d...\n", i+1, len(addresses))
		addressID, err := createAddress(customerID, addr)
		if err != nil {
			log.Printf("Error creating address %d: %v", i+1, err)
			continue
		}
		createdAddressIDs = append(createdAddressIDs, addressID)
		fmt.Printf("✓ Created address ID: %s\n\n", addressID)
	}

	if len(createdAddressIDs) == 0 {
		log.Fatal("No addresses were created")
	}

	// Set first address as default
	fmt.Println("Setting first address as default...")
	err = setDefaultAddress(customerID, createdAddressIDs[0])
	if err != nil {
		log.Printf("Warning: Could not set default address: %v", err)
	} else {
		fmt.Println("✓ Default address set successfully!")
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Created %d addresses\n", len(createdAddressIDs))
	fmt.Printf("Default address ID: %s\n", createdAddressIDs[0])
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

// createAddress creates a new address for a customer
func createAddress(customerID string, addr AddressInput) (string, error) {
	const mutation = `
		mutation CustomerAddressCreate($customerId: ID!, $address: MailingAddressInput!) {
			customerAddressCreate(customerId: $customerId, address: $address) {
				address {
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
				userErrors {
					field
					message
				}
			}
		}`

	// Build address input, only include non-empty fields
	addressInput := map[string]interface{}{
		"firstName": addr.FirstName,
		"lastName":  addr.LastName,
		"city":      addr.City,
		"country":   addr.Country,
		"zip":       addr.Zip,
	}

	if addr.Address1 != "" {
		addressInput["address1"] = addr.Address1
	}
	if addr.Address2 != "" {
		addressInput["address2"] = addr.Address2
	}
	if addr.Province != "" {
		addressInput["province"] = addr.Province
	}
	if addr.Phone != "" {
		addressInput["phone"] = addr.Phone
	}

	variables := map[string]interface{}{
		"customerId": customerID,
		"address":    addressInput,
	}

	resp, err := app.CallAdminGraphQL(mutation, variables)
	if err != nil {
		return "", fmt.Errorf("failed to call API: %w", err)
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
		return "", fmt.Errorf("GraphQL errors: %v", errorMsgs)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("no data in response")
	}

	customerCreate, ok := data["customerAddressCreate"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("customerAddressCreate not found in response")
	}

	// Check for user errors
	if userErrors, ok := customerCreate["userErrors"].([]interface{}); ok && len(userErrors) > 0 {
		errorMsgs := []string{}
		for _, err := range userErrors {
			if errMap, ok := err.(map[string]interface{}); ok {
				if msg, ok := errMap["message"].(string); ok {
					errorMsgs = append(errorMsgs, msg)
				}
			}
		}
		return "", fmt.Errorf("user errors: %v", errorMsgs)
	}

	customerAddr, ok := customerCreate["address"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("address not found in response")
	}

	addressID, ok := customerAddr["id"].(string)
	if !ok {
		return "", fmt.Errorf("address ID not found")
	}

	// Print created address info
	fmt.Printf("  Address: ")
	if addr1, ok := customerAddr["address1"].(string); ok && addr1 != "" {
		fmt.Printf("%s", addr1)
		if addr2, ok := customerAddr["address2"].(string); ok && addr2 != "" {
			fmt.Printf(", %s", addr2)
		}
		fmt.Printf(", ")
	}
	if city, ok := customerAddr["city"].(string); ok {
		fmt.Printf("%s", city)
	}
	if province, ok := customerAddr["province"].(string); ok && province != "" {
		fmt.Printf(", %s", province)
	}
	if zip, ok := customerAddr["zip"].(string); ok && zip != "" {
		fmt.Printf(" %s", zip)
	}
	fmt.Println()
	if country, ok := customerAddr["country"].(string); ok {
		fmt.Printf("  Country: %s\n", country)
	}
	if phone, ok := customerAddr["phone"].(string); ok && phone != "" {
		fmt.Printf("  Phone: %s\n", phone)
	}

	return addressID, nil
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
