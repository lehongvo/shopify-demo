package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// DraftOrderInput represents the input for creating a draft order
type DraftOrderInput struct {
	Email           string               `json:"email,omitempty"`
	LineItems       []DraftLineItemInput `json:"lineItems"`
	ShippingAddress *MailingAddressInput `json:"shippingAddress,omitempty"`
	BillingAddress  *MailingAddressInput `json:"billingAddress,omitempty"`
	Customer        *CustomerInput       `json:"customer,omitempty"`
	Note            string               `json:"note,omitempty"`
	Tags            []string             `json:"tags,omitempty"`
	Metafields      []MetafieldInput     `json:"metafields,omitempty"`
}

// DraftLineItemInput represents a line item in a draft order with discount support
type DraftLineItemInput struct {
	VariantID       string                `json:"variantId"`
	Quantity        int                   `json:"quantity"`
	Price           string                `json:"price,omitempty"`
	Title           string                `json:"title,omitempty"`
	AppliedDiscount *AppliedDiscountInput `json:"appliedDiscount,omitempty"`
}

// AppliedDiscountInput represents a discount applied to a line item
type AppliedDiscountInput struct {
	Description string  `json:"description,omitempty"`
	ValueType   string  `json:"valueType"` // PERCENTAGE or FIXED_AMOUNT
	Value       float64 `json:"value"`
	Title       string  `json:"title,omitempty"`
}

// OrderInput represents the input for creating an order (kept for backward compatibility)
type OrderInput struct {
	Email           string               `json:"email,omitempty"`
	LineItems       []LineItemInput      `json:"lineItems"`
	ShippingAddress *MailingAddressInput `json:"shippingAddress,omitempty"`
	BillingAddress  *MailingAddressInput `json:"billingAddress,omitempty"`
	FinancialStatus string               `json:"financialStatus,omitempty"` // PENDING, AUTHORIZED, PARTIALLY_PAID, PAID, PARTIALLY_REFUNDED, REFUNDED, VOIDED
	Customer        *CustomerInput       `json:"customer,omitempty"`
	Note            string               `json:"note,omitempty"`
	Tags            []string             `json:"tags,omitempty"`
	Metafields      []MetafieldInput     `json:"metafields,omitempty"`
}

// LineItemInput represents a line item in an order
type LineItemInput struct {
	VariantID string `json:"variantId"`
	Quantity  int    `json:"quantity"`
	Price     string `json:"price,omitempty"`
	Title     string `json:"title,omitempty"`
}

// MailingAddressInput represents a mailing address
type MailingAddressInput struct {
	Address1  string `json:"address1,omitempty"`
	Address2  string `json:"address2,omitempty"`
	City      string `json:"city,omitempty"`
	Province  string `json:"province,omitempty"`
	Country   string `json:"country,omitempty"`
	Zip       string `json:"zip,omitempty"`
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	Phone     string `json:"phone,omitempty"`
}

// CustomerInput represents customer information
type CustomerInput struct {
	ID        string `json:"id,omitempty"`
	Email     string `json:"email,omitempty"`
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	Phone     string `json:"phone,omitempty"`
}

// MetafieldInput represents a metafield
type MetafieldInput struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
}

