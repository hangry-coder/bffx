package storage

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

// Change represents a structural or data change detected during reconciliation.
type Change struct {
	Resource string
	Action   string // "added", "removed", "modified"
	Field    string
}

// Store is the persistence surface for BFFX resources. Every method takes
// ctx as the first argument so cancellation, deadlines, and trace context flow
// from HTTP handlers and workers into database/sql and remote drivers.
// Call sites should pass request-scoped contexts (for example r.Context()) on
// the hot path rather than context.Background().
type Store interface {
	// List returns a slice of maps representing the resources found.
	List(ctx context.Context, resource string, limit, offset int) ([]map[string]any, error)
	// Get retrieves a single resource by its primary ID.
	Get(ctx context.Context, resource, id string) (map[string]any, error)
	// GetByField retrieves a single resource by a specific unique field value.
	GetByField(ctx context.Context, resource, field, value string) (map[string]any, error)
	// Create persists a new resource and returns the created record.
	Create(ctx context.Context, resource string, payload map[string]any) (map[string]any, error)
	// Update modifies an existing resource by ID and returns the updated record.
	Update(ctx context.Context, resource, id string, payload map[string]any) (map[string]any, error)
	// Delete removes a resource by its ID.
	Delete(ctx context.Context, resource, id string) error
	// ListByOwner returns resources belonging to a specific owner ID.
	ListByOwner(ctx context.Context, resource, ownerID string, limit, offset int) ([]map[string]any, error)
	// Reconcile synchronizes the database schema with the provided registry.
	Reconcile(ctx context.Context, reg *manifest.Registry) ([]Change, error)

	// Tree Helpers (Self-referential or hierarchical resource support)
	
	// GetChildren returns immediate child resources.
	GetChildren(ctx context.Context, resource, id string) ([]map[string]any, error)
	// GetAncestors returns the breadcrumb path to the root.
	GetAncestors(ctx context.Context, resource, id string) ([]map[string]any, error)

	// Query Power (ActiveRecord-like)
	
	// Query starts a new fluent query builder for the given resource.
	Query(ctx context.Context, resource string) QueryBuilder
}

// QueryBuilder provides a fluent interface for building complex database queries.
type QueryBuilder interface {
	// Where adds a simple equality or comparison filter.
	Where(field, op string, value any) QueryBuilder
	// WhereIn adds a membership filter.
	WhereIn(field string, values []any) QueryBuilder
	// OrderBy sets the sort order.
	OrderBy(field string, desc bool) QueryBuilder
	// Limit restricts the result set size.
	Limit(n int) QueryBuilder
	// Offset sets the starting point for results.
	Offset(n int) QueryBuilder
	// Execute performs the query and returns the results.
	Execute(ctx context.Context) ([]map[string]any, error)
	// Count returns the number of records matching the query filters, ignoring Limit/Offset.
	Count(ctx context.Context) (int, error)
}

// RouterStore wraps multiple storage backends and routes requests based on resource configuration.
// It typically routes telemetry resources to a separate store (e.g. Postgres) from primary data.
type RouterStore struct {
	Primary   Store
	Telemetry Store
	Registry  *manifest.Registry
}

func (s *RouterStore) getStore(resource string) Store {
	if s.Registry != nil {
		if res, ok := s.Registry.GetResource(resource); ok {
			var spec manifest.ResourceSpec
			res.UnmarshalSpec(&spec)
			if spec.Telemetry && s.Telemetry != nil {
				return s.Telemetry
			}
		}
	}
	return s.Primary
}

func (s *RouterStore) List(ctx context.Context, resource string, limit, offset int) ([]map[string]any, error) {
	return s.getStore(resource).List(ctx, resource, limit, offset)
}

func (s *RouterStore) Get(ctx context.Context, resource, id string) (map[string]any, error) {
	return s.getStore(resource).Get(ctx, resource, id)
}

func (s *RouterStore) GetByField(ctx context.Context, resource, field, value string) (map[string]any, error) {
	return s.getStore(resource).GetByField(ctx, resource, field, value)
}

func (s *RouterStore) Create(ctx context.Context, resource string, payload map[string]any) (map[string]any, error) {
	if payload == nil {
		payload = make(map[string]any)
	}
	now := time.Now().Format(time.RFC3339)
	if payload["created_at"] == nil {
		payload["created_at"] = now
	}
	if payload["updated_at"] == nil {
		payload["updated_at"] = now
	}
	return s.getStore(resource).Create(ctx, resource, payload)
}

