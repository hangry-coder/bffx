# AI Pipelines: Catalog Addons

BFFX supports decoupled catalog resolution to prevent heavy domain logic from polluting the core VLM/LLM orchestrator. 

By defining catalog adapters in the project, the VLM can query databases (like USDA FoodData Central, custom SQLite local databases, or postgres catalogs) during standard pipeline parsing.

---

## 1. The `Catalog` Interface

Custom catalog providers implement the simple `orchestrator.Catalog` interface:

```go
type Catalog interface {
    // Resolve matches the query from LLM against the catalog database.
    // Returns any matched item payload, or an error.
    Resolve(ctx context.Context, query string, hints map[string]string) (any, error)
    
    // Source returns the unique adapter key (e.g. "openfoodfacts", "usda").
    Source() string
}
```

---

## 2. Implementing a Catalog Adapter

When generating a pipeline with a catalog:
```bash
bffx generate pipeline ingestion --name Scanner --feature meals --catalog=openfoodfacts
```

BFFX creates `internal/features/meals/pipelines/scanner/adapters.go`. You can define your local database or remote API calls within this file:

```go
package scanner

import (
    "context"
    "fmt"
)

type NutritionCatalogAdapter struct{}

func (a *NutritionCatalogAdapter) Resolve(ctx context.Context, query string, hints map[string]string) (any, error) {
    // Perform database query, Elasticsearch, or external USDA API lookups here
    return map[string]any{
        "name":     query,
        "calories": 150,
        "source":   "local_sqlite",
    }, nil
}

func (a *NutritionCatalogAdapter) Source() string {
    return "openfoodfacts"
}
```

---

## 3. Registering the Catalog

To register your catalog with the framework, pass it to the orchestrator registry inside the main project configuration:

```go
// cmd/orchestrator/main.go
cat := &scanner.NutritionCatalogAdapter{}
r := router.NewRouter(router.RouterConfig{
    TagCache: memoryCache,
    // Register custom catalog adapter
    NutritionProvider: cat, 
})
```
