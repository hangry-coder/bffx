package storage

import (
	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage/schema"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	bffx_errors "github.com/hangry-coder/bffx/pkg/errors"
	"github.com/google/uuid"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db            *sql.DB
	complexFields map[string]map[string]bool // resource -> field -> isComplex
	reconcileMu   sync.Mutex                 // serializes Reconcile (single-writer; see BFFX_SCHEMA_SINGLETON)
}

func (s *SQLiteStore) GetDB() *sql.DB {
	return s.db
}

func (s *SQLiteStore) tableExists(tableName string) bool {
	var name string
	err := s.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&name)
	return err == nil
}

func (s *SQLiteStore) resolveTable(resourceName string) string {
	table := schema.ResolveTable(resourceName)
	if !schema.IsSystemResource(resourceName) {
		return table
	}

	if os.Getenv("BFFX_SYSTEM_TABLES_COMPAT_READ") == "false" {
		return table
	}

	if os.Getenv("BFFX_SYSTEM_TABLES_STRICT") == "true" {
		return table
	}

	if s.tableExists(table) {
		return table
	}

	return strings.ToLower(resourceName)
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	return &SQLiteStore{
		db:            db,
		complexFields: make(map[string]map[string]bool),
	}, nil
}

func (s *SQLiteStore) Reconcile(ctx context.Context, reg *manifest.Registry) ([]Change, error) {
	if os.Getenv("BFFX_SCHEMA_SINGLETON") != "false" {
		s.reconcileMu.Lock()
		defer s.reconcileMu.Unlock()
	}

	hash := reg.ResourceHash()

	var dbHash string
	var tableCount int
	err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='bffx_schema_hash'").Scan(&tableCount)
	if err == nil && tableCount > 0 {
		err = s.db.QueryRowContext(ctx, "SELECT hash FROM bffx_schema_hash LIMIT 1").Scan(&dbHash)
		if err == nil && dbHash == hash {
			var count int
			err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table'").Scan(&count)
			if err == nil && count > 1 {
				logger.DebugCtx(ctx, "sqlite reconcile: schema is up to date (hash matches), skipping.")
				return nil, nil
			}
		}
	}

	var changes []Change
	s.complexFields = make(map[string]map[string]bool)

	for _, r := range reg.Resources {
		var spec manifest.ResourceSpec
		if err := r.UnmarshalSpec(&spec); err != nil {
			logger.ErrorCtx(ctx, "sqlite reconcile: failed to unmarshal spec for %s: %v", r.Metadata.Name, err)
			continue
		}
		table := schema.ResolveTable(r.Metadata.Name)
		s.complexFields[table] = make(map[string]bool)
		for _, f := range spec.Fields {
			if f.Type == "json" || f.Type == "map" || f.Type == "array" {
				s.complexFields[table][strings.ToLower(f.Name)] = true
			}
		}

		if err := validateName(table); err != nil {
			logger.ErrorCtx(ctx, "sqlite reconcile: invalid table name %q: %v", table, err)
			continue
		}

		query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id TEXT PRIMARY KEY)", table)
		if _, err := s.db.ExecContext(ctx, query); err != nil {
			logger.ErrorCtx(ctx, "sqlite reconcile: failed to create table %s: %v", table, err)
			return nil, fmt.Errorf("create table %s: %w", table, err)
		}

		for _, col := range []string{"created_by", "created_at", "updated_at"} {
			var exists bool
			rows, _ := s.db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
			if rows != nil {
				for rows.Next() {
					var cid int
					var name string
					var dummy any
					rows.Scan(&cid, &name, &dummy, &dummy, &dummy, &dummy)
					if name == col {
						exists = true
					}
				}
				rows.Close()
			}
			if !exists {
				alter := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s TEXT", table, col)
				logger.InfoCtx(ctx, "sqlite reconcile: adding column %s to %s", col, table)
				s.db.ExecContext(ctx, alter)
			}
		}

		for _, f := range spec.Fields {
			colName := strings.ToLower(f.Name)
			if colName == "id" {
				continue
			}

			var exists bool
			rows, _ := s.db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
			if rows != nil {
				for rows.Next() {
					var cid int
					var name, typ string
					var notnull, pk int
					var dflt_value any
					rows.Scan(&cid, &name, &typ, &notnull, &dflt_value, &pk)
					if strings.ToLower(name) == colName {
						exists = true
					}
				}
				rows.Close()
			}

			if !exists {
				typ := "TEXT"
				switch f.Type {
				case "int":
					typ = "INTEGER"
				case "float":
					typ = "REAL"
				case "bool":
					typ = "INTEGER"
				}

				alter := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, colName, typ)
				logger.Info("sqlite reconcile: running %s", alter)
				if _, err := s.db.ExecContext(ctx, alter); err != nil {
					logger.Error("sqlite reconcile: failed to add column %s to %s: %v", colName, table, err)
				} else {
					changes = append(changes, Change{Resource: r.Metadata.Name, Action: "added", Field: colName})
				}
			}
		}
		for _, f := range spec.Fields {
			colName := strings.ToLower(f.Name)
			shouldIndex := f.Target != "" || f.Unique || strings.HasSuffix(colName, "_id")
			if colName == "created_by" || colName == "status" || colName == "created_at" {
				shouldIndex = true
			}
			if shouldIndex {
				indexName := fmt.Sprintf("idx_%s_%s", table, colName)
				uniqueStr := ""
				if f.Unique {
					uniqueStr = "UNIQUE"
				}
				createIndex := fmt.Sprintf("CREATE %s INDEX IF NOT EXISTS %s ON %s(%s)", uniqueStr, indexName, table, colName)
				if _, err := s.db.ExecContext(ctx, createIndex); err != nil {
					logger.Warn("sqlite reconcile: failed to create index %s: %v", indexName, err)
				}
			}
		}
	}

	_, err = s.db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS bffx_schema_hash (hash TEXT PRIMARY KEY)")
	if err == nil {
		s.db.ExecContext(ctx, "DELETE FROM bffx_schema_hash")
		s.db.ExecContext(ctx, "INSERT INTO bffx_schema_hash (hash) VALUES (?)", hash)
	}

	return changes, nil
}

