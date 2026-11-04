package social

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bradez-rd/fbreego/pkg/core/contracts"
	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

// MarketplaceCategoryIDs mapea los nombres canónicos de categoría a sus IDs oficiales en Facebook.
var MarketplaceCategoryIDs = map[string]int64{
	"Tools":                    2171028376552553,
	"Furniture":                1583634935226685,
	"Household":                1569171756675761,
	"Garden":                   800089866739547,
	"Appliances":               678754142233400,
	"Video Games":              686977074745292,
	"Books Movies&Music":       613858625416355,
	"Bags & Luggage":           1567543000236608,
	"Women's clothing & shoes": 1266429133383966,
	"Men's clothing & shoes":   931157863635831,
	"Jewelry & Accessories":    214968118845643,
	"Health & beauty":          1555452698044988,
	"Pet Supplies":             1550246318620997,
	"Baby & kids":              624859874282116,
	"Toys & Games":             606456512821491,
	"Electronics & computers":  1792291877663080,
	"Mobile phones":            1557869527812749,
	"Bicycles":                 1658310421102081,
	"Arts & Crafts":            1534799543476160,
	"Sports & Outdoors":        1383948661922113,
	"Auto parts":               757715671026531,
	"Musical Instruments":      676772489112490,
	"Antiques & Collectibles":  393860164117441,
	"Garage Sale":              1834536343472201,
	"Miscellaneous":            895487550471874,
}

// ResolveCategoryID resuelve el ID de categoría de Meta a partir de un nombre amigable o ID en string.
func ResolveCategoryID(category string) int64 {
	trimmed := strings.TrimSpace(category)
	if trimmed == "" {
		return MarketplaceCategoryIDs["Electronics & computers"]
	}
	for k, v := range MarketplaceCategoryIDs {
		if strings.EqualFold(k, trimmed) {
			return v
		}
	}
	// Fallback estándar
	return MarketplaceCategoryIDs["Electronics & computers"]
}

