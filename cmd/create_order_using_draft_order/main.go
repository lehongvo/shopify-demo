package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"

	"shopify-demo/app"
)

// InputData represents the structure of input.json
type InputData struct {
	Order OrderData `json:"order"`
}

type DiscountApplicationData struct {
	Title     string `json:"title"`
	Value     string `json:"value"`
	ValueType string `json:"valueType"`
	Amount    string `json:"amount"`
}

type CustomerData struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
}

type NoteAttributeData struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// AdditionalData holds extra fields that may come from the POS payload
type AdditionalData struct {
	ShippingNote string `json:"shipping_note,omitempty"`
}

type OrderData struct {
	TaxLines            []TaxLineData            `json:"taxLines"`
	TaxesIncluded       bool                     `json:"taxesIncluded"`
	TotalTax            string                   `json:"totalTax"`
	Email               string                   `json:"email"`
	Items               []ItemData               `json:"items"`
	ShippingAddress     *AddressData             `json:"shippingAddress"`
	BillingAddress      *AddressData             `json:"billingAddress"`
	Customer            *CustomerData            `json:"customer,omitempty"`
	Note                string                   `json:"note"`
	ShippingNote        string                   `json:"shippingNote,omitempty"` // deprecated: use AdditionalData.ShippingNote
	AdditionalData      *AdditionalData          `json:"additionalData,omitempty"`
	NoteAttributes      []NoteAttributeData      `json:"noteAttributes,omitempty"`
	Tags                string                   `json:"tags"`
	TotalDiscounts      string                   `json:"totalDiscounts,omitempty"`
	DiscountApplications []DiscountApplicationData `json:"discountApplications,omitempty"`
	DiscountCodes       []string                 `json:"discountCodes,omitempty"`
	SubtotalPrice       string                   `json:"subtotalPrice,omitempty"`
	TotalPrice          string                   `json:"totalPrice,omitempty"`
	ShippingMethod      string                   `json:"shippingMethod,omitempty"`
	TotalShipping       string                   `json:"totalShipping,omitempty"`
	TotalShippingIncTax string                   `json:"totalShippingIncTax,omitempty"`
	TotalShippingExTax  string                   `json:"totalShippingExTax,omitempty"`
	TotalTaxShipping    string                   `json:"totalTaxShipping,omitempty"`
}

type TaxLineData struct {
	ID         string `json:"id"`
	Rate       string `json:"rate"`
	Price      string `json:"price"`
	Title      string `json:"title"`
	TaxClassID string `json:"taxClassId"`
	Code       string `json:"code"`
	IsUsed     bool   `json:"isUsed"`
}

type ItemData struct {
	ProductID            string                   `json:"productId"`
	Quantity             int                      `json:"quantity"`
	Price                string                   `json:"price"`
	OriginPrice          string                   `json:"originPrice"`
	TotalTax             string                   `json:"totalTax"`
	Taxable              bool                     `json:"taxable"`
	TaxesIncluded        bool                     `json:"taxesIncluded"`
	Name                 string                   `json:"name,omitempty"`
	TotalDiscount        string                   `json:"totalDiscount,omitempty"`
	DiscountApplications []DiscountApplicationData `json:"discountApplications,omitempty"`
}

type AddressData struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Address1  string `json:"address1"`
	City      string `json:"city"`
	Province  string `json:"province"`
	Country   string `json:"country"`
	Zip       string `json:"zip"`
	Phone     string `json:"phone"`
}

