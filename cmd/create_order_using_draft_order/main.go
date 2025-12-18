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
	var taxLines []app.TaxLineInput
	if len(inputData.Order.TaxLines) > 0 {
		taxLines = make([]app.TaxLineInput, len(inputData.Order.TaxLines))
		for i, tl := range inputData.Order.TaxLines {
			rate, _ := strconv.ParseFloat(tl.Rate, 64)
			taxLines[i] = app.TaxLineInput{
				Title: tl.Title,
				Rate:  rate,
				PriceSet: &app.MoneyBagInput{
					ShopMoney: &app.MoneyInput{
						Amount:       tl.Price,
						CurrencyCode: "USD",
					},
				},
			}
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
		}
	}

	// Step 7: Add tax lines to order if provided (after completion)
	if len(taxLines) > 0 && orderInfo.OrderID != "" {
		if err := app.AddTaxToOrder(orderInfo.OrderID, taxLines); err != nil {
			log.Printf("Warning: Failed to add tax to order: %v\n", err)
		}
	}

	// Print order details
	fmt.Printf("✓ Order created successfully: %s (%s)\n", orderInfo.OrderName, orderInfo.OrderID)

	// Query order details to show tax information
	if orderInfo.OrderID != "" {
		queryOrderDetails(orderInfo.OrderID)
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
		mutation CreateMetafield($metafield: MetafieldInput!) {
			metafieldCreate(metafield: $metafield) {
				metafield {
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
		"metafield": metafieldInput,
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
			if createResult, ok := data["metafieldCreate"].(map[string]interface{}); ok {
				if userErrors, ok := createResult["userErrors"].([]interface{}); ok && len(userErrors) > 0 {
					return fmt.Errorf("user errors: %v", userErrors)
				}
				return nil
			}
		}

	return fmt.Errorf("unexpected response format")
}