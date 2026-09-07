package providers

import (
	"github.com/hangry-coder/bffx/pkg/featureflags"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// BffxProvider is the default flag provider. It persists flags directly via
// the underlying SQL connection when one is available (SQLite/Postgres) and
// falls back to an in-process map for memory / nosql stores. The provider
// owns its own table (`bffx_feature_flag`) so it does not require a Resource
// manifest and does not interact with the typed Store / validateName path
// (which rejects internal names).
type BffxProvider struct {
	store     storage.Store
	reg       *manifest.Registry
	db        *sql.DB
	driver    string
	evaluator *featureflags.Evaluator

	mu  sync.RWMutex
	mem map[string]featureflags.FlagDefinition
}

const flagTableName = "bffx_feature_flag"

func NewBffxProvider(store storage.Store, reg *manifest.Registry) *BffxProvider {
	p := &BffxProvider{
		store:     store,
		reg:       reg,
		evaluator: featureflags.NewEvaluator(),
		mem:       make(map[string]featureflags.FlagDefinition),
	}
	p.detectSQL(store)
	return p
}

func (p *BffxProvider) detectSQL(store storage.Store) {
	unwrapped := storage.UnwrapStore(store)
	type sqlBacked interface{ GetDB() *sql.DB }
	if router, ok := unwrapped.(*storage.RouterStore); ok {
		if rb, ok := router.Primary.(sqlBacked); ok {
			p.db = rb.GetDB()
			switch router.Primary.(type) {
			case *storage.SQLiteStore:
				p.driver = "sqlite"
			case *storage.PostgresStore:
				p.driver = "postgres"
			}
		}
	} else if rb, ok := unwrapped.(sqlBacked); ok {
		p.db = rb.GetDB()
		switch unwrapped.(type) {
		case *storage.SQLiteStore:
			p.driver = "sqlite"
		case *storage.PostgresStore:
			p.driver = "postgres"
		}
	}
}

func (p *BffxProvider) tableExists(tableName string) (bool, error) {
	if p.db == nil {
		return false, nil
	}
	var query string
	var args []any
	if p.driver == "postgres" {
		query = "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)"
		args = append(args, tableName)
	} else {
		query = "SELECT EXISTS (SELECT 1 FROM sqlite_master WHERE type='table' AND name = ?)"
		args = append(args, tableName)
	}
	var exists bool
	err := p.db.QueryRow(query, args...).Scan(&exists)
	return exists, err
}

func (p *BffxProvider) columnExists(tableName, columnName string) (bool, error) {
	if p.db == nil {
		return false, nil
	}
	if p.driver == "postgres" {
		var exists bool
		err := p.db.QueryRow(`
			SELECT EXISTS (
				SELECT 1 
				FROM information_schema.columns 
				WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2
			)
		`, tableName, columnName).Scan(&exists)
		return exists, err
	} else {
		rows, err := p.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
		if err != nil {
			return false, err
		}
		defer rows.Close()
		for rows.Next() {
			var cid int
			var name, typ string
			var notnull, pk int
			var dflt_value any
			if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt_value, &pk); err != nil {
				return false, err
			}
			if name == columnName {
				return true, nil
			}
		}
		return false, nil
	}
}

func (p *BffxProvider) Init(config featureflags.ProviderConfig) error {
	if p.db != nil {
		// Self-healing schema migration:
		// If bffx_feature_flag table exists but lacks the "definition" column (schema mismatch from legacy),
		// we rename it back to feature_flag so migrateLegacyFeatureFlagResource can import it.
		exists, err := p.tableExists(flagTableName)
		if err != nil {
			return fmt.Errorf("check table %s existence: %w", flagTableName, err)
		}
		if exists {
			hasDef, err := p.columnExists(flagTableName, "definition")
			if err != nil {
				return fmt.Errorf("check column definition existence on table %s: %w", flagTableName, err)
			}
			if !hasDef {
				// Drop the legacy feature_flag table if it already exists, to avoid conflicts during rename.
				legacyExists, err := p.tableExists("feature_flag")
				if err != nil {
					return fmt.Errorf("check legacy table existence: %w", err)
				}
				if legacyExists {
					if _, err := p.db.Exec("DROP TABLE feature_flag"); err != nil {
						return fmt.Errorf("drop legacy feature_flag table: %w", err)
					}
				}
				// Rename bffx_feature_flag (which is actually the legacy table) to feature_flag.
				if _, err := p.db.Exec(`ALTER TABLE ` + flagTableName + ` RENAME TO feature_flag`); err != nil {
					return fmt.Errorf("rename legacy %s table: %w", flagTableName, err)
				}
			}
		}

		var stmt string
		switch p.driver {
		case "postgres":
			stmt = `CREATE TABLE IF NOT EXISTS ` + flagTableName + ` (
				id TEXT PRIMARY KEY,
				key TEXT UNIQUE NOT NULL,
				definition JSONB NOT NULL,
				version BIGINT NOT NULL DEFAULT 1,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			)`
		default:
			stmt = `CREATE TABLE IF NOT EXISTS ` + flagTableName + ` (
				id TEXT PRIMARY KEY,
				key TEXT UNIQUE NOT NULL,
				definition TEXT NOT NULL,
				version INTEGER NOT NULL DEFAULT 1,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			)`
		}
		if _, err := p.db.Exec(stmt); err != nil {
			return fmt.Errorf("create %s: %w", flagTableName, err)
		}

		// Best-effort one-time import from a legacy `feature_flag` Resource
		// table. Older projects (e.g. legacy pilot pre v0.1.0) declared
		// `feature_flag` as a CRUD Resource before BFFX gained its dedicated
		// flag provider; their existing rows must surface in the new admin UI
		// or operators perceive the Feature Flags page as broken. We import
		// only when `bffx_feature_flag` is still empty so we never overwrite
		// curated state. Errors here are non-fatal: provider startup must
		// not depend on a legacy table.
		_ = p.migrateLegacyFeatureFlagResource()
	}

	return p.Reload()
}