func main() {
	// Kiểm tra environment variables
	shopDomain := os.Getenv("SHOPIFY_SHOP_DOMAIN")
	accessToken := os.Getenv("SHOPIFY_API_SECRET")

	if shopDomain == "" || accessToken == "" {
		log.Fatal("SHOPIFY_SHOP_DOMAIN and SHOPIFY_API_SECRET must be set in environment variables")
	}

	inputPath := "cmd/create_order_using_draft_order/input.json"
	if len(os.Args) > 1 {
		inputPath = os.Args[1]
	}

	inputData, err := loadInputData(inputPath)
	if err != nil {
		log.Fatalf("Failed to load input data: %v", err)
	}

	// Convert input.json data to DraftOrderInput
	draftInput, err := buildDraftOrderFromInput(inputData)
	if err != nil {
		log.Fatalf("Failed to build draft order input: %v", err)
	}

	// Step 1: Create draft order
	draftResp, err := app.CreateDraftOrder(draftInput)
	if err != nil {
		log.Fatalf("Failed to create draft order: %v", err)
	}

	draftID := draftResp.Data.DraftOrderCreate.DraftOrder.ID

	// Step 3: Convert tax lines from input.json if available
	// Separate product tax lines from shipping tax
	var productTaxLines []app.TaxLineInput
	var shippingTaxLine *app.TaxLineInput

	if len(inputData.Order.TaxLines) > 0 {
		productTaxLines = make([]app.TaxLineInput, 0, len(inputData.Order.TaxLines))
		for _, tl := range inputData.Order.TaxLines {
			// Skip shipping tax lines (they will be added separately)
			// Shipping tax is identified by checking if it's related to shipping
			// For now, we'll add all tax lines as product tax, and shipping tax separately
			rate, _ := strconv.ParseFloat(tl.Rate, 64)
			productTaxLines = append(productTaxLines, app.TaxLineInput{
				Title: tl.Title,
				Rate:  rate,
				PriceSet: &app.MoneyBagInput{
					ShopMoney: &app.MoneyInput{
						Amount:       tl.Price,
						CurrencyCode: "USD",
					},
				},
			})
		}
	}

	// Calculate shipping tax line if shipping tax data is available
	if inputData.Order.TotalTaxShipping != "" {
		if shippingTaxAmount, ok := parsePrice(inputData.Order.TotalTaxShipping); ok && shippingTaxAmount > 0 {
			// Calculate shipping tax rate from totalTaxShipping and totalShippingExTax
			shippingTaxRate := 0.0
			if inputData.Order.TotalShippingExTax != "" {
				if shippingExTax, ok := parsePrice(inputData.Order.TotalShippingExTax); ok && shippingExTax > 0 {
					shippingTaxRate = shippingTaxAmount / shippingExTax
				}
			}
			
			// If rate is 0, try to get from tax lines (look for shipping-related tax)
			if shippingTaxRate == 0 && len(inputData.Order.TaxLines) > 0 {
				// Use the rate from the last tax line (often shipping tax is last)
				// Or use a common shipping tax rate like 0.065 (6.5%)
				for _, tl := range inputData.Order.TaxLines {
					if rate, err := strconv.ParseFloat(tl.Rate, 64); err == nil {
						shippingTaxRate = rate
						break
					}
				}
			}

			shippingTaxLine = &app.TaxLineInput{
				Title: "Shipping Tax",
				Rate:  shippingTaxRate,
				PriceSet: &app.MoneyBagInput{
					ShopMoney: &app.MoneyInput{
						Amount:       fmt.Sprintf("%.2f", shippingTaxAmount),
						CurrencyCode: "USD",
					},
				},
				Source: "external", // Mark as external source
			}
			fmt.Printf("Debug: Shipping tax calculated: %.2f (rate: %.4f)\n", shippingTaxAmount, shippingTaxRate)
		}
	}

	// Step 4: Ensure metafield definition exists (for shipping note)
	shippingNote := getShippingNote(inputData.Order)
	if shippingNote != "" {
		if err := app.EnsureShippingNoteMetafieldDefinition(); err != nil {
			log.Printf("Warning: Could not ensure metafield definition: %v\n", err)
		}
	}

	// Step 5: Complete draft order
	paymentPending := false
	orderInfo, err := app.CompleteDraftOrder(draftID, paymentPending)
	if err != nil {
		log.Fatalf("Failed to complete draft order: %v", err)
	}

	// Step 6: Add shipping note metafield to order (after completion)
	if shippingNote != "" && orderInfo.OrderID != "" {
		if err := addShippingNoteMetafield(orderInfo.OrderID, shippingNote); err != nil {
			log.Printf("Warning: Failed to add shipping note metafield: %v\n", err)
		} else {
			log.Printf("✓ Successfully added shipping note metafield to order\n")
		}
	}

	// Step 6b: Add shipping tax info to order note if available
	// This ensures shipping tax is visible in admin even if Shopify merges tax lines
	if shippingTaxLine != nil && orderInfo.OrderID != "" {
		if err := addShippingTaxToOrderNote(orderInfo.OrderID, shippingTaxLine); err != nil {
			log.Printf("Warning: Failed to add shipping tax to order note: %v\n", err)
		}
	}

	// Step 7: Combine product tax and shipping tax, then add all tax lines together
	// Shopify may merge tax lines, so we'll add them all at once
	allTaxLines := make([]app.TaxLineInput, 0)
	
	// Add product tax lines
	allTaxLines = append(allTaxLines, productTaxLines...)
	
	// Add shipping tax line if available
	if shippingTaxLine != nil {
		allTaxLines = append(allTaxLines, *shippingTaxLine)
		fmt.Printf("Debug: Shipping tax will be added: %s (rate: %.4f)\n", 
			shippingTaxLine.PriceSet.ShopMoney.Amount, 
			shippingTaxLine.Rate)
	}

	// Add all tax lines to order (after completion)
	if len(allTaxLines) > 0 && orderInfo.OrderID != "" {
		if err := app.AddTaxToOrder(orderInfo.OrderID, allTaxLines); err != nil {
			log.Printf("Warning: Failed to add tax to order: %v\n", err)
		} else {
			if shippingTaxLine != nil {
				fmt.Printf("✓ Tax lines added (including shipping tax: %s)\n", 
					shippingTaxLine.PriceSet.ShopMoney.Amount)
			}
		}
	}

	// Print order details
	fmt.Printf("✓ Order created successfully: %s (%s)\n", orderInfo.OrderName, orderInfo.OrderID)

	// Query order details to show tax information and metafields
	if orderInfo.OrderID != "" {
		queryOrderDetails(orderInfo.OrderID)
		if shippingNote != "" {
			queryOrderMetafields(orderInfo.OrderID)
		}
	}
}

