package router

import (
	"context"
	"testing"

	"github.com/hangry-coder/bffx/pkg/api/handlers"
	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/api/validation"
	"github.com/hangry-coder/bffx/pkg/events"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"gopkg.in/yaml.v3"
)

func TestCreateResourceGRPC_injectsOwnerAndRunsBeforeCreateHook(t *testing.T) {
	var mealSpec yaml.Node
	if err := yaml.Unmarshal([]byte(`routes:
  crud: true
policy:
  read: owner
  write: owner
fields:
  - {name: description, type: string}
  - {name: logged_at, type: datetime, required: true}
hooks:
  beforeCreate:
    - {action: BeforeCreateMealLog, mode: sync}
`), &mealSpec); err != nil {
		t.Fatal(err)
	}

	reg := &manifest.Registry{
		Resources: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "MealLog"}, Spec: mealSpec},
		},
	}

	store := storage.NewMemoryStore()
	hookCalled := false
	r := &Router{
		reg:       reg,
		store:     store,
		validator: validation.NewValidator(),
		hooks: map[string]HookFunc{
			"BeforeCreateMealLog": func(_ *handlers.ActionContext, _ map[string]any) error {
				hookCalled = true
				return nil
			},
		},
		bus: events.NewMemoryBus(),
	}

	ctx := middleware.WithClaims(context.Background(), map[string]any{"sub": "user-42"})
	result, err := r.CreateResourceGRPC(ctx, "MealLog", map[string]any{
		"description": "oatmeal",
		"logged_at":   "2026-05-30T12:00:00Z",
	})
	if err != nil {
		t.Fatalf("CreateResourceGRPC: %v", err)
	}
	if !hookCalled {
		t.Fatal("expected BeforeCreateMealLog hook to run")
	}
	if result["created_by"] != "user-42" {
		t.Fatalf("created_by: got %v want user-42", result["created_by"])
	}
}

func TestAlignGRPCPayloadToSpec_mapsProtoJSONNames(t *testing.T) {
	spec := manifest.ResourceSpec{
		Fields: []manifest.ResourceField{
			{Name: "meal_type", Type: "string"},
			{Name: "logged_at", Type: "datetime", Required: true},
			{Name: "total_kcal", Type: "int"},
		},
	}
	in := map[string]any{
		"mealType":  "lunch",
		"loggedAt":  "2026-06-05T12:00:00Z",
		"totalKcal": 420,
	}
	out := alignGRPCPayloadToSpec(&spec, in)
	if out["meal_type"] != "lunch" {
		t.Fatalf("meal_type: %#v", out["meal_type"])
	}
	if out["logged_at"] != "2026-06-05T12:00:00Z" {
		t.Fatalf("logged_at: %#v", out["logged_at"])
	}
	if out["total_kcal"] != 420 {
		t.Fatalf("total_kcal: %#v", out["total_kcal"])
	}
}

func TestUpdateResourceGRPC_runsBeforeUpdateHook(t *testing.T) {
	var mealSpec yaml.Node
	if err := yaml.Unmarshal([]byte(`routes:
  crud: true
policy:
  read: owner
  write: owner
fields:
  - {name: description, type: string}
hooks:
  beforeUpdate:
    - {action: TouchMealLog, mode: sync}
`), &mealSpec); err != nil {
		t.Fatal(err)
	}

	reg := &manifest.Registry{
		Resources: []*manifest.Manifest{
			{Metadata: manifest.Metadata{Name: "MealLog"}, Spec: mealSpec},
		},
	}

	store := storage.NewMemoryStore()
	_, err := store.Create(context.Background(), "MealLog", map[string]any{
		"id":          "meal-1",
		"description": "old",
		"created_by":  "user-42",
	})
	if err != nil {
		t.Fatal(err)
	}

	hookCalled := false
	r := &Router{
		reg:       reg,
		store:     store,
		validator: validation.NewValidator(),
		hooks: map[string]HookFunc{
			"TouchMealLog": func(_ *handlers.ActionContext, _ map[string]any) error {
				hookCalled = true
				return nil
			},
		},
		bus: events.NewMemoryBus(),
	}

	ctx := middleware.WithClaims(context.Background(), map[string]any{"sub": "user-42"})
	result, err := r.UpdateResourceGRPC(ctx, "MealLog", "meal-1", map[string]any{
		"description": "new",
	})
	if err != nil {
		t.Fatalf("UpdateResourceGRPC: %v", err)
	}
	if !hookCalled {
		t.Fatal("expected beforeUpdate hook to run")
	}
	if result["description"] != "new" {
		t.Fatalf("description: %#v", result["description"])
	}
}
