package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

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
	PriceExTax           string                   `json:"priceExTax,omitempty"`
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

	// =================================================================
	// HYBRID STRATEGY: orderCreate + orderEdit for strikethrough + tax
	// =================================================================
	// Step 1: orderCreate with ORIGINAL PRICE (no discount) + custom tax lines
	// Step 2: orderEditBegin → orderEditAddLineItemDiscount → orderEditCommit
	//         This adds discounts AFTER order creation, showing strikethrough!
	// =================================================================
	
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  HYBRID STRATEGY: orderCreate + Order Edit                    ║")
	fmt.Println("╠═══════════════════════════════════════════════════════════════╣")
	fmt.Println("║  ✓ Step 1: orderCreate - original price + custom tax lines    ║")
	fmt.Println("║  ✓ Step 2: Order Edit API - add discounts (strikethrough!)    ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Step 1: Build order input with ORIGINAL PRICE (no discount applied yet)
	fmt.Println("Step 1: Creating order with ORIGINAL PRICE + custom tax lines...")
	orderInput, err := buildOrderInputWithOriginalPrice(inputData)
	if err != nil {
		log.Fatalf("Failed to build order input: %v", err)
	}

	// Create order using orderCreate
	orderResp, err := app.CreateOrderGraphQL(orderInput)
	if err != nil {
		log.Fatalf("Failed to create order: %v", err)
	}

	orderID := orderResp.Data.OrderCreate.Order.ID
	orderName := orderResp.Data.OrderCreate.Order.Name
	fmt.Printf("✓ Order created: %s (ID: %s)\n", orderName, orderID)
	fmt.Printf("  → Original price (before discount)\n")
	fmt.Printf("  → Custom tax lines included\n\n")

	// Step 2: Use Order Edit API to add discounts (this shows strikethrough!)
	fmt.Println("Step 2: Adding discounts via Order Edit API...")
	fmt.Println("  (This will show strikethrough pricing in Shopify Admin!)")
	
	// Start order edit session
	editResp, err := app.OrderEditBegin(orderID)
	if err != nil {
		log.Printf("⚠ Warning: Could not begin order edit: %v\n", err)
		fmt.Println("  Order created but discounts could not be added via Order Edit")
	} else {
		fmt.Printf("  ✓ Order edit session started: %s\n", editResp.CalculatedOrderID)
		
		// Add line item discounts
		for i, lineItem := range editResp.LineItems {
			if i < len(inputData.Order.Items) {
				item := inputData.Order.Items[i]
				
				// Check for line item discount
				if len(item.DiscountApplications) > 0 {
					discount := item.DiscountApplications[0]
					
					discountInput := app.OrderEditAddLineItemDiscountInput{
						CalculatedOrderID: editResp.CalculatedOrderID,
						LineItemID:        lineItem.ID,
						DiscountTitle:     discount.Title,
					}
					
					if discount.ValueType == "percentage" {
						value, _ := strconv.ParseFloat(discount.Value, 64)
						discountInput.IsPercentage = true
						discountInput.PercentValue = value
						fmt.Printf("  Adding discount: %s (%.2f%%) to %s\n", discount.Title, value, lineItem.Title)
					} else {
						amount, _ := parsePrice(discount.Amount)
						discountInput.IsPercentage = false
						discountInput.FixedValue = amount
						fmt.Printf("  Adding discount: %s ($%.2f) to %s\n", discount.Title, amount, lineItem.Title)
					}
					
					if err := app.OrderEditAddLineItemDiscount(discountInput); err != nil {
						log.Printf("  ⚠ Warning: Could not add discount: %v\n", err)
					}
				}
			}
		}
		
		// Commit order edit
		fmt.Println("  Committing order edit...")
		if err := app.OrderEditCommit(editResp.CalculatedOrderID, false); err != nil {
			log.Printf("  ⚠ Warning: Could not commit order edit: %v\n", err)
		} else {
			fmt.Println("  ✓ Discounts applied with strikethrough!")
		}
	}

	// Step 3: Restore custom tax lines (Order Edit recalculates tax and overwrites our custom tax lines)
	fmt.Println("\nStep 3: Restoring custom tax lines...")
	fmt.Println("  (Order Edit recalculates tax, so we restore custom tax lines via REST API)")
	
	// Extract numeric order ID
	orderNum := strings.TrimPrefix(orderID, "gid://shopify/Order/")
	
	// Convert TaxLineData to app.TaxLineRestInput
	var restTaxLines []app.TaxLineRestInput
	for _, tl := range inputData.Order.TaxLines {
		if !tl.IsUsed {
			continue
		}
		rate, _ := strconv.ParseFloat(tl.Rate, 64)
		price, _ := parsePrice(tl.Price)
		restTaxLines = append(restTaxLines, app.TaxLineRestInput{
			Title: tl.Title,
			Rate:  rate,
			Price: price,
		})
	}
	
	if err := app.UpdateOrderTaxLinesREST(orderNum, restTaxLines); err != nil {
		log.Printf("  ⚠ Warning: Could not restore custom tax lines: %v\n", err)
	} else {
		fmt.Println("  ✓ Custom tax lines restored!")
	}

	// Step 4: Query and display order details
	fmt.Println("\nStep 4: Verifying order details...")
	if orderID != "" {
		queryOrderDetails(orderID)
	}
}