// queryOrderDetails queries the order to get detailed information including tax
func queryOrderDetails(orderID string) {
	const query = `
		query GetOrder($id: ID!) {
			order(id: $id) {
				id
				name
				email
				legacyResourceId
				totalPriceSet {
					shopMoney {
						amount
						currencyCode
					}
				}
				totalTaxSet {
					shopMoney {
						amount
						currencyCode
					}
				}
				taxLines {
					title
					rate
					priceSet {
						shopMoney {
							amount
							currencyCode
						}
					}
				}
				shippingLine {
					title
					priceSet {
						shopMoney {
							amount
							currencyCode
						}
					}
					discountedPriceSet {
						shopMoney {
							amount
							currencyCode
						}
					}
				}
			}
		}`

	variables := map[string]interface{}{
		"id": orderID,
	}

	resp, err := app.CallAdminGraphQL(query, variables)
	if err != nil {
		return
	}

	if data, ok := resp["data"].(map[string]interface{}); ok {
		if order, ok := data["order"].(map[string]interface{}); ok {
			if totalPrice, ok := order["totalPriceSet"].(map[string]interface{}); ok {
				if shopMoney, ok := totalPrice["shopMoney"].(map[string]interface{}); ok {
					if amount, ok := shopMoney["amount"].(string); ok {
						fmt.Printf("Total Price: %s %s\n", amount, shopMoney["currencyCode"])
					}
				}
			}
			if totalTax, ok := order["totalTaxSet"].(map[string]interface{}); ok {
				if shopMoney, ok := totalTax["shopMoney"].(map[string]interface{}); ok {
					if amount, ok := shopMoney["amount"].(string); ok && amount != "" {
						fmt.Printf("Total Tax: %s %s\n", amount, shopMoney["currencyCode"])
					}
				}
			}
			if shippingLine, ok := order["shippingLine"].(map[string]interface{}); ok {
				if priceSet, ok := shippingLine["priceSet"].(map[string]interface{}); ok {
					if shopMoney, ok := priceSet["shopMoney"].(map[string]interface{}); ok {
						if amount, ok := shopMoney["amount"].(string); ok && amount != "" {
							title := ""
							if titleVal, ok := shippingLine["title"].(string); ok {
								title = titleVal
							}
							fmt.Printf("Shipping Line: %s - %s %s\n", title, amount, shopMoney["currencyCode"])
						}
					}
				}
			}
		}
	}
}

