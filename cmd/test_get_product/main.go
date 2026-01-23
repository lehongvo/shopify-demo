package main

import (
	"encoding/json"
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

	fmt.Println("=== Getting 10 Products from Shopify ===")
	fmt.Printf("Shop Domain: %s\n", shopDomain)
	fmt.Printf("Access Token: %s...%s\n\n", accessToken[:10], accessToken[len(accessToken)-10:])

	// GraphQL query để lấy 10 products
	const query = `
		query GetProducts($first: Int!) {
			products(first: $first) {
				edges {
					node {
						id
						title
						description
						handle
						status
						productType
						vendor
						createdAt
						updatedAt
						variants(first: 5) {
							edges {
								node {
									id
									title
									price
									sku
									barcode
									inventoryQuantity
								}
							}
						}
						images(first: 3) {
							edges {
								node {
									id
									url
									altText
								}
							}
						}
					}
				}
				pageInfo {
					hasNextPage
					endCursor
				}
			}
		}`

	variables := map[string]interface{}{
		"first": 10,
	}

	resp, err := app.CallAdminGraphQL(query, variables)
	if err != nil {
		log.Fatalf("Error calling Shopify API: %v", err)
	}

	// Pretty print full response
	jsonData, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println("=== Full Response ===")
	fmt.Println(string(jsonData))

	// Parse và hiển thị thông tin products
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		log.Fatal("No data in response")
	}

	products, ok := data["products"].(map[string]interface{})
	if !ok {
		log.Fatal("Products not found in response")
	}

	edges, ok := products["edges"].([]interface{})
	if !ok {
		log.Fatal("No edges found in products")
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Total products retrieved: %d\n\n", len(edges))

	// Hiển thị thông tin từng product
	for i, edge := range edges {
		edgeMap, ok := edge.(map[string]interface{})
		if !ok {
			continue
		}

		node, ok := edgeMap["node"].(map[string]interface{})
		if !ok {
			continue
		}

		fmt.Printf("--- Product %d ---\n", i+1)
		fmt.Printf("ID: %v\n", node["id"])
		fmt.Printf("Title: %v\n", node["title"])
		fmt.Printf("Handle: %v\n", node["handle"])
		fmt.Printf("Status: %v\n", node["status"])
		if node["vendor"] != nil {
			fmt.Printf("Vendor: %v\n", node["vendor"])
		}
		if node["productType"] != nil {
			fmt.Printf("Product Type: %v\n", node["productType"])
		}
		if node["description"] != nil {
			desc := fmt.Sprintf("%v", node["description"])
			if len(desc) > 100 {
				desc = desc[:100] + "..."
			}
			fmt.Printf("Description: %s\n", desc)
		}

		// Variants
		if variants, ok := node["variants"].(map[string]interface{}); ok {
			if variantEdges, ok := variants["edges"].([]interface{}); ok {
				fmt.Printf("Variants (%d):\n", len(variantEdges))
				for j, vEdge := range variantEdges {
					if vEdgeMap, ok := vEdge.(map[string]interface{}); ok {
						if vNode, ok := vEdgeMap["node"].(map[string]interface{}); ok {
							fmt.Printf("  Variant %d: %v - Price: %v", j+1, vNode["title"], vNode["price"])
							if vNode["sku"] != nil {
								fmt.Printf(" - SKU: %v", vNode["sku"])
							}
							if vNode["inventoryQuantity"] != nil {
								fmt.Printf(" - Inventory: %v", vNode["inventoryQuantity"])
							}
							fmt.Println()
						}
					}
				}
			}
		}

		// Images
		if images, ok := node["images"].(map[string]interface{}); ok {
			if imageEdges, ok := images["edges"].([]interface{}); ok {
				if len(imageEdges) > 0 {
					fmt.Printf("Images (%d):\n", len(imageEdges))
					for j, imgEdge := range imageEdges {
						if imgEdgeMap, ok := imgEdge.(map[string]interface{}); ok {
							if imgNode, ok := imgEdgeMap["node"].(map[string]interface{}); ok {
								fmt.Printf("  Image %d: %v\n", j+1, imgNode["url"])
							}
						}
					}
				}
			}
		}

		fmt.Println()
	}

	// Page info
	if pageInfo, ok := products["pageInfo"].(map[string]interface{}); ok {
		fmt.Println("=== Page Info ===")
		fmt.Printf("Has Next Page: %v\n", pageInfo["hasNextPage"])
		if pageInfo["endCursor"] != nil {
			fmt.Printf("End Cursor: %v\n", pageInfo["endCursor"])
		}
	}
}