// MarketplaceSearchParams define los filtros para la búsqueda de artículos en Marketplace.
type MarketplaceSearchParams struct {
	Query     string  `json:"query"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	RadiusKm  int     `json:"radiusKm"`
	MinPrice  int64   `json:"minPrice"`
	MaxPrice  int64   `json:"maxPrice"`
	Limit     int     `json:"limit"`
	Cursor    string  `json:"cursor,omitempty"`
}

// MarketplaceListingItem representa un artículo devuelto por la búsqueda en Marketplace.
type MarketplaceListingItem struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Price      string `json:"price"`
	Location   string `json:"location"`
	ImageURL   string `json:"imageUrl"`
	SellerName string `json:"sellerName"`
	PostedDate string `json:"postedDate"`
	URL        string `json:"url"`
	IsPending  bool   `json:"isPending"`
}

// MarketplaceSearchResult estructura la respuesta paginada de búsqueda.
type MarketplaceSearchResult struct {
	Listings    []MarketplaceListingItem `json:"listings"`
	HasNextPage bool                     `json:"hasNextPage"`
	EndCursor   string                   `json:"endCursor,omitempty"`
	Raw         map[string]any           `json:"raw,omitempty"`
}

// SearchMarketplace ejecuta una búsqueda pública en Facebook Marketplace utilizando GraphQL y la sesión activa.
func SearchMarketplace(ctx context.Context, client *http.Client, fb *types.DataFB, params MarketplaceSearchParams) (*MarketplaceSearchResult, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.RadiusKm <= 0 {
		params.RadiusKm = 50
	}
	lat := params.Latitude
	lng := params.Longitude
	if lat == 0 && lng == 0 {
		lat = 18.4861
		lng = -69.9312
	}

	browseParams := map[string]any{
		"commerce_enable_local_pickup":      true,
		"commerce_enable_shipping":          true,
		"commerce_search_and_rp_available":  true,
		"commerce_search_and_rp_condition":  nil,
		"commerce_search_and_rp_ctime_days": nil,
		"filter_location_latitude":          lat,
		"filter_location_longitude":         lng,
		"filter_radius_km":                  params.RadiusKm,
		"filter_price_lower_bound":          params.MinPrice * 100,
	}
	if params.MaxPrice > 0 {
		browseParams["filter_price_upper_bound"] = params.MaxPrice * 100
	} else {
		browseParams["filter_price_upper_bound"] = 214748364700
	}

	variables := map[string]any{
		"count": params.Limit,
		"params": map[string]any{
			"bqf": map[string]any{
				"callsite": "COMMERCE_MKTPLACE_WWW",
				"query":    params.Query,
			},
			"browse_request_params": browseParams,
			"custom_request_params": map[string]any{
				"surface": "SEARCH",
			},
		},
	}
	if params.Cursor != "" {
		variables["cursor"] = params.Cursor
	}

	varBytes, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}

	docID := contracts.DefaultRegistry().GetDocID("MarketplaceSearchQuery")
	if docID == "" {
		docID = contracts.DefaultDocIDMarketplaceSearch
	}

	form := utils.BuildFormAll(fb, "MarketplaceSearchQuery", docID, true)
	form.Set("variables", string(varBytes))

	res, err := executePost(ctx, client, fb, GraphQLURL, form)
	if err != nil {
		return nil, err
	}

	return ParseMarketplaceSearchResult(res), nil
}

// ParseMarketplaceSearchResult decodifica la respuesta JSON de GraphQL de Marketplace en una lista normalizada.
func ParseMarketplaceSearchResult(res map[string]any) *MarketplaceSearchResult {
	result := &MarketplaceSearchResult{
		Listings: make([]MarketplaceListingItem, 0),
		Raw:      res,
	}

	dataMap, _ := res["data"].(map[string]any)
	if dataMap == nil {
		return result
	}

	mktSearch, _ := dataMap["marketplace_search"].(map[string]any)
	if mktSearch == nil {
		return result
	}

	feedUnits, _ := mktSearch["feed_units"].(map[string]any)
	if feedUnits == nil {
		return result
	}

	if pageInfo, ok := feedUnits["page_info"].(map[string]any); ok {
		if hasNext, ok := pageInfo["has_next_page"].(bool); ok {
			result.HasNextPage = hasNext
		}
		if endCursor, ok := pageInfo["end_cursor"].(string); ok {
			result.EndCursor = endCursor
		}
	}

	edges, _ := feedUnits["edges"].([]any)
	for _, edgeObj := range edges {
		edge, _ := edgeObj.(map[string]any)
		if edge == nil {
			continue
		}
		node, _ := edge["node"].(map[string]any)
		if node == nil {
			continue
		}
		var item MarketplaceListingItem
		listing, _ := node["listing"].(map[string]any)
		if listing != nil {
			item = MarketplaceListingItem{
				ID:    fmt.Sprintf("%v", listing["id"]),
				Title: fmt.Sprintf("%v", listing["marketplace_listing_title"]),
				URL:   fmt.Sprintf("https://www.facebook.com/marketplace/item/%v/", listing["id"]),
			}
		} else if storyKey, ok := node["story_key"].(string); ok && storyKey != "" {
			item = MarketplaceListingItem{
				ID:    storyKey,
				Title: fmt.Sprintf("Marketplace Listing %s", storyKey),
				URL:   fmt.Sprintf("https://www.facebook.com/marketplace/item/%s/", storyKey),
			}
		} else if nodeID, ok := node["id"].(string); ok && nodeID != "" {
			item = MarketplaceListingItem{
				ID:    nodeID,
				Title: fmt.Sprintf("Marketplace Listing %s", nodeID),
				URL:   fmt.Sprintf("https://www.facebook.com/marketplace/item/%s/", nodeID),
			}
		} else {
			continue
		}

		if priceMap, ok := listing["listing_price"].(map[string]any); ok {
			if formatted, ok := priceMap["formatted_amount"].(string); ok && formatted != "" {
				item.Price = formatted
			} else if amt, ok := priceMap["amount"]; ok {
				item.Price = fmt.Sprintf("%v", amt)
			}
		}

		if locMap, ok := listing["location"].(map[string]any); ok {
			if revGeocode, ok := locMap["reverse_geocode"].(map[string]any); ok {
				if cityPage, ok := revGeocode["city_page"].(map[string]any); ok {
					item.Location = fmt.Sprintf("%v", cityPage["display_name"])
				} else if city, ok := revGeocode["city"].(string); ok {
					item.Location = city
				}
			}
		}

		if photoMap, ok := listing["primary_listing_photo"].(map[string]any); ok {
			if imgMap, ok := photoMap["image"].(map[string]any); ok {
				item.ImageURL = fmt.Sprintf("%v", imgMap["uri"])
			}
		}

		if sellerMap, ok := listing["marketplace_listing_seller"].(map[string]any); ok {
			item.SellerName = fmt.Sprintf("%v", sellerMap["name"])
		}

		if cTime, ok := listing["creation_time"].(float64); ok && cTime > 0 {
			item.PostedDate = time.Unix(int64(cTime), 0).UTC().Format(time.RFC3339)
		}

		if pending, ok := listing["is_pending"].(bool); ok {
			item.IsPending = pending
		}

		result.Listings = append(result.Listings, item)
	}

	return result
}

// MarketplaceItemInput define los datos para publicar un producto en Facebook Marketplace.
type MarketplaceItemInput struct {
	Title       string   `json:"title"`
	Brand       string   `json:"brand,omitempty"`
	Price       int64    `json:"price"`
	Currency    string   `json:"currency"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Latitude    float64  `json:"latitude"`
	Longitude   float64  `json:"longitude"`
	ImageIDs    []string `json:"imageIds"`
	MediaIDs    []string `json:"mediaIds,omitempty"`
	Hashtags    []string `json:"hashtags,omitempty"`
}