// migrateLegacyFeatureFlagResource copies rows from a legacy `feature_flag`
// Resource table into `bffx_feature_flag` when the latter is empty. The
// legacy schema is roughly:
//
//	feature_flag(id TEXT, key TEXT, enabled BOOL, rules TEXT, description TEXT, ...)
//
// We synthesize a FlagDefinition with `Enabled`, `Description`, and a single
// catch-all rule string in `Metadata["legacy_rules"]` so nothing is lost.
// Returns nil on any non-existence error path so cold starts are unaffected.
func (p *BffxProvider) migrateLegacyFeatureFlagResource() error {
	if p.db == nil {
		return nil
	}
	// Only import if the new table is empty.
	var count int
	if err := p.db.QueryRow(`SELECT COUNT(*) FROM ` + flagTableName).Scan(&count); err != nil || count > 0 {
		return nil
	}

	type legacyFlag struct {
		key         string
		enabled     bool
		rules       string
		description string
	}

	var legacyFlags []legacyFlag

	// Probe legacy table; absent table = nothing to do.
	rows, err := p.db.Query(`SELECT key, enabled, COALESCE(rules,'') AS rules, COALESCE(description,'') AS description FROM feature_flag`)
	if err != nil {
		return nil
	}
	for rows.Next() {
		var lf legacyFlag
		if err := rows.Scan(&lf.key, &lf.enabled, &lf.rules, &lf.description); err == nil {
			if strings.TrimSpace(lf.key) != "" {
				legacyFlags = append(legacyFlags, lf)
			}
		}
	}
	rows.Close()

	now := time.Now().UTC()
	imported := 0
	for _, lf := range legacyFlags {
		flag := featureflags.FlagDefinition{
			Key:         lf.key,
			Enabled:     lf.enabled,
			Description: lf.description,
			Type:        featureflags.FlagTypeBoolean,
			Variations: []featureflags.Variation{
				{Key: "on", Value: true},
				{Key: "off", Value: false},
			},
			CreatedAt: now,
			UpdatedAt: now,
			Version:   1,
		}
		if strings.TrimSpace(lf.rules) != "" && flag.Tags == nil {
			// Preserve the legacy `rules` text as a tag so operators can
			// audit migrated flags. Tags are visible in the admin UI list.
			flag.Tags = []string{"legacy:" + lf.rules}
		}
		def, err := json.Marshal(flag)
		if err != nil {
			continue
		}
		stmt := p.placeholders(`INSERT INTO ` + flagTableName + ` (id, key, definition, version, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`)
		if _, err := p.db.Exec(stmt, uuid.NewString(), flag.Key, string(def), flag.Version, now.Format(time.RFC3339), now.Format(time.RFC3339)); err != nil {
			// Skip; duplicate-keys mean we already migrated; row errors should not abort the import.
			continue
		}
		imported++
	}
	// Reload at the end of Init() picks up the freshly inserted rows, so we
	// don't need to refresh the cache here.
	_ = imported
	return nil
}

// Reload loads all flags from manifests and database into memory.
func (p *BffxProvider) Reload() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	newMem := make(map[string]featureflags.FlagDefinition)

	// 1. Load from manifests
	if p.reg != nil {
		for _, m := range p.reg.FeatureFlags {
			var flag featureflags.FlagDefinition
			if err := m.UnmarshalSpec(&flag); err == nil {
				if flag.Key == "" {
					flag.Key = m.Metadata.Name
				}
				newMem[flag.Key] = flag
			}
		}
	}

	// 2. Load from database (overrides manifests)
	if p.db != nil {
		rows, err := p.db.Query(`SELECT definition FROM ` + flagTableName)
		if err != nil {
			// If table doesn't exist yet, it's fine (Init handles creation)
			return nil
		}
		defer rows.Close()
		for rows.Next() {
			var def string
			if err := rows.Scan(&def); err != nil {
				continue
			}
			var flag featureflags.FlagDefinition
			if err := json.Unmarshal([]byte(def), &flag); err == nil {
				newMem[flag.Key] = flag
			}
		}
	}

	p.mem = newMem
	return nil
}

