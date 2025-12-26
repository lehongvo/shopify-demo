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
	ShippingNote        string                   `json:"shippingNote,omitempty"`
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

	inputPath := "cmd/test_draft_order_tax/input.json"
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

	// Step 1: Create draft order using DraftOrderCreate mutation
	fmt.Println("Creating draft order...")
	draftResp, err := app.CreateDraftOrder(draftInput)
	if err != nil {
		log.Fatalf("Failed to create draft order: %v", err)
	}

	draftID := draftResp.Data.DraftOrderCreate.DraftOrder.ID
	draftName := draftResp.Data.DraftOrderCreate.DraftOrder.Name
	fmt.Printf("✓ Draft order created: %s (ID: %s)\n", draftName, draftID)

	// Step 2: Complete draft order using DraftOrderComplete mutation
	fmt.Println("Completing draft order...")
	paymentPending := false // Set to true if payment is pending
	orderInfo, err := app.CompleteDraftOrder(draftID, paymentPending)
	if err != nil {
		log.Fatalf("Failed to complete draft order: %v", err)
	}

	fmt.Printf("✓ Order created successfully: %s (ID: %s)\n", orderInfo.OrderName, orderInfo.OrderID)

	// Step 3: Add tax lines to order if available
	if len(inputData.Order.TaxLines) > 0 && orderInfo.OrderID != "" {
		fmt.Println("Adding tax lines to order...")
		taxLines := make([]app.TaxLineInput, len(inputData.Order.TaxLines))
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

		if err := app.AddTaxToOrder(orderInfo.OrderID, taxLines); err != nil {
			log.Printf("Warning: Failed to add tax to order: %v\n", err)
		} else {
			fmt.Printf("✓ Tax lines added successfully\n")
		}
	}

	// Step 4: Query and display order details
	if orderInfo.OrderID != "" {
		queryOrderDetails(orderInfo.OrderID)
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
func buildDraftOrderFromInput(inputData *InputData) (app.DraftOrderInput, error) {
	draftInput := app.DraftOrderInput{
		Email: inputData.Order.Email,
		Note:  inputData.Order.Note,
		Tags:  parseTags(inputData.Order.Tags),
	}

	// Add order-level discount from input.json if available
	if len(inputData.Order.DiscountApplications) > 0 {
		discount := inputData.Order.DiscountApplications[0]
		value, _ := strconv.ParseFloat(discount.Value, 64)
		draftInput.AppliedDiscount = &app.AppliedDiscountInput{
			Title:       discount.Title,
			Description: discount.Title,
		}
		if discount.ValueType == "percentage" {
			draftInput.AppliedDiscount.ValueType = "PERCENTAGE"
			draftInput.AppliedDiscount.Value = math.Round(value*100) / 100
		} else {
			draftInput.AppliedDiscount.ValueType = "FIXED_AMOUNT"
			if amount, ok := parsePrice(discount.Amount); ok {
				draftInput.AppliedDiscount.Value = math.Round(amount*100) / 100
			}
		}
	} else if inputData.Order.TotalDiscounts != "" {
		if totalDiscount, ok := parsePrice(inputData.Order.TotalDiscounts); ok {
			if subtotal, ok := parsePrice(inputData.Order.SubtotalPrice); ok && subtotal > 0 {
				percentage := (totalDiscount / subtotal) * 100
				draftInput.AppliedDiscount = &app.AppliedDiscountInput{
					ValueType:   "PERCENTAGE",
					Value:       math.Round(percentage*100) / 100,
					Title:       "Order Discount",
					Description: fmt.Sprintf("Discount: %s", inputData.Order.TotalDiscounts),
				}
			} else {
				draftInput.AppliedDiscount = &app.AppliedDiscountInput{
					ValueType:   "FIXED_AMOUNT",
					Value:       math.Round(totalDiscount*100) / 100,
					Title:       "Order Discount",
					Description: fmt.Sprintf("Discount: %s", inputData.Order.TotalDiscounts),
				}
			}
		}
	}

	// Map line items from input.json
	for _, item := range inputData.Order.Items {
		lineItem := app.DraftLineItemInput{
			VariantID: toVariantGID(item.ProductID),
			Quantity:  item.Quantity,
			Taxable:   item.Taxable,
		}

		// Set originalUnitPrice if available
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
				lineItem.AppliedDiscount.Value = math.Round(value*100) / 100
			} else {
				lineItem.AppliedDiscount.ValueType = "FIXED_AMOUNT"
				if amount, ok := parsePrice(discount.Amount); ok {
					lineItem.AppliedDiscount.Value = math.Round(amount*100) / 100
				}
			}
		} else if item.TotalDiscount != "" {
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

	// Set tax exempt
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
		log.Printf("Warning: Failed to query order details: %v\n", err)
		return
	}

	if data, ok := resp["data"].(map[string]interface{}); ok {
		if order, ok := data["order"].(map[string]interface{}); ok {
			fmt.Println("\n=== Order Details ===")
			if name, ok := order["name"].(string); ok {
				fmt.Printf("Order Name: %s\n", name)
			}
			if email, ok := order["email"].(string); ok {
				fmt.Printf("Email: %s\n", email)
			}
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
			if taxLines, ok := order["taxLines"].([]interface{}); ok && len(taxLines) > 0 {
				fmt.Println("\nTax Lines:")
				for i, tl := range taxLines {
					if taxLine, ok := tl.(map[string]interface{}); ok {
						title := ""
						rate := ""
						amount := ""
						if titleVal, ok := taxLine["title"].(string); ok {
							title = titleVal
						}
						if rateVal, ok := taxLine["rate"].(float64); ok {
							rate = fmt.Sprintf("%.4f", rateVal)
						} else if rateStr, ok := taxLine["rate"].(string); ok {
							rate = rateStr
						}
						if priceSet, ok := taxLine["priceSet"].(map[string]interface{}); ok {
							if shopMoney, ok := priceSet["shopMoney"].(map[string]interface{}); ok {
								if amountVal, ok := shopMoney["amount"].(string); ok {
									amount = amountVal
								}
							}
						}
						fmt.Printf("  %d. %s - Rate: %s, Amount: %s\n", i+1, title, rate, amount)
					}
				}
			}
		}
	}
}
