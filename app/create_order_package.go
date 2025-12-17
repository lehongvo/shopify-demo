package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// DraftOrderInput represents the input for creating a draft order
type DraftOrderInput struct {
	// The discount that will be applied to the draft order.
	// A draft order line item can have one discount. A draft order can also have one order-level discount.
	AppliedDiscount *AppliedDiscountInput `json:"appliedDiscount,omitempty"`
	// The list of discount codes that will be attempted to be applied to the draft order.
	// If the draft isn't eligible for any given discount code it will be skipped during calculation.
	DiscountCodes []string `json:"discountCodes,omitempty"`
	// Whether or not to accept automatic discounts on the draft order during calculation.
	// If false, only discount codes and custom draft order discounts (see `appliedDiscount`) will be applied.
	// If true, eligible automatic discounts will be applied in addition to discount codes and custom draft order discounts.
	AcceptAutomaticDiscounts bool `json:"acceptAutomaticDiscounts,omitempty"`
	// The mailing address associated with the payment method.
	BillingAddress *MailingAddressInput `json:"billingAddress,omitempty"`
	// The extra information added to the draft order on behalf of the customer.
	CustomAttributes []AttributeInput `json:"customAttributes,omitempty"`
	// The customer's email address.
	Email string `json:"email,omitempty"`
	// The list of product variant or custom line item.
	// Each draft order must include at least one line item.
	LineItems []DraftLineItemInput `json:"lineItems,omitempty"`
	// The list of metafields attached to the draft order. An existing metafield can not be used when creating a draft order.
	Metafields []MetafieldInput `json:"metafields,omitempty"`
	// The localized fields attached to the draft order. For example, Tax IDs.
	LocalizedFields []LocalizedFieldInput `json:"localizedFields,omitempty"`
	// The text of an optional note that a shop owner can attach to the draft order.
	Note string `json:"note,omitempty"`
	// The mailing address to where the order will be shipped.
	ShippingAddress *MailingAddressInput `json:"shippingAddress,omitempty"`
	// The shipping line object, which details the shipping method used.
	ShippingLine *ShippingLineInput `json:"shippingLine,omitempty"`
	// A comma separated list of tags that have been added to the draft order.
	Tags []string `json:"tags,omitempty"`
	// Whether or not taxes are exempt for the draft order.
	// If false, then Shopify will refer to the taxable field for each line item.
	// If a customer is applied to the draft order, then Shopify will use the customer's tax exempt field instead.
	TaxExempt bool `json:"taxExempt,omitempty"`
	// Whether to use the customer's default address.
	UseCustomerDefaultAddress bool `json:"useCustomerDefaultAddress,omitempty"`
	// Whether the draft order will be visible to the customer on the self-serve portal.
	VisibleToCustomer bool `json:"visibleToCustomer,omitempty"`
	// The time after which inventory reservation will expire.
	ReserveInventoryUntil *time.Time `json:"reserveInventoryUntil,omitempty"`
	// The payment currency of the customer for this draft order.
	PresentmentCurrencyCode string `json:"presentmentCurrencyCode,omitempty"`
	// The customer's phone number.
	Phone string `json:"phone,omitempty"`
	// The fields used to create payment terms.
	PaymentTerms *PaymentTermsInput `json:"paymentTerms,omitempty"`
	// The purchasing entity for the draft order.
	PurchasingEntity *PurchasingEntityInput `json:"purchasingEntity,omitempty"`
	// The source of the checkout.
	SourceName string `json:"sourceName,omitempty"`
	// Whether discount codes are allowed during checkout of this draft order.
	AllowDiscountCodesInCheckout bool `json:"allowDiscountCodesInCheckout,omitempty"`
	// The purchase order number.
	PoNumber string `json:"poNumber,omitempty"`
	// The unique token identifying the draft order.
	SessionToken string `json:"sessionToken,omitempty"`
	// Fingerprint to guarantee bundles are handled correctly.
	TransformerFingerprint string `json:"transformerFingerprint,omitempty"`
	// Customer information (kept for backward compatibility)
	Customer *CustomerInput `json:"customer,omitempty"`
	// Note: DraftOrderInput does NOT support taxLines field (verified via GraphQL introspection)
	// Tax will be calculated automatically by Shopify when completing draft order
	// if shipping address and tax configuration are provided
}

// DraftLineItemInput represents a line item in a draft order with discount support
type DraftLineItemInput struct {
	VariantID       string                `json:"variantId"`
	Quantity        int                   `json:"quantity"`
	OriginalUnitPrice float64             `json:"originalUnitPrice,omitempty"` // Custom price for the line item
	Title           string                `json:"title,omitempty"`
	AppliedDiscount *AppliedDiscountInput `json:"appliedDiscount,omitempty"`
	Taxable         bool                  `json:"taxable,omitempty"`
	// Tax lines for this line item
	TaxLines []TaxLineInput `json:"taxLines,omitempty"`
}

// AppliedDiscountInput represents a discount applied to a line item
type AppliedDiscountInput struct {
	Description string  `json:"description,omitempty"`
	ValueType   string  `json:"valueType"` // PERCENTAGE or FIXED_AMOUNT
	Value       float64 `json:"value"`
	Title       string  `json:"title,omitempty"`
}

// TaxLineInput represents a tax line input for draft order line items
// Based on Shopify TaxLine object: https://shopify.dev/docs/api/admin-graphql/latest/objects/taxline
type TaxLineInput struct {
	// The name of the tax (required)
	Title string `json:"title"`
	// The proportion of the line item price that the tax represents as a decimal
	Rate float64 `json:"rate,omitempty"`
	// The amount of tax, in shop and presentment currencies
	PriceSet *MoneyBagInput `json:"priceSet,omitempty"`
	// Whether the channel that submitted the tax line is liable for remitting
	ChannelLiable *bool `json:"channelLiable,omitempty"`
	// The source of the tax
	Source string `json:"source,omitempty"`
}

// OrderCreateTaxLineInput represents tax line input for orderCreate mutation
// Based on: https://shopify.dev/docs/api/admin-graphql/latest/input-objects/ordercreatetaxlineinput
type OrderCreateTaxLineInput struct {
	// The name of the tax line to create (required)
	Title string `json:"title"`
	// The proportion of the item price that the tax represents as a decimal (required)
	Rate string `json:"rate"` // Using string for Decimal type
	// The amount of tax to be charged on the item
	PriceSet *MoneyBagInput `json:"priceSet,omitempty"`
	// Whether the channel that submitted the tax line is liable for remitting (default: false)
	ChannelLiable *bool `json:"channelLiable,omitempty"`
}

// MoneyBagInput represents money in shop and presentment currencies
type MoneyBagInput struct {
	ShopMoney *MoneyInput `json:"shopMoney,omitempty"`
}

// MoneyInput represents a monetary value
type MoneyInput struct {
	Amount       string `json:"amount"`
	CurrencyCode string `json:"currencyCode"`
}