// DraftOrderResponse represents the response from draftOrderCreate mutation
type DraftOrderResponse struct {
	Data struct {
		DraftOrderCreate struct {
			DraftOrder struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"draftOrder"`
			UserErrors []UserError `json:"userErrors"`
		} `json:"draftOrderCreate"`
	} `json:"data"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

// CompleteDraftOrderResponse represents the response from draftOrderComplete mutation
type CompleteDraftOrderResponse struct {
	Data struct {
		DraftOrderComplete struct {
			DraftOrder struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"draftOrder"`
			UserErrors []UserError `json:"userErrors"`
		} `json:"draftOrderComplete"`
	} `json:"data"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

// OrderResponse represents the response from orderCreate mutation (kept for backward compatibility)
type OrderResponse struct {
	Data struct {
		OrderCreate struct {
			Order struct {
				ID            string `json:"id"`
				Name          string `json:"name"`
				Email         string `json:"email"`
				TotalPriceSet struct {
					ShopMoney struct {
						Amount       string `json:"amount"`
						CurrencyCode string `json:"currencyCode"`
					} `json:"shopMoney"`
				} `json:"totalPriceSet"`
				CreatedAt   string `json:"createdAt"`
				OrderNumber int    `json:"orderNumber"`
			} `json:"order"`
			UserErrors []UserError `json:"userErrors"`
		} `json:"orderCreate"`
	} `json:"data"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

// OrderInfo represents order information after completing draft order
type OrderInfo struct {
	OrderID           string
	OrderName         string
	DraftID           string
	DraftName         string
	FulfillmentOrders []FulfillmentOrderInfo
}

// FulfillmentOrderInfo represents fulfillment order information
type FulfillmentOrderInfo struct {
	ID                 string
	Status             string
	RequestStatus      string
	AssignedLocationID string
	LineItems          []FulfillmentOrderLineItem
}

// FulfillmentOrderLineItem represents a line item in a fulfillment order
type FulfillmentOrderLineItem struct {
	ID         string
	Quantity   int
	LineItemID string
}

// UserError represents a user error from Shopify
type UserError struct {
	Field   []string `json:"field"`
	Message string   `json:"message"`
}

// GraphQLError represents a GraphQL error
type GraphQLError struct {
	Message    string `json:"message"`
	Extensions struct {
		Code string `json:"code,omitempty"`
	} `json:"extensions,omitempty"`
}

// CallAdminGraphQL is a helper function to call Shopify Admin GraphQL API
// Exported for use in other packages
func CallAdminGraphQL(query string, variables map[string]interface{}) (map[string]interface{}, error) {
	return callAdminGraphQL(query, variables)
}

// callAdminGraphQL is a helper function to call Shopify Admin GraphQL API
func callAdminGraphQL(query string, variables map[string]interface{}) (map[string]interface{}, error) {
	shopDomain := os.Getenv("SHOPIFY_SHOP_DOMAIN")
	accessToken := os.Getenv("SHOPIFY_API_SECRET")

	if shopDomain == "" || accessToken == "" {
		return nil, fmt.Errorf("SHOPIFY_SHOP_DOMAIN and SHOPIFY_API_SECRET must be set")
	}

	apiVersion := "2025-10"
	url := fmt.Sprintf("https://%s/admin/api/%s/graphql.json", shopDomain, apiVersion)

	payload := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Shopify-Access-Token", accessToken)

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var respData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &respData); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// GraphQL có thể trả lỗi nhưng HTTP vẫn 200
	if errs, ok := respData["errors"]; ok {
		return nil, fmt.Errorf("GraphQL errors: %v", errs)
	}

	return respData, nil
}

// CreateDraftOrder creates a draft order in Shopify using GraphQL Admin API
func CreateDraftOrder(input DraftOrderInput) (*DraftOrderResponse, error) {
	const mutation = `
		mutation CreateDraftOrder($input: DraftOrderInput!) {
			draftOrderCreate(input: $input) {
				draftOrder {
					id
					name
				}
				userErrors {
					field
					message
				}
			}
		}`

	variables := map[string]interface{}{
		"input": input,
	}

	resp, err := callAdminGraphQL(mutation, variables)
	if err != nil {
		return nil, err
	}

	// Convert to structured response
	jsonData, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	var response DraftOrderResponse
	if err := json.Unmarshal(jsonData, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for user errors
	if len(response.Data.DraftOrderCreate.UserErrors) > 0 {
		errorMsg := "User errors: "
		for _, err := range response.Data.DraftOrderCreate.UserErrors {
			errorMsg += fmt.Sprintf("%v: %s; ", err.Field, err.Message)
		}
		return nil, errors.New(errorMsg)
	}

	return &response, nil
}

// CompleteDraftOrder completes a draft order to create a real order
// paymentPending: false means the order will be marked as paid
func CompleteDraftOrder(draftID string, paymentPending bool) (*OrderInfo, error) {
	const mutation = `
		mutation CompleteDraftOrder($id: ID!, $paymentPending: Boolean!) {
			draftOrderComplete(id: $id, paymentPending: $paymentPending) {
				draftOrder {
					id
					name
				}
				userErrors {
					field
					message
				}
			}
		}`

	variables := map[string]interface{}{
		"id":             draftID,
		"paymentPending": paymentPending,
	}

	resp, err := callAdminGraphQL(mutation, variables)
	if err != nil {
		return nil, err
	}

	// Convert to structured response
	jsonData, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	var response CompleteDraftOrderResponse
	if err := json.Unmarshal(jsonData, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for user errors
	if len(response.Data.DraftOrderComplete.UserErrors) > 0 {
		errorMsg := "User errors: "
		for _, err := range response.Data.DraftOrderComplete.UserErrors {
			errorMsg += fmt.Sprintf("%v: %s; ", err.Field, err.Message)
		}
		return nil, errors.New(errorMsg)
	}

	// Get order info from draft
	orderID, orderName, err := getOrderFromDraft(draftID)
	if err == nil && orderID != "" {
		// Query fulfillment orders (automatically created by Shopify)
		fulfillmentOrders, _ := GetFulfillmentOrders(orderID)

		return &OrderInfo{
			OrderID:           orderID,
			OrderName:         orderName,
			DraftID:           response.Data.DraftOrderComplete.DraftOrder.ID,
			DraftName:         response.Data.DraftOrderComplete.DraftOrder.Name,
			FulfillmentOrders: fulfillmentOrders,
		}, nil
	}

	// Fallback: return draft info if order not found yet
	return &OrderInfo{
		DraftID:   response.Data.DraftOrderComplete.DraftOrder.ID,
		DraftName: response.Data.DraftOrderComplete.DraftOrder.Name,
	}, nil
}

// getOrderFromDraft queries the draft order to get the created order after completion
func getOrderFromDraft(draftID string) (string, string, error) {
	const query = `
		query GetDraftOrderOrder($id: ID!) {
			node(id: $id) {
				... on DraftOrder {
					id
					name
					order {
						id
						name
					}
				}
			}
		}`

	variables := map[string]interface{}{
		"id": draftID,
	}

	resp, err := callAdminGraphQL(query, variables)
	if err != nil {
		return "", "", err
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return "", "", fmt.Errorf("unexpected GraphQL response, missing data")
	}

	node, ok := data["node"].(map[string]interface{})
	if !ok || node == nil {
		return "", "", fmt.Errorf("no node found for draft id %s", draftID)
	}

	order, ok := node["order"].(map[string]interface{})
	if !ok || order == nil {
		return "", "", fmt.Errorf("draft has no linked order yet")
	}

	orderID, _ := order["id"].(string)
	orderName, _ := order["name"].(string)
	return orderID, orderName, nil
}

// GetFulfillmentOrders queries fulfillment orders for a given order ID
// Note: FulfillmentOrders are automatically created when draftOrderComplete is called
// This function will retry up to maxRetries times with increasing delays
func GetFulfillmentOrders(orderID string) ([]FulfillmentOrderInfo, error) {
	return GetFulfillmentOrdersWithRetry(orderID, 5, 3*time.Second)
}

// GetFulfillmentOrdersWithRetry queries fulfillment orders with retry logic
// maxRetries: maximum number of retry attempts
// initialDelay: initial delay between retries (will increase exponentially)
func GetFulfillmentOrdersWithRetry(orderID string, maxRetries int, initialDelay time.Duration) ([]FulfillmentOrderInfo, error) {
	var lastErr error
	delay := initialDelay

	for i := 0; i < maxRetries; i++ {
		fulfillmentOrders, err := getFulfillmentOrdersOnce(orderID)
		if err == nil {
			// If we got results (even if empty), return them
			// Empty might mean routing not complete, but no error
			return fulfillmentOrders, nil
		}

		lastErr = err

		// If not the last retry, wait before retrying
		if i < maxRetries-1 {
			time.Sleep(delay)
			delay = time.Duration(float64(delay) * 1.5) // Exponential backoff
		}
	}

	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// getFulfillmentOrdersOnce performs a single query for fulfillment orders
func getFulfillmentOrdersOnce(orderID string) ([]FulfillmentOrderInfo, error) {
	const query = `
		query GetFulfillmentOrders($id: ID!) {
			order(id: $id) {
				id
				name
				fulfillmentOrders(first: 10) {
					edges {
						node {
							id
							status
							requestStatus
							assignedLocation {
								location {
									id
								}
							}
							lineItems(first: 50) {
								edges {
									node {
										id
										remainingQuantity
										totalQuantity
										lineItem {
											id
											title
										}
									}
								}
							}
						}
					}
				}
			}
		}`

	variables := map[string]interface{}{
		"id": orderID,
	}

	resp, err := callAdminGraphQL(query, variables)
	if err != nil {
		return nil, err
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected GraphQL response, missing data")
	}

	order, ok := data["order"].(map[string]interface{})
	if !ok || order == nil {
		return nil, fmt.Errorf("order not found for id %s", orderID)
	}

	fulfillmentOrdersData, ok := order["fulfillmentOrders"].(map[string]interface{})
	if !ok {
		// Debug: Check if order exists but fulfillmentOrders field is missing
		// This might indicate access scope issues
		return []FulfillmentOrderInfo{}, nil
	}

	edges, ok := fulfillmentOrdersData["edges"].([]interface{})
	if !ok {
		return []FulfillmentOrderInfo{}, nil
	}

	var fulfillmentOrders []FulfillmentOrderInfo
	for _, edge := range edges {
		edgeMap, ok := edge.(map[string]interface{})
		if !ok {
			continue
		}

		node, ok := edgeMap["node"].(map[string]interface{})
		if !ok {
			continue
		}

		fo := FulfillmentOrderInfo{
			ID:            getString(node, "id"),
			Status:        getString(node, "status"),
			RequestStatus: getString(node, "requestStatus"),
		}

		// Get assigned location
		if assignedLocation, ok := node["assignedLocation"].(map[string]interface{}); ok {
			if location, ok := assignedLocation["location"].(map[string]interface{}); ok {
				fo.AssignedLocationID = getString(location, "id")
			}
		}

		// Get line items
		if lineItemsData, ok := node["lineItems"].(map[string]interface{}); ok {
			if lineItemEdges, ok := lineItemsData["edges"].([]interface{}); ok {
				for _, liEdge := range lineItemEdges {
					if liEdgeMap, ok := liEdge.(map[string]interface{}); ok {
						if liNode, ok := liEdgeMap["node"].(map[string]interface{}); ok {
							lineItem := FulfillmentOrderLineItem{
								ID:       getString(liNode, "id"),
								Quantity: getInt(liNode, "remainingQuantity"), // Use remainingQuantity instead of quantity
							}
							// Fallback to totalQuantity if remainingQuantity is 0
							if lineItem.Quantity == 0 {
								lineItem.Quantity = getInt(liNode, "totalQuantity")
							}
							if lineItemObj, ok := liNode["lineItem"].(map[string]interface{}); ok {
								lineItem.LineItemID = getString(lineItemObj, "id")
							}
							fo.LineItems = append(fo.LineItems, lineItem)
						}
					}
				}
			}
		}

		fulfillmentOrders = append(fulfillmentOrders, fo)
	}

	return fulfillmentOrders, nil
}

// Helper functions for type conversion
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if val, ok := m[key].(float64); ok {
		return int(val)
	}
	return 0
}

// CreateFulfillment creates a fulfillment for one or more fulfillment orders
func CreateFulfillment(fulfillmentOrderIDs []string, trackingInfo *TrackingInfo) (string, error) {
	const mutation = `
		mutation CreateFulfillment($fulfillment: FulfillmentV2Input!) {
			fulfillmentCreateV2(fulfillment: $fulfillment) {
				fulfillment {
					id
					status
				}
				userErrors {
					field
					message
				}
			}
		}`

	fulfillmentInput := map[string]interface{}{
		"notifyCustomer":              true,
		"lineItemsByFulfillmentOrder": []map[string]interface{}{},
	}

	// Add fulfillment orders
	for _, foID := range fulfillmentOrderIDs {
		fulfillmentInput["lineItemsByFulfillmentOrder"] = append(
			fulfillmentInput["lineItemsByFulfillmentOrder"].([]map[string]interface{}),
			map[string]interface{}{
				"fulfillmentOrderId": foID,
			},
		)
	}

	// Add tracking info if provided
	if trackingInfo != nil {
		if trackingInfo.TrackingNumber != "" {
			fulfillmentInput["trackingInfo"] = map[string]interface{}{
				"number":  trackingInfo.TrackingNumber,
				"company": trackingInfo.TrackingCompany,
				"url":     trackingInfo.TrackingURL,
			}
		}
	}

	variables := map[string]interface{}{
		"fulfillment": fulfillmentInput,
	}

	resp, err := callAdminGraphQL(mutation, variables)
	if err != nil {
		return "", err
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected GraphQL response, missing data")
	}

	createRes, ok := data["fulfillmentCreateV2"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("missing fulfillmentCreateV2 in response")
	}

	// Check for user errors
	if ue, ok := createRes["userErrors"].([]interface{}); ok && len(ue) > 0 {
		errorMsg := "User errors: "
		for _, err := range ue {
			if errMap, ok := err.(map[string]interface{}); ok {
				errorMsg += fmt.Sprintf("%v: %v; ", errMap["field"], errMap["message"])
			}
		}
		return "", errors.New(errorMsg)
	}

	fulfillment, ok := createRes["fulfillment"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("missing fulfillment in response")
	}

	fulfillmentID := getString(fulfillment, "id")
	return fulfillmentID, nil
}

// TrackingInfo represents tracking information for fulfillment
type TrackingInfo struct {
	TrackingNumber  string
	TrackingCompany string
	TrackingURL     string
}

// CreateOrderFromDraft is a convenience function that creates a draft order and completes it
// This is the recommended way to create orders with discounts
func CreateOrderFromDraft(input DraftOrderInput, paymentPending bool) (*OrderInfo, error) {
	// Step 1: Create draft order
	draftResp, err := CreateDraftOrder(input)
	if err != nil {
		return nil, fmt.Errorf("failed to create draft order: %w", err)
	}

	draftID := draftResp.Data.DraftOrderCreate.DraftOrder.ID

	// Step 2: Complete draft order
	orderInfo, err := CompleteDraftOrder(draftID, paymentPending)
	if err != nil {
		return nil, fmt.Errorf("failed to complete draft order: %w", err)
	}

	return orderInfo, nil
}

// CreateOrder creates a new order in Shopify using GraphQL Admin API (legacy method, kept for backward compatibility)
func CreateOrder(input OrderInput) (*OrderResponse, error) {
	// Get environment variables
	shopDomain := os.Getenv("SHOPIFY_SHOP_DOMAIN")
	accessToken := os.Getenv("SHOPIFY_API_SECRET")

	if shopDomain == "" || accessToken == "" {
		return nil, fmt.Errorf("SHOPIFY_SHOP_DOMAIN and SHOPIFY_API_SECRET must be set")
	}

	// Construct GraphQL mutation
	mutation := `
		mutation orderCreate($input: OrderCreateInput!) {
			orderCreate(input: $input) {
				order {
					id
					name
					email
					totalPriceSet {
						shopMoney {
							amount
							currencyCode
						}
					}
					createdAt
					orderNumber
				}
				userErrors {
					field
					message
				}
			}
		}
	`

	// Prepare GraphQL request
	requestBody := map[string]interface{}{
		"query": mutation,
		"variables": map[string]interface{}{
			"input": input,
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Construct API endpoint
	apiVersion := "2025-10"
	url := fmt.Sprintf("https://%s/admin/api/%s/graphql.json", shopDomain, apiVersion)

	// Create HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Shopify-Access-Token", accessToken)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var response OrderResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for GraphQL errors
	if len(response.Errors) > 0 {
		errorMsg := "GraphQL errors: "
		for _, err := range response.Errors {
			errorMsg += err.Message + "; "
		}
		return nil, errors.New(errorMsg)
	}

	// Check for user errors
	if len(response.Data.OrderCreate.UserErrors) > 0 {
		errorMsg := "User errors: "
		for _, err := range response.Data.OrderCreate.UserErrors {
			errorMsg += fmt.Sprintf("%v: %s; ", err.Field, err.Message)
		}
		return nil, errors.New(errorMsg)
	}

	return &response, nil
}

// ExampleCreateOrderFromDraft demonstrates how to use the CreateOrderFromDraft function
// This is the recommended way to create orders with discounts
func ExampleCreateOrderFromDraft() {
	// Example: Create a draft order with line items and discounts
	draftInput := DraftOrderInput{
		Email: "customer@example.com",
		LineItems: []DraftLineItemInput{
			{
				VariantID: "gid://shopify/ProductVariant/48360775188720", // Replace with actual variant ID
				Quantity:  1,
				AppliedDiscount: &AppliedDiscountInput{
					Description: "20% off",
					ValueType:   "PERCENTAGE",
					Value:       20.0,
					Title:       "20PERCENT",
				},
			},
			{
				VariantID: "gid://shopify/ProductVariant/48360774369520", // Another variant
				Quantity:  2,
				AppliedDiscount: &AppliedDiscountInput{
					Description: "15% off",
					ValueType:   "PERCENTAGE",
					Value:       15.0,
					Title:       "15PERCENT",
				},
			},
		},
		ShippingAddress: &MailingAddressInput{
			Address1:  "123 Main Street",
			City:      "New York",
			Province:  "NY",
			Country:   "US",
			Zip:       "10001",
			FirstName: "John",
			LastName:  "Doe",
			Phone:     "+1234567890",
		},
		BillingAddress: &MailingAddressInput{
			Address1:  "123 Main Street",
			City:      "New York",
			Province:  "NY",
			Country:   "US",
			Zip:       "10001",
			FirstName: "John",
			LastName:  "Doe",
		},
		Customer: &CustomerInput{
			Email:     "customer@example.com",
			FirstName: "John",
			LastName:  "Doe",
		},
		Note: "Order created via API with discounts",
		Tags: []string{"api", "test", "discount"},
	}

	// Create order from draft (paymentPending=false means order will be marked as paid)
	orderInfo, err := CreateOrderFromDraft(draftInput, false)
	if err != nil {
		panic(fmt.Sprintf("Failed to create order: %v", err))
	}

	// Print order details
	fmt.Printf("Order created successfully!\n")
	fmt.Printf("Order ID: %s\n", orderInfo.OrderID)
	fmt.Printf("Order Name: %s\n", orderInfo.OrderName)
	fmt.Printf("Draft ID: %s\n", orderInfo.DraftID)
	fmt.Printf("Draft Name: %s\n", orderInfo.DraftName)
}

// ExampleCreateDraftOrderStepByStep demonstrates step-by-step draft order creation
func ExampleCreateDraftOrderStepByStep() {
	// Step 1: Create draft order
	draftInput := DraftOrderInput{
		Email: "customer@example.com",
		LineItems: []DraftLineItemInput{
			{
				VariantID: "gid://shopify/ProductVariant/48360775188720",
				Quantity:  1,
				AppliedDiscount: &AppliedDiscountInput{
					Description: "17% off",
					ValueType:   "PERCENTAGE",
					Value:       17.0,
					Title:       "17PERCENT",
				},
			},
		},
	}

	draftResp, err := CreateDraftOrder(draftInput)
	if err != nil {
		panic(fmt.Sprintf("Failed to create draft order: %v", err))
	}

	draftID := draftResp.Data.DraftOrderCreate.DraftOrder.ID
	draftName := draftResp.Data.DraftOrderCreate.DraftOrder.Name
	fmt.Printf("Draft order created: %s (ID: %s)\n", draftName, draftID)

	// Step 2: Complete draft order
	// paymentPending=false means the order will be marked as paid
	orderInfo, err := CompleteDraftOrder(draftID, false)
	if err != nil {
		panic(fmt.Sprintf("Failed to complete draft order: %v", err))
	}

	fmt.Printf("Order completed successfully!\n")
	fmt.Printf("Order ID: %s\n", orderInfo.OrderID)
	fmt.Printf("Order Name: %s\n", orderInfo.OrderName)
}

// ExampleCreateOrderLegacy demonstrates the legacy CreateOrder function (without discounts)
func ExampleCreateOrderLegacy() {
	// Example: Create an order directly (legacy method, no discount support)
	orderInput := OrderInput{
		Email: "customer@example.com",
		LineItems: []LineItemInput{
			{
				VariantID: "gid://shopify/ProductVariant/1234567890", // Replace with actual variant ID
				Quantity:  1,
			},
		},
		ShippingAddress: &MailingAddressInput{
			Address1:  "123 Main Street",
			City:      "New York",
			Province:  "NY",
			Country:   "US",
			Zip:       "10001",
			FirstName: "John",
			LastName:  "Doe",
			Phone:     "+1234567890",
		},
		BillingAddress: &MailingAddressInput{
			Address1:  "123 Main Street",
			City:      "New York",
			Province:  "NY",
			Country:   "US",
			Zip:       "10001",
			FirstName: "John",
			LastName:  "Doe",
		},
		FinancialStatus: "PAID", // Options: PENDING, AUTHORIZED, PARTIALLY_PAID, PAID, PARTIALLY_REFUNDED, REFUNDED, VOIDED
		Customer: &CustomerInput{
			Email:     "customer@example.com",
			FirstName: "John",
			LastName:  "Doe",
		},
		Note: "Order created via API",
		Tags: []string{"api", "test"},
	}

	// Create the order
	response, err := CreateOrder(orderInput)
	if err != nil {
		panic(fmt.Sprintf("Failed to create order: %v", err))
	}

	// Print order details
	order := response.Data.OrderCreate.Order
	fmt.Printf("Order created successfully!\n")
	fmt.Printf("Order ID: %s\n", order.ID)
	fmt.Printf("Order Name: %s\n", order.Name)
	fmt.Printf("Order Number: %d\n", order.OrderNumber)
	fmt.Printf("Email: %s\n", order.Email)
	fmt.Printf("Total Price: %s %s\n",
		order.TotalPriceSet.ShopMoney.Amount,
		order.TotalPriceSet.ShopMoney.CurrencyCode)
	fmt.Printf("Created At: %s\n", order.CreatedAt)
}
