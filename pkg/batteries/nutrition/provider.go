package nutrition

import (
	"context"
)

// Provider is the legacy nutrition lookup battery.
// Deprecated: Use the decoupled orchestrator.Catalog and pkg/addons/catalog/nutrition addon instead.
type Provider interface {
	Type() string
	FetchNutrition(ctx context.Context, query string) (*NutritionData, error)
}

type NutritionData struct {
	Name        string  `json:"name"`
	Calories    float64 `json:"calories"`
	Protein     float64 `json:"protein"`     // grams
	Carbs       float64 `json:"carbs"`       // grams
	Fat         float64 `json:"fat"`         // grams
	Fiber       float64 `json:"fiber"`       // grams
	ServingSize float64 `json:"serving_size"`
	ServingUnit string  `json:"serving_unit"`
}