// CreateMarketplaceItem publica un nuevo listado en Facebook Marketplace usando la mutación Comet de Facebook.
func CreateMarketplaceItem(ctx context.Context, client *http.Client, fb *types.DataFB, item MarketplaceItemInput) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if item.Currency == "" {
		item.Currency = "USD"
	}
	if strings.TrimSpace(item.Title) == "" {
		return nil, fmt.Errorf("el título del producto es obligatorio")
	}

	lat := item.Latitude
	lng := item.Longitude
	if lat == 0 && lng == 0 {
		lat = 18.4861
		lng = -69.9312
	}

	categoryID := ResolveCategoryID(item.Category)

	brand := item.Brand
	if brand == "" {
		brand = "Generic"
	}

	attributeData := map[string]any{
		"vt_attributes_free_form": map[string]string{
			"372885700169792": brand,
		},
		"vt_attributes_normalized": map[string]any{},
		"condition":               "new",
		"brand":                   brand,
	}
	attrBytes, _ := json.Marshal(attributeData)

	photoIDs := make([]string, 0)
	for _, id := range item.ImageIDs {
		if strings.TrimSpace(id) != "" {
			photoIDs = append(photoIDs, strings.TrimSpace(id))
		}
	}

	nowMs := time.Now().UnixNano() / int64(time.Millisecond)
	attributionID := fmt.Sprintf("CometMarketplaceComposerRoot.react,comet.marketplace.composer,via_cold_start,%d,%s,1606854132932955,", nowMs, fb.Jazoest)

	hashtags := item.Hashtags
	if hashtags == nil {
		hashtags = []string{}
	}

	priceStr := fmt.Sprintf("%d", item.Price)

	variables := map[string]any{
		"input": map[string]any{
			"client_mutation_id": fmt.Sprintf("%d", utils.GenerateSessionID()),
			"actor_id":           fb.FacebookID,
			"attribution_id_v2":  attributionID,
			"audience": map[string]any{
				"marketplace": map[string]any{
					"marketplace_id": fmt.Sprintf("%d", categoryID),
				},
			},
			"data": map[string]any{
				"common": map[string]any{
					"attribute_data_json":            string(attrBytes),
					"category_id":                    categoryID,
					"commerce_shipping_carrier":      nil,
					"commerce_shipping_carriers":     []any{},
					"comparable_price":               "null",
					"cost_per_additional_item":       nil,
					"delivery_types":                 []string{"IN_PERSON"},
					"description":                    map[string]any{"text": item.Description},
					"draft_type":                     nil,
					"hidden_from_friends_visibility": "VISIBLE_TO_EVERYONE",
					"is_personalization_required":    nil,
					"is_preview":                     false,
					"item_price": map[string]any{
						"currency": strings.ToUpper(item.Currency),
						"price":    priceStr,
					},
					"latitude":                            lat,
					"longitude":                           lng,
					"min_acceptable_checkout_offer_price": "null",
					"personalization_info":                nil,
					"product_hashtag_names":               hashtags,
					"quantity":                            -1,
					"shipping_calculation_logic_version":  nil,
					"shipping_cost_option":                "BUYER_PAID_SHIPPING",
					"shipping_cost_range_lower_cost":      nil,
					"shipping_cost_range_upper_cost":      nil,
					"shipping_label_price":                "0",
					"shipping_label_rate_code":            nil,
					"shipping_label_rate_type":            nil,
					"shipping_offered":                    false,
					"shipping_options_data":               []any{},
					"shipping_package_weight":             nil,
					"shipping_price":                      "null",
					"shipping_service_type":               nil,
					"sku":                                 "",
					"source_type":                         "marketplace_unknown",
					"suggested_hashtag_names":             []any{},
					"surface":                             "composer",
					"title":                               strings.TrimSpace(item.Title),
					"variants":                            []any{},
					"video_ids":                           []any{},
					"xpost_target_ids":                    []any{},
					"photo_ids":                           photoIDs,
				},
			},
		},
	}

	varBytes, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}

	docID := contracts.DefaultRegistry().GetDocID("MarketplaceCreateMutation")
	if docID == "" {
		docID = contracts.DefaultDocIDMarketplaceCreate
	}

	form := utils.BuildFormAll(fb, "useCometMarketplaceListingCreateMutation", docID, true)
	form.Set("variables", string(varBytes))

	return executePost(ctx, client, fb, GraphQLURL, form)
}

// GetMarketplaceItem consulta los detalles y precio de un producto en Marketplace por su ID.
func GetMarketplaceItem(ctx context.Context, client *http.Client, fb *types.DataFB, itemID string) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}

	variables := map[string]any{
		"target_id": itemID,
	}
	varBytes, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}

	docID := contracts.DefaultRegistry().GetDocID("MarketplaceDetailQuery")
	if docID == "" {
		docID = contracts.DefaultDocIDMarketplaceDetail
	}

	form := utils.BuildFormAll(fb, "MarketplaceProductItemDetailQuery", docID, true)
	form.Set("variables", string(varBytes))

	res, err := executePost(ctx, client, fb, GraphQLURL, form)
	if err != nil {
		return nil, err
	}

	dataMap, _ := res["data"].(map[string]any)
	if dataMap == nil {
		return nil, fmt.Errorf("producto de marketplace no encontrado: %s", itemID)
	}
	return dataMap, nil
}
