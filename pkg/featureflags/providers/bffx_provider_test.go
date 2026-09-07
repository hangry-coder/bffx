package providers

import (
	"fmt"
	"github.com/hangry-coder/bffx/pkg/featureflags"
	"github.com/hangry-coder/bffx/pkg/storage"
	"path/filepath"
	"testing"
)

func TestBffxProvider_Evaluate(t *testing.T) {
	store := storage.NewMemoryStore()
	provider := NewBffxProvider(store, nil)
	if err := provider.Init(featureflags.ProviderConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	flag := featureflags.FlagDefinition{
		Key:     "test_flag",
		Name:    "Test Flag",
		Enabled: true,
		Type:    featureflags.FlagTypeBoolean,
		Variations: []featureflags.Variation{
			{Key: "off", Value: false},
			{Key: "on", Value: true},
		},
		Rules: []featureflags.TargetingRule{
			{
				Clauses: []featureflags.Clause{
					{
						Attribute: "user_id",
						Operator:  "in",
						Values:    []interface{}{"user_123"},
					},
				},
				Variation: intPtr(1),
			},
		},
		Fallthrough: featureflags.VariationSelection{
			Variation: intPtr(0),
		},
	}

	if _, err := provider.CreateFlag(flag); err != nil {
		t.Fatalf("failed to create flag: %v", err)
	}

	ctxMatched := featureflags.EvalContext{UserID: "user_123"}
	val, err := provider.Evaluate("test_flag", ctxMatched)
	if err != nil {
		t.Fatalf("failed to evaluate: %v", err)
	}
	if val != true {
		t.Errorf("expected true, got %v", val)
	}

	ctxUnmatched := featureflags.EvalContext{UserID: "user_456"}
	val, err = provider.Evaluate("test_flag", ctxUnmatched)
	if err != nil {
		t.Fatalf("failed to evaluate: %v", err)
	}
	if val != false {
		t.Errorf("expected false, got %v", val)
	}

	// Test simplified targeting rules
	flagTargeting := featureflags.FlagDefinition{
		Key:     "targeting_flag",
		Enabled: true,
		Type:    featureflags.FlagTypeBoolean,
		Variations: []featureflags.Variation{
			{Key: "on", Value: true},
			{Key: "off", Value: false},
		},
		OffVariation: 1,
		Targeting: &featureflags.TargetingRules{
			Users:    []string{"target_user"},
			Segments: []string{"Beta"},
			Rollout:  50,
		},
	}
	if _, err := provider.CreateFlag(flagTargeting); err != nil {
		t.Fatalf("failed to create flagTargeting: %v", err)
	}

	// 1. User match
	val, _ = provider.Evaluate("targeting_flag", featureflags.EvalContext{UserID: "target_user"})
	if val != true {
		t.Errorf("expected user match to return true, got %v", val)
	}

	// 2. Segment match
	val, _ = provider.Evaluate("targeting_flag", featureflags.EvalContext{UserID: "other", Segments: []string{"Beta"}})
	if val != true {
		t.Errorf("expected segment match to return true, got %v", val)
	}

	// 3. Rollout match / mismatch
	hasTrue := false
	hasFalse := false
	for i := 0; i < 20; i++ {
		uid := fmt.Sprintf("user_seq_%d", i)
		v, _ := provider.Evaluate("targeting_flag", featureflags.EvalContext{UserID: uid})
		if v == true {
			hasTrue = true
		} else if v == false {
			hasFalse = true
		}
	}
	if !hasTrue || !hasFalse {
		t.Errorf("expected both true and false rollout variations for seq, got hasTrue=%v hasFalse=%v", hasTrue, hasFalse)
	}
}

// TestBffxProvider_UpdateRoundtrip exercises the path that previously panicked
// because it used unchecked `existing["version"].(int)` after a JSON round-trip.
func TestBffxProvider_UpdateRoundtrip(t *testing.T) {
	store := storage.NewMemoryStore()
	provider := NewBffxProvider(store, nil)
	if err := provider.Init(featureflags.ProviderConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	flag := featureflags.FlagDefinition{
		Key:     "rolling_flag",
		Enabled: true,
		Type:    featureflags.FlagTypeBoolean,
		Variations: []featureflags.Variation{
			{Key: "off", Value: false},
			{Key: "on", Value: true},
		},
		Fallthrough: featureflags.VariationSelection{Variation: intPtr(0)},
	}
	if _, err := provider.CreateFlag(flag); err != nil {
		t.Fatalf("create: %v", err)
	}

	flag.Fallthrough = featureflags.VariationSelection{Variation: intPtr(1)}
	updated, err := provider.UpdateFlag("rolling_flag", flag)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Version != 2 {
		t.Errorf("expected version 2, got %d", updated.Version)
	}

	got, err := provider.GetFlag("rolling_flag")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Version != 2 {
		t.Errorf("expected persisted version 2, got %d", got.Version)
	}
}

// TestBffxProvider_SQLiteBacked verifies that the provider creates its own
// table on a SQL-backed store and can persist flags through Init→Create→
// Evaluate without going through the typed Store / validateName path.
func TestBffxProvider_SQLiteBacked(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "flags.db")
	sqliteStore, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	provider := NewBffxProvider(sqliteStore, nil)
	if err := provider.Init(featureflags.ProviderConfig{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	flag := featureflags.FlagDefinition{
		Key:     "sql_flag",
		Enabled: true,
		Type:    featureflags.FlagTypeBoolean,
		Variations: []featureflags.Variation{
			{Key: "off", Value: false},
			{Key: "on", Value: true},
		},
		Fallthrough: featureflags.VariationSelection{Variation: intPtr(1)},
	}
	if _, err := provider.CreateFlag(flag); err != nil {
		t.Fatalf("create: %v", err)
	}

	val, err := provider.Evaluate("sql_flag", featureflags.EvalContext{UserID: "u"})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if val != true {
		t.Errorf("expected true, got %v", val)
	}

	// Update should round-trip without panicking.
	flag.Fallthrough = featureflags.VariationSelection{Variation: intPtr(0)}
	if _, err := provider.UpdateFlag("sql_flag", flag); err != nil {
		t.Fatalf("update: %v", err)
	}

	if err := provider.DeleteFlag("sql_flag"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestBffxProvider_SelfHealingMigration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "flags_migration.db")
	sqliteStore, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db := sqliteStore.GetDB()

	// 1. Manually create the legacy `bffx_feature_flag` table structure (schema mismatch)
	legacySchema := `CREATE TABLE bffx_feature_flag (
		id TEXT PRIMARY KEY,
		created_by TEXT,
		created_at TEXT,
		updated_at TEXT,
		key TEXT,
		enabled INTEGER,
		rules TEXT,
		description TEXT
	)`
	if _, err := db.Exec(legacySchema); err != nil {
		t.Fatalf("create legacy bffx_feature_flag table: %v", err)
	}

	// 2. Insert a legacy flag record
	insertLegacy := `INSERT INTO bffx_feature_flag (id, key, enabled, rules, description, created_at, updated_at) 
		VALUES ('flag-123', 'legacy_flag', 1, 'legacy_rules_value', 'my legacy description', '2026-05-21T00:00:00Z', '2026-05-21T00:00:00Z')`
	if _, err := db.Exec(insertLegacy); err != nil {
		t.Fatalf("insert legacy record: %v", err)
	}

	// 3. Initialize BffxProvider over the DB. It should self-heal and migrate the legacy data.
	provider := NewBffxProvider(sqliteStore, nil)
	if err := provider.Init(featureflags.ProviderConfig{}); err != nil {
		t.Fatalf("init provider: %v", err)
	}

	// 4. Verify the legacy flag is migrated and loaded into memory
	flag, err := provider.GetFlag("legacy_flag")
	if err != nil {
		t.Fatalf("failed to retrieve migrated legacy_flag: %v", err)
	}

	if flag.Key != "legacy_flag" {
		t.Errorf("expected key 'legacy_flag', got %s", flag.Key)
	}
	if !flag.Enabled {
		t.Errorf("expected flag to be enabled")
	}
	if flag.Description != "my legacy description" {
		t.Errorf("expected description 'my legacy description', got %s", flag.Description)
	}
	if len(flag.Tags) != 1 || flag.Tags[0] != "legacy:legacy_rules_value" {
		t.Errorf("expected legacy tags, got %v", flag.Tags)
	}

	// 5. Verify we can create a new flag now that the schema is corrected
	newFlag := featureflags.FlagDefinition{
		Key:     "new_flag",
		Enabled: true,
		Type:    featureflags.FlagTypeBoolean,
		Variations: []featureflags.Variation{
			{Key: "off", Value: false},
			{Key: "on", Value: true},
		},
		Fallthrough: featureflags.VariationSelection{Variation: intPtr(1)},
	}
	if _, err := provider.CreateFlag(newFlag); err != nil {
		t.Fatalf("create new flag after migration failed: %v", err)
	}

	// Double check memory cache contains the new flag
	if _, err := provider.GetFlag("new_flag"); err != nil {
		t.Fatalf("newly created flag not found: %v", err)
	}
}

func intPtr(i int) *int { return &i }

