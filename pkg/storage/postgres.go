package storage

import (
	bffx_errors "github.com/hangry-coder/bffx/pkg/errors"
	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage/schema"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	_ "github.com/lib/pq"
)

type PostgresStore struct {
	db            *sql.DB
	reg           *manifest.Registry
	complexFields map[string]map[string]bool // resource -> field -> isComplex
	idSerial      map[string]bool            // table -> id uses DB serial/bigint
}

func NewPostgresStore(url string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &PostgresStore{
		db:            db,
		complexFields: make(map[string]map[string]bool),
		idSerial:      make(map[string]bool),
	}, nil
}

func (s *PostgresStore) idUsesDBSerial(table string) bool {
	if v, ok := s.idSerial[table]; ok {
		return v
	}
	var dt string
	err := s.db.QueryRow(
		`SELECT data_type FROM information_schema.columns
		 WHERE table_schema = 'public' AND table_name = $1 AND column_name = 'id'`,
		table,
	).Scan(&dt)
	serial := err == nil && (dt == "bigint" || dt == "integer" || dt == "smallint")
	s.idSerial[table] = serial
	return serial
}

func (s *PostgresStore) GetDB() *sql.DB {
	return s.db
}

func (s *PostgresStore) tableExists(tableName string) bool {
	var name string
	err := s.db.QueryRow("SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_name=$1", tableName).Scan(&name)
	return err == nil
}

func (s *PostgresStore) resolveTable(resourceName string) string {
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

// Reconcile is the legacy declarative-sync engine. It is preserved as a fallback
// for projects that have NOT yet adopted the versioned migration system
// (`bffx migrate plan/apply`). When `migrations/` is present, the orchestrator
// in pkg/app/server.go will use the migrator instead of calling this method.
const pgSchemaSingletonLockKey int64 = 783_921_001

func (s *PostgresStore) Reconcile(ctx context.Context, reg *manifest.Registry) ([]Change, error) {
	if os.Getenv("BFFX_SCHEMA_SINGLETON") != "false" {
		if _, err := s.db.ExecContext(ctx, "SELECT pg_advisory_lock($1)", pgSchemaSingletonLockKey); err != nil {
			return nil, fmt.Errorf("schema singleton lock: %w", err)
		}
		defer func() {
			if _, err := s.db.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", pgSchemaSingletonLockKey); err != nil {
				logger.WarnCtx(ctx, "pg_advisory_unlock: %v", err)
			}
		}()
	}

	hash := reg.ResourceHash()
	hashFile := ".bffx/schema.hash"

	if cached, err := os.ReadFile(hashFile); err == nil {
		if strings.TrimSpace(string(cached)) == hash {
			logger.DebugCtx(ctx, "postgres reconcile: schema is up to date (hash matches), skipping.")
			return nil, nil
		}
	}

	var changes []Change
	s.complexFields = make(map[string]map[string]bool)

	for _, r := range reg.Resources {
		var spec manifest.ResourceSpec
		if err := r.UnmarshalSpec(&spec); err != nil {
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
			logger.ErrorCtx(ctx, "postgres reconcile error: %v", err)
			continue
		}

		query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id BIGSERIAL PRIMARY KEY)", table)
		if _, err := s.db.ExecContext(ctx, query); err != nil {
			return nil, fmt.Errorf("create table %s: %w", table, err)
		}

		rows, err := s.db.QueryContext(ctx, "SELECT column_name FROM information_schema.columns WHERE table_name = $1", table)
		if err != nil {
			return nil, fmt.Errorf("lookup columns for %s: %w", table, err)
		}

		existingCols := make(map[string]bool)
		for rows.Next() {
			var colName string
			if err := rows.Scan(&colName); err == nil {
				existingCols[colName] = true
			}
		}
		rows.Close()

		for _, col := range []string{"created_by", "created_at", "updated_at"} {
			if !existingCols[col] {
				alter := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s TEXT", table, col)
				logger.InfoCtx(ctx, "postgres reconcile: adding column %s to %s", col, table)
				s.db.ExecContext(ctx, alter)
				changes = append(changes, Change{Resource: r.Metadata.Name, Action: "added", Field: col})
			}
		}

		for _, f := range spec.Fields {
			if err := validateName(f.Name); err != nil {
				logger.ErrorCtx(ctx, "postgres reconcile error: invalid column %q for table %q", f.Name, table)
				continue
			}
			colName := strings.ToLower(f.Name)
			if colName == "id" || existingCols[colName] {
				continue
			}
			typ := "TEXT"
			switch f.Type {
			case "int":
				typ = "BIGINT"
			case "float":
				typ = "DOUBLE PRECISION"
			case "bool":
				typ = "BOOLEAN"
			}
			alter := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, colName, typ)
			if _, err := s.db.ExecContext(ctx, alter); err == nil {
				changes = append(changes, Change{Resource: r.Metadata.Name, Action: "added", Field: colName})
			} else {
				logger.ErrorCtx(ctx, "failed to add column %s to %s: %v", colName, table, err)
			}
		}

		if manifest.TreeEnabled(spec.Tree) {
			if !existingCols["parent_id"] {
				alter := fmt.Sprintf("ALTER TABLE %s ADD COLUMN parent_id BIGINT REFERENCES %s(id)", table, table)
				if _, err := s.db.ExecContext(ctx, alter); err == nil {
					changes = append(changes, Change{Resource: r.Metadata.Name, Action: "added", Field: "parent_id"})
				} else {
					logger.ErrorCtx(ctx, "failed to add parent_id to tree table %s: %v", table, err)
				}
			}

			hTable := table + "_hierarchies"
			hQuery := fmt.Sprintf(`
				CREATE TABLE IF NOT EXISTS %s (
					ancestor_id BIGINT NOT NULL,
					descendant_id BIGINT NOT NULL,
					generations INTEGER NOT NULL,
					PRIMARY KEY (ancestor_id, descendant_id),
					FOREIGN KEY (ancestor_id) REFERENCES %s(id) ON DELETE CASCADE,
					FOREIGN KEY (descendant_id) REFERENCES %s(id) ON DELETE CASCADE
				)`, hTable, table, table)
			if _, err := s.db.ExecContext(ctx, hQuery); err != nil {
				return nil, fmt.Errorf("create hierarchy table %s: %w", hTable, err)
			}
			s.db.ExecContext(ctx, fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_descendant ON %s(descendant_id)", hTable, hTable))
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
					logger.WarnCtx(ctx, "postgres reconcile: failed to create index %s: %v", indexName, err)
				}
			}
		}
	}

	os.MkdirAll(".bffx", 0o755)
	os.WriteFile(hashFile, []byte(hash), 0o644)

	return changes, nil
}