// queryOrderMetafields queries the order to get metafields information
func queryOrderMetafields(orderID string) {
	const query = `
		query GetOrderMetafields($id: ID!) {
			order(id: $id) {
				id
				name
				metafields(first: 10, namespace: "connectpos") {
					edges {
						node {
							id
							namespace
							key
							value
							type
						}
					}
				}
			}
		}`

	variables := map[string]interface{}{
		"id": orderID,
	}

	resp, err := app.CallAdminGraphQL(query, variables)
	if err != nil {
		log.Printf("Warning: Failed to query order metafields: %v\n", err)
		return
	}

	if data, ok := resp["data"].(map[string]interface{}); ok {
		if order, ok := data["order"].(map[string]interface{}); ok {
			if metafields, ok := order["metafields"].(map[string]interface{}); ok {
				if edges, ok := metafields["edges"].([]interface{}); ok {
					if len(edges) > 0 {
						fmt.Printf("\nOrder Metafields:\n")
						for _, edge := range edges {
							if edgeMap, ok := edge.(map[string]interface{}); ok {
								if node, ok := edgeMap["node"].(map[string]interface{}); ok {
									key := node["key"]
									value := node["value"]
									fmt.Printf("  - %s: %v\n", key, value)
								}
							}
						}
					} else {
						fmt.Printf("\n⚠ No metafields found in order (namespace: connectpos)\n")
					}
				}
			}
		}
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

func toVariantGID(id string) string {
	if strings.HasPrefix(id, "gid://") {
		return id
	}
	return fmt.Sprintf("gid://shopify/ProductVariant/%s", id)
}

func convertAddress(addr *AddressData) *app.MailingAddressInput {
	if addr == nil {
		return nil
	}
	return &app.MailingAddressInput{
		Address1:  addr.Address1,
		City:      addr.City,
		Province:  addr.Province,
		Country:   addr.Country,
		Zip:       addr.Zip,
		FirstName: addr.FirstName,
		LastName:  addr.LastName,
		Phone:     addr.Phone,
	}
}

func parseTags(tags string) []string {
	if strings.TrimSpace(tags) == "" {
		return nil
	}
	var result []string
	for _, t := range strings.Split(tags, ",") {
		tag := strings.TrimSpace(t)
		if tag != "" {
			result = append(result, tag)
		}
	}
	return result
}

// buildDraftOrderFromInput maps data from input.json to DraftOrderInput
// This ensures that when there's no tax, we use CreateDraftOrder and draftOrderComplete
// with proper data from input.json instead of hardcoded values
// If tax lines exist, we try to add them to line items in draft order
func buildDraftOrderFromInput(inputData *InputData) (app.DraftOrderInput, error) {
	draftInput := app.DraftOrderInput{
		Email: inputData.Order.Email,
		Note:  inputData.Order.Note,
		Tags:  parseTags(inputData.Order.Tags),
	}

	// Add order-level discount from input.json if available
	if len(inputData.Order.DiscountApplications) > 0 {
		// Use first discount application for order-level discount
		discount := inputData.Order.DiscountApplications[0]
		value, _ := strconv.ParseFloat(discount.Value, 64)
		draftInput.AppliedDiscount = &app.AppliedDiscountInput{
			Title:       discount.Title,
			Description: discount.Title,
		}
		if discount.ValueType == "percentage" {
			draftInput.AppliedDiscount.ValueType = "PERCENTAGE"
			// Round to 2 decimal places
			draftInput.AppliedDiscount.Value = math.Round(value*100) / 100
		} else {
			draftInput.AppliedDiscount.ValueType = "FIXED_AMOUNT"
			if amount, ok := parsePrice(discount.Amount); ok {
				// Round to 2 decimal places
				draftInput.AppliedDiscount.Value = math.Round(amount*100) / 100
			}
		}
	} else if inputData.Order.TotalDiscounts != "" {
		// If no discount applications but totalDiscounts exists, calculate percentage
		// This is a fallback - ideally discountApplications should be provided
		if totalDiscount, ok := parsePrice(inputData.Order.TotalDiscounts); ok {
			if subtotal, ok := parsePrice(inputData.Order.SubtotalPrice); ok && subtotal > 0 {
				percentage := (totalDiscount / subtotal) * 100
				// Round to 2 decimal places
				draftInput.AppliedDiscount = &app.AppliedDiscountInput{
					ValueType:   "PERCENTAGE",
					Value:       math.Round(percentage*100) / 100,
					Title:       "Order Discount",
					Description: fmt.Sprintf("Discount: %s", inputData.Order.TotalDiscounts),
				}
			} else {
				// Use fixed amount
				draftInput.AppliedDiscount = &app.AppliedDiscountInput{
					ValueType:   "FIXED_AMOUNT",
					Value:       math.Round(totalDiscount*100) / 100,
					Title:       "Order Discount",
					Description: fmt.Sprintf("Discount: %s", inputData.Order.TotalDiscounts),
				}
			}
		}
	}

	// Note: Tax lines cannot be added to DraftOrderInput directly
	// They will be handled after draft order is completed

	// Map line items from input.json
	for _, item := range inputData.Order.Items {
		lineItem := app.DraftLineItemInput{
			VariantID: toVariantGID(item.ProductID),
			Quantity:  item.Quantity,
			Taxable:   item.Taxable,
		}

		// Set originalUnitPrice if available (use price or originPrice)
		// This allows custom pricing for the line item
		if price, ok := parsePrice(item.Price); ok {
			lineItem.OriginalUnitPrice = price
		} else if item.OriginPrice != "" {
			if originPrice, ok := parsePrice(item.OriginPrice); ok {
				lineItem.OriginalUnitPrice = originPrice
			}
		}

		// Add discount from input.json for this line item
		if len(item.DiscountApplications) > 0 {
			discount := item.DiscountApplications[0]
			value, _ := strconv.ParseFloat(discount.Value, 64)
			lineItem.AppliedDiscount = &app.AppliedDiscountInput{
				Title:       discount.Title,
				Description: discount.Title,
			}
			if discount.ValueType == "percentage" {
				lineItem.AppliedDiscount.ValueType = "PERCENTAGE"
				// Round to 2 decimal places
				lineItem.AppliedDiscount.Value = math.Round(value*100) / 100
			} else {
				lineItem.AppliedDiscount.ValueType = "FIXED_AMOUNT"
				if amount, ok := parsePrice(discount.Amount); ok {
					// Round to 2 decimal places
					lineItem.AppliedDiscount.Value = math.Round(amount*100) / 100
				}
			}
		} else if item.TotalDiscount != "" {
			// Fallback: use totalDiscount if discountApplications not available
			if totalDiscount, ok := parsePrice(item.TotalDiscount); ok {
				if price, ok := parsePrice(item.Price); ok && price > 0 {
					percentage := (totalDiscount / price) * 100
					lineItem.AppliedDiscount = &app.AppliedDiscountInput{
						ValueType:   "PERCENTAGE",
						Value:       percentage,
						Title:       "Item Discount",
						Description: fmt.Sprintf("Discount: %s", item.TotalDiscount),
		}
	} else {
					lineItem.AppliedDiscount = &app.AppliedDiscountInput{
						ValueType:   "FIXED_AMOUNT",
						Value:       totalDiscount,
						Title:       "Item Discount",
						Description: fmt.Sprintf("Discount: %s", item.TotalDiscount),
					}
				}
			}
		}

		// Note: DraftOrderLineItemInput does NOT support taxLines field
		// Tax will be handled after draft order is completed
		// We'll remove taxLines from lineItem to avoid GraphQL errors

		draftInput.LineItems = append(draftInput.LineItems, lineItem)
	}

	// Map shipping address
	if inputData.Order.ShippingAddress != nil {
		draftInput.ShippingAddress = convertAddress(inputData.Order.ShippingAddress)
	}

	// Map shipping line (shipping cost)
	// If totalShippingIncTax is provided, use it (includes tax)
	// Otherwise, use totalShippingExTax or totalShipping
	// Note: ShippingLineInput only supports 'title' and 'price' fields
	// Shipping tax will be calculated automatically by Shopify based on shipping address and tax settings
	// To specify shipping tax explicitly, add it as a separate tax line after order completion
	if inputData.Order.TotalShippingIncTax != "" {
		if shippingPrice, ok := parsePrice(inputData.Order.TotalShippingIncTax); ok && shippingPrice > 0 {
			shippingTitle := inputData.Order.ShippingMethod
			if shippingTitle == "" {
				shippingTitle = "Shipping"
			}
			draftInput.ShippingLine = &app.ShippingLineInput{
				Title: shippingTitle,
				Price: math.Round(shippingPrice*100) / 100,
			}
			fmt.Printf("Debug: Setting shipping line with totalShippingIncTax: %s (title: %s, price: %.2f)\n",
				inputData.Order.TotalShippingIncTax, shippingTitle, shippingPrice)
		}
	} else if inputData.Order.TotalShippingExTax != "" {
		if shippingPrice, ok := parsePrice(inputData.Order.TotalShippingExTax); ok && shippingPrice > 0 {
			shippingTitle := inputData.Order.ShippingMethod
			if shippingTitle == "" {
				shippingTitle = "Shipping"
			}
			draftInput.ShippingLine = &app.ShippingLineInput{
				Title: shippingTitle,
				Price: math.Round(shippingPrice*100) / 100,
			}
			fmt.Printf("Debug: Setting shipping line with totalShippingExTax: %s (title: %s, price: %.2f)\n",
				inputData.Order.TotalShippingExTax, shippingTitle, shippingPrice)
		}
	} else if inputData.Order.TotalShipping != "" {
		if shippingPrice, ok := parsePrice(inputData.Order.TotalShipping); ok && shippingPrice > 0 {
			shippingTitle := inputData.Order.ShippingMethod
			if shippingTitle == "" {
				shippingTitle = "Shipping"
			}
			draftInput.ShippingLine = &app.ShippingLineInput{
				Title: shippingTitle,
				Price: math.Round(shippingPrice*100) / 100,
			}
			fmt.Printf("Debug: Setting shipping line with totalShipping: %s (title: %s, price: %.2f)\n",
				inputData.Order.TotalShipping, shippingTitle, shippingPrice)
		}
	}

	// Map billing address
	if inputData.Order.BillingAddress != nil {
		draftInput.BillingAddress = convertAddress(inputData.Order.BillingAddress)
	}

	// Map customer information
	// Note: DraftOrderInput doesn't have customer field, only email
	// Customer info will be set via email field
	if inputData.Order.Customer != nil && inputData.Order.Customer.Email != "" {
		draftInput.Email = inputData.Order.Customer.Email
	} else if inputData.Order.Email != "" {
		draftInput.Email = inputData.Order.Email
	}

	// Map note attributes (custom attributes)
	if len(inputData.Order.NoteAttributes) > 0 {
		draftInput.CustomAttributes = make([]app.AttributeInput, len(inputData.Order.NoteAttributes))
		for i, attr := range inputData.Order.NoteAttributes {
			draftInput.CustomAttributes[i] = app.AttributeInput{
				Key:   attr.Name,
				Value: attr.Value,
			}
		}
	}

	// Note: Metafields cannot be added to draft orders directly
	// Shipping note will be added to order after draft order is completed

	// Set tax exempt - if no tax lines and total tax is 0, consider tax exempt
	// But we'll set TaxExempt to false to allow Shopify to calculate tax if configured
	draftInput.TaxExempt = false

	return draftInput, nil
}

// parsePrice parses a price string to float64
func parsePrice(priceStr string) (float64, bool) {
	if priceStr == "" {
		return 0, false
	}
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		return 0, false
	}
	return price, true
}

// getShippingNote picks shipping note from additionalData.shipping_note first, then falls back to shippingNote
func getShippingNote(order OrderData) string {
	if order.AdditionalData != nil {
		if note := strings.TrimSpace(order.AdditionalData.ShippingNote); note != "" {
			return note
		}
	}
	if note := strings.TrimSpace(order.ShippingNote); note != "" {
		return note
	}
	return ""
}

// addShippingNoteMetafield adds shipping note metafield to an order using GraphQL
func addShippingNoteMetafield(orderID string, shippingNote string) error {
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
		"namespace":    "connectpos",
		"key":          "shipping_note",
		"type":         "multi_line_text_field",
		"value":        shippingNote,
		"ownerId":      orderID,
	}

	variables := map[string]interface{}{
		"metafields": []interface{}{metafieldInput},
	}

	resp, err := app.CallAdminGraphQL(mutation, variables)
	if err != nil {
		return fmt.Errorf("failed to call GraphQL: %w", err)
	}

	// Check for GraphQL errors
	if errors, ok := resp["errors"].([]interface{}); ok && len(errors) > 0 {
		return fmt.Errorf("GraphQL errors: %v", errors)
	}

	// Check for user errors
	if data, ok := resp["data"].(map[string]interface{}); ok {
		if setResult, ok := data["metafieldsSet"].(map[string]interface{}); ok {
			if userErrors, ok := setResult["userErrors"].([]interface{}); ok && len(userErrors) > 0 {
				return fmt.Errorf("user errors: %v", userErrors)
			}
			// Log success with metafield details
			if metafields, ok := setResult["metafields"].([]interface{}); ok && len(metafields) > 0 {
				if mf, ok := metafields[0].(map[string]interface{}); ok {
					log.Printf("Debug: Metafield created - ID: %v, Key: %v, Value: %v\n", 
						mf["id"], mf["key"], mf["value"])
				}
			}
			return nil
		}
	}

	return fmt.Errorf("unexpected response format")
}

