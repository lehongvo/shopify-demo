package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"shopify-demo/app"
)

// InputData represents the structure of input.json
type InputData struct {
	Order OrderData `json:"order"`
}

type OrderData struct {
	Items   []ItemData   `json:"items"`
	Email   string       `json:"email"`
	Payments []PaymentData `json:"payments"`
}

type ItemData struct {
	ProductID string `json:"productId"`
	Quantity  int    `json:"quantity"`
}

type PaymentData struct {
	PaymentCode string  `json:"paymentCode"`
	PaymentName string  `json:"paymentName"`
	Amount      string  `json:"amount"`
	Type        string  `json:"type"`
	Currency    string  `json:"currency"`
}

func main() {
	// Check environment variables
	shopDomain := os.Getenv("SHOPIFY_SHOP_DOMAIN")
	accessToken := os.Getenv("SHOPIFY_API_SECRET")

	if shopDomain == "" || accessToken == "" {
		log.Fatal("SHOPIFY_SHOP_DOMAIN and SHOPIFY_API_SECRET must be set in environment variables")
	}

	// Load input data from a JSON file
	inputPath := "cmd/completeDraftOrderWithPaymentPendingTrue/input.json"
	if len(os.Args) > 1 {
		inputPath = os.Args[1]
	}

	inputData, err := loadInputData(inputPath)
	if err != nil {
		log.Fatalf("Failed to load input data: %v", err)
	}

	// Build the draft order input from the loaded data
	draftInput := buildDraftOrderFromInput(inputData)

	// Step 1: Create the draft order
	draftResp, err := app.CreateDraftOrder(draftInput)
	if err != nil {
		log.Fatalf("Failed to create draft order: %v", err)
	}

	draftID := draftResp.Data.DraftOrderCreate.DraftOrder.ID
	log.Printf("✓ Draft order created successfully: %s\n", draftID)

	// Step 2: Complete the draft order with paymentPending = true
	paymentPending := false
	orderInfo, err := app.CompleteDraftOrder(draftID, paymentPending)
	if err != nil {
		log.Fatalf("Failed to complete draft order: %v", err)
	}

	fmt.Printf("✓ Order created successfully with payment pending: %s (%s)\n", orderInfo.OrderName, orderInfo.OrderID)

	// Step 3: Create transaction(s) for the order using REST API directly
	if len(inputData.Order.Payments) > 0 {
		// Convert OrderID from GID to numeric ID
		orderIDNum, err := convertGIDToNumericID(orderInfo.OrderID)
		if err != nil {
			log.Fatalf("Failed to convert order ID: %v", err)
		}

		// Create transaction for each payment using REST API
		for _, payment := range inputData.Order.Payments {
			// Use REST API to create transaction
			transaction, err := createTransactionViaREST(shopDomain, accessToken, orderIDNum, payment)
			if err != nil {
				log.Printf("Warning: Failed to create transaction for payment %s: %v", payment.PaymentCode, err)
				continue
			}

			fmt.Printf("✓ Transaction created: %d (Amount: %s %s, Gateway: %s)\n",
				transaction.ID, payment.Amount, payment.Currency, transaction.Gateway)
		}
	} else {
		log.Println("No payments found in input data, skipping transaction creation")
	}
}

func loadInputData(path string) (*InputData, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read file: %w", err)
	}

	var data InputData
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	return &data, nil
}

func buildDraftOrderFromInput(inputData *InputData) app.DraftOrderInput {
	draftInput := app.DraftOrderInput{
		Email: inputData.Order.Email,
	}

	for _, item := range inputData.Order.Items {
		lineItem := app.DraftLineItemInput{
			VariantID: fmt.Sprintf("gid://shopify/ProductVariant/%s", item.ProductID),
			Quantity:  item.Quantity,
		}
		draftInput.LineItems = append(draftInput.LineItems, lineItem)
	}

	return draftInput
}

// convertGIDToNumericID converts Shopify GID format to numeric ID
// e.g., "gid://shopify/Order/6643316130032" -> 6643316130032
func convertGIDToNumericID(gid string) (int64, error) {
	// Remove GID prefix
	if strings.HasPrefix(gid, "gid://shopify/Order/") {
		idStr := strings.TrimPrefix(gid, "gid://shopify/Order/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse order ID from GID: %w", err)
		}
		return id, nil
	}

	// If it's already numeric, parse it directly
	id, err := strconv.ParseInt(gid, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("order ID is not in GID format and not numeric: %s", gid)
	}
	return id, nil
}

// TransactionResponse represents the response from Shopify transaction API
type TransactionResponse struct {
	Transaction TransactionData `json:"transaction"`
}

type TransactionData struct {
	ID       int64  `json:"id"`
	Kind     string `json:"kind"`
	Status   string `json:"status"`
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
	Gateway  string `json:"gateway"`
}

// createTransactionViaREST creates a transaction using REST API directly
func createTransactionViaREST(shopDomain, accessToken string, orderID int64, payment PaymentData) (*TransactionData, error) {
	apiVersion := "2025-01"
	url := fmt.Sprintf("https://%s/admin/api/%s/orders/%d/transactions.json", shopDomain, apiVersion, orderID)

	// For manual payments, use "manual" gateway and "sale" kind
	// IMPORTANT: Add "source": "external" to allow "sale" kind for API-created orders
	payload := map[string]interface{}{
		"transaction": map[string]interface{}{
			"amount":   payment.Amount,
			"currency": payment.Currency,
			"kind":     "sale",
			"gateway":  "manual",
			"status":   "success",
			"source":   "external", // Required for API-created orders to accept "sale" kind
		},
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

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result TransactionResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result.Transaction, nil
}
