package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func main() {
	shopDomain := os.Getenv("SHOPIFY_SHOP_DOMAIN")
	accessToken := os.Getenv("SHOPIFY_API_SECRET")

	if shopDomain == "" || accessToken == "" {
		log.Fatal("SHOPIFY_SHOP_DOMAIN and SHOPIFY_API_SECRET must be set in environment variables")
	}

	inputPath := "cmd/CreateOrderWithPickUpMethod/input.json"
	if len(os.Args) > 1 {
		inputPath = os.Args[1]
	}

	rawInput, err := loadRawInput(inputPath)
	if err != nil {
		log.Fatalf("failed to load input: %v", err)
	}

	// Verify input has order object
	if _, ok := rawInput["order"].(map[string]interface{}); !ok {
		log.Fatalf("input.json must contain an \"order\" object")
	}

	// Build Shopify order payload using ONLY data from input.json
	// No additional data will be added - everything must come from input.json
	shopifyOrder, err := buildShopifyOrderPayload(rawInput)
	if err != nil {
		log.Fatalf("failed to build Shopify order payload: %v", err)
	}

	bodyBytes, err := json.Marshal(map[string]interface{}{
		"order": shopifyOrder,
	})
	if err != nil {
		log.Fatalf("failed to marshal order payload: %v", err)
	}

	url := fmt.Sprintf("https://%s/admin/api/2025-10/orders.json", shopDomain)
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		log.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Shopify-Access-Token", accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	var respData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		log.Fatalf("failed to parse response: %v", err)
	}

	if resp.StatusCode >= 400 {
		log.Fatalf("Shopify returned %d: %+v", resp.StatusCode, respData)
	}

	// Print minimal order info for quick verification.
	order, ok := respData["order"].(map[string]interface{})
	if ok {
		fmt.Println("✓ Order created successfully with pickup delivery method")
		fmt.Printf("Order ID: %v\n", order["id"])
		fmt.Printf("Order Name: %v\n", order["name"])
		if shippingLines, ok := order["shipping_lines"].([]interface{}); ok && len(shippingLines) > 0 {
			if sl, ok := shippingLines[0].(map[string]interface{}); ok {
				fmt.Printf("Delivery Method: %v (delivery_category=%v)\n", sl["title"], sl["delivery_category"])
			}
		}
		if fo, ok := order["fulfillment_status"].(string); ok {
			fmt.Printf("Fulfillment Status: %s\n", fo)
		}
	} else {
		fmt.Printf("Order created, raw response: %+v\n", respData)
	}
}