func (s *SQLiteStore) List(ctx context.Context, resource string, limit, offset int) ([]map[string]any, error) {
	if err := validateName(resource); err != nil {
		return nil, err
	}
	table := s.resolveTable(resource)

	query := fmt.Sprintf("SELECT * FROM %s", table)
	args := []any{}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	} else if offset > 0 {
		query += " LIMIT -1"
	}
	if offset > 0 {
		query += " OFFSET ?"
		args = append(args, offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRows(rows), nil
}

func (s *SQLiteStore) ListByOwner(ctx context.Context, resource, ownerID string, limit, offset int) ([]map[string]any, error) {
	if err := validateName(resource); err != nil {
		return nil, err
	}
	table := s.resolveTable(resource)

	query := fmt.Sprintf("SELECT * FROM %s WHERE created_by = ?", table)
	args := []any{ownerID}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	} else if offset > 0 {
		query += " LIMIT -1"
	}
	if offset > 0 {
		query += " OFFSET ?"
		args = append(args, offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRows(rows), nil
}

func (s *SQLiteStore) Get(ctx context.Context, resource, id string) (map[string]any, error) {
	if err := validateName(resource); err != nil {
		return nil, err
	}
	table := s.resolveTable(resource)
	query := fmt.Sprintf("SELECT * FROM %s WHERE id = ?", table)
	rows, err := s.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, bffx_errors.ErrNotFound
	}

	return scanRow(rows)
}

func (s *SQLiteStore) GetByField(ctx context.Context, resource, field, value string) (map[string]any, error) {
	if err := validateName(resource); err != nil {
		return nil, err
	}
	if err := validateName(field); err != nil {
		return nil, err
	}

	table := s.resolveTable(resource)
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = ? LIMIT 1", table, strings.ToLower(field))
	rows, err := s.db.QueryContext(ctx, query, value)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, bffx_errors.ErrNotFound
	}

	return scanRow(rows)
}

func (s *SQLiteStore) Create(ctx context.Context, resource string, payload map[string]any) (map[string]any, error) {
	if payload == nil {
		payload = make(map[string]any)
	}
	if err := validateName(resource); err != nil {
		return nil, err
	}
	table := s.resolveTable(resource)
	if payload["id"] == nil || payload["id"] == "" {
		payload["id"] = uuid.NewString()
	}

	now := time.Now().UTC().Format(time.RFC3339)
	payload["created_at"] = now
	payload["updated_at"] = now

	var cols []string
	var placeHolders []string
	var args []any

	for k, v := range payload {
		if err := validateName(k); err != nil {
			return nil, err
		}
		colName := strings.ToLower(k)
		cols = append(cols, colName)
		placeHolders = append(placeHolders, "?")

		if s.isFieldComplex(resource, k, v) {
			b, _ := json.Marshal(v)
			args = append(args, string(b))
		} else {
			args = append(args, v)
		}
	}

	query := ""
	if len(cols) == 0 {
		query = fmt.Sprintf("INSERT INTO %s DEFAULT VALUES", table)
	} else {
		query = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(cols, ","), strings.Join(placeHolders, ","))
	}

	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite create error: %w", err)
	}

	return payload, nil
}

