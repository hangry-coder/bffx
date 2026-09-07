package testing

import (
	"github.com/hangry-coder/bffx/pkg/storage"
	"strings"
	"testing"
)

func TestFactory_Sequences(t *testing.T) {
	s1 := NextSequence("test")
	s2 := NextSequence("test")
	if s2 != s1+1 {
		t.Errorf("Expected sequence to increment, got %d then %d", s1, s2)
	}
}

func TestFactory_Faker(t *testing.T) {
	email := FakeValue("email").(string)
	if !strings.Contains(email, "@example.com") {
		t.Errorf("Expected fake email, got %s", email)
	}

	name := FakeValue("name").(string)
	if len(name) == 0 {
		t.Error("Expected fake name, got empty string")
	}
}

func TestFactory_Traits(t *testing.T) {
	store := storage.NewMemoryStore()
	factory := NewFactory(store, nil)

	// Test built-in trait
	data := factory.WithTrait("admin").For("User").Build(nil)
	if data["role"] != "admin" {
		t.Errorf("Expected role admin, got %v", data["role"])
	}

	// Test override
	data = factory.WithTrait("admin").For("User").Build(map[string]any{"role": "superman"})
	if data["role"] != "superman" {
		t.Errorf("Expected override role superman, got %v", data["role"])
	}
}

func TestFactory_SmartDefaults(t *testing.T) {
	store := storage.NewMemoryStore()
	factory := NewFactory(store, nil)

	data := factory.Build("User", nil)
	if data["email"] == nil {
		t.Error("Expected auto-generated email for User")
	}
	if data["name"] == nil {
		t.Error("Expected auto-generated name for User")
	}
}