func (s *RouterStore) Update(ctx context.Context, resource, id string, payload map[string]any) (map[string]any, error) {
	if payload == nil {
		payload = make(map[string]any)
	}
	payload["updated_at"] = time.Now().Format(time.RFC3339)
	return s.getStore(resource).Update(ctx, resource, id, payload)
}

func (s *RouterStore) Delete(ctx context.Context, resource, id string) error {
	return s.getStore(resource).Delete(ctx, resource, id)
}

func (s *RouterStore) ListByOwner(ctx context.Context, resource, ownerID string, limit, offset int) ([]map[string]any, error) {
	return s.getStore(resource).ListByOwner(ctx, resource, ownerID, limit, offset)
}

func (s *RouterStore) Reconcile(ctx context.Context, reg *manifest.Registry) ([]Change, error) {
	return s.Primary.Reconcile(ctx, reg)
}

func (s *RouterStore) GetChildren(ctx context.Context, resource, id string) ([]map[string]any, error) {
	return s.getStore(resource).GetChildren(ctx, resource, id)
}

func (s *RouterStore) GetAncestors(ctx context.Context, resource, id string) ([]map[string]any, error) {
	return s.getStore(resource).GetAncestors(ctx, resource, id)
}

func (s *RouterStore) Query(ctx context.Context, resource string) QueryBuilder {
	return s.getStore(resource).Query(ctx, resource)
}

// InvalidatingStore wraps a Store and executes a callback after successful mutations.
// It is used by the framework to trigger cache invalidation.
type InvalidatingStore struct {
	Store
	OnWrite func(ctx context.Context, resource string, id string)
}

func (s *InvalidatingStore) Create(ctx context.Context, resource string, payload map[string]any) (map[string]any, error) {
	res, err := s.Store.Create(ctx, resource, payload)
	if err == nil && s.OnWrite != nil {
		id, _ := res["id"].(string)
		s.OnWrite(ctx, resource, id)
	}
	return res, err
}

func (s *InvalidatingStore) Update(ctx context.Context, resource, id string, payload map[string]any) (map[string]any, error) {
	res, err := s.Store.Update(ctx, resource, id, payload)
	if err == nil && s.OnWrite != nil {
		s.OnWrite(ctx, resource, id)
	}
	return res, err
}

func (s *InvalidatingStore) Delete(ctx context.Context, resource, id string) error {
	err := s.Store.Delete(ctx, resource, id)
	if err == nil && s.OnWrite != nil {
		s.OnWrite(ctx, resource, id)
	}
	return err
}

func DeepCopy(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	cp := make(map[string]any)
	for k, v := range m {
		if vm, ok := v.(map[string]any); ok {
			cp[k] = DeepCopy(vm)
		} else if vs, ok := v.([]any); ok {
			cp[k] = deepCopySlice(vs)
		} else {
			cp[k] = v
		}
	}
	return cp
}

func deepCopySlice(s []any) []any {
	cp := make([]any, len(s))
	for i, v := range s {
		if vm, ok := v.(map[string]any); ok {
			cp[i] = DeepCopy(vm)
		} else if vs, ok := v.([]any); ok {
			cp[i] = deepCopySlice(vs)
		} else {
			cp[i] = v
		}
	}
	return cp
}

var nameRegexp = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

func validateName(name string) error {
	if !nameRegexp.MatchString(name) {
		return fmt.Errorf("invalid name (SQL injection risk): %q", name)
	}
	return nil
}

func scanRows(rows *sql.Rows) []map[string]any {
	var results []map[string]any

	for rows.Next() {
		m, err := scanRow(rows)
		if err != nil {
			continue
		}
		results = append(results, m)
	}
	return results
}

func scanRow(rows *sql.Rows) (map[string]any, error) {
	cols, _ := rows.Columns()
	values := make([]any, len(cols))
	pointers := make([]any, len(cols))
	for i := range values {
		pointers[i] = &values[i]
	}

	if err := rows.Scan(pointers...); err != nil {
		return nil, err
	}

	m := make(map[string]any)
	for i, col := range cols {
		val := values[i]
		if b, ok := val.([]byte); ok {
			str := string(b)
			if len(str) > 0 && (str[0] == '{' || str[0] == '[') {
				var complex any
				if err := json.Unmarshal(b, &complex); err == nil {
					m[col] = complex
					continue
				}
			}
			m[col] = str
		} else if s, ok := val.(string); ok {
			if len(s) > 0 && (s[0] == '{' || s[0] == '[') {
				var complex any
				if err := json.Unmarshal([]byte(s), &complex); err == nil {
					m[col] = complex
					continue
				}
			}
			m[col] = s
		} else {
			m[col] = val
		}
	}
	return m, nil
}