func (s *PostgresStore) List(ctx context.Context, resource string, limit, offset int) ([]map[string]any, error) {
	if err := validateName(resource); err != nil {
		return nil, err
	}
	table := s.resolveTable(resource)

	query := fmt.Sprintf("SELECT * FROM %s", table)
	args := []any{}
	arg := 1
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", arg)
		args = append(args, limit)
		arg++
	}
	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", arg)
		args = append(args, offset)
		arg++
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRows(rows), nil
}

func (s *PostgresStore) ListByOwner(ctx context.Context, resource, ownerID string, limit, offset int) ([]map[string]any, error) {
	if err := validateName(resource); err != nil {
		return nil, err
	}
	table := s.resolveTable(resource)

	query := fmt.Sprintf("SELECT * FROM %s WHERE created_by = $1", table)
	args := []any{ownerID}
	arg := 2
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", arg)
		args = append(args, limit)
		arg++
	}
	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", arg)
		args = append(args, offset)
		arg++
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRows(rows), nil
}

func (s *PostgresStore) Get(ctx context.Context, resource, id string) (map[string]any, error) {
	if err := validateName(resource); err != nil {
		return nil, err
	}
	table := s.resolveTable(resource)
	query := fmt.Sprintf("SELECT * FROM %s WHERE id = $1", table)
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

func (s *PostgresStore) GetByField(ctx context.Context, resource, field, value string) (map[string]any, error) {
	if err := validateName(resource); err != nil {
		return nil, err
	}
	if err := validateName(field); err != nil {
		return nil, err
	}

	table := s.resolveTable(resource)
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = $1 LIMIT 1", table, strings.ToLower(field))
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

func (s *PostgresStore) Create(ctx context.Context, resource string, payload map[string]any) (map[string]any, error) {
	if err := validateName(resource); err != nil {
		return nil, err
	}
	table := s.resolveTable(resource)
	useSerialID := s.idUsesDBSerial(table)
	if !useSerialID {
		if payload["id"] == nil || strings.TrimSpace(fmt.Sprintf("%v", payload["id"])) == "" {
			payload["id"] = uuid.NewString()
		}
	}

	var cols []string
	var placeHolders []string
	var args []any

	i := 1
	for k, v := range payload {
		if k == "created_at" || k == "updated_at" {
			continue
		}
		if k == "id" && useSerialID {
			continue
		}
		if err := validateName(k); err != nil {
			return nil, err
		}
		cols = append(cols, k)
		placeHolders = append(placeHolders, fmt.Sprintf("$%d", i))
		i++

		if s.isFieldComplex(resource, k, v) {
			b, _ := json.Marshal(v)
			args = append(args, string(b))
		} else {
			args = append(args, v)
		}
	}

	// Auto-Timestamps
	now := time.Now().UTC().Format(time.RFC3339)
	cols = append(cols, "created_at", "updated_at")
	placeHolders = append(placeHolders, fmt.Sprintf("$%d", i), fmt.Sprintf("$%d", i+1))
	args = append(args, now, now)

	colsQuery := strings.Join(cols, ",")
	placeholderQuery := strings.Join(placeHolders, ",")
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING id", table, colsQuery, placeholderQuery)

	var rawID any
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&rawID)
	if err != nil {
		return nil, err
	}
	if rawID != nil {
		payload["id"] = fmt.Sprintf("%v", rawID)
	}

	// Tree Logic: Update Hierarchy Table
	if s.isTree(resource) {
		hTable := table + "_hierarchies"

		// 1. Insert self-reference
		selfQuery := fmt.Sprintf("INSERT INTO %s (ancestor_id, descendant_id, generations) VALUES ($1, $1, 0)", hTable)
		s.db.ExecContext(ctx, selfQuery, payload["id"])

		// 2. Insert ancestor paths if parent_id exists
		if pid, ok := payload["parent_id"]; ok && pid != nil {
			parentID := fmt.Sprintf("%v", pid)
			if parentID != "" && parentID != "0" {
				pathQuery := fmt.Sprintf(`
					INSERT INTO %s (ancestor_id, descendant_id, generations)
					SELECT ancestor_id, $1, generations + 1
					FROM %s
					WHERE descendant_id = $2
				`, hTable, hTable)
				_, err := s.db.ExecContext(ctx, pathQuery, payload["id"], parentID)
				if err != nil {
					logger.ErrorCtx(ctx, "postgres tree path insert error: %v", err)
				}
			}
		}
	}

	return payload, nil
}

func (s *PostgresStore) isTree(resource string) bool {
	if s.reg == nil {
		return false
	}
	for _, r := range s.reg.Resources {
		if r.Metadata.Name == resource {
			var spec manifest.ResourceSpec
			_ = r.UnmarshalSpec(&spec)
			return manifest.TreeEnabled(spec.Tree)
		}
	}
	return false
}

func (s *PostgresStore) Update(ctx context.Context, resource, id string, payload map[string]any) (map[string]any, error) {
	if err := validateName(resource); err != nil {
		return nil, err
	}
	table := s.resolveTable(resource)
	var sets []string
	var args []any

	i := 1
	for k, v := range payload {
		if k == "id" || k == "created_at" || k == "updated_at" {
			continue
		}
		if err := validateName(k); err != nil {
			return nil, err
		}
		sets = append(sets, fmt.Sprintf("%s = $%d", k, i))
		i++
		if s.isFieldComplex(resource, k, v) {
			b, _ := json.Marshal(v)
			args = append(args, string(b))
		} else {
			args = append(args, v)
		}
	}

	// Update timestamp
	now := time.Now().UTC().Format(time.RFC3339)
	sets = append(sets, fmt.Sprintf("updated_at = $%d", i))
	args = append(args, now)
	i++

	args = append(args, id)
	setsQuery := strings.Join(sets, ",")

	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = $%d", table, setsQuery, i)
	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	// Tree Logic: Handle Move
	if s.isTree(resource) {
		if newPID, ok := payload["parent_id"]; ok {
			s.moveTreeNode(ctx, resource, id, fmt.Sprintf("%v", newPID))
		}
	}

	return s.Get(ctx, resource, id)
}

func (s *PostgresStore) moveTreeNode(ctx context.Context, resource, id, newParentID string) {
	table := s.resolveTable(resource)
	hTable := table + "_hierarchies"

	// 1. Disconnect subtree from old ancestors
	disconnectQuery := fmt.Sprintf(`
		DELETE FROM %s
		WHERE descendant_id IN (SELECT descendant_id FROM %s WHERE ancestor_id = $1)
		AND ancestor_id IN (SELECT ancestor_id FROM %s WHERE descendant_id = $1 AND ancestor_id != descendant_id)
	`, hTable, hTable, hTable)
	s.db.ExecContext(ctx, disconnectQuery, id)

	// 2. Connect subtree to new ancestors
	if newParentID != "" && newParentID != "0" && newParentID != "<nil>" {
		connectQuery := fmt.Sprintf(`
			INSERT INTO %s (ancestor_id, descendant_id, generations)
			SELECT super.ancestor_id, sub.descendant_id, super.generations + sub.generations + 1
			FROM %s as super, %s as sub
			WHERE super.descendant_id = $1
			AND sub.ancestor_id = $2
		`, hTable, hTable, hTable)
		// $1 is new parent, $2 is the node being moved
		_, err := s.db.ExecContext(ctx, connectQuery, newParentID, id)
		if err != nil {
			logger.ErrorCtx(ctx, "postgres tree move connect error: %v", err)
		}
	}
}

func (s *PostgresStore) Delete(ctx context.Context, resource, id string) error {
	if err := validateName(resource); err != nil {
		return err
	}
	table := s.resolveTable(resource)
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", table)
	res, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return bffx_errors.ErrNotFound
	}
	return nil
}

