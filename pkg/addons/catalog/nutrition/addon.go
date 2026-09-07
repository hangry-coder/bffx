// Package nutrition provides the Open Food Facts catalog addon for ingestion pipelines.
// This is an addon (domain catalog adapter), not a batteries.nutrition provider.
// Prefer spec.catalog.adapter: openfoodfacts on Pipeline manifests; batteries.nutrition is deprecated.
package nutrition

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// AddonCatalog implements nutrition lookup using the Open Food Facts API.
type AddonCatalog struct {
	client *http.Client
}

func NewAddonCatalog() *AddonCatalog {
	return &AddonCatalog{client: &http.Client{}}
}

// Source identifies this catalog adapter for pipeline observability.
func (c *AddonCatalog) Source() string {
	return "openfoodfacts"
}

// Resolve queries world.openfoodfacts.org.
func (c *AddonCatalog) Resolve(ctx context.Context, query string, hints map[string]string) (any, error) {
	if query == "" {
		return nil, fmt.Errorf("query is empty")
	}

	apiURL := fmt.Sprintf("https://world.openfoodfacts.org/cgi/search.pl?search_terms=%s&json=1&page_size=1", url.QueryEscape(query))
	
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
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
				EnergyKcal100g    float64 `json:"energy-kcal_100g"`
				Proteins100g       float64 `json:"proteins_100g"`
				Carbohydrates100g float64 `json:"carbohydrates_100g"`
				Fat100g            float64 `json:"fat_100g"`
				Fiber100g          float64 `json:"fiber_100g"`
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
	return map[string]any{
		"name":         prod.ProductName,
		"calories":     prod.Nutriments.EnergyKcal100g,
		"protein":      prod.Nutriments.Proteins100g,
		"carbs":        prod.Nutriments.Carbohydrates100g,
		"fat":          prod.Nutriments.Fat100g,
		"fiber":        prod.Nutriments.Fiber100g,
		"serving_size": 100.0,
		"serving_unit": "g",
	}, nil
}
