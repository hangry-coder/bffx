package unit

import (
	"testing"
	"github.com/hangry-coder/bffx/pkg/api/handlers"
	"github.com/hangry-coder/bffx/pkg/storage"
)

func TestExampleHook(t *testing.T) {
	store := storage.NewMemoryStore()
	ctx := &handlers.ActionContext{
		Store: store,
	}

	// Mock payload
	payload := map[string]any{
		"name": "Test",
	}

	// In a real app, you would import your hooks and call them here
	// err := hooks.MyHook(ctx, payload)
	// if err != nil { t.Error(err) }
	
	if payload["name"] != "Test" {
		t.Errorf("expected Test, got %v", payload["name"])
	}
}