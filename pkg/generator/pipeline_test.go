package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratePipeline_V2Layout(t *testing.T) {
	tmp, err := os.MkdirTemp("", "bffx-pipe-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	err = GeneratePipeline(tmp, "CalorieTracker", "meals", "ingestion", "openfoodfacts", LayoutV2)
	if err != nil {
		t.Fatalf("GeneratePipeline failed: %v", err)
	}

	// 1. Verify Manifest
	manifestPath := filepath.Join(tmp, "internal/features/meals/manifests/calorietracker_pipeline.yaml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Fatalf("pipeline manifest missing at %s", manifestPath)
	}
	content, _ := os.ReadFile(manifestPath)
	manifestStr := string(content)
	if !strings.Contains(manifestStr, "kind: Pipeline") {
		t.Errorf("expected kind: Pipeline, got: %s", manifestStr)
	}
	if !strings.Contains(manifestStr, "name: CalorieTracker") {
		t.Errorf("expected name: CalorieTracker, got: %s", manifestStr)
	}
	if !strings.Contains(manifestStr, "adapter: openfoodfacts") {
		t.Errorf("expected adapter: openfoodfacts, got: %s", manifestStr)
	}

	// 2. Verify Pipeline files
	pipelinePath := filepath.Join(tmp, "internal/features/meals/pipelines/calorietracker/pipeline.go")
	if _, err := os.Stat(pipelinePath); os.IsNotExist(err) {
		t.Fatalf("pipeline.go missing at %s", pipelinePath)
	}
	pipelineContent, _ := os.ReadFile(pipelinePath)
	if !strings.Contains(string(pipelineContent), "package calorietracker") {
		t.Errorf("pipeline.go package name mismatch")
	}

	adaptersPath := filepath.Join(tmp, "internal/features/meals/pipelines/calorietracker/adapters.go")
	if _, err := os.Stat(adaptersPath); os.IsNotExist(err) {
		t.Fatalf("adapters.go missing at %s", adaptersPath)
	}
	adaptersContent, _ := os.ReadFile(adaptersPath)
	if !strings.Contains(string(adaptersContent), "NutritionCatalogAdapter") {
		t.Errorf("expected NutritionCatalogAdapter in adapters.go")
	}

	promptPath := filepath.Join(tmp, "internal/features/meals/pipelines/calorietracker/system_prompt.txt")
	if _, err := os.Stat(promptPath); os.IsNotExist(err) {
		t.Fatalf("system_prompt.txt missing at %s", promptPath)
	}

	// 3. Verify Hooks
	hookPath := filepath.Join(tmp, "internal/features/meals/hooks/calorietracker.go")
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		t.Fatalf("hook file missing at %s", hookPath)
	}
	hookContent, _ := os.ReadFile(hookPath)
	if !strings.Contains(string(hookContent), "func BeforeCalorieTracker") {
		t.Errorf("BeforeCalorieTracker hook missing")
	}
}