// Tree Helpers
func (s *PostgresStore) GetChildren(ctx context.Context, resource, id string) ([]map[string]any, error) {
	if err := validateName(resource); err != nil {
		return nil, err
	}
	table := s.resolveTable(resource)
	hTable := table + "_hierarchies"
	query := fmt.Sprintf(`
		SELECT t.* 
		FROM %s t
		JOIN %s h ON t.id = h.descendant_id
		WHERE h.ancestor_id = $1 AND h.generations = 1
	`, table, hTable)
	rows, err := s.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows), nil
}

func (s *PostgresStore) GetAncestors(ctx context.Context, resource, id string) ([]map[string]any, error) {
	if err := validateName(resource); err != nil {
		return nil, err
	}
	table := s.resolveTable(resource)
	hTable := table + "_hierarchies"
	query := fmt.Sprintf(`
		SELECT t.* 
		FROM %s t
		JOIN %s h ON t.id = h.ancestor_id
		WHERE h.descendant_id = $1 AND h.generations > 0
		ORDER BY h.generations DESC
	`, table, hTable)
	rows, err := s.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows), nil
}

// Query Power (ActiveRecord-like)
func (s *PostgresStore) Query(ctx context.Context, resource string) QueryBuilder {
	return &postgresQueryBuilder{
		store:    s,
		resource: resource,
	}
}