// addShippingTaxToOrderNote adds shipping tax information to order note
// This ensures shipping tax is visible in admin even if Shopify merges tax lines
func addShippingTaxToOrderNote(orderID string, shippingTaxLine *app.TaxLineInput) error {
	// First, get current order note
	const getQuery = `
		query GetOrder($id: ID!) {
			order(id: $id) {
				id
				note
			}
		}`

	variables := map[string]interface{}{
		"id": orderID,
	}

	resp, err := app.CallAdminGraphQL(getQuery, variables)
	if err != nil {
		return fmt.Errorf("failed to get order: %w", err)
	}

	currentNote := ""
	if data, ok := resp["data"].(map[string]interface{}); ok {
		if order, ok := data["order"].(map[string]interface{}); ok {
			if note, ok := order["note"].(string); ok {
				currentNote = note
			}
		}
	}

	// Append shipping tax info to note
	shippingTaxNote := fmt.Sprintf("\n\n--- Shipping Tax ---\n%s: %s (Rate: %.2f%%)",
		shippingTaxLine.Title,
		shippingTaxLine.PriceSet.ShopMoney.Amount,
		shippingTaxLine.Rate*100)

	newNote := currentNote + shippingTaxNote

	// Update order note
	const updateMutation = `
		mutation UpdateOrderNote($id: ID!, $note: String!) {
			orderUpdate(input: { id: $id, note: $note }) {
				order {
					id
					note
				}
				userErrors {
					field
					message
				}
			}
		}`

	updateVars := map[string]interface{}{
		"id":   orderID,
		"note": newNote,
	}

	updateResp, err := app.CallAdminGraphQL(updateMutation, updateVars)
	if err != nil {
		return fmt.Errorf("failed to update order note: %w", err)
	}

	// Check for errors
	if errors, ok := updateResp["errors"].([]interface{}); ok && len(errors) > 0 {
		return fmt.Errorf("GraphQL errors: %v", errors)
	}

	if data, ok := updateResp["data"].(map[string]interface{}); ok {
		if orderUpdate, ok := data["orderUpdate"].(map[string]interface{}); ok {
			if userErrors, ok := orderUpdate["userErrors"].([]interface{}); ok && len(userErrors) > 0 {
				return fmt.Errorf("user errors: %v", userErrors)
			}
			fmt.Printf("✓ Shipping tax info added to order note\n")
			return nil
		}
	}

	return fmt.Errorf("unexpected response format")
}