func (s *SQLiteStore) Update(ctx context.Context, resource, id string, payload map[string]any) (map[string]any, error) {
	if err := validateName(resource); err != nil {
		return nil, err
	}
	table := s.resolveTable(resource)
	var sets []string
	var args []any

	for k, v := range payload {
		if k == "id" || k == "created_at" || k == "updated_at" {
			continue
		}
		if err := validateName(k); err != nil {
			return nil, err
		}
		colName := strings.ToLower(k)
		sets = append(sets, fmt.Sprintf("%s = ?", colName))
		if s.isFieldComplex(resource, k, v) {
			b, _ := json.Marshal(v)
			args = append(args, string(b))
		} else {
			args = append(args, v)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	sets = append(sets, "updated_at = ?")
	args = append(args, now)

	args = append(args, id)

	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = ?", table, strings.Join(sets, ","))
	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, resource, id)
}

func (s *SQLiteStore) Delete(ctx context.Context, resource, id string) error {
	res, err := s.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id = ?", s.resolveTable(resource)), id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return bffx_errors.ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) GetChildren(ctx context.Context, resource, id string) ([]map[string]any, error) {
	return nil, nil
}

func (s *SQLiteStore) GetAncestors(ctx context.Context, resource, id string) ([]map[string]any, error) {
	return nil, nil
}

func (s *SQLiteStore) Query(ctx context.Context, resource string) QueryBuilder {
	return &sqliteQueryBuilder{
		store:    s,
		resource: resource,
	}
}

type sqliteQueryBuilder struct {
	store    *SQLiteStore
	resource string
	wheres   []string
	args     []any
	order    string
	limit    int
	offset   int
}

func (b *sqliteQueryBuilder) Where(field, op string, value any) QueryBuilder {
	if err := validateName(field); err != nil {
		return b
	}
	opUpper := strings.ToUpper(op)
	validOps := map[string]bool{"=": true, "!=": true, ">": true, "<": true, ">=": true, "<=": true, "LIKE": true, "IN": true}
	if !validOps[opUpper] {
		return b
	}

	if opUpper == "IN" {
		if vals, ok := value.([]any); ok {
			return b.WhereIn(field, vals)
		}
	}

	b.args = append(b.args, value)
	b.wheres = append(b.wheres, fmt.Sprintf("%s %s ?", strings.ToLower(field), op))
	return b
}

func (b *sqliteQueryBuilder) WhereIn(field string, values []any) QueryBuilder {
	if len(values) == 0 {
		b.wheres = append(b.wheres, "0 = 1")
		return b
	}
	if err := validateName(field); err != nil {
		return b
	}
	placeholders := make([]string, len(values))
	for i := range values {
		placeholders[i] = "?"
		b.args = append(b.args, values[i])
	}
	b.wheres = append(b.wheres, fmt.Sprintf("%s IN (%s)", strings.ToLower(field), strings.Join(placeholders, ",")))
	return b
}

func (b *sqliteQueryBuilder) OrderBy(field string, desc bool) QueryBuilder {
	if err := validateName(field); err != nil {
		return b
	}
	dir := "ASC"
	if desc {
		dir = "DESC"
	}
	b.order = fmt.Sprintf("ORDER BY %s %s", strings.ToLower(field), dir)
	return b
}

func (b *sqliteQueryBuilder) Limit(n int) QueryBuilder {
	b.limit = n
	return b
}

func (b *sqliteQueryBuilder) Offset(n int) QueryBuilder {
	b.offset = n
	return b
}

func (b *sqliteQueryBuilder) Execute(ctx context.Context) ([]map[string]any, error) {
	if err := validateName(b.resource); err != nil {
		return nil, err
	}
	table := b.store.resolveTable(b.resource)
	query := fmt.Sprintf("SELECT * FROM %s", table)
	if len(b.wheres) > 0 {
		query += " WHERE " + strings.Join(b.wheres, " AND ")
	}
	if b.order != "" {
		query += " " + b.order
	}
	if b.limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", b.limit)
	}
	if b.offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", b.offset)
	}

	rows, err := b.store.db.QueryContext(ctx, query, b.args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows), nil
}

func (b *sqliteQueryBuilder) Count(ctx context.Context) (int, error) {
	if err := validateName(b.resource); err != nil {
		return 0, err
	}
	table := b.store.resolveTable(b.resource)
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
	if len(b.wheres) > 0 {
		query += " WHERE " + strings.Join(b.wheres, " AND ")
	}

	var count int
	err := b.store.db.QueryRowContext(ctx, query, b.args...).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (s *SQLiteStore) isFieldComplex(resource, field string, v any) bool {
	table := s.resolveTable(resource)
	col := strings.ToLower(field)

	if s.complexFields != nil && s.complexFields[table] != nil {
		if s.complexFields[table][col] {
			return true
		}
	}

	switch v.(type) {
	case map[string]any, []any, []map[string]any:
		return true
	default:
		return false
	}
}

func (s *SQLiteStore) Backup(destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o700); err != nil {
		return err
	}
	os.Remove(destPath)
	_, err := s.db.Exec(fmt.Sprintf("VACUUM INTO '%s'", destPath))
	return err
}

func Restore(srcPath, destPath string) error {
	input, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	return os.WriteFile(destPath, input, 0644)
}
