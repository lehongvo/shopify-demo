package main

import (
	"encoding/json"
	"fmt"
	"log"

	"shopify-demo/app"
)

// This command creates a demo product on Shopify with metafields for
// Length, Width, Radius Corners, Drill Holes, Edge Finish.
// Usage:
//   go run cmd/addNewProduct/main.go
// (you can change the payload inside buildProductInput if needed)

func main() {
	fmt.Println("=== Creating demo product with custom fields ===")

	input := buildProductInput()

	const mutation = `
		mutation AddNewProduct($input: ProductInput!) {
			productCreate(input: $input) {
				product {
					id
					title
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

	resp, err := app.CallAdminGraphQL(mutation, variables)
	if err != nil {
		log.Fatalf("Error calling productCreate: %v", err)
	}

	// Pretty print full response for debugging
	jsonData, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println("Full Response:")
	fmt.Println(string(jsonData))

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		log.Fatal("No data in response")
	}

	pc, ok := data["productCreate"].(map[string]interface{})
	if !ok {
		log.Fatal("productCreate not found in response")
	}

	if userErrors, ok := pc["userErrors"].([]interface{}); ok && len(userErrors) > 0 {
		fmt.Println("\nUser errors:")
		for _, ue := range userErrors {
			if m, ok := ue.(map[string]interface{}); ok {
				fmt.Printf("- %v: %v\n", m["field"], m["message"])
			}
		}
		log.Fatal("productCreate returned user errors")
	}

	product, ok := pc["product"].(map[string]interface{})
	if !ok {
		log.Fatal("product not found in response")
	}

	fmt.Println("\n=== Product Created ===")
	fmt.Printf("ID: %s\n", product["id"])
	fmt.Printf("Title: %s\n", product["title"])
}

// buildProductInput builds the ProductInput payload with metafields
// for Length, Width, Radius Corners, Drill Holes, Edge Finish.
func buildProductInput() map[string]interface{} {
	// You can change these values as needed, this is a demo product.
	title := "Custom Colored Acrylic (ConnectPOS Demo)"
	descriptionHTML := "<p>Custom colored acrylic sheet with configurable size and finish options.</p>"

	// Metafields will be used for the extra fields the customer cares about.
	// Namespace/key can be aligned with POS mapping later.
	metafields := []map[string]interface{}{
		{
			"namespace": "connectpos",
			"key":       "length",
			"type":      "single_line_text_field",
			"value":     "Length (inches)",
		},
		{
			"namespace": "connectpos",
			"key":       "width",
			"type":      "single_line_text_field",
			"value":     "Width (inches)",
		},
		{
			"namespace": "connectpos",
			"key":       "radius_corners",
			"type":      "single_line_text_field",
			"value":     "Radius Corners",
		},
		{
			"namespace": "connectpos",
			"key":       "drill_holes",
			"type":      "single_line_text_field",
			"value":     "Drill Holes",
		},
		{
			"namespace": "connectpos",
			"key":       "edge_finish",
			"type":      "single_line_text_field",
			"value":     "Edge Finish",
		},
	}

	input := map[string]interface{}{
		"title":           title,
		"descriptionHtml": descriptionHTML,
		"status":          "ACTIVE",
		"metafields":      metafields,
	}

	return input
}

