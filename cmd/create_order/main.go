package main

import (
	"fmt"
	"log"
	"os"

	"shopify-demo/app"
)

func main() {
	// Kiểm tra environment variables
	shopDomain := os.Getenv("SHOPIFY_SHOP_DOMAIN")
	accessToken := os.Getenv("SHOPIFY_API_SECRET")

	if shopDomain == "" || accessToken == "" {
		log.Fatal("SHOPIFY_SHOP_DOMAIN and SHOPIFY_API_SECRET must be set in environment variables")
	}

	fmt.Println("Creating order with discount...")
	fmt.Println("Shop Domain:", shopDomain)
	fmt.Println("---")

	// Tạo draft order với line items và discount
	draftInput := app.DraftOrderInput{
		Email: "customer@example.com",
		LineItems: []app.DraftLineItemInput{
			{
				VariantID: "gid://shopify/ProductVariant/48360775188720", // Thay bằng variant ID thực tế
				Quantity:  1,
				AppliedDiscount: &app.AppliedDiscountInput{
					Description: "20% off",
					ValueType:   "PERCENTAGE",
					Value:       20.0,
					Title:       "20PERCENT",
				},
			},
		},
		ShippingAddress: &app.MailingAddressInput{
			Address1:  "123 Main Street",
			City:      "New York",
			Province:  "NY",
			Country:   "US",
			Zip:       "10001",
			FirstName: "John",
			LastName:  "Doe",
			Phone:     "+1234567890",
		},
		BillingAddress: &app.MailingAddressInput{
			Address1:  "123 Main Street",
			City:      "New York",
			Province:  "NY",
			Country:   "US",
			Zip:       "10001",
			FirstName: "John",
			LastName:  "Doe",
		},
		Note: "Order created via API with discount",
		Tags: []string{"api", "test", "discount"},
	}

	// Tạo order từ draft (paymentPending=false nghĩa là order sẽ được đánh dấu là đã thanh toán)
	orderInfo, err := app.CreateOrderFromDraft(draftInput, false)
	if err != nil {
		log.Fatalf("Failed to create order: %v", err)
	}

	// In thông tin order
	fmt.Println("✓ Order created successfully!")
	fmt.Println("---")
	fmt.Printf("Order ID: %s\n", orderInfo.OrderID)
	fmt.Printf("Order Name: %s\n", orderInfo.OrderName)
	fmt.Printf("Draft ID: %s\n", orderInfo.DraftID)
	fmt.Printf("Draft Name: %s\n", orderInfo.DraftName)

	// Hiển thị FulfillmentOrders (tự động tạo bởi Shopify)
	fmt.Println("\n--- Fulfillment Orders (Auto-created by Shopify) ---")
	if len(orderInfo.FulfillmentOrders) > 0 {
		for i, fo := range orderInfo.FulfillmentOrders {
			fmt.Printf("\nFulfillmentOrder #%d:\n", i+1)
			fmt.Printf("  ID: %s\n", fo.ID)
			fmt.Printf("  Status: %s\n", fo.Status)
			fmt.Printf("  Request Status: %s\n", fo.RequestStatus)
			fmt.Printf("  Assigned Location ID: %s\n", fo.AssignedLocationID)
			fmt.Printf("  Line Items: %d\n", len(fo.LineItems))
			for j, li := range fo.LineItems {
				fmt.Printf("    [%d] LineItem ID: %s, Quantity: %d\n", j+1, li.LineItemID, li.Quantity)
			}
		}
	} else {
		fmt.Println("No FulfillmentOrders found (may need to wait a moment for Shopify to create them)")
	}

	fmt.Println("\n---")
	fmt.Println("Note: FulfillmentOrders are automatically created by Shopify when draftOrderComplete is called.")
	fmt.Println("To fulfill the order, use app.CreateFulfillment() function.")
}