// buildOrderInputWithOriginalPrice builds order input with ORIGINAL prices (before discount)
// Discounts will be added later via Order Edit API to show strikethrough
func buildOrderInputWithOriginalPrice(inputData *InputData) (app.OrderInput, error) {
	orderInput := app.OrderInput{
		Email: inputData.Order.Email,
		Note:  inputData.Order.Note,
		Tags:  parseTags(inputData.Order.Tags),
		FinancialStatus: "PAID",
	}

	// Map line items with ORIGINAL price (no discount)
	for _, item := range inputData.Order.Items {
		lineItem := app.LineItemInput{
			VariantID: toVariantGID(item.ProductID),
			Quantity:  item.Quantity,
		}

		// Set ORIGINAL price (before discount) using priceSet
		// The discount will be added via Order Edit API later
		if item.OriginPrice != "" {
			if originPrice, ok := parsePrice(item.OriginPrice); ok {
				lineItem.PriceSet = &app.MoneyBagInput{
					ShopMoney: &app.MoneyInput{
						Amount:       fmt.Sprintf("%.2f", originPrice),
						CurrencyCode: "USD",
					},
				}
				fmt.Printf("  → Line item: %s at original price $%.2f\n", item.Name, originPrice)
			}
		} else if price, ok := parsePrice(item.Price); ok {
			lineItem.PriceSet = &app.MoneyBagInput{
				ShopMoney: &app.MoneyInput{
					Amount:       fmt.Sprintf("%.2f", price),
					CurrencyCode: "USD",
				},
			}
			fmt.Printf("  → Line item at price $%.2f\n", price)
		}

		// Add tax lines to line item
		if len(inputData.Order.TaxLines) > 0 {
			// Calculate tax proportion for this line item
			var itemPriceExTax float64
			if priceExTax, ok := parsePrice(item.PriceExTax); ok {
				itemPriceExTax = priceExTax * float64(item.Quantity)
			} else if price, ok := parsePrice(item.Price); ok {
				if totalTax, ok := parsePrice(item.TotalTax); ok {
					itemPriceExTax = (price - totalTax) * float64(item.Quantity)
				} else {
					itemPriceExTax = price * float64(item.Quantity)
				}
			}

			// Calculate total price ex tax
			var totalPriceExTax float64
			for _, it := range inputData.Order.Items {
				if px, ok := parsePrice(it.PriceExTax); ok {
					totalPriceExTax += px * float64(it.Quantity)
				} else if p, ok := parsePrice(it.Price); ok {
					if tt, ok := parsePrice(it.TotalTax); ok {
						totalPriceExTax += (p - tt) * float64(it.Quantity)
					} else {
						totalPriceExTax += p * float64(it.Quantity)
					}
				}
			}

			// Distribute tax lines proportionally
			if totalPriceExTax > 0 {
				itemProportion := itemPriceExTax / totalPriceExTax
				for _, orderTaxLine := range inputData.Order.TaxLines {
					if !orderTaxLine.IsUsed {
						continue
					}
					orderTaxPrice, _ := parsePrice(orderTaxLine.Price)
					itemTaxAmount := orderTaxPrice * itemProportion
					taxRate, _ := strconv.ParseFloat(orderTaxLine.Rate, 64)

					lineItem.TaxLines = append(lineItem.TaxLines, app.OrderCreateTaxLineInput{
						Title: orderTaxLine.Title,
						Rate:  fmt.Sprintf("%.4f", taxRate),
						PriceSet: &app.MoneyBagInput{
							ShopMoney: &app.MoneyInput{
								Amount:       fmt.Sprintf("%.2f", math.Round(itemTaxAmount*100)/100),
								CurrencyCode: "USD",
							},
						},
					})
					fmt.Printf("  → Tax: %s (%.2f%%) = $%.2f\n", 
						orderTaxLine.Title, taxRate*100, math.Round(itemTaxAmount*100)/100)
				}
			}
		}

		orderInput.LineItems = append(orderInput.LineItems, lineItem)
	}

	// Map shipping address
	if inputData.Order.ShippingAddress != nil {
		orderInput.ShippingAddress = convertAddress(inputData.Order.ShippingAddress)
	}

	// Map billing address
	if inputData.Order.BillingAddress != nil {
		orderInput.BillingAddress = convertAddress(inputData.Order.BillingAddress)
	}

	return orderInput, nil
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

// buildOrderInputFromInput maps data from input.json to OrderInput for orderCreate mutation
// This supports tax lines, discounts, and compare at price directly in the mutation
func buildOrderInputFromInput(inputData *InputData) (app.OrderInput, error) {
	orderInput := app.OrderInput{
		Email: inputData.Order.Email,
		Note:  inputData.Order.Note,
		Tags:  parseTags(inputData.Order.Tags),
		FinancialStatus: "PAID", // Set to PAID since payment is complete
	}

	// Add order-level discount from input.json if available
	if len(inputData.Order.DiscountApplications) > 0 {
		discount := inputData.Order.DiscountApplications[0]
		value, _ := strconv.ParseFloat(discount.Value, 64)
		if discount.ValueType == "percentage" {
			// Use DiscountCode for percentage discount
			orderInput.DiscountCode = &app.OrderCreateDiscountCodeInput{
				ItemPercentageDiscountCode: &app.ItemPercentageDiscountCodeInput{
					Code:       discount.Title,
					Percentage: value,
				},
			}
			fmt.Printf("  → Adding order-level discount: %s (%.2f%%)\n", discount.Title, value)
		} else {
			// Use DiscountCode for fixed amount discount
			if amount, ok := parsePrice(discount.Amount); ok {
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
				fmt.Printf("  → Adding order-level discount: %s ($%.2f)\n", discount.Title, amount)
			}
		}
	} else if inputData.Order.TotalDiscounts != "" {
		if totalDiscount, ok := parsePrice(inputData.Order.TotalDiscounts); ok {
			if subtotal, ok := parsePrice(inputData.Order.SubtotalPrice); ok && subtotal > 0 {
				percentage := (totalDiscount / subtotal) * 100
				orderInput.DiscountCode = &app.OrderCreateDiscountCodeInput{
					ItemPercentageDiscountCode: &app.ItemPercentageDiscountCodeInput{
						Code:       "Order Discount",
						Percentage: math.Round(percentage*100) / 100,
					},
				}
				fmt.Printf("  → Adding order-level discount: Order Discount (%.2f%%)\n", percentage)
			} else {
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
				fmt.Printf("  → Adding order-level discount: Order Discount ($%.2f)\n", totalDiscount)
			}
		}
	}

	// Map line items from input.json
	for _, item := range inputData.Order.Items {
		lineItem := app.LineItemInput{
			VariantID: toVariantGID(item.ProductID),
			Quantity:  item.Quantity,
		}

		// Add property to store compare-at price (Shopify orderCreate doesn't support compare_at directly)
		if item.OriginPrice != "" {
			if originPrice, ok := parsePrice(item.OriginPrice); ok {
				lineItem.Properties = append(lineItem.Properties, app.LineItemPropertyInput{
					Name:  "compare_at_price",
					Value: fmt.Sprintf("%.2f", originPrice),
				})
				fmt.Printf("  → Storing compare at price property: $%.2f\n", originPrice)
			}
		}

		// IMPORTANT: Don't set priceSet here!
		// We've already updated variant price = discounted price
		// Shopify will use variant price automatically, and show strikethrough from compare_at_price
		// Setting priceSet here would override variant price and hide strikethrough
		
		// Note discount in properties for reference
		if len(item.DiscountApplications) > 0 {
			discount := item.DiscountApplications[0]
			if discountAmount, ok := parsePrice(item.TotalDiscount); ok {
				lineItem.Properties = append(lineItem.Properties, app.LineItemPropertyInput{
					Name:  "line_discount_amount",
					Value: fmt.Sprintf("%.2f", discountAmount),
				})
				fmt.Printf("  → Line item discount: %s ($%.2f) - will use variant price\n", 
					discount.Title, discountAmount)
			}
		} else if item.TotalDiscount != "" {
			if discountAmount, ok := parsePrice(item.TotalDiscount); ok {
				lineItem.Properties = append(lineItem.Properties, app.LineItemPropertyInput{
					Name:  "line_discount_amount",
					Value: fmt.Sprintf("%.2f", discountAmount),
				})
			}
		}

		// Add tax lines to line item if available
		// Calculate tax amount per line item based on order tax lines
		if len(inputData.Order.TaxLines) > 0 {
			// Get item price ex tax for proportion calculation
			var itemPriceExTax float64
			if priceExTax, ok := parsePrice(item.PriceExTax); ok {
				itemPriceExTax = priceExTax * float64(item.Quantity)
			} else if price, ok := parsePrice(item.Price); ok {
				if totalTax, ok := parsePrice(item.TotalTax); ok {
					itemPriceExTax = (price - totalTax) * float64(item.Quantity)
				} else {
					itemPriceExTax = price * float64(item.Quantity)
				}
			}

			// Calculate total price ex tax for all items
			var totalPriceExTax float64
			for _, it := range inputData.Order.Items {
				if px, ok := parsePrice(it.PriceExTax); ok {
					totalPriceExTax += px * float64(it.Quantity)
				} else if p, ok := parsePrice(it.Price); ok {
					if tt, ok := parsePrice(it.TotalTax); ok {
						totalPriceExTax += (p - tt) * float64(it.Quantity)
					} else {
						totalPriceExTax += p * float64(it.Quantity)
					}
				}
			}

			// Distribute tax lines proportionally to this line item
			if totalPriceExTax > 0 {
				itemProportion := itemPriceExTax / totalPriceExTax
				for _, orderTaxLine := range inputData.Order.TaxLines {
					if !orderTaxLine.IsUsed {
						continue
					}
					orderTaxPrice, _ := parsePrice(orderTaxLine.Price)
					itemTaxAmount := orderTaxPrice * itemProportion
					taxRate, _ := strconv.ParseFloat(orderTaxLine.Rate, 64)

					lineItem.TaxLines = append(lineItem.TaxLines, app.OrderCreateTaxLineInput{
						Title: orderTaxLine.Title,
						Rate:  fmt.Sprintf("%.4f", taxRate),
						PriceSet: &app.MoneyBagInput{
							ShopMoney: &app.MoneyInput{
								Amount:       fmt.Sprintf("%.2f", math.Round(itemTaxAmount*100)/100),
								CurrencyCode: "USD",
							},
						},
					})
					fmt.Printf("  → Adding tax line to line item: %s (rate: %.4f, amount: $%.2f)\n", 
						orderTaxLine.Title, taxRate, math.Round(itemTaxAmount*100)/100)
				}
			}
		}

		orderInput.LineItems = append(orderInput.LineItems, lineItem)
	}

	// Add order-level tax lines if available (as fallback if line-item tax fails)
	// Note: Shopify prefers tax lines on line items, but we can also add at order level
	if len(inputData.Order.TaxLines) > 0 {
		var orderTaxLines []app.OrderCreateTaxLineInput
		for _, tl := range inputData.Order.TaxLines {
			if !tl.IsUsed {
				continue
			}
			rate, _ := strconv.ParseFloat(tl.Rate, 64)
			orderTaxLines = append(orderTaxLines, app.OrderCreateTaxLineInput{
				Title: tl.Title,
				Rate:  fmt.Sprintf("%.4f", rate),
				PriceSet: &app.MoneyBagInput{
					ShopMoney: &app.MoneyInput{
						Amount:       tl.Price,
						CurrencyCode: "USD",
					},
				},
			})
		}
		// Only add order-level tax if no line-item tax was added
		// (Shopify prefers line-item tax, but order-level works as fallback)
		if len(orderTaxLines) > 0 && len(orderInput.LineItems) == 0 {
			orderInput.TaxLines = orderTaxLines
		}
	}

	// Map shipping address
	if inputData.Order.ShippingAddress != nil {
		orderInput.ShippingAddress = convertAddress(inputData.Order.ShippingAddress)
	}

	// Map billing address
	if inputData.Order.BillingAddress != nil {
		orderInput.BillingAddress = convertAddress(inputData.Order.BillingAddress)
	}

	// Map customer information
	if inputData.Order.Customer != nil && inputData.Order.Customer.Email != "" {
		orderInput.Email = inputData.Order.Customer.Email
	} else if inputData.Order.Email != "" {
		orderInput.Email = inputData.Order.Email
	}

	// Map note attributes (custom attributes) - convert to metafields
	if len(inputData.Order.NoteAttributes) > 0 {
		orderInput.Metafields = make([]app.MetafieldInput, len(inputData.Order.NoteAttributes))
		for i, attr := range inputData.Order.NoteAttributes {
			orderInput.Metafields[i] = app.MetafieldInput{
				Key:   attr.Name,
				Value: attr.Value,
				Type:  "single_line_text_field",
				Namespace: "custom",
			}
		}
	}

	// Note: Tax lines should be added to line items, NOT order level
	// Shopify requires tax lines to be either on order OR line items, not both

	return orderInput, nil
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
	// Note: Shopify GraphQL DraftOrderLineItemInput does NOT support taxLines field
	// Tax lines must be added AFTER completing the order using REST API
	for _, item := range inputData.Order.Items {
		lineItem := app.DraftLineItemInput{
			VariantID: toVariantGID(item.ProductID),
			Quantity:  item.Quantity,
			Taxable:   item.Taxable,
		}

		// Set originalUnitPrice for strikethrough display
		// IMPORTANT: originalUnitPrice = original price (will show strikethrough)
		// The discounted price will be calculated by Shopify based on appliedDiscount
		if item.OriginPrice != "" {
			if originPrice, ok := parsePrice(item.OriginPrice); ok {
				lineItem.OriginalUnitPrice = originPrice
				fmt.Printf("  → Setting originalUnitPrice: $%.2f (will show strikethrough)\n", originPrice)
			}
		} else if price, ok := parsePrice(item.Price); ok {
			// If no OriginPrice, use Price as originalUnitPrice
			lineItem.OriginalUnitPrice = price
			fmt.Printf("  → Setting originalUnitPrice: $%.2f (will show strikethrough)\n", price)
		}

		// Add discount from input.json for this line item
		if len(item.DiscountApplications) > 0 {
			discount := item.DiscountApplications[0]
			lineItem.AppliedDiscount = &app.AppliedDiscountInput{
				Title:       discount.Title,
				Description: discount.Title,
			}
			if discount.ValueType == "percentage" {
				value, _ := strconv.ParseFloat(discount.Value, 64)
				lineItem.AppliedDiscount.ValueType = "PERCENTAGE"
				// For percentage, value should be the percentage (e.g., 10 for 10%)
				lineItem.AppliedDiscount.Value = math.Round(value*100) / 100
				fmt.Printf("  → Adding line item discount: %s (%.2f%%)\n", discount.Title, value)
			} else {
				lineItem.AppliedDiscount.ValueType = "FIXED_AMOUNT"
				if amount, ok := parsePrice(discount.Amount); ok {
					lineItem.AppliedDiscount.Value = math.Round(amount*100) / 100
					fmt.Printf("  → Adding line item discount: %s ($%.2f)\n", discount.Title, amount)
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

	// Don't set taxExempt - let Shopify calculate tax normally
	// We'll replace Shopify's calculated tax with custom tax from input.json after completion
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

// parseFloat is a helper to parse float with error handling (returns 0 on error)
func parseFloat(s string) float64 {
	val, _ := strconv.ParseFloat(s, 64)
	return val
}

// addCustomTaxLineItemToDraftOrder adds a custom line item (tax) to draft order using REST API
func addCustomTaxLineItemToDraftOrder(draftOrderNum, taxTitle string, taxAmount float64) error {
	shopDomain := os.Getenv("SHOPIFY_SHOP_DOMAIN")
	accessToken := os.Getenv("SHOPIFY_API_SECRET")

	if shopDomain == "" || accessToken == "" {
		return fmt.Errorf("SHOPIFY_SHOP_DOMAIN and SHOPIFY_API_SECRET must be set")
	}

	apiVersion := "2025-10"
	url := fmt.Sprintf("https://%s/admin/api/%s/draft_orders/%s.json", shopDomain, apiVersion, draftOrderNum)

	// First, get the draft order to see existing line items
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	getReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create GET request: %w", err)
	}
	getReq.Header.Set("X-Shopify-Access-Token", accessToken)

	getResp, err := client.Do(getReq)
	if err != nil {
		return fmt.Errorf("failed to fetch draft order: %w", err)
	}
	defer getResp.Body.Close()

	getBodyBytes, err := io.ReadAll(getResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var draftOrderData map[string]interface{}
	if err := json.Unmarshal(getBodyBytes, &draftOrderData); err != nil {
		return fmt.Errorf("failed to parse draft order: %w", err)
	}

	draftOrder, ok := draftOrderData["draft_order"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid draft order response")
	}

	// Get existing line items
	lineItems, ok := draftOrder["line_items"].([]interface{})
	if !ok {
		lineItems = []interface{}{}
	}

	// Add custom tax line item
	customTaxItem := map[string]interface{}{
		"title":   taxTitle,
		"price":   fmt.Sprintf("%.2f", taxAmount),
		"quantity": 1,
		"taxable": false,
		"requires_shipping": false,
		"variant_id": nil, // Custom line item doesn't need variant_id
	}

	lineItems = append(lineItems, customTaxItem)

	// Update draft order with new line items
	updatePayload := map[string]interface{}{
		"draft_order": map[string]interface{}{
			"id":         draftOrderNum,
			"line_items": lineItems,
		},
	}

	updateBody, err := json.Marshal(updatePayload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	putReq, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(updateBody))
	if err != nil {
		return fmt.Errorf("failed to create PUT request: %w", err)
	}
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set("X-Shopify-Access-Token", accessToken)

	putResp, err := client.Do(putReq)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer putResp.Body.Close()

	if putResp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(putResp.Body)
		return fmt.Errorf("API returned status %d: %s", putResp.StatusCode, string(bodyBytes))
	}

	return nil
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
				lineItems(first: 10) {
					edges {
						node {
							id
							title
							quantity
							originalUnitPriceSet {
								shopMoney {
									amount
									currencyCode
								}
							}
							discountAllocations {
								allocatedAmountSet {
									shopMoney {
										amount
										currencyCode
									}
								}
								discountApplication {
									... on DiscountCodeApplication {
										code
										value
									}
									... on ScriptDiscountApplication {
										title
										value
									}
									... on AutomaticDiscountApplication {
										title
									}
									... on ManualDiscountApplication {
										title
										value
									}
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
				fmt.Println("\nOrder-Level Tax Lines:")
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
			
			// Check tax lines and discounts in line items
			if lineItems, ok := order["lineItems"].(map[string]interface{}); ok {
				if edges, ok := lineItems["edges"].([]interface{}); ok {
					fmt.Println("\nLine Items Details:")
					for i, edge := range edges {
						if edgeMap, ok := edge.(map[string]interface{}); ok {
							if node, ok := edgeMap["node"].(map[string]interface{}); ok {
								title := ""
								if titleVal, ok := node["title"].(string); ok {
									title = titleVal
								}
								fmt.Printf("  Line Item %d (%s):\n", i+1, title)
								
								// Check for discounts
								if discountAllocations, ok := node["discountAllocations"].([]interface{}); ok && len(discountAllocations) > 0 {
									fmt.Printf("    Discounts:\n")
									for j, da := range discountAllocations {
										if daMap, ok := da.(map[string]interface{}); ok {
											discountAmount := ""
											discountTitle := ""
											if allocatedAmount, ok := daMap["allocatedAmountSet"].(map[string]interface{}); ok {
												if shopMoney, ok := allocatedAmount["shopMoney"].(map[string]interface{}); ok {
													if amount, ok := shopMoney["amount"].(string); ok {
														discountAmount = amount
													}
												}
											}
											if discountApp, ok := daMap["discountApplication"].(map[string]interface{}); ok {
												if t, ok := discountApp["title"].(string); ok {
													discountTitle = t
												} else if code, ok := discountApp["code"].(string); ok {
													discountTitle = code
												}
											}
											fmt.Printf("      Discount %d: %s - Amount: %s\n", j+1, discountTitle, discountAmount)
										}
									}
								} else {
									fmt.Printf("    ⚠ No discounts found\n")
								}
								
								// Check for tax lines
								if taxLines, ok := node["taxLines"].([]interface{}); ok && len(taxLines) > 0 {
									fmt.Printf("    Tax Lines:\n")
									for j, tl := range taxLines {
										if taxLine, ok := tl.(map[string]interface{}); ok {
											taxTitle := ""
											rate := ""
											amount := ""
											if titleVal, ok := taxLine["title"].(string); ok {
												taxTitle = titleVal
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
											fmt.Printf("      Tax %d: %s - Rate: %s, Amount: %s\n", j+1, taxTitle, rate, amount)
										}
									}
								} else {
									fmt.Printf("    No tax lines\n")
								}
							}
						}
					}
				}
			}
		}
	}
}
