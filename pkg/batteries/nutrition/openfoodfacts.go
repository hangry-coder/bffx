package nutrition

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type OpenFoodFactsProvider struct {
	client *http.Client
}

func NewOpenFoodFactsProvider() *OpenFoodFactsProvider {
	return &OpenFoodFactsProvider{client: &http.Client{}}
}

func (p *OpenFoodFactsProvider) Type() string {
	return "openfoodfacts"
}

func (p *OpenFoodFactsProvider) FetchNutrition(ctx context.Context, query string) (*NutritionData, error) {
	// Simple search by name
	apiURL := fmt.Sprintf("https://world.openfoodfacts.org/cgi/search.pl?search_terms=%s&json=1&page_size=1", url.QueryEscape(query))
	
	req, _ := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openfoodfacts api returned status %d", resp.StatusCode)
	}

	var result struct {
		Products []struct {
			ProductName string `json:"product_name"`
			Nutriments  struct {
				EnergyKcal100g float64 `json:"energy-kcal_100g"`
				Proteins100g   float64 `json:"proteins_100g"`
				Carbohydrates100g float64 `json:"carbohydrates_100g"`
				Fat100g        float64 `json:"fat_100g"`
				Fiber100g      float64 `json:"fiber_100g"`
			} `json:"nutriments"`
		} `json:"products"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Products) == 0 {
		return nil, fmt.Errorf("no products found for query: %s", query)
	}

	prod := result.Products[0]
	return &NutritionData{
		Name:        prod.ProductName,
		Calories:    prod.Nutriments.EnergyKcal100g,
		Protein:     prod.Nutriments.Proteins100g,
		Carbs:       prod.Nutriments.Carbohydrates100g,
		Fat:         prod.Nutriments.Fat100g,
		Fiber:       prod.Nutriments.Fiber100g,
		ServingSize: 100,
		ServingUnit: "g",
	}, nil
}

type NoopProvider struct{}

func NewNoopProvider() *NoopProvider { return &NoopProvider{} }
func (p *NoopProvider) Type() string { return "noop" }
func (p *NoopProvider) FetchNutrition(ctx context.Context, query string) (*NutritionData, error) {
	return nil, fmt.Errorf("nutrition battery not configured")
}
