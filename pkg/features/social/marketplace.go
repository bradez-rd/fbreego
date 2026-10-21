package social

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

// MarketplaceItemInput define los datos para publicar un producto en Facebook Marketplace.
type MarketplaceItemInput struct {
	Title       string   `json:"title"`
	Price       int64    `json:"price"`
	Currency    string   `json:"currency"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Latitude    float64  `json:"latitude"`
	Longitude   float64  `json:"longitude"`
	ImageIDs    []string `json:"imageIds"`
}

// CreateMarketplaceItem publica un nuevo listado en Facebook Marketplace.
func CreateMarketplaceItem(ctx context.Context, client *http.Client, fb *types.DataFB, item MarketplaceItemInput) (map[string]any, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if item.Currency == "" {
		item.Currency = "USD"
	}

	variables := map[string]any{
		"input": map[string]any{
			"title":       item.Title,
			"price":       item.Price,
			"currency":    item.Currency,
			"description": item.Description,
			"location": map[string]float64{
				"latitude":  item.Latitude,
				"longitude": item.Longitude,
			},
			"actor_id":           fb.FacebookID,
			"client_mutation_id": utils.NextRequestBase36(),
		},
	}
	varBytes, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}

	form := utils.BuildFormAll(fb, "MarketplaceListingCreateMutation", "3719284019284715", true)
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

	form := utils.BuildFormAll(fb, "MarketplaceProductItemDetailQuery", "3619284019284716", true)
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
