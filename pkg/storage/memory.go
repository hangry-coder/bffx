package storage

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	bffx_errors "github.com/hangry-coder/bffx/pkg/errors"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

type MemoryStore struct {
	mu   sync.RWMutex
	seq  map[string]int
	data map[string]map[string]map[string]any
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		seq:  map[string]int{},
		data: map[string]map[string]map[string]any{},
	}
}

func (s *MemoryStore) Reconcile(ctx context.Context, reg *manifest.Registry) ([]Change, error) {
	return nil, nil
}

func (s *MemoryStore) List(ctx context.Context, resource string, limit, offset int) ([]map[string]any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	records := s.data[resource]
	items := make([]map[string]any, 0, len(records))
	for _, item := range records {
		items = append(items, copyMap(item))
	}

	if offset >= len(items) {
		return []map[string]any{}, nil
	}

	end := offset + limit
	if limit <= 0 || end > len(items) {
		end = len(items)
	}

	return items[offset:end], nil
}

func (s *MemoryStore) ListByOwner(ctx context.Context, resource, ownerID string, limit, offset int) ([]map[string]any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	records := s.data[resource]
	var filtered []map[string]any
	for _, item := range records {
		if fmt.Sprintf("%v", item["created_by"]) == ownerID {
			filtered = append(filtered, copyMap(item))
		}
	}

	if offset >= len(filtered) {
		return []map[string]any{}, nil
	}

	end := offset + limit
	if limit <= 0 || end > len(filtered) {
		end = len(filtered)
	}

	return filtered[offset:end], nil
}

func (s *MemoryStore) Get(ctx context.Context, resource, id string) (map[string]any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.data[resource][id]
	if !ok {
		return nil, bffx_errors.ErrNotFound
	}
	return copyMap(record), nil
}

func (s *MemoryStore) GetByField(ctx context.Context, resource, field, value string) (map[string]any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	records, ok := s.data[resource]
	if !ok {
		return nil, bffx_errors.ErrNotFound
	}

	for _, item := range records {
		if fmt.Sprintf("%v", item[field]) == value {
			return copyMap(item), nil
		}
	}

	return nil, bffx_errors.ErrNotFound
}

func (s *MemoryStore) Create(ctx context.Context, resource string, payload map[string]any) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq[resource]++
	id := fmt.Sprintf("%d", s.seq[resource])
	if existing, ok := payload["id"].(string); ok && existing != "" {
		id = existing
	}

	if s.data[resource] == nil {
		s.data[resource] = map[string]map[string]any{}
	}
	record := copyMap(payload)
	record["id"] = id

	now := time.Now().UTC().Format(time.RFC3339)
	record["created_at"] = now
	record["updated_at"] = now

	s.data[resource][id] = record
	return copyMap(record), nil
}

func (s *MemoryStore) Update(ctx context.Context, resource, id string, payload map[string]any) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.data[resource][id]
	if !ok {
		return nil, bffx_errors.ErrNotFound
	}
	for k, v := range payload {
		if k == "id" || k == "created_at" {
			continue
		}
		record[k] = v
	}
	record["updated_at"] = time.Now().UTC().Format(time.RFC3339)
	return copyMap(record), nil
}

func (s *MemoryStore) Delete(ctx context.Context, resource, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, ok := s.data[resource]
	if !ok {
		return bffx_errors.ErrNotFound
	}
	_, ok = res[id]
	if ok {
		delete(res, id)
		return nil
	}
	return bffx_errors.ErrNotFound
}

func (s *MemoryStore) GetChildren(ctx context.Context, resource, id string) ([]map[string]any, error) {
	return nil, bffx_errors.ErrNotImplemented
}

func (s *MemoryStore) GetAncestors(ctx context.Context, resource, id string) ([]map[string]any, error) {
	return nil, bffx_errors.ErrNotImplemented
}

func (s *MemoryStore) Query(ctx context.Context, resource string) QueryBuilder {
	return &memoryQueryBuilder{store: s, resource: resource}
}

type memoryQueryBuilder struct {
	store     *MemoryStore
	resource  string
	filters   []filter
	limit     int
	offset    int
	orderBy   string
	orderDesc bool
}

type filter struct {
	field string
	op    string
	value any
}

func (b *memoryQueryBuilder) Where(field, op string, value any) QueryBuilder {
	b.filters = append(b.filters, filter{field, op, value})
	return b
}

func (b *memoryQueryBuilder) WhereIn(field string, values []any) QueryBuilder {
	b.filters = append(b.filters, filter{field, "in", values})
	return b
}