func (p *BffxProvider) Close() error { return nil }

func (p *BffxProvider) Name() string { return "bffx" }

func (p *BffxProvider) CreateFlag(flag featureflags.FlagDefinition) (*featureflags.FlagDefinition, error) {
	now := time.Now().UTC()
	flag.CreatedAt = now
	flag.UpdatedAt = now
	flag.Version = 1
	if flag.Key == "" {
		return nil, errors.New("flag key is required")
	}

	def, err := json.Marshal(flag)
	if err != nil {
		return nil, fmt.Errorf("marshal flag: %w", err)
	}

	if p.db != nil {
		id := uuid.NewString()
		stmt := p.placeholders(`INSERT INTO ` + flagTableName + ` (id, key, definition, version, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`)
		if _, err := p.db.Exec(stmt, id, flag.Key, string(def), flag.Version, now.Format(time.RFC3339), now.Format(time.RFC3339)); err != nil {
			if isUniqueViolation(err) {
				return nil, fmt.Errorf("flag with key %q already exists", flag.Key)
			}
			return nil, fmt.Errorf("insert flag: %w", err)
		}
	}

	p.mu.Lock()
	p.mem[flag.Key] = flag
	p.mu.Unlock()
	return &flag, nil
}

func (p *BffxProvider) UpdateFlag(key string, flag featureflags.FlagDefinition) (*featureflags.FlagDefinition, error) {
	existing, err := p.GetFlag(key)
	if err != nil {
		return nil, err
	}
	flag.Key = existing.Key
	flag.CreatedAt = existing.CreatedAt
	flag.UpdatedAt = time.Now().UTC()
	flag.Version = existing.Version + 1

	def, err := json.Marshal(flag)
	if err != nil {
		return nil, fmt.Errorf("marshal flag: %w", err)
	}

	if p.db != nil {
		stmt := p.placeholders(`UPDATE ` + flagTableName + ` SET definition = ?, version = ?, updated_at = ? WHERE key = ?`)
		if _, err := p.db.Exec(stmt, string(def), flag.Version, flag.UpdatedAt.Format(time.RFC3339), key); err != nil {
			return nil, fmt.Errorf("update flag: %w", err)
		}
	}

	p.mu.Lock()
	p.mem[key] = flag
	p.mu.Unlock()
	return &flag, nil
}

func (p *BffxProvider) DeleteFlag(key string) error {
	if p.db != nil {
		stmt := p.placeholders(`DELETE FROM ` + flagTableName + ` WHERE key = ?`)
		res, err := p.db.Exec(stmt, key)
		if err != nil {
			return fmt.Errorf("delete flag: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			// If not in DB, it might be in manifests, but we can't delete from manifests.
			// However, if we're calling DeleteFlag, we usually expect it to be a managed flag.
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.mem[key]; !ok {
		return errors.New("flag not found")
	}
	delete(p.mem, key)
	return nil
}

func (p *BffxProvider) GetFlag(key string) (*featureflags.FlagDefinition, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	flag, ok := p.mem[key]
	if !ok {
		return nil, errors.New("flag not found")
	}
	return &flag, nil
}

func (p *BffxProvider) ListFlags() ([]featureflags.FlagDefinition, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]featureflags.FlagDefinition, 0, len(p.mem))
	for _, f := range p.mem {
		out = append(out, f)
	}
	return out, nil
}

func (p *BffxProvider) Evaluate(key string, ctx featureflags.EvalContext) (interface{}, error) {
	flag, err := p.GetFlag(key)
	if err != nil {
		return nil, err
	}
	return p.evaluator.Evaluate(*flag, ctx), nil
}

func (p *BffxProvider) EvaluateAll(ctx featureflags.EvalContext) (map[string]featureflags.ResolvedFlag, error) {
	flags, err := p.ListFlags()
	if err != nil {
		return nil, err
	}
	results := make(map[string]featureflags.ResolvedFlag, len(flags))
	for _, f := range flags {
		results[f.Key] = featureflags.ResolvedFlag{
			Key:   f.Key,
			Value: p.evaluator.Evaluate(f, ctx),
		}
	}
	return results, nil
}

func (p *BffxProvider) Subscribe(ctx context.Context, onChange func(key string, value interface{})) error {
	return errors.New("subscribe not implemented for bffx provider yet")
}

// placeholders rewrites `?` placeholders to `$1, $2, ...` for Postgres. SQLite
// uses `?` natively, so the input passes through unchanged.
func (p *BffxProvider) placeholders(stmt string) string {
	if p.driver != "postgres" {
		return stmt
	}
	var b strings.Builder
	b.Grow(len(stmt))
	idx := 0
	for _, r := range stmt {
		if r == '?' {
			idx++
			fmt.Fprintf(&b, "$%d", idx)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate")
}
