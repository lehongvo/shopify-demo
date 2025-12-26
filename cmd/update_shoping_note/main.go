package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"shopify-demo/app"
)

// Input represents the data needed to update the shipping note metafield.
type Input struct {
	OrderID      string `json:"orderId"`
	ShippingNote string `json:"shippingNote"`
}

func main() {
	// Require env vars for Shopify Admin API
	shopDomain := os.Getenv("SHOPIFY_SHOP_DOMAIN")
	accessToken := os.Getenv("SHOPIFY_API_SECRET")
	if shopDomain == "" || accessToken == "" {
		log.Fatal("SHOPIFY_SHOP_DOMAIN and SHOPIFY_API_SECRET must be set in environment variables")
	}

	inputPath := "cmd/update_shoping_note/input.json"
	if len(os.Args) > 1 {
		inputPath = os.Args[1]
	}

	input, err := loadInput(inputPath)
	if err != nil {
		log.Fatalf("Failed to load input: %v", err)
	}
	if input.OrderID == "" {
		log.Fatal("orderId is required in input.json")
	}

	// Ensure the metafield definition exists (needed even when setting empty value)
	if err := app.EnsureShippingNoteMetafieldDefinition(); err != nil {
		log.Printf("Warning: could not ensure metafield definition: %v\n", err)
	}

	if strings.TrimSpace(input.ShippingNote) == "" {
		if err := deleteShippingNoteMetafield(input.OrderID); err != nil {
			log.Fatalf("Failed to clear shipping note metafield: %v", err)
		}
		fmt.Printf("✓ Shipping note cleared for order %s\n", input.OrderID)
	} else {
		// Update shipping note metafield (set value)
		if err := addShippingNoteMetafield(input.OrderID, input.ShippingNote); err != nil {
			log.Fatalf("Failed to update shipping note metafield: %v", err)
		}
		fmt.Printf("✓ Shipping note updated to \"%s\" for order %s\n", input.ShippingNote, input.OrderID)
	}
}

func loadInput(path string) (*Input, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read file: %w", err)
	}
	var input Input
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return &input, nil
}

// addShippingNoteMetafield adds or updates the shipping note metafield for an order.
func addShippingNoteMetafield(orderID, shippingNote string) error {
	const mutation = `
		mutation SetMetafields($metafields: [MetafieldsSetInput!]!) {
			metafieldsSet(metafields: $metafields) {
				metafields {
					id
					namespace
					key
					value
				}
				userErrors {
					field
					message
				}
			}
		}`

	metafieldInput := map[string]interface{}{
		"namespace": "connectpos",
		"key":       "shipping_note",
		"type":      "multi_line_text_field",
		"value":     shippingNote,
		"ownerId":   orderID,
	}

	variables := map[string]interface{}{
		"metafields": []interface{}{metafieldInput},
	}

	resp, err := app.CallAdminGraphQL(mutation, variables)
	if err != nil {
		return fmt.Errorf("failed to call GraphQL: %w", err)
	}

	// Check GraphQL errors
	if errors, ok := resp["errors"].([]interface{}); ok && len(errors) > 0 {
		return fmt.Errorf("GraphQL errors: %v", errors)
	}

	// Check user errors
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("unexpected response format: missing data")
	}

	setResult, ok := data["metafieldsSet"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("unexpected response format: missing metafieldsSet")
	}

	if userErrors, ok := setResult["userErrors"].([]interface{}); ok && len(userErrors) > 0 {
		return fmt.Errorf("user errors: %v", userErrors)
	}

	return nil
}

// deleteShippingNoteMetafield removes the shipping note metafield (if exists) using REST API.
func deleteShippingNoteMetafield(orderID string) error {
	metafieldID, err := getShippingNoteMetafieldID(orderID)
	if err != nil {
		return err
	}
	if metafieldID == "" {
		// Nothing to delete
		return nil
	}

	shopDomain := os.Getenv("SHOPIFY_SHOP_DOMAIN")
	accessToken := os.Getenv("SHOPIFY_API_SECRET")
	if shopDomain == "" || accessToken == "" {
		return fmt.Errorf("SHOPIFY_SHOP_DOMAIN and SHOPIFY_API_SECRET must be set in environment variables")
	}

	numericID := extractIDFromGID(metafieldID)
	if numericID == "" {
		return fmt.Errorf("invalid metafield gid: %s", metafieldID)
	}

	url := fmt.Sprintf("https://%s/admin/api/2024-07/metafields/%s.json", shopDomain, numericID)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to build DELETE request: %w", err)
	}
	req.Header.Set("X-Shopify-Access-Token", accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call REST delete: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("REST delete failed with status %d", resp.StatusCode)
	}

	return nil
}

// getShippingNoteMetafieldID fetches the metafield ID for shipping note.
func getShippingNoteMetafieldID(orderID string) (string, error) {
	const query = `
		query GetShippingNoteMetafield($id: ID!) {
			order(id: $id) {
				metafield(namespace: "connectpos", key: "shipping_note") {
					id
				}
			}
		}`

	variables := map[string]interface{}{
		"id": orderID,
	}

	resp, err := app.CallAdminGraphQL(query, variables)
	if err != nil {
		return "", fmt.Errorf("failed to query metafield: %w", err)
	}

	if data, ok := resp["data"].(map[string]interface{}); ok {
		if order, ok := data["order"].(map[string]interface{}); ok {
			if mf, ok := order["metafield"].(map[string]interface{}); ok {
				if id, ok := mf["id"].(string); ok {
					return id, nil
				}
			}
		}
	}

	return "", nil
}

// extractIDFromGID converts gid://shopify/Metafield/123 -> 123
func extractIDFromGID(gid string) string {
	if gid == "" {
		return ""
	}
	parts := strings.Split(gid, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