func (b *memoryQueryBuilder) OrderBy(field string, desc bool) QueryBuilder {
	b.orderBy = field
	b.orderDesc = desc
	return b
}

func (b *memoryQueryBuilder) Limit(n int) QueryBuilder {
	b.limit = n
	return b
}

func (b *memoryQueryBuilder) Offset(n int) QueryBuilder {
	b.offset = n
	return b
}

func (b *memoryQueryBuilder) Execute(ctx context.Context) ([]map[string]any, error) {
	b.store.mu.RLock()
	defer b.store.mu.RUnlock()

	records := b.store.data[b.resource]
	var results []map[string]any

	for _, item := range records {
		match := true
		for _, f := range b.filters {
			val := item[f.field]
			switch f.op {
			case "=", "eq":
				if fmt.Sprintf("%v", val) != fmt.Sprintf("%v", f.value) {
					match = false
				}
			case "!=", "neq":
				if fmt.Sprintf("%v", val) == fmt.Sprintf("%v", f.value) {
					match = false
				}
			case ">", "gt":
				match = compareValues(val, f.value) > 0
			case "<", "lt":
				match = compareValues(val, f.value) < 0
			case ">=", "gte":
				match = compareValues(val, f.value) >= 0
			case "<=", "lte":
				match = compareValues(val, f.value) <= 0
			case "in":
				vals, ok := f.value.([]any)
				if !ok {
					match = false
					break
				}
				found := false
				for _, v := range vals {
					if fmt.Sprintf("%v", val) == fmt.Sprintf("%v", v) {
						found = true
						break
					}
				}
				if !found {
					match = false
				}
			case "like":
				pattern := fmt.Sprintf("%v", f.value)
				pattern = strings.ReplaceAll(pattern, "%", ".*")
				matched, _ := regexp.MatchString("(?i)^"+pattern+"$", fmt.Sprintf("%v", val))
				if !matched {
					match = false
				}
			}
			if !match {
				break
			}
		}
		if match {
			results = append(results, copyMap(item))
		}
	}

	// Sort results if orderBy is specified
	if b.orderBy != "" {
		sort.Slice(results, func(i, j int) bool {
			valI := results[i][b.orderBy]
			valJ := results[j][b.orderBy]
			comp := compareValues(valI, valJ)
			if b.orderDesc {
				return comp > 0
			}
			return comp < 0
		})
	}

	// Apply Offset and Limit
	if b.offset > 0 && b.offset < len(results) {
		results = results[b.offset:]
	} else if b.offset >= len(results) {
		return []map[string]any{}, nil
	}

	if b.limit > 0 && b.limit < len(results) {
		results = results[:b.limit]
	}

	return results, nil
}

func (b *memoryQueryBuilder) Count(ctx context.Context) (int, error) {
	b.store.mu.RLock()
	defer b.store.mu.RUnlock()

	records := b.store.data[b.resource]
	count := 0

	for _, item := range records {
		match := true
		for _, f := range b.filters {
			val := item[f.field]
			switch strings.ToLower(f.op) {
			case "=", "eq":
				if fmt.Sprintf("%v", val) != fmt.Sprintf("%v", f.value) {
					match = false
				}
			case "!=", "neq":
				if fmt.Sprintf("%v", val) == fmt.Sprintf("%v", f.value) {
					match = false
				}
			case ">", "gt":
				match = compareValues(val, f.value) > 0
			case "<", "lt":
				match = compareValues(val, f.value) < 0
			case ">=", "gte":
				match = compareValues(val, f.value) >= 0
			case "<=", "lte":
				match = compareValues(val, f.value) <= 0
			case "in":
				vals, ok := f.value.([]any)
				if !ok {
					match = false
					break
				}
				found := false
				for _, v := range vals {
					if fmt.Sprintf("%v", val) == fmt.Sprintf("%v", v) {
						found = true
						break
					}
				}
				if !found {
					match = false
				}
			case "like":
				pattern := fmt.Sprintf("%v", f.value)
				pattern = strings.ReplaceAll(pattern, "%", ".*")
				matched, _ := regexp.MatchString("(?i)^"+pattern+"$", fmt.Sprintf("%v", val))
				if !matched {
					match = false
				}
			}
			if !match {
				break
			}
		}
		if match {
			count++
		}
	}

	return count, nil
}

func compareValues(a, b any) int {
	sa := fmt.Sprintf("%v", a)
	sb := fmt.Sprintf("%v", b)
	if sa < sb {
		return -1
	}
	if sa > sb {
		return 1
	}
	return 0
}

func copyMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}