// fetchFirstLocationID calls Shopify REST API to get locations and returns the first ID.
// NOTE: This is a convenience fallback; prefer setting SHOPIFY_PICKUP_LOCATION_ID explicitly.
func fetchFirstLocationID(shopDomain, accessToken string) (string, error) {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("curl -s -H \"X-Shopify-Access-Token: %s\" https://%s/admin/api/2025-10/locations.json", accessToken, shopDomain))
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("curl locations failed: %w", err)
	}
	var resp struct {
		Locations []struct {
			ID json.Number `json:"id"`
			// There is no explicit pickup flag in this endpoint; assumes location is pickup-enabled.
		} `json:"locations"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", fmt.Errorf("parse locations failed: %w", err)
	}
	if len(resp.Locations) == 0 {
		return "", fmt.Errorf("no locations returned from API")
	}
	return resp.Locations[0].ID.String(), nil
}

func loadRawInput(path string) (map[string]interface{}, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read file: %w", err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return data, nil
}

// buildShopifyOrderPayload copies all data from input.json and only transforms field names needed for Shopify API
// Does NOT add any data that is not in input.json
func buildShopifyOrderPayload(raw map[string]interface{}) (map[string]interface{}, error) {
	orderRaw, ok := raw["order"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("input missing order object")
	}

	// Deep copy the entire order object to preserve all data
	result := deepCopyMap(orderRaw)

	// Only transform field names that Shopify REST API requires (camelCase -> snake_case)
	// Keep ALL other fields as-is from input.json

	// Transform shippingAddress -> shipping_address (keep all fields)
	if sa, ok := result["shippingAddress"].(map[string]interface{}); ok {
		delete(result, "shippingAddress")
		shopifyAddr := map[string]interface{}{}
		for k, v := range sa {
			// Transform camelCase to snake_case for Shopify
			switch k {
			case "firstName":
				shopifyAddr["first_name"] = v
			case "lastName":
				shopifyAddr["last_name"] = v
			case "address1", "street":
				shopifyAddr["address1"] = coerceString(sa["address1"], sa["street"])
			case "province", "provinceCode":
				if _, exists := shopifyAddr["province"]; !exists {
					shopifyAddr["province"] = coerceString(sa["province"], sa["provinceCode"])
				}
			case "country", "countryCode":
				if _, exists := shopifyAddr["country"]; !exists {
					shopifyAddr["country"] = coerceString(sa["country"], sa["countryCode"])
				}
			default:
				shopifyAddr[k] = v // Keep other fields as-is
			}
		}
		result["shipping_address"] = shopifyAddr
	}

	// Transform billingAddress -> billing_address (keep all fields)
	if ba, ok := result["billingAddress"].(map[string]interface{}); ok && ba != nil {
		delete(result, "billingAddress")
		shopifyAddr := map[string]interface{}{}
		for k, v := range ba {
			switch k {
			case "firstName":
				shopifyAddr["first_name"] = v
			case "lastName":
				shopifyAddr["last_name"] = v
			case "address1", "street":
				shopifyAddr["address1"] = coerceString(ba["address1"], ba["street"])
			case "province", "provinceCode":
				if _, exists := shopifyAddr["province"]; !exists {
					shopifyAddr["province"] = coerceString(ba["province"], ba["provinceCode"])
				}
			case "country", "countryCode":
				if _, exists := shopifyAddr["country"]; !exists {
					shopifyAddr["country"] = coerceString(ba["country"], ba["countryCode"])
				}
			default:
				shopifyAddr[k] = v
			}
		}
		result["billing_address"] = shopifyAddr
	}

	// Transform customer (keep all fields)
	if c, ok := result["customer"].(map[string]interface{}); ok {
		shopifyCustomer := map[string]interface{}{}
		for k, v := range c {
			switch k {
			case "firstName":
				shopifyCustomer["first_name"] = v
			case "lastName":
				shopifyCustomer["last_name"] = v
			default:
				shopifyCustomer[k] = v // Keep other fields
			}
		}
		result["customer"] = shopifyCustomer
	}

	// Transform items -> line_items (keep all item fields, only transform variant_id format)
	if items, ok := result["items"].([]interface{}); ok {
		delete(result, "items")
		var lineItems []map[string]interface{}
		for _, it := range items {
			m, ok := it.(map[string]interface{})
			if !ok {
				continue
			}
			// Copy all fields from item
			lineItem := deepCopyMap(m)
			// Only transform variant_id format (productId/variantId -> variant_id as int64)
			if variantID := coerceString(m["productId"], m["variantId"]); variantID != "" {
				lineItem["variant_id"] = mustParseVariantID(variantID)
				delete(lineItem, "productId")
				delete(lineItem, "variantId")
			}
			// Transform name -> title if needed (Shopify uses title)
			if name, ok := m["name"].(string); ok {
				lineItem["title"] = name
			}
			lineItems = append(lineItems, lineItem)
		}
		result["line_items"] = lineItems
	}

	// Transform noteAttributes -> note_attributes (keep all)
	if na, ok := result["noteAttributes"].([]interface{}); ok {
		delete(result, "noteAttributes")
		var noteAttrs []map[string]interface{}
		for _, n := range na {
			if m, ok := n.(map[string]interface{}); ok {
				noteAttrs = append(noteAttrs, deepCopyMap(m))
			}
		}
		result["note_attributes"] = noteAttrs
	}

	// Transform source -> source_name
	if v, ok := result["source"].(string); ok && v != "" {
		result["source_name"] = v
		delete(result, "source")
	}

	// Transform status -> financial_status (Shopify REST API uses financial_status)
	if v, ok := result["status"].(string); ok && v != "" {
		result["financial_status"] = v
		delete(result, "status")
	}

	// Transform locationId -> location_id (only if present in input)
	if v, ok := result["locationId"].(string); ok && v != "" {
		result["location_id"] = v
		delete(result, "locationId")
	}
	// If location_id already exists, keep it as-is
	// Do NOT add location_id if not in input - all data must come from input.json only

	// Handle tags: if empty string, remove it (Shopify doesn't accept empty tags)
	if tags, ok := result["tags"].(string); ok && strings.TrimSpace(tags) == "" {
		delete(result, "tags")
	}

	// Create shipping_lines from shippingMethod/shippingMethodTitle and totalShipping if available
	// Only if these fields have values in input.json
	if _, exists := result["shipping_lines"]; !exists {
		var title string
		if v, ok := result["shippingMethodTitle"].(string); ok && strings.TrimSpace(v) != "" {
			title = strings.TrimSpace(v)
		} else if v, ok := result["shippingMethod"].(string); ok && strings.TrimSpace(v) != "" {
			title = strings.TrimSpace(v)
		}

		var price string
		if v, ok := result["totalShipping"].(string); ok && strings.TrimSpace(v) != "" {
			price = strings.TrimSpace(v)
		} else if v, ok := result["totalShippingIncTax"].(string); ok && strings.TrimSpace(v) != "" {
			price = strings.TrimSpace(v)
		}

		// Only create shipping_lines if we have both title and price from input
		if title != "" && price != "" {
			code := strings.ToUpper(strings.ReplaceAll(title, " ", ""))
			// If title is not empty, code will never be empty (title already trimmed)

			// Get discounted_price from input if available, otherwise use price
			discountedPrice := price
			if v, ok := result["totalShippingExTax"].(string); ok && strings.TrimSpace(v) != "" {
				discountedPrice = strings.TrimSpace(v)
			}

			shippingLine := map[string]interface{}{
				"title":              title,
				"price":              price,
				"code":               code,
				"source":             "custom", // Required by Shopify API
				"delivery_category":  "pickup", // Required for pickup orders
				"discounted_price":   discountedPrice,
				"requires_shipping":  false,    // Required for pickup
				"carrier_identifier": "pickup", // Default, can be overridden below
			}

			// Add carrier info if available from input (priority: shippingMethodCarrierCode > shippingCarrier)
			if v, ok := result["shippingMethodCarrierCode"].(string); ok && strings.TrimSpace(v) != "" {
				shippingLine["carrier_identifier"] = strings.TrimSpace(v)
			} else if v, ok := result["shippingCarrier"].(string); ok && strings.TrimSpace(v) != "" {
				shippingLine["carrier_identifier"] = strings.TrimSpace(v)
			}

			result["shipping_lines"] = []map[string]interface{}{shippingLine}

			// Remove shippingMethod fields from result (they're not part of Shopify order API)
			// These fields were only used to create shipping_lines
			delete(result, "shippingMethod")
			delete(result, "shippingMethodTitle")
			delete(result, "shippingMethodCarrierTitle")
			delete(result, "shippingMethodCarrierCode")
			delete(result, "shippingMethodId")
			delete(result, "totalShipping")
			delete(result, "totalShippingIncTax")
			delete(result, "totalShippingExTax")
			delete(result, "totalTaxShipping")
			delete(result, "shippingCarrier")
		}
	}
	// If shipping_lines already present in input, keep as-is
	// If shippingMethod/totalShipping not available, don't add anything (keep current logic)

	return result, nil
}

// deepCopyMap creates a deep copy of a map[string]interface{}
func deepCopyMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		switch val := v.(type) {
		case map[string]interface{}:
			result[k] = deepCopyMap(val)
		case []interface{}:
			result[k] = deepCopySlice(val)
		default:
			result[k] = v
		}
	}
	return result
}

// deepCopySlice creates a deep copy of a []interface{}
func deepCopySlice(s []interface{}) []interface{} {
	result := make([]interface{}, len(s))
	for i, v := range s {
		switch val := v.(type) {
		case map[string]interface{}:
			result[i] = deepCopyMap(val)
		case []interface{}:
			result[i] = deepCopySlice(val)
		default:
			result[i] = v
		}
	}
	return result
}

func coerceString(values ...interface{}) string {
	for _, v := range values {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// mustParseVariantID accepts either a numeric ID or a Shopify gid:// string.
// REST order create expects the numeric variant ID.
func mustParseVariantID(id string) int64 {
	id = strings.TrimSpace(id)
	if id == "" {
		return 0
	}
	if strings.HasPrefix(id, "gid://") {
		parts := strings.Split(id, "/")
		id = parts[len(parts)-1]
	}
	val, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		log.Fatalf("variantId must be numeric or gid://ProductVariant/<id>; got %q", id)
	}
	return val
}