// OrderInput represents the input for creating an order (kept for backward compatibility)
type OrderInput struct {
	Email           string                   `json:"email,omitempty"`
	LineItems       []LineItemInput          `json:"lineItems"`
	ShippingAddress *MailingAddressInput     `json:"shippingAddress,omitempty"`
	BillingAddress  *MailingAddressInput     `json:"billingAddress,omitempty"`
	FinancialStatus string                   `json:"financialStatus,omitempty"` // PENDING, AUTHORIZED, PARTIALLY_PAID, PAID, PARTIALLY_REFUNDED, REFUNDED, VOIDED
	Customer        *CustomerInput           `json:"customer,omitempty"`
	Note            string                   `json:"note,omitempty"`
	Tags            []string                 `json:"tags,omitempty"`
	Metafields      []MetafieldInput         `json:"metafields,omitempty"`
	TaxLines        []OrderCreateTaxLineInput `json:"taxLines,omitempty"` // Order-level tax lines
	AppliedDiscount *AppliedDiscountInput    `json:"appliedDiscount,omitempty"` // Order-level discount (deprecated, use DiscountCode instead)
	DiscountCode    *OrderCreateDiscountCodeInput `json:"discountCode,omitempty"` // Order-level discount via discountCode
}

// OrderCreateDiscountCodeInput represents discount code input for orderCreate mutation
// Based on: https://shopify.dev/docs/api/admin-graphql/latest/input-objects/ordercreatediscountcodeinput
type OrderCreateDiscountCodeInput struct {
	ItemFixedDiscountCode *ItemFixedDiscountCodeInput `json:"itemFixedDiscountCode,omitempty"`
	ItemPercentageDiscountCode *ItemPercentageDiscountCodeInput `json:"itemPercentageDiscountCode,omitempty"`
}

// ItemFixedDiscountCodeInput represents a fixed amount discount code
type ItemFixedDiscountCodeInput struct {
	Code      string    `json:"code"`      // Description of the discount
	AmountSet *MoneyBagInput `json:"amountSet"` // Fixed discount amount
}

// ItemPercentageDiscountCodeInput represents a percentage discount code
type ItemPercentageDiscountCodeInput struct {
	Code      string  `json:"code"`      // Description of the discount
	Percentage float64 `json:"percentage"` // Percentage discount (0-100)
}

// LineItemInput represents a line item in an order
type LineItemInput struct {
	VariantID          string                   `json:"variantId"`
	Quantity           int                       `json:"quantity"`
	Price              string                    `json:"price,omitempty"` // Deprecated: use priceSet instead
	PriceSet           *MoneyBagInput            `json:"priceSet,omitempty"` // Custom price after discount
	Title              string                    `json:"title,omitempty"`
	Properties         []LineItemPropertyInput   `json:"properties,omitempty"` // For notes about discounts
	TaxLines           []OrderCreateTaxLineInput `json:"taxLines,omitempty"`
	DiscountAllocations []DiscountAllocationInput `json:"discountAllocations,omitempty"`
}

// LineItemPropertyInput represents a property/note on a line item
type LineItemPropertyInput struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// DiscountAllocationInput represents discount allocation for a line item
type DiscountAllocationInput struct {
	Amount string `json:"amount"`
	Title  string `json:"title,omitempty"`
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

// AttributeInput represents a custom attribute
type AttributeInput struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// LocalizedFieldInput represents a localized field (e.g., Tax IDs)
type LocalizedFieldInput struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ShippingLineInput represents shipping line details
type ShippingLineInput struct {
	Title string  `json:"title,omitempty"`
	Price float64 `json:"price,omitempty"`
}

// PaymentTermsInput represents payment terms
type PaymentTermsInput struct {
	PaymentTermsType string `json:"paymentTermsType,omitempty"`
	PaymentSchedules []struct {
		DueAt time.Time `json:"dueAt,omitempty"`
	} `json:"paymentSchedules,omitempty"`
}

// PurchasingEntityInput represents purchasing entity
type PurchasingEntityInput struct {
	CustomerID string `json:"customerId,omitempty"`
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
				TotalTaxSet struct {
					ShopMoney struct {
						Amount       string `json:"amount"`
						CurrencyCode string `json:"currencyCode"`
					} `json:"shopMoney"`
				} `json:"totalTaxSet"`
				TaxLines []struct {
					Title string      `json:"title"`
					Rate  interface{} `json:"rate"` // Can be number or string
					PriceSet struct {
						ShopMoney struct {
							Amount       string `json:"amount"`
							CurrencyCode string `json:"currencyCode"`
						} `json:"shopMoney"`
					} `json:"priceSet"`
				} `json:"taxLines,omitempty"`
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

// QueryDraftOrder queries a draft order to get tax information
func QueryDraftOrder(draftID string) (map[string]interface{}, error) {
	const query = `
		query GetDraftOrder($id: ID!) {
			draftOrder(id: $id) {
				id
				name
				totalTaxSet {
					shopMoney {
						amount
						currencyCode
					}
				}
				taxLines {
					title
					priceSet {
						shopMoney {
							amount
							currencyCode
						}
					}
					rate
					ratePercentage
				}
				taxesIncluded
				taxExempt
				lineItems {
					edges {
						node {
							id
							title
							taxable
							originalUnitPriceSet {
								shopMoney {
									amount
									currencyCode
								}
							}
						}
					}
				}
			}
		}`

	variables := map[string]interface{}{
		"id": draftID,
	}

	resp, err := callAdminGraphQL(query, variables)
	if err != nil {
		return nil, err
	}

	if data, ok := resp["data"].(map[string]interface{}); ok {
		if draftOrder, ok := data["draftOrder"].(map[string]interface{}); ok {
			return draftOrder, nil
		}
	}

	return nil, fmt.Errorf("draft order not found")
}

// CalculateDraftOrder calculates tax and totals for a draft order using the draft order input
// This allows previewing tax before completing the draft order
// Note: draftOrderCalculate takes DraftOrderInput, not a draft order ID
func CalculateDraftOrder(input DraftOrderInput) (map[string]interface{}, error) {
	const mutation = `
		mutation CalculateDraftOrder($input: DraftOrderInput!) {
			draftOrderCalculate(input: $input) {
				calculatedDraftOrder {
					totalTaxSet {
						shopMoney {
							amount
							currencyCode
						}
					}
					taxLines {
						title
						priceSet {
							shopMoney {
								amount
								currencyCode
							}
						}
						rate
						ratePercentage
					}
					totalPriceSet {
						shopMoney {
							amount
							currencyCode
						}
					}
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

	// Check for errors
	if errors, ok := resp["errors"].([]interface{}); ok && len(errors) > 0 {
		return nil, fmt.Errorf("GraphQL errors: %v", errors)
	}

	if data, ok := resp["data"].(map[string]interface{}); ok {
		if calculate, ok := data["draftOrderCalculate"].(map[string]interface{}); ok {
			if userErrors, ok := calculate["userErrors"].([]interface{}); ok && len(userErrors) > 0 {
				return nil, fmt.Errorf("user errors: %v", userErrors)
			}
			if calculated, ok := calculate["calculatedDraftOrder"].(map[string]interface{}); ok {
				return calculated, nil
			}
		}
	}

	return nil, fmt.Errorf("failed to calculate draft order")
}

// UpdateDraftOrder updates a draft order using draftOrderUpdate mutation
// This allows updating draft order before completing it
func UpdateDraftOrder(draftID string, input DraftOrderInput) (*DraftOrderResponse, error) {
	const mutation = `
		mutation UpdateDraftOrder($id: ID!, $input: DraftOrderInput!) {
			draftOrderUpdate(id: $id, input: $input) {
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
		"id":    draftID,
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

// AddTaxToOrder adds tax lines to an order using REST API
// Tries multiple approaches: order-level tax, then line-item level tax
func AddTaxToOrder(orderID string, taxLines []TaxLineInput) error {
	shopDomain := os.Getenv("SHOPIFY_SHOP_DOMAIN")
	accessToken := os.Getenv("SHOPIFY_API_SECRET")

	if shopDomain == "" || accessToken == "" {
		return fmt.Errorf("SHOPIFY_SHOP_DOMAIN and SHOPIFY_API_SECRET must be set")
	}

	// Extract numeric order ID from GID format
	orderNum := orderID
	if strings.HasPrefix(orderID, "gid://shopify/Order/") {
		orderNum = strings.TrimPrefix(orderID, "gid://shopify/Order/")
	}

	apiVersion := "2025-10"
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Convert tax lines to REST API format
	taxLinesRest := make([]map[string]interface{}, len(taxLines))
	for i, tl := range taxLines {
		taxLineMap := map[string]interface{}{
			"title": tl.Title,
		}
		if tl.Rate > 0 {
			taxLineMap["rate"] = tl.Rate
		}
		if tl.PriceSet != nil && tl.PriceSet.ShopMoney != nil {
			taxLineMap["price"] = tl.PriceSet.ShopMoney.Amount
		}
		if tl.Source != "" {
			taxLineMap["source"] = tl.Source
		}
		if tl.ChannelLiable != nil {
			taxLineMap["channel_liable"] = *tl.ChannelLiable
		}
		taxLinesRest[i] = taxLineMap
	}

	// Approach 1: Try adding tax_lines at order level
	fmt.Printf("Debug: Trying to add tax at order level...\n")
	url := fmt.Sprintf("https://%s/admin/api/%s/orders/%s.json", shopDomain, apiVersion, orderNum)
	payload := map[string]interface{}{
		"order": map[string]interface{}{
			"id":        orderNum,
			"tax_lines": taxLinesRest,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Shopify-Access-Token", accessToken)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check if order-level tax was successful
	if resp.StatusCode == http.StatusOK {
		var updateResponse map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &updateResponse); err == nil {
			if order, ok := updateResponse["order"].(map[string]interface{}); ok {
				if taxLines, ok := order["tax_lines"].([]interface{}); ok {
					fmt.Printf("Debug: Response tax_lines count: %d (sent: %d)\n", len(taxLines), len(taxLinesRest))
					for i, tl := range taxLines {
						if tlMap, ok := tl.(map[string]interface{}); ok {
							fmt.Printf("Debug: Tax line %d: title=%v, rate=%v, price=%v\n", 
								i+1, 
								tlMap["title"], 
								tlMap["rate"], 
								tlMap["price"])
						}
					}
					if len(taxLines) > 0 {
						if len(taxLines) < len(taxLinesRest) {
							fmt.Printf("Debug: ⚠ Only %d tax lines added (expected: %d). Shopify may have merged or ignored some tax lines.\n", 
								len(taxLines), len(taxLinesRest))
						} else {
							fmt.Printf("Debug: ✓ Tax lines successfully added at order level: %d\n", len(taxLines))
						}
						return nil
					}
				} else {
					fmt.Printf("Debug: No tax_lines field in response\n")
				}
			}
		} else {
			fmt.Printf("Debug: Failed to parse response: %v\n", err)
		}
	} else {
		fmt.Printf("Debug: Response status: %d\n", resp.StatusCode)
	}

	// Approach 2: If order-level failed, try adding tax to line items
	fmt.Printf("Debug: Order-level tax failed, trying line-item level...\n")
	
	// First, fetch the order to get line items
	getURL := fmt.Sprintf("https://%s/admin/api/%s/orders/%s.json", shopDomain, apiVersion, orderNum)
	getReq, err := http.NewRequest(http.MethodGet, getURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create GET request: %w", err)
	}
	getReq.Header.Set("X-Shopify-Access-Token", accessToken)

	getResp, err := client.Do(getReq)
	if err != nil {
		return fmt.Errorf("failed to fetch order: %w", err)
	}
	defer getResp.Body.Close()

	getBodyBytes, err := io.ReadAll(getResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read order response: %w", err)
	}

	var orderData map[string]interface{}
	if err := json.Unmarshal(getBodyBytes, &orderData); err != nil {
		return fmt.Errorf("failed to parse order: %w", err)
	}

	order, ok := orderData["order"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid order response")
	}

	// Get line items
	lineItems, ok := order["line_items"].([]interface{})
	if !ok {
		return fmt.Errorf("no line items found in order")
	}

	// Calculate tax per line item (distribute total tax proportionally)
	totalPrice := 0.0
	for _, li := range lineItems {
		if liMap, ok := li.(map[string]interface{}); ok {
			if price, ok := liMap["price"].(string); ok {
				if p, err := strconv.ParseFloat(price, 64); err == nil {
					if qty, ok := liMap["quantity"].(float64); ok {
						totalPrice += p * qty
					}
				}
			}
		}
	}

	// Update each line item with tax
	updatedLineItems := make([]map[string]interface{}, len(lineItems))
	for i, li := range lineItems {
		liMap, ok := li.(map[string]interface{})
		if !ok {
			continue
		}

		// Calculate tax for this line item
		lineItemPrice := 0.0
		if price, ok := liMap["price"].(string); ok {
			if p, err := strconv.ParseFloat(price, 64); err == nil {
				if qty, ok := liMap["quantity"].(float64); ok {
					lineItemPrice = p * qty
				}
			}
		}

		// Distribute tax proportionally
		lineItemTax := 0.0
		if totalPrice > 0 && len(taxLinesRest) > 0 {
			if totalTaxPrice, ok := taxLinesRest[0]["price"].(string); ok {
				if totalTax, err := strconv.ParseFloat(totalTaxPrice, 64); err == nil {
					lineItemTax = (lineItemPrice / totalPrice) * totalTax
				}
			}
		}

		// Create tax line for this line item
		lineItemTaxLines := []map[string]interface{}{}
		if lineItemTax > 0 && len(taxLinesRest) > 0 {
			lineTaxLine := map[string]interface{}{
				"title": taxLinesRest[0]["title"],
				"rate":  taxLinesRest[0]["rate"],
				"price": fmt.Sprintf("%.2f", lineItemTax),
			}
			if source, ok := taxLinesRest[0]["source"].(string); ok && source != "" {
				lineTaxLine["source"] = source
			}
			lineItemTaxLines = append(lineItemTaxLines, lineTaxLine)
		}

		// Update line item
		updatedLineItem := map[string]interface{}{
			"id":        liMap["id"],
			"tax_lines": lineItemTaxLines,
		}
		updatedLineItems[i] = updatedLineItem
	}

	// Update order with line items that have tax
	updatePayload := map[string]interface{}{
		"order": map[string]interface{}{
			"id":         orderNum,
			"line_items": updatedLineItems,
		},
	}

	updateBody, err := json.Marshal(updatePayload)
	if err != nil {
		return fmt.Errorf("failed to marshal update request: %w", err)
	}

	updateReq, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(updateBody))
	if err != nil {
		return fmt.Errorf("failed to create update request: %w", err)
	}

	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("X-Shopify-Access-Token", accessToken)

	updateResp, err := client.Do(updateReq)
	if err != nil {
		return fmt.Errorf("failed to execute update request: %w", err)
	}
	defer updateResp.Body.Close()

	updateBodyBytes, err := io.ReadAll(updateResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read update response: %w", err)
	}

	if updateResp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d: %s", updateResp.StatusCode, string(updateBodyBytes))
	}

	// Check if tax was added
	var finalResponse map[string]interface{}
	if err := json.Unmarshal(updateBodyBytes, &finalResponse); err == nil {
		if finalOrder, ok := finalResponse["order"].(map[string]interface{}); ok {
			// Check order-level tax
			if taxLines, ok := finalOrder["tax_lines"].([]interface{}); ok {
				if len(taxLines) > 0 {
					fmt.Printf("Debug: ✓ Tax lines successfully added: %d\n", len(taxLines))
					return nil
				}
			}
			// Check line-item tax
			if lineItems, ok := finalOrder["line_items"].([]interface{}); ok {
				totalLineItemTax := 0
				for _, li := range lineItems {
					if liMap, ok := li.(map[string]interface{}); ok {
						if taxLines, ok := liMap["tax_lines"].([]interface{}); ok {
							if len(taxLines) > 0 {
								totalLineItemTax += len(taxLines)
							}
						}
					}
				}
				if totalLineItemTax > 0 {
					fmt.Printf("Debug: ✓ Tax lines successfully added to line items: %d\n", totalLineItemTax)
					return nil
				}
			}
		}
	}

	return fmt.Errorf("failed to add tax lines (tried both order-level and line-item level)")
}

// UpdateOrderTaxGraphQL attempts to update order tax using GraphQL orderUpdate mutation
// Note: orderUpdate may not support tax lines directly, but we'll try
func UpdateOrderTaxGraphQL(orderID string, taxLines []TaxLineInput) error {
	const mutation = `
		mutation UpdateOrderTax($id: ID!, $input: OrderUpdateInput!) {
			orderUpdate(id: $id, input: $input) {
				order {
					id
					totalTaxSet {
						shopMoney {
							amount
							currencyCode
						}
					}
					taxLines {
						title
						priceSet {
							shopMoney {
								amount
								currencyCode
							}
						}
						rate
					}
				}
				userErrors {
					field
					message
				}
			}
		}`

	// Note: OrderUpdateInput may not support taxLines directly
	// This is an attempt - may need to use orderEditBegin instead
	input := map[string]interface{}{
		// Try to add tax via custom attributes or other supported fields
		// If taxLines is not supported, this will fail gracefully
	}

	variables := map[string]interface{}{
		"id":    orderID,
		"input": input,
	}

	resp, err := callAdminGraphQL(mutation, variables)
	if err != nil {
		return fmt.Errorf("failed to call GraphQL: %w", err)
	}

	// Check for errors
	if errors, ok := resp["errors"].([]interface{}); ok && len(errors) > 0 {
		return fmt.Errorf("GraphQL errors: %v", errors)
	}

	// Check user errors
	if data, ok := resp["data"].(map[string]interface{}); ok {
		if orderUpdate, ok := data["orderUpdate"].(map[string]interface{}); ok {
			if userErrors, ok := orderUpdate["userErrors"].([]interface{}); ok && len(userErrors) > 0 {
				return fmt.Errorf("user errors: %v", userErrors)
			}
			// Check if tax was updated
			if order, ok := orderUpdate["order"].(map[string]interface{}); ok {
				if taxLines, ok := order["taxLines"].([]interface{}); ok && len(taxLines) > 0 {
					fmt.Printf("Debug: ✓ Tax updated via GraphQL: %d tax lines\n", len(taxLines))
					return nil
				}
			}
		}
	}

	return fmt.Errorf("orderUpdate mutation does not support tax lines directly")
}

// UpdateOrderTaxViaEdit attempts to update order tax using orderEditBegin flow
// This is the recommended way to make significant changes to an order
func UpdateOrderTaxViaEdit(orderID string, taxLines []TaxLineInput) error {
	// Step 1: Begin order edit
	const beginMutation = `
		mutation BeginOrderEdit($id: ID!) {
			orderEditBegin(id: $id) {
				calculatedOrder {
					id
				}
				userErrors {
					field
					message
				}
			}
		}`

	variables := map[string]interface{}{
		"id": orderID,
	}

	resp, err := callAdminGraphQL(beginMutation, variables)
	if err != nil {
		return fmt.Errorf("failed to begin order edit: %w", err)
	}

	// Check for errors
	if errors, ok := resp["errors"].([]interface{}); ok && len(errors) > 0 {
		return fmt.Errorf("GraphQL errors: %v", errors)
	}

	// Get calculated order ID
	var calculatedOrderID string
	if data, ok := resp["data"].(map[string]interface{}); ok {
		if orderEditBegin, ok := data["orderEditBegin"].(map[string]interface{}); ok {
			if userErrors, ok := orderEditBegin["userErrors"].([]interface{}); ok && len(userErrors) > 0 {
				return fmt.Errorf("user errors: %v", userErrors)
			}
			if calculatedOrder, ok := orderEditBegin["calculatedOrder"].(map[string]interface{}); ok {
				if id, ok := calculatedOrder["id"].(string); ok {
					calculatedOrderID = id
				}
			}
		}
	}

	if calculatedOrderID == "" {
		return fmt.Errorf("failed to get calculated order ID")
	}

	// Step 2: Add tax to calculated order
	// Note: This may require using orderEditAddTaxLine mutation if available
	// For now, we'll try to complete the edit and see if we can add tax
	// This is a simplified version - full implementation may require more steps

	fmt.Printf("Debug: Order edit begun, calculated order ID: %s\n", calculatedOrderID)
	fmt.Println("Note: Adding tax via order edit requires additional mutations")
	fmt.Println("This is a placeholder - full implementation needed")

	return fmt.Errorf("orderEditAddTaxLine mutation not yet implemented")
}

// Helper function to get keys from map
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// CreateOrderFromDraft is a convenience function that creates a draft order and completes it
// This is the recommended way to create orders with discounts
// Shopify will automatically calculate tax when completing if:
// - Store has tax rates configured in Settings → Taxes
// - Shipping address is provided
// - Line items have taxable=true
func CreateOrderFromDraft(input DraftOrderInput, paymentPending bool) (*OrderInfo, error) {
	// Step 1: Create draft order
	draftResp, err := CreateDraftOrder(input)
	if err != nil {
		return nil, fmt.Errorf("failed to create draft order: %w", err)
	}

	draftID := draftResp.Data.DraftOrderCreate.DraftOrder.ID

	// Step 2: Calculate tax before completing (optional - to preview tax)
	// This helps ensure tax will be calculated when completing
	// We use the same input to calculate tax
	fmt.Println("Calculating tax for draft order...")
	calculated, err := CalculateDraftOrder(input)
	if err != nil {
		fmt.Printf("Warning: Could not calculate tax preview: %v\n", err)
		fmt.Println("Tax will still be calculated when completing draft order if store has tax configured")
	} else {
		// Check if tax was calculated
		if taxLines, ok := calculated["taxLines"].([]interface{}); ok && len(taxLines) > 0 {
			fmt.Printf("✓ Tax preview calculated: %d tax line(s)\n", len(taxLines))
			if totalTax, ok := calculated["totalTaxSet"].(map[string]interface{}); ok {
				if shopMoney, ok := totalTax["shopMoney"].(map[string]interface{}); ok {
					if amount, ok := shopMoney["amount"].(string); ok {
						fmt.Printf("  Total Tax: %s %s\n", amount, shopMoney["currencyCode"])
					}
				}
			}
		} else {
			fmt.Println("⚠️  No tax calculated in preview")
			fmt.Println("  This may mean:")
			fmt.Println("    - Store tax rates are not configured in Settings → Taxes")
			fmt.Println("    - Shipping address is missing or incomplete")
			fmt.Println("    - Line items are marked as taxExempt")
		}
	}

	// Step 3: Complete draft order
	// Shopify will automatically calculate tax when completing if conditions are met
	orderInfo, err := CompleteDraftOrder(draftID, paymentPending)
	if err != nil {
		return nil, fmt.Errorf("failed to complete draft order: %w", err)
	}

	return orderInfo, nil
}

// CreateOrderFromDraftWithTaxAttempt creates a draft order, attempts to add tax, then completes it
// This tries to add tax to draft order before completing (may not work if DraftOrderInput doesn't support tax)
func CreateOrderFromDraftWithTaxAttempt(input DraftOrderInput, taxLines []TaxLineInput, paymentPending bool) (*OrderInfo, error) {
	// Step 1: Create draft order
	draftResp, err := CreateDraftOrder(input)
	if err != nil {
		return nil, fmt.Errorf("failed to create draft order: %w", err)
	}

	draftID := draftResp.Data.DraftOrderCreate.DraftOrder.ID

	// Step 2: Try to update draft order with tax lines (if supported)
	// Note: This may not work as DraftOrderInput may not support taxLines
	if len(taxLines) > 0 {
		fmt.Printf("Attempting to add tax to draft order %s before completing...\n", draftID)
		// Try to add tax to line items in draft order
		updateInput := input
		for i := range updateInput.LineItems {
			updateInput.LineItems[i].TaxLines = taxLines
		}
		
		_, updateErr := UpdateDraftOrder(draftID, updateInput)
		if updateErr != nil {
			fmt.Printf("Warning: Failed to update draft order with tax: %v\n", updateErr)
			fmt.Println("Note: DraftOrderInput may not support taxLines. Will complete without tax.")
		} else {
			fmt.Println("✓ Draft order updated with tax lines")
		}
	}

	// Step 3: Complete draft order
	orderInfo, err := CompleteDraftOrder(draftID, paymentPending)
	if err != nil {
		return nil, fmt.Errorf("failed to complete draft order: %w", err)
	}

	return orderInfo, nil
}

// CreateOrderFromDraftWithTax creates a draft order, completes it, and adds tax lines
// If tax needs to be added, it completes with paymentPending=true first, adds tax, then marks as paid
func CreateOrderFromDraftWithTax(input DraftOrderInput, taxLines []TaxLineInput, paymentPending bool) (*OrderInfo, error) {
	// Step 1: Create draft order
	draftResp, err := CreateDraftOrder(input)
	if err != nil {
		return nil, fmt.Errorf("failed to create draft order: %w", err)
	}

	draftID := draftResp.Data.DraftOrderCreate.DraftOrder.ID

	// Step 2: Complete draft order
	// If we need to add tax, complete with paymentPending=true first (not paid yet)
	// This allows us to add tax before marking as paid
	shouldAddTax := len(taxLines) > 0
	completeAsPending := shouldAddTax && !paymentPending

	orderInfo, err := CompleteDraftOrder(draftID, completeAsPending)
	if err != nil {
		return nil, fmt.Errorf("failed to complete draft order: %w", err)
	}

	// Step 3: Add tax lines to order if provided
	if shouldAddTax && orderInfo.OrderID != "" {
		fmt.Printf("Attempting to add tax to order %s (status: pending)...\n", orderInfo.OrderID)
		if err := AddTaxToOrder(orderInfo.OrderID, taxLines); err != nil {
			// Log error but don't fail - order is already created
			fmt.Printf("Warning: Failed to add tax to order: %v\n", err)
			// Try to continue anyway
		} else {
			fmt.Println("✓ Tax lines added successfully")
		}

		// If we completed as pending to add tax, now mark as paid
		if completeAsPending && paymentPending == false {
			fmt.Println("Marking order as paid...")
			// Mark order as paid using REST API
			if err := MarkOrderAsPaid(orderInfo.OrderID); err != nil {
				return orderInfo, fmt.Errorf("failed to mark order as paid: %w", err)
			}
			fmt.Println("✓ Order marked as paid")
		}
	}

	return orderInfo, nil
}

// MarkOrderAsPaid marks an order as paid using REST API
func MarkOrderAsPaid(orderID string) error {
	shopDomain := os.Getenv("SHOPIFY_SHOP_DOMAIN")
	accessToken := os.Getenv("SHOPIFY_API_SECRET")

	if shopDomain == "" || accessToken == "" {
		return fmt.Errorf("SHOPIFY_SHOP_DOMAIN and SHOPIFY_API_SECRET must be set")
	}

	// Extract numeric order ID from GID format
	orderNum := orderID
	if strings.HasPrefix(orderID, "gid://shopify/Order/") {
		orderNum = strings.TrimPrefix(orderID, "gid://shopify/Order/")
	}

	apiVersion := "2025-10"
	url := fmt.Sprintf("https://%s/admin/api/%s/orders/%s.json", shopDomain, apiVersion, orderNum)

	payload := map[string]interface{}{
		"order": map[string]interface{}{
			"id":              orderNum,
			"financial_status": "paid",
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Shopify-Access-Token", accessToken)

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// CreateOrderWithTax creates a new order with tax lines using REST API
// This allows adding tax lines directly when creating the order
func CreateOrderWithTax(input OrderInput) (*OrderResponse, error) {
	shopDomain := os.Getenv("SHOPIFY_SHOP_DOMAIN")
	accessToken := os.Getenv("SHOPIFY_API_SECRET")

	if shopDomain == "" || accessToken == "" {
		return nil, fmt.Errorf("SHOPIFY_SHOP_DOMAIN and SHOPIFY_API_SECRET must be set")
	}

	// Use REST API to create order with tax lines
	apiVersion := "2025-10"
	url := fmt.Sprintf("https://%s/admin/api/%s/orders.json", shopDomain, apiVersion)

	// Convert input to REST API format
	orderPayload := map[string]interface{}{
		"email":           input.Email,
		"line_items":      []map[string]interface{}{},
		"financial_status": strings.ToLower(input.FinancialStatus),
		"note":            input.Note,
	}
	
	// Add tags only if not empty
	if len(input.Tags) > 0 {
		orderPayload["tags"] = strings.Join(input.Tags, ",")
	}

	// Convert line items
	for _, item := range input.LineItems {
		// Extract numeric variant ID from GID format
		variantID := item.VariantID
		if strings.HasPrefix(variantID, "gid://shopify/ProductVariant/") {
			variantID = strings.TrimPrefix(variantID, "gid://shopify/ProductVariant/")
		}
		
		lineItem := map[string]interface{}{
			"variant_id": variantID,
			"quantity":   item.Quantity,
		}
		if item.Price != "" {
			lineItem["price"] = item.Price
		}
		if item.Title != "" {
			lineItem["title"] = item.Title
		}
		// Note: Don't add tax lines to line items if order-level tax lines exist
		// Shopify doesn't allow tax lines at both levels
		orderPayload["line_items"] = append(orderPayload["line_items"].([]map[string]interface{}), lineItem)
	}

	// Add shipping address
	if input.ShippingAddress != nil {
		orderPayload["shipping_address"] = map[string]interface{}{
			"first_name": input.ShippingAddress.FirstName,
			"last_name":  input.ShippingAddress.LastName,
			"address1":   input.ShippingAddress.Address1,
			"city":       input.ShippingAddress.City,
			"province":   input.ShippingAddress.Province,
			"country":    input.ShippingAddress.Country,
			"zip":        input.ShippingAddress.Zip,
			"phone":      input.ShippingAddress.Phone,
		}
	}

	// Add billing address
	if input.BillingAddress != nil {
		orderPayload["billing_address"] = map[string]interface{}{
			"first_name": input.BillingAddress.FirstName,
			"last_name":  input.BillingAddress.LastName,
			"address1":   input.BillingAddress.Address1,
			"city":       input.BillingAddress.City,
			"province":   input.BillingAddress.Province,
			"country":    input.BillingAddress.Country,
			"zip":        input.BillingAddress.Zip,
			"phone":      input.BillingAddress.Phone,
		}
	}

	// Add order-level tax lines
	if len(input.TaxLines) > 0 {
		taxLines := make([]map[string]interface{}, len(input.TaxLines))
		for i, tl := range input.TaxLines {
			taxLine := map[string]interface{}{
				"title": tl.Title,
				"rate":  tl.Rate,
			}
			if tl.PriceSet != nil && tl.PriceSet.ShopMoney != nil {
				taxLine["price"] = tl.PriceSet.ShopMoney.Amount
			}
			if tl.ChannelLiable != nil {
				taxLine["channel_liable"] = *tl.ChannelLiable
			}
			taxLines[i] = taxLine
		}
		orderPayload["tax_lines"] = taxLines
	}

	// Create request payload
	requestPayload := map[string]interface{}{
		"order": orderPayload,
	}

	jsonData, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

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
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse REST API response
	var restResponse map[string]interface{}
	if err := json.Unmarshal(body, &restResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for errors
	if errors, ok := restResponse["errors"]; ok {
		return nil, fmt.Errorf("REST API errors: %v", errors)
	}

	// Convert REST API response to OrderResponse format
	order, ok := restResponse["order"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing order in response")
	}

	// Extract order ID (convert from numeric to GID format)
	orderID := ""
	if id, ok := order["id"].(float64); ok {
		orderID = fmt.Sprintf("gid://shopify/Order/%.0f", id)
	} else if idStr, ok := order["id"].(string); ok {
		orderID = fmt.Sprintf("gid://shopify/Order/%s", idStr)
	}

	// Extract total tax
	totalTax := "0.00"
	if taxLines, ok := order["tax_lines"].([]interface{}); ok {
		total := 0.0
		for _, tl := range taxLines {
			if tlMap, ok := tl.(map[string]interface{}); ok {
				if price, ok := tlMap["price"].(string); ok {
					if p, err := strconv.ParseFloat(price, 64); err == nil {
						total += p
					}
				}
			}
		}
		totalTax = fmt.Sprintf("%.2f", total)
	}

	// Build response
	response := &OrderResponse{
		Data: struct {
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
					TotalTaxSet struct {
						ShopMoney struct {
							Amount       string `json:"amount"`
							CurrencyCode string `json:"currencyCode"`
						} `json:"shopMoney"`
					} `json:"totalTaxSet"`
					TaxLines []struct {
						Title string      `json:"title"`
						Rate  interface{} `json:"rate"` // Can be number or string
						PriceSet struct {
							ShopMoney struct {
								Amount       string `json:"amount"`
								CurrencyCode string `json:"currencyCode"`
							} `json:"shopMoney"`
						} `json:"priceSet"`
					} `json:"taxLines,omitempty"`
					CreatedAt   string `json:"createdAt"`
					OrderNumber int    `json:"orderNumber"`
				} `json:"order"`
				UserErrors []UserError `json:"userErrors"`
			} `json:"orderCreate"`
		}{},
	}

	response.Data.OrderCreate.Order.ID = orderID
	if name, ok := order["name"].(string); ok {
		response.Data.OrderCreate.Order.Name = name
	}
	if email, ok := order["email"].(string); ok {
		response.Data.OrderCreate.Order.Email = email
	}
	if totalPrice, ok := order["total_price"].(string); ok {
		response.Data.OrderCreate.Order.TotalPriceSet.ShopMoney.Amount = totalPrice
		response.Data.OrderCreate.Order.TotalPriceSet.ShopMoney.CurrencyCode = "USD"
	}
	response.Data.OrderCreate.Order.TotalTaxSet.ShopMoney.Amount = totalTax
	response.Data.OrderCreate.Order.TotalTaxSet.ShopMoney.CurrencyCode = "USD"
	if orderNum, ok := order["order_number"].(float64); ok {
		response.Data.OrderCreate.Order.OrderNumber = int(orderNum)
	}

	return response, nil
}

// CreateOrder creates a new order in Shopify using GraphQL Admin API (legacy method, kept for backward compatibility)
func CreateOrder(input OrderInput) (*OrderResponse, error) {
	// Get environment variables
	shopDomain := os.Getenv("SHOPIFY_SHOP_DOMAIN")
	accessToken := os.Getenv("SHOPIFY_API_SECRET")

	if shopDomain == "" || accessToken == "" {
		return nil, fmt.Errorf("SHOPIFY_SHOP_DOMAIN and SHOPIFY_API_SECRET must be set")
	}

	// Construct GraphQL mutation with tax lines support
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

// CreateOrderGraphQL creates a new order using GraphQL orderCreate mutation
// This supports custom tax lines and discounts as recommended by Shopify
// - Custom tax lines: supported via taxLines field
// - Order-level discount: via discountCode field
// - Line-item discount: calculate discounted price and set in priceSet, add note in properties
func CreateOrderGraphQL(input OrderInput) (*OrderResponse, error) {
	// Get environment variables
	shopDomain := os.Getenv("SHOPIFY_SHOP_DOMAIN")
	accessToken := os.Getenv("SHOPIFY_API_SECRET")

	if shopDomain == "" || accessToken == "" {
		return nil, fmt.Errorf("SHOPIFY_SHOP_DOMAIN and SHOPIFY_API_SECRET must be set")
	}

	// Construct GraphQL mutation with tax lines and discount support
	// According to Shopify GraphQL schema, orderCreate takes 'order' argument with type OrderCreateOrderInput!
	mutation := `
		mutation orderCreate($order: OrderCreateOrderInput!) {
			orderCreate(order: $order) {
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
					createdAt
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
			"order": input,
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Debug: print request payload
	fmt.Printf("Debug: GraphQL orderCreate request:\n%s\n\n", string(jsonData))

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

	// Debug: print response
	fmt.Printf("Debug: GraphQL orderCreate response status: %d\n", resp.StatusCode)
	fmt.Printf("Debug: GraphQL orderCreate response body:\n%s\n\n", string(body))

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

	// Print tax lines if present
	if len(response.Data.OrderCreate.Order.TaxLines) > 0 {
		fmt.Printf("✓ Custom tax lines applied: %d tax line(s)\n", len(response.Data.OrderCreate.Order.TaxLines))
		for i, tl := range response.Data.OrderCreate.Order.TaxLines {
			rateStr := ""
			if rateFloat, ok := tl.Rate.(float64); ok {
				rateStr = fmt.Sprintf("%.3f", rateFloat)
			} else if rateStrVal, ok := tl.Rate.(string); ok {
				rateStr = rateStrVal
			} else {
				rateStr = fmt.Sprintf("%v", tl.Rate)
			}
			fmt.Printf("  Tax Line %d: %s (rate: %s, amount: %s)\n", 
				i+1, tl.Title, rateStr, tl.PriceSet.ShopMoney.Amount)
		}
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

// CreateDraftOrderREST creates a draft order using REST API with discount and tax support
// Returns draft order ID and order ID after completion
func CreateDraftOrderREST(input OrderInput) (map[string]string, error) {
	shopDomain := os.Getenv("SHOPIFY_SHOP_DOMAIN")
	accessToken := os.Getenv("SHOPIFY_API_SECRET")

	if shopDomain == "" || accessToken == "" {
		return nil, fmt.Errorf("SHOPIFY_SHOP_DOMAIN and SHOPIFY_API_SECRET must be set")
	}

	apiVersion := "2025-10"
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Build draft order payload
	draftOrderPayload := map[string]interface{}{
		"email":      input.Email,
		"tax_exempt": false, // Set tax_exempt to false to allow tax calculation
	}

	// Add customer information if available
	if input.Customer != nil {
		if input.Customer.Email != "" {
			draftOrderPayload["email"] = input.Customer.Email
		}
	}

	// Add note
	if input.Note != "" {
		draftOrderPayload["note"] = input.Note
	}

	// Add note attributes (custom attributes)
	if len(input.Metafields) > 0 {
		noteAttributes := make([]map[string]interface{}, len(input.Metafields))
		for i, mf := range input.Metafields {
			noteAttributes[i] = map[string]interface{}{
				"name":  mf.Key,
				"value": mf.Value,
			}
		}
		draftOrderPayload["note_attributes"] = noteAttributes
	}

	// Build line items with discount
	lineItems := make([]map[string]interface{}, 0, len(input.LineItems))
	for _, item := range input.LineItems {
		lineItem := map[string]interface{}{
			"variant_id": strings.TrimPrefix(item.VariantID, "gid://shopify/ProductVariant/"),
			"quantity":   item.Quantity,
		}

		// Add original price if available
		if item.Price != "" {
			lineItem["price"] = item.Price
		}

		// Add title if available
		if item.Title != "" {
			lineItem["title"] = item.Title
		}

		// Add discount using applied_discount (REST API format)
		if len(item.DiscountAllocations) > 0 {
			discount := item.DiscountAllocations[0]
			appliedDiscount := map[string]interface{}{
				"amount": discount.Amount,
			}
			if discount.Title != "" {
				appliedDiscount["title"] = discount.Title
			}
			// Try to determine if it's percentage or fixed
			if item.Price != "" {
				price, _ := strconv.ParseFloat(item.Price, 64)
				amount, _ := strconv.ParseFloat(discount.Amount, 64)
				if price > 0 {
					percentage := (amount / price) * 100
					appliedDiscount["value_type"] = "percentage"
					appliedDiscount["value"] = fmt.Sprintf("%.2f", percentage)
				}
			}
			lineItem["applied_discount"] = appliedDiscount
		}

		// Try adding tax_lines to line item (may not work, but worth trying)
		// Note: According to Shopify docs, tax is auto-calculated, but we'll try anyway
		if len(item.TaxLines) > 0 {
			lineItemTaxLines := make([]map[string]interface{}, len(item.TaxLines))
			for i, tl := range item.TaxLines {
				taxLine := map[string]interface{}{
					"title": tl.Title,
				}
				if tl.Rate != "" {
					if rate, err := strconv.ParseFloat(tl.Rate, 64); err == nil && rate > 0 {
						taxLine["rate"] = fmt.Sprintf("%.4f", rate)
					} else {
						taxLine["rate"] = tl.Rate
					}
				}
				if tl.PriceSet != nil && tl.PriceSet.ShopMoney != nil {
					taxLine["price"] = tl.PriceSet.ShopMoney.Amount
				}
				lineItemTaxLines[i] = taxLine
			}
			lineItem["tax_lines"] = lineItemTaxLines
		}

		lineItems = append(lineItems, lineItem)
	}
	draftOrderPayload["line_items"] = lineItems

	// Add order-level discount if available
	if input.AppliedDiscount != nil {
		orderDiscount := map[string]interface{}{
			"amount": fmt.Sprintf("%.2f", input.AppliedDiscount.Value),
		}
		if input.AppliedDiscount.Title != "" {
			orderDiscount["title"] = input.AppliedDiscount.Title
		}
		if input.AppliedDiscount.ValueType == "PERCENTAGE" {
			orderDiscount["value_type"] = "percentage"
			orderDiscount["value"] = fmt.Sprintf("%.2f", input.AppliedDiscount.Value)
		} else {
			orderDiscount["value_type"] = "fixed_amount"
		}
		draftOrderPayload["applied_discount"] = orderDiscount
	}

	// Add shipping address
	if input.ShippingAddress != nil {
		shippingAddr := map[string]interface{}{
			"address1": input.ShippingAddress.Address1,
			"city":     input.ShippingAddress.City,
			"country":  input.ShippingAddress.Country,
		}
		if input.ShippingAddress.Province != "" {
			shippingAddr["province"] = input.ShippingAddress.Province
		}
		if input.ShippingAddress.Zip != "" {
			shippingAddr["zip"] = input.ShippingAddress.Zip
		}
		if input.ShippingAddress.FirstName != "" {
			shippingAddr["first_name"] = input.ShippingAddress.FirstName
		}
		if input.ShippingAddress.LastName != "" {
			shippingAddr["last_name"] = input.ShippingAddress.LastName
		}
		if input.ShippingAddress.Phone != "" {
			shippingAddr["phone"] = input.ShippingAddress.Phone
		}
		draftOrderPayload["shipping_address"] = shippingAddr
	}

	// Add tax lines (REST API draft orders may support this)
	// According to Shopify REST API and goshopify library, tax_lines should have:
	// - title: string
	// - rate: decimal/float (not string)
	// - price: decimal/float (not string)
	if len(input.TaxLines) > 0 {
		taxLines := make([]map[string]interface{}, len(input.TaxLines))
		for i, tl := range input.TaxLines {
			taxLine := map[string]interface{}{
				"title": tl.Title,
			}
			// Rate should be float/decimal, not string
			if tl.Rate != "" {
				if rate, err := strconv.ParseFloat(tl.Rate, 64); err == nil && rate > 0 {
					taxLine["rate"] = rate // Use float
				}
			}
			// Price should also be float/decimal, not string
			if tl.PriceSet != nil && tl.PriceSet.ShopMoney != nil {
				if price, err := strconv.ParseFloat(tl.PriceSet.ShopMoney.Amount, 64); err == nil {
					taxLine["price"] = price // Use float, not string
				} else {
					// Fallback to string if parsing fails
					taxLine["price"] = tl.PriceSet.ShopMoney.Amount
				}
			}
			taxLines[i] = taxLine
		}
		fmt.Printf("Debug: Adding %d tax lines at order level with format: rate as float, price as float\n", len(taxLines))
		for i, tl := range taxLines {
			fmt.Printf("Debug: Tax line %d: %v\n", i+1, tl)
		}
		draftOrderPayload["tax_lines"] = taxLines
	}

	// Wrap in draft_order
	requestPayload := map[string]interface{}{
		"draft_order": draftOrderPayload,
	}

	jsonData, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	fmt.Printf("Debug: Creating draft order via REST API with discount and tax...\n")
	fmt.Printf("Debug: Request payload:\n%s\n\n", string(jsonData))

	// Create draft order
	url := fmt.Sprintf("https://%s/admin/api/%s/draft_orders.json", shopDomain, apiVersion)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Shopify-Access-Token", accessToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	fmt.Printf("Debug: Draft order creation response status: %d\n", resp.StatusCode)
	fmt.Printf("Debug: Response body:\n%s\n\n", string(bodyBytes))

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to create draft order: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var restResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &restResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for errors
	if errors, ok := restResponse["errors"]; ok {
		return nil, fmt.Errorf("API errors: %v", errors)
	}

	draftOrder, ok := restResponse["draft_order"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing draft_order in response")
	}

	draftOrderID, ok := draftOrder["id"].(float64)
	if !ok {
		return nil, fmt.Errorf("could not extract draft order ID")
	}

	fmt.Printf("✓ Draft order created successfully (ID: %.0f)\n", draftOrderID)

	// Check tax lines in draft order
	if taxLines, ok := draftOrder["tax_lines"].([]interface{}); ok {
		fmt.Printf("Debug: Tax lines in draft order after creation: %d\n", len(taxLines))
		for i, tl := range taxLines {
			if tlMap, ok := tl.(map[string]interface{}); ok {
				fmt.Printf("  Tax Line %d: %v\n", i+1, tlMap)
			}
		}
		// Check if custom tax_lines were applied or ignored
		if len(taxLines) > 0 {
			firstTaxLine, ok := taxLines[0].(map[string]interface{})
			if ok {
				if title, ok := firstTaxLine["title"].(string); ok {
					// Check if it's our custom tax or Shopify's auto tax
					customTaxFound := false
					for _, inputTL := range input.TaxLines {
						if title == inputTL.Title {
							customTaxFound = true
							break
						}
					}
					if !customTaxFound {
						fmt.Printf("⚠️  Custom tax_lines were ignored. Shopify used auto-calculated tax: %s\n", title)
					}
				}
			}
		}
	}

	// Complete the draft order
	fmt.Printf("\nCompleting draft order...\n")
	completeURL := fmt.Sprintf("https://%s/admin/api/%s/draft_orders/%.0f/complete.json?payment_pending=true",
		shopDomain, apiVersion, draftOrderID)

	completeReq, err := http.NewRequest("PUT", completeURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create complete request: %w", err)
	}

	completeReq.Header.Set("Content-Type", "application/json")
	completeReq.Header.Set("X-Shopify-Access-Token", accessToken)

	completeResp, err := client.Do(completeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to complete draft order: %w", err)
	}
	defer completeResp.Body.Close()

	completeBody, err := io.ReadAll(completeResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read complete response: %w", err)
	}

	fmt.Printf("Debug: Complete response status: %d\n", completeResp.StatusCode)
	fmt.Printf("Debug: Complete response body:\n%s\n\n", string(completeBody))

	if completeResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to complete draft order: status %d, body: %s", completeResp.StatusCode, string(completeBody))
	}

	var completedResponse map[string]interface{}
	if err := json.Unmarshal(completeBody, &completedResponse); err != nil {
		return nil, fmt.Errorf("failed to parse completed response: %w", err)
	}

	if errors, ok := completedResponse["errors"]; ok {
		return nil, fmt.Errorf("complete API errors: %v", errors)
	}

	completedDraftOrder, ok := completedResponse["draft_order"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing draft_order in completed response")
	}

	result := map[string]string{
		"draft_id": fmt.Sprintf("%.0f", draftOrderID),
	}

	if draftName, ok := completedDraftOrder["name"].(string); ok {
		result["draft_name"] = draftName
	}

	// Check if order was created
	if orderID, ok := completedDraftOrder["order_id"]; ok && orderID != nil {
		result["order_id"] = fmt.Sprintf("gid://shopify/Order/%.0f", orderID.(float64))
		if orderNumber, ok := completedDraftOrder["order_number"].(float64); ok {
			result["order_number"] = fmt.Sprintf("%.0f", orderNumber)
		}
		fmt.Printf("✓ Order created successfully!\n")
	} else {
		fmt.Printf("⚠ Order ID not found (may be pending payment)\n")
	}

	// Check tax lines in completed order
	if taxLines, ok := completedDraftOrder["tax_lines"].([]interface{}); ok {
		fmt.Printf("Debug: Tax lines in completed order: %d\n", len(taxLines))
		for i, tl := range taxLines {
			if tlMap, ok := tl.(map[string]interface{}); ok {
				fmt.Printf("  Tax Line %d: %v\n", i+1, tlMap)
			}
		}
	}

	return result, nil
}