type postgresQueryBuilder struct {
	store    *PostgresStore
	resource string
	wheres   []string
	args     []any
	order    string
	limit    int
	offset   int
}

func (b *postgresQueryBuilder) Where(field, op string, value any) QueryBuilder {
	if err := validateName(field); err != nil {
		logger.Error("postgres query builder where error: %v", err) // No ctx here
		return b
	}
	// Whitelist operators
	opUpper := strings.ToUpper(op)
	validOps := map[string]bool{"=": true, "!=": true, ">": true, "<": true, ">=": true, "<=": true, "LIKE": true, "IN": true}
	if !validOps[opUpper] {
		logger.Error("postgres query builder where error: invalid operator %q", op) // No ctx here
		return b
	}

	if opUpper == "IN" {
		if vals, ok := value.([]any); ok {
			return b.WhereIn(field, vals)
		}
	}

	b.args = append(b.args, value)
	b.wheres = append(b.wheres, fmt.Sprintf("%s %s $%d", strings.ToLower(field), op, len(b.args)))
	return b
}

func (b *postgresQueryBuilder) WhereIn(field string, values []any) QueryBuilder {
	if len(values) == 0 {
		// If empty, we add a condition that is always false to ensure no results
		b.wheres = append(b.wheres, "1 = 0")
		return b
	}
	if err := validateName(field); err != nil {
		logger.Error("postgres query builder wherein error: %v", err)
		return b
	}
	placeholders := make([]string, len(values))
	for i := range values {
		b.args = append(b.args, values[i])
		placeholders[i] = fmt.Sprintf("$%d", len(b.args))
	}
	b.wheres = append(b.wheres, fmt.Sprintf("%s IN (%s)", strings.ToLower(field), strings.Join(placeholders, ",")))
	return b
}

func (b *postgresQueryBuilder) OrderBy(field string, desc bool) QueryBuilder {
	if err := validateName(field); err != nil {
		logger.Error("postgres query builder orderby error: %v", err)
		return b
	}
	dir := "ASC"
	if desc {
		dir = "DESC"
	}
	b.order = fmt.Sprintf("ORDER BY %s %s", strings.ToLower(field), dir)
	return b
}

func (b *postgresQueryBuilder) Limit(n int) QueryBuilder {
	b.limit = n
	return b
}

func (b *postgresQueryBuilder) Offset(n int) QueryBuilder {
	b.offset = n
	return b
}

func (b *postgresQueryBuilder) Execute(ctx context.Context) ([]map[string]any, error) {
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
	args := append([]any(nil), b.args...)
	argNum := len(args) + 1
	if b.limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, b.limit)
		argNum++
	}
	if b.offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argNum)
		args = append(args, b.offset)
		argNum++
	}

	rows, err := b.store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows), nil
}

func (b *postgresQueryBuilder) Count(ctx context.Context) (int, error) {
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

func (s *PostgresStore) isFieldComplex(resource, field string, v any) bool {
	table := s.resolveTable(resource)
	col := strings.ToLower(field)

	if s.complexFields != nil && s.complexFields[table] != nil {
		if s.complexFields[table][col] {
			return true
		}
	}

	// Fallback to type switch for safety
	switch v.(type) {
	case map[string]any, []any, []map[string]any:
		return true
	default:
		return false
	}
}
