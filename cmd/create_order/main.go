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

	inputPath := "cmd/create_order/input.json"
	if len(os.Args) > 1 {
		inputPath = os.Args[1]
	}

	inputData, err := loadInputData(inputPath)
	if err != nil {
		log.Fatalf("Failed to load input data: %v", err)
	}

	// Convert input.json data to OrderInput for CreateOrderGraphQL
	orderInput := buildOrderInputForGraphQL(inputData)

	// Create order using GraphQL orderCreate
	response, err := app.CreateOrderGraphQL(orderInput)
	if err != nil {
		log.Fatalf("Failed to create order: %v", err)
	}

	// Print order details
	order := response.Data.OrderCreate.Order
	fmt.Println("✓ Order created successfully!")
	fmt.Printf("Order ID: %s\n", order.ID)
	fmt.Printf("Order Name: %s\n", order.Name)
	fmt.Printf("Order Number: %d\n", order.OrderNumber)
	fmt.Printf("Email: %s\n", order.Email)
	fmt.Printf("Total Price: %s %s\n",
		order.TotalPriceSet.ShopMoney.Amount,
		order.TotalPriceSet.ShopMoney.CurrencyCode)
	if order.TotalTaxSet.ShopMoney.Amount != "" {
		fmt.Printf("Total Tax: %s %s\n",
			order.TotalTaxSet.ShopMoney.Amount,
			order.TotalTaxSet.ShopMoney.CurrencyCode)
	}
	if len(order.TaxLines) > 0 {
		for i, tl := range order.TaxLines {
			rateStr := ""
			if rateFloat, ok := tl.Rate.(float64); ok {
				rateStr = fmt.Sprintf("%.3f", rateFloat)
			} else if rateStrVal, ok := tl.Rate.(string); ok {
				rateStr = rateStrVal
			} else {
				rateStr = fmt.Sprintf("%v", tl.Rate)
			}
			fmt.Printf("Tax Line %d: %s (rate: %s, amount: %s)\n",
				i+1, tl.Title, rateStr, tl.PriceSet.ShopMoney.Amount)
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

// buildOrderInputFromInput converts input.json OrderData to OrderInput for CreateOrderWithTax
func buildOrderInputFromInput(inputData *InputData) app.OrderInput {
	orderInput := app.OrderInput{
		Email:           inputData.Order.Email,
		Note:            inputData.Order.Note,
		FinancialStatus: "PAID", // Default to PAID, can be changed if needed
		Tags:            parseTags(inputData.Order.Tags),
	}

	// Convert line items
	for _, item := range inputData.Order.Items {
		lineItem := app.LineItemInput{
			VariantID: toVariantGID(item.ProductID),
			Quantity:  item.Quantity,
		}
		
		// Keep original price (don't apply discount to price)
		// Instead, use discount_allocations to show discount separately
		originalPrice, _ := parsePrice(item.Price)
		lineItem.Price = fmt.Sprintf("%.2f", originalPrice)
		
		// Add discount allocations if discount exists
		if len(item.DiscountApplications) > 0 {
			discount := item.DiscountApplications[0]
			discountAmount := discount.Amount
			
			// If amount is not provided, calculate it
			if discountAmount == "" {
				if discount.ValueType == "percentage" {
					value, _ := strconv.ParseFloat(discount.Value, 64)
					discountAmount = fmt.Sprintf("%.2f", originalPrice*value/100)
				} else if discount.ValueType == "fixed_amount" {
					discountAmount = discount.Value
				}
			}
			
			if discountAmount != "" {
				lineItem.DiscountAllocations = []app.DiscountAllocationInput{
					{
						Amount: discountAmount,
						Title:  discount.Title,
					},
				}
			}
		} else if item.TotalDiscount != "" {
			// Fallback: use totalDiscount
			lineItem.DiscountAllocations = []app.DiscountAllocationInput{
				{
					Amount: item.TotalDiscount,
					Title:  "Item Discount",
				},
			}
		}
		
		if item.Name != "" {
			lineItem.Title = item.Name
		}
		
		orderInput.LineItems = append(orderInput.LineItems, lineItem)
	}

	// Convert shipping address
	if inputData.Order.ShippingAddress != nil {
		orderInput.ShippingAddress = convertAddress(inputData.Order.ShippingAddress)
	}

	// Convert billing address
	if inputData.Order.BillingAddress != nil {
		orderInput.BillingAddress = convertAddress(inputData.Order.BillingAddress)
	}

	// Convert tax lines from input.json
	if len(inputData.Order.TaxLines) > 0 {
		taxLines := make([]app.OrderCreateTaxLineInput, len(inputData.Order.TaxLines))
		for i, tl := range inputData.Order.TaxLines {
			taxLines[i] = app.OrderCreateTaxLineInput{
				Title: tl.Title,
				Rate:  tl.Rate, // Keep as string for OrderCreateTaxLineInput
				PriceSet: &app.MoneyBagInput{
					ShopMoney: &app.MoneyInput{
						Amount:       tl.Price,
						CurrencyCode: "USD", // Default to USD, can be made dynamic
					},
				},
			}
		}
		orderInput.TaxLines = taxLines
	}

	return orderInput
}

// buildOrderInputForGraphQL converts input.json OrderData to OrderInput for CreateOrderGraphQL
// According to Shopify support:
// - Custom tax lines: supported via taxLines field
// - Order-level discount: via discountCode field
// - Line-item discount: calculate discounted price and set in priceSet, add note in properties
func buildOrderInputForGraphQL(inputData *InputData) app.OrderInput {
	orderInput := app.OrderInput{
		Email:           inputData.Order.Email,
		Note:            inputData.Order.Note,
		FinancialStatus: "PAID", // Default to PAID
		Tags:            parseTags(inputData.Order.Tags),
	}

	// Convert line items with discount handling
	// For line-item discounts: calculate price after discount and set in priceSet, add note in properties
	for _, item := range inputData.Order.Items {
		lineItem := app.LineItemInput{
			VariantID: toVariantGID(item.ProductID),
			Quantity:  item.Quantity,
		}

		// Get original price
		originalPrice, _ := parsePrice(item.Price)
		if originalPrice == 0 && item.OriginPrice != "" {
			originalPrice, _ = parsePrice(item.OriginPrice)
		}

		// Calculate discounted price if discount exists
		// Apply discounts sequentially: each discount uses the already-discounted price
		discountedPrice := originalPrice
		var discountNotes []string
		
		if len(item.DiscountApplications) > 0 {
			currentPrice := originalPrice
			for _, discount := range item.DiscountApplications {
				// Skip discount if title contains "Original Price" or HTML tags (these are metadata, not real discounts)
				title := strings.TrimSpace(discount.Title)
				if strings.Contains(title, "Original Price") || strings.Contains(title, "<s>") || strings.Contains(title, "</s>") {
					continue // Skip this discount application
				}
				
				var discountAmount float64
				if discount.ValueType == "percentage" {
					value, _ := strconv.ParseFloat(discount.Value, 64)
					// Calculate discount from current price (sequential)
					discountAmount = currentPrice * value / 100
					// Professional format: "• X% off (Title)"
					discountNotes = append(discountNotes, fmt.Sprintf("• %s%% off (%s)", discount.Value, discount.Title))
				} else if discount.ValueType == "fixed_amount" {
					discountAmount, _ = parsePrice(discount.Amount)
					// Professional format: "• $X.XX (Title)"
					discountNotes = append(discountNotes, fmt.Sprintf("• $%s (%s)", discount.Amount, discount.Title))
				}
				// Apply discount to current price
				currentPrice = currentPrice - discountAmount
			}
			discountedPrice = currentPrice
		} else if item.TotalDiscount != "" {
			discountAmount, _ := parsePrice(item.TotalDiscount)
			discountedPrice = originalPrice - discountAmount
			discountNotes = append(discountNotes, fmt.Sprintf("$%s", item.TotalDiscount))
		}
		
		// Build 2 properties only:
		// 1) Original Price: shows the original price (strikethrough)
		// 2) Line item discount: shows the list of discounts
		originalPriceValue := ""
		lineItemDiscountValue := ""
		if len(discountNotes) > 0 {
			// Use ONLY U+0336 per character (most compatible in Shopify Admin UI)
			originalPriceStr := fmt.Sprintf("$%.2f", originalPrice)
			originalPriceValue = applyUnicodeStrikethrough(originalPriceStr)

			// Format: each discount on its own line for easier reading in Shopify Admin UI
			lineItemDiscountValue = strings.Join(discountNotes, "\n")
		}

		// Set priceSet with discounted price (as recommended by Shopify)
		lineItem.PriceSet = &app.MoneyBagInput{
			ShopMoney: &app.MoneyInput{
				Amount:       fmt.Sprintf("%.2f", discountedPrice),
				CurrencyCode: "USD",
			},
		}

		// Add properties
		if originalPriceValue != "" || lineItemDiscountValue != "" {
			var props []app.LineItemPropertyInput
			if originalPriceValue != "" {
				props = append(props, app.LineItemPropertyInput{
					Name:  "Original Price",
					Value: originalPriceValue,
				})
			}
			if lineItemDiscountValue != "" {
				props = append(props, app.LineItemPropertyInput{
					Name:  "Line item discount",
					Value: lineItemDiscountValue,
				})
			}
			lineItem.Properties = props
		}

		if item.Name != "" {
			lineItem.Title = item.Name
		}

		orderInput.LineItems = append(orderInput.LineItems, lineItem)
	}

	// Convert shipping address
	if inputData.Order.ShippingAddress != nil {
		orderInput.ShippingAddress = convertAddress(inputData.Order.ShippingAddress)
	}

	// Convert billing address
	if inputData.Order.BillingAddress != nil {
		orderInput.BillingAddress = convertAddress(inputData.Order.BillingAddress)
	}

	// Convert tax lines from input.json (custom tax lines are supported in orderCreate)
	if len(inputData.Order.TaxLines) > 0 {
		taxLines := make([]app.OrderCreateTaxLineInput, len(inputData.Order.TaxLines))
		for i, tl := range inputData.Order.TaxLines {
			taxLines[i] = app.OrderCreateTaxLineInput{
				Title: tl.Title,
				Rate:  tl.Rate, // Keep as string for OrderCreateTaxLineInput
				PriceSet: &app.MoneyBagInput{
					ShopMoney: &app.MoneyInput{
						Amount:       tl.Price,
						CurrencyCode: "USD",
					},
				},
			}
		}
		orderInput.TaxLines = taxLines
	}

	// Convert order-level discount via discountCode (as recommended by Shopify)
	if len(inputData.Order.DiscountApplications) > 0 {
		discount := inputData.Order.DiscountApplications[0]
		if discount.ValueType == "percentage" {
			value, _ := strconv.ParseFloat(discount.Value, 64)
			orderInput.DiscountCode = &app.OrderCreateDiscountCodeInput{
				ItemPercentageDiscountCode: &app.ItemPercentageDiscountCodeInput{
					Code:       discount.Title,
					Percentage: value,
				},
			}
		} else if discount.ValueType == "fixed_amount" {
			amount, _ := parsePrice(discount.Amount)
			orderInput.DiscountCode = &app.OrderCreateDiscountCodeInput{
				ItemFixedDiscountCode: &app.ItemFixedDiscountCodeInput{
					Code: discount.Title,
					AmountSet: &app.MoneyBagInput{
						ShopMoney: &app.MoneyInput{
							Amount:       fmt.Sprintf("%.2f", amount),
							CurrencyCode: "USD",
						},
					},
				},
			}
		}
	} else if inputData.Order.TotalDiscounts != "" {
		// Fallback: use totalDiscounts
		totalDiscount, _ := parsePrice(inputData.Order.TotalDiscounts)
		orderInput.DiscountCode = &app.OrderCreateDiscountCodeInput{
			ItemFixedDiscountCode: &app.ItemFixedDiscountCodeInput{
				Code: "Order Discount",
				AmountSet: &app.MoneyBagInput{
					ShopMoney: &app.MoneyInput{
						Amount:       fmt.Sprintf("%.2f", totalDiscount),
						CurrencyCode: "USD",
					},
				},
			},
		}
	}

	return orderInput
}

func applyUnicodeStrikethrough(text string) string {
	var result strings.Builder
	for _, r := range text {
		result.WriteRune(r)
		result.WriteRune(0x0336)
	}
	return result.String()
}