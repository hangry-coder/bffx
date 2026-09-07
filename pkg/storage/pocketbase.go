//go:build experimental_pocketbase

// Experimental: This driver is experimental and subject to change.
package storage

import (
	"context"
	"github.com/hangry-coder/bffx/pkg/errors"
	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type PocketBaseStore struct {
	url    string
	apiKey string
}

func NewPocketBaseStore(url, apiKey string) *PocketBaseStore {
	return &PocketBaseStore{
		url:    strings.TrimSuffix(url, "/"),
		apiKey: apiKey,
	}
}

func (s *PocketBaseStore) Reconcile(ctx context.Context, reg *manifest.Registry) ([]Change, error) {
	for _, r := range reg.Resources {
		var spec manifest.ResourceSpec
		if err := r.UnmarshalSpec(&spec); err != nil {
			continue
		}
		table := strings.ToLower(r.Metadata.Name)

		// check if collection exists
		checkUrl := fmt.Sprintf("%s/api/collections/%s", s.url, table)
		resp, err := s.doRequest(ctx, "GET", checkUrl, nil)
		if err != nil {
			return nil, fmt.Errorf("check collection %s: %w", table, err)
		}
		
		status := resp.StatusCode
		resp.Body.Close()

		if status == 404 {
			// Create collection
			createUrl := fmt.Sprintf("%s/api/collections", s.url)
			fields := []map[string]any{}
			for _, f := range spec.Fields {
				fields = append(fields, map[string]any{
					"name": strings.ToLower(f.Name),
					"type": mapPBType(f.Type),
				})
			}
			payload := map[string]any{
				"name":   table,
				"type":   "base",
				"schema": fields,
			}
			body, _ := json.Marshal(payload)
			resp, err := s.doRequest(ctx, "POST", createUrl, bytes.NewReader(body))
			if err != nil {
				return nil, fmt.Errorf("create collection %s: %w", table, err)
			}
			resp.Body.Close()
			logger.InfoCtx(ctx, "Created PocketBase collection: %s", table)
		} else {
			// Update collection (not implemented for alpha)
			return nil, fmt.Errorf("%w: PocketBase schema reconciliation (diff/update) not implemented", errors.ErrNotImplemented)
		}
	}
	return nil, nil
}

func mapPBType(t string) string {
	switch t {
	case "int", "float":
		return "number"
	case "bool":
		return "bool"
	case "file":
		return "file"
	default:
		return "text"
	}
}

func (s *PocketBaseStore) List(ctx context.Context, resource string, limit, offset int) ([]map[string]any, error) {
	table := strings.ToLower(resource)
	if limit <= 0 {
		limit = 30
	}
	page := (offset / limit) + 1

	url := fmt.Sprintf("%s/api/collections/%s/records?page=%d&perPage=%d", s.url, table, page, limit)
	resp, err := s.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (s *PocketBaseStore) ListByOwner(ctx context.Context, resource, ownerID string, limit, offset int) ([]map[string]any, error) {
	table := strings.ToLower(resource)
	if limit <= 0 {
		limit = 30
	}
	page := (offset / limit) + 1
	filter := fmt.Sprintf("(created_by='%s')", ownerID)

	url := fmt.Sprintf("%s/api/collections/%s/records?page=%d&perPage=%d&filter=%s", s.url, table, page, limit, filter)
	resp, err := s.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (s *PocketBaseStore) Get(ctx context.Context, resource, id string) (map[string]any, error) {
	table := strings.ToLower(resource)
	url := fmt.Sprintf("%s/api/collections/%s/records/%s", s.url, table, id)
	resp, err := s.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, errors.ErrNotFound
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("pocketbase error: %d", resp.StatusCode)
	}

	var item map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *PocketBaseStore) GetByField(ctx context.Context, resource, field, value string) (map[string]any, error) {
	table := strings.ToLower(resource)
	// PocketBase filter syntax: (field='value')
	filter := fmt.Sprintf("(%s='%s')", field, value)
	url := fmt.Sprintf("%s/api/collections/%s/records?page=1&perPage=1&filter=%s", s.url, table, filter)

	resp, err := s.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("pocketbase error: %d", resp.StatusCode)
	}

	var result struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Items) == 0 {
		return nil, errors.ErrNotFound
	}
	return result.Items[0], nil
}

func (s *PocketBaseStore) Create(ctx context.Context, resource string, payload map[string]any) (map[string]any, error) {
	table := strings.ToLower(resource)
	url := fmt.Sprintf("%s/api/collections/%s/records", s.url, table)
	
	body, _ := json.Marshal(payload)
	resp, err := s.doRequest(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("pocketbase create error: %d", resp.StatusCode)
	}

	var item map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *PocketBaseStore) Update(ctx context.Context, resource, id string, payload map[string]any) (map[string]any, error) {
	table := strings.ToLower(resource)
	url := fmt.Sprintf("%s/api/collections/%s/records/%s", s.url, table, id)
	
	body, _ := json.Marshal(payload)
	resp, err := s.doRequest(ctx, "PATCH", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, errors.ErrNotFound
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("pocketbase update error: %d", resp.StatusCode)
	}

	var item map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *PocketBaseStore) Delete(ctx context.Context, resource, id string) error {
	table := strings.ToLower(resource)
	url := fmt.Sprintf("%s/api/collections/%s/records/%s", s.url, table, id)
	resp, err := s.doRequest(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return errors.ErrNotFound
	}
	if resp.StatusCode != 204 {
		return fmt.Errorf("pocketbase delete error: %d", resp.StatusCode)
	}
	return nil
}

func (s *PocketBaseStore) GetChildren(ctx context.Context, resource, id string) ([]map[string]any, error) {
	return nil, fmt.Errorf("%w: GetChildren not implemented for PocketBase", errors.ErrNotImplemented)
}

func (s *PocketBaseStore) GetAncestors(ctx context.Context, resource, id string) ([]map[string]any, error) {
	return nil, fmt.Errorf("%w: GetAncestors not implemented for PocketBase", errors.ErrNotImplemented)
}

func (s *PocketBaseStore) Query(ctx context.Context, resource string) QueryBuilder {
	return &pocketbaseQueryBuilder{store: s, resource: resource}
}

type pocketbaseQueryBuilder struct {
	store    *PocketBaseStore
	resource string
}

func (b *pocketbaseQueryBuilder) Where(field, op string, value any) QueryBuilder   { return b }
func (b *pocketbaseQueryBuilder) WhereIn(field string, values []any) QueryBuilder { return b }
func (b *pocketbaseQueryBuilder) OrderBy(field string, desc bool) QueryBuilder     { return b }
func (b *pocketbaseQueryBuilder) Limit(n int) QueryBuilder                         { return b }
func (b *pocketbaseQueryBuilder) Offset(n int) QueryBuilder                        { return b }
func (b *pocketbaseQueryBuilder) Execute(ctx context.Context) ([]map[string]any, error) {
	return nil, fmt.Errorf("%w: QueryBuilder.Execute not implemented for PocketBase", errors.ErrNotImplemented)
}

func (b *pocketbaseQueryBuilder) Count(ctx context.Context) (int, error) {
	return 0, fmt.Errorf("%w: QueryBuilder.Count not implemented for PocketBase", errors.ErrNotImplemented)
}

func (s *PocketBaseStore) doRequest(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	if s.apiKey != "" {
		req.Header.Set("Authorization", s.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")
	
	return http.DefaultClient.Do(req)
}
