package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hangry-coder/bffx/pkg/admin/hooks"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"gopkg.in/yaml.v3"
)

func newResourceManifest(t *testing.T, name, feature, group string) *manifest.Manifest {
	t.Helper()
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(`
group: `+group+`
fields:
  - { name: key, type: string }
`), &node); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = *node.Content[0]
	}
	return &manifest.Manifest{
		Kind:     "Resource",
		Metadata: manifest.Metadata{Name: name},
		Feature:  feature,
		Spec:     node,
	}
}

// TestListResources_HidesReservedResources guards the fix for the
// "feature_flag table showing under Other Resources" report. Resources
// that have a dedicated admin UI surface (Feature Flags, Audit Log,
// Incidents, BFFX internal tables) MUST be filtered out of the generic
// CRUD listing, otherwise operators end up with two diverging stores.
func TestListResources_HidesReservedResources(t *testing.T) {
	reg := &manifest.Registry{
		Resources: []*manifest.Manifest{
			newResourceManifest(t, "user", "mobile", "mobile"),
			newResourceManifest(t, "feature_flag", "system", "system"),
			newResourceManifest(t, "audit_log", "system", "system"),
			newResourceManifest(t, "incident", "system", "system"),
			newResourceManifest(t, "bffx_incident", "system", "system"),
			newResourceManifest(t, "AdminUser", "system", "system"),
		},
	}

	h := NewResourceHandler(storage.NewMemoryStore(), storage.NewMemoryStore(), reg, nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/resources", nil)
	h.ListResources(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var groups map[string][]map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &groups); err != nil {
		t.Fatalf("json: %v", err)
	}

	for group, entries := range groups {
		for _, e := range entries {
			name, _ := e["name"].(string)
			if name == "feature_flag" || name == "audit_log" || name == "incident" || name == "AdminUser" || name == "bffx_incident" {
				t.Errorf("reserved resource %q leaked into Other Resources (group %q)", name, group)
			}
		}
	}

	// `user` is allowed to surface.
	found := false
	for _, entries := range groups {
		for _, e := range entries {
			if n, _ := e["name"].(string); n == "user" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected `user` to surface in Other Resources, groups=%+v", groups)
	}
}

func TestIsReservedAdminResource(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"feature_flag", true},
		{"FeatureFlag", true},
		{"audit_log", true},
		{"incident", true},
		{"bffx_screen_kill_switch", true},
		{"bffx_feature_flag", true},
		{"user", false},
		{"meal_log", false},
		{"workout", false},
	}
	for _, tc := range cases {
		if got := isReservedAdminResource(tc.in); got != tc.want {
			t.Errorf("isReservedAdminResource(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestGetConfig_DynamicFallback(t *testing.T) {
	reg := &manifest.Registry{
		Resources: []*manifest.Manifest{
			newResourceManifest(t, "user", "mobile", "mobile"),
		},
	}

	h := NewResourceHandler(storage.NewMemoryStore(), storage.NewMemoryStore(), reg, nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/config", nil)
	h.GetConfig(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var graph manifest.AdminGraph
	if err := json.Unmarshal(w.Body.Bytes(), &graph); err != nil {
		t.Fatalf("json: %v", err)
	}

	if graph.Site.Title == "" {
		t.Errorf("expected site title to be set, got empty")
	}

	foundUser := false
	for _, r := range graph.Resources {
		if r.Resource == "user" {
			foundUser = true
		}
	}
	if !foundUser {
		t.Errorf("expected to find synthesized user resource in admin graph")
	}
}

func TestGetRecords_FilteringAndPagination(t *testing.T) {
	// 1. Create backing resource manifest
	userManifest := newResourceManifest(t, "user", "mobile", "mobile")

	// 2. Create AdminResource manifest with scopes and filters
	var node yaml.Node
	yaml.Unmarshal([]byte(`
resource: user
index:
  per_page: 2
  default_sort: key_desc
  columns: [key]
  scopes:
    - name: all
      default: true
    - name: special
      where:
        key: special-val
  filters:
    - field: key
      as: string
      label: Key Filter
`), &node)
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = *node.Content[0]
	}
	adminResManifest := &manifest.Manifest{
		Kind:     "AdminResource",
		Metadata: manifest.Metadata{Name: "user_admin"},
		Spec:     node,
	}

	reg := &manifest.Registry{
		Resources:      []*manifest.Manifest{userManifest},
		AdminResources: []*manifest.Manifest{adminResManifest},
	}

	// 3. Setup store and populate with mock data
	store := storage.NewMemoryStore()
	ctx := context.Background()
	_, _ = store.Create(ctx, "user", map[string]any{"key": "special-val"})
	_, _ = store.Create(ctx, "user", map[string]any{"key": "other-val-1"})
	_, _ = store.Create(ctx, "user", map[string]any{"key": "other-val-2"})

	h := NewResourceHandler(store, store, reg, nil)

	// 4. Test filtering (contains)
	{
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/resources/user?filter_key_contains=other", nil)
		req.SetPathValue("name", "user")
		h.GetRecords(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}

		var records []map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &records); err != nil {
			t.Fatalf("json: %v", err)
		}

		if len(records) != 2 {
			t.Errorf("expected 2 records, got %d. records: %+v", len(records), records)
		}

		if w.Header().Get("X-Total-Count") != "2" {
			t.Errorf("expected X-Total-Count 2, got %q", w.Header().Get("X-Total-Count"))
		}
	}

	// 5. Test scope
	{
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/resources/user?scope=special", nil)
		req.SetPathValue("name", "user")
		h.GetRecords(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}

		var records []map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &records); err != nil {
			t.Fatalf("json: %v", err)
		}

		if len(records) != 1 {
			t.Errorf("expected 1 record, got %d", len(records))
		}
		if records[0]["key"] != "special-val" {
			t.Errorf("expected special-val, got %v", records[0]["key"])
		}
	}

	// 6. Test pagination
	{
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/resources/user?page=1&per_page=2", nil)
		req.SetPathValue("name", "user")
		h.GetRecords(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}

		var records []map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &records); err != nil {
			t.Fatalf("json: %v", err)
		}

		if len(records) != 2 {
			t.Errorf("expected 2 records, got %d", len(records))
		}

		if w.Header().Get("X-Total-Count") != "3" {
			t.Errorf("expected X-Total-Count 3, got %q", w.Header().Get("X-Total-Count"))
		}

		if records[0]["key"] != "special-val" {
			t.Errorf("expected first key to be special-val, got %v", records[0]["key"])
		}
	}
}

func TestGetRecord_AndSanitization(t *testing.T) {
	// Setup user manifest with some sensitive fields
	userManifest := newResourceManifest(t, "user", "mobile", "mobile")
	reg := &manifest.Registry{
		Resources: []*manifest.Manifest{userManifest},
	}

	store := storage.NewMemoryStore()
	ctx := context.Background()

	// Create a user record containing a password hash, token, and a regular name
	pwdHash := "$2a$10$abcdefghijklmnopqrstuvwxyz"
	rec, err := store.Create(ctx, "user", map[string]any{
		"name":          "John Doe",
		"password":      pwdHash,
		"password_hash": pwdHash,
		"otp_code":      "123456",
		"secret":        "my-secret",
		"token":         "my-token",
		"non_sensitive": "visible-value",
	})
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	id := rec["id"].(string)

	h := NewResourceHandler(store, store, reg, nil)

	// 1. Test GetRecord (detail endpoint) sanitizes sensitive fields
	{
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/resources/user/"+id, nil)
		req.SetPathValue("name", "user")
		req.SetPathValue("id", id)
		h.GetRecord(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}

		var record map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &record); err != nil {
			t.Fatalf("json: %v", err)
		}

		// Verify non-sensitive fields are present
		if record["name"] != "John Doe" || record["non_sensitive"] != "visible-value" {
			t.Errorf("missing non-sensitive fields in response: %+v", record)
		}

		// Verify sensitive fields are missing
		sensitiveKeys := []string{"password", "password_hash", "otp_code", "secret", "token"}
		for _, k := range sensitiveKeys {
			if _, exists := record[k]; exists {
				t.Errorf("sensitive field %q leaked in GetRecord: %+v", k, record)
			}
		}
	}

	// 2. Test GetRecords (list endpoint) sanitizes sensitive fields
	{
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/resources/user", nil)
		req.SetPathValue("name", "user")
		h.GetRecords(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}

		var records []map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &records); err != nil {
			t.Fatalf("json: %v", err)
		}

		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}

		record := records[0]
		// Verify sensitive fields are missing
		sensitiveKeys := []string{"password", "password_hash", "otp_code", "secret", "token"}
		for _, k := range sensitiveKeys {
			if _, exists := record[k]; exists {
				t.Errorf("sensitive field %q leaked in GetRecords (list): %+v", k, record)
			}
		}
	}

	// 3. Test UpdateRecord prevents overwriting password when empty or omitted
	{
		// First verify current password in storage is the original hash
		original, err := store.Get(ctx, "user", id)
		if err != nil {
			t.Fatalf("store get failed: %v", err)
		}
		if original["password"] != pwdHash {
			t.Fatalf("expected original password in store to be %q, got %q", pwdHash, original["password"])
		}

		// Perform an update where name changes, but password is sent as empty string
		w := httptest.NewRecorder()
		body := `{"name":"John Updated","password":"","non_sensitive":"updated-value"}`
		req := httptest.NewRequest(http.MethodPatch, "/api/admin/resources/user/"+id, strings.NewReader(body))
		req.SetPathValue("name", "user")
		req.SetPathValue("id", id)
		h.UpdateRecord(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("update status = %d, want 200", w.Code)
		}

		// Verify returned object is sanitized
		var record map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &record); err != nil {
			t.Fatalf("json: %v", err)
		}
		if record["name"] != "John Updated" {
			t.Errorf("name was not updated, got: %+v", record)
		}
		if _, exists := record["password"]; exists {
			t.Errorf("password leaked in UpdateRecord response: %+v", record)
		}

		// Verify database still has the original password hash and has not been overwritten with empty string
		updatedInDb, err := store.Get(ctx, "user", id)
		if err != nil {
			t.Fatalf("store get updated failed: %v", err)
		}
		if updatedInDb["password"] != pwdHash {
			t.Errorf("password was overwritten in DB! got %q, want %q", updatedInDb["password"], pwdHash)
		}
	}
}

func TestAdminActions(t *testing.T) {
	// Import hooks from bffx/pkg/admin/hooks
	userManifest := newResourceManifest(t, "user", "mobile", "mobile")
	var node yaml.Node
	yaml.Unmarshal([]byte(`
resource: user
batch_actions:
  - name: suspend
    label: Suspend Users
    hook: test.user.suspend
member_actions:
  - name: reset_password
    label: Reset Password
    hook: test.user.reset_password
collection_actions:
  - name: import_data
    label: Import Data
    hook: test.user.import_data
`), &node)
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = *node.Content[0]
	}
	adminResManifest := &manifest.Manifest{
		Kind:     "AdminResource",
		Metadata: manifest.Metadata{Name: "user_admin"},
		Spec:     node,
	}

	reg := &manifest.Registry{
		Resources:      []*manifest.Manifest{userManifest},
		AdminResources: []*manifest.Manifest{adminResManifest},
	}

	// Register test hooks
	var batchHookCalled, memberHookCalled, collectionHookCalled bool
	var batchIDs []string
	var batchPayload, memberPayload, collectionPayload map[string]any

	hooks.Register("test.user.suspend", func(ctx *hooks.Context, resource string, ids []string, payload map[string]any) (map[string]any, error) {
		batchHookCalled = true
		batchIDs = ids
		batchPayload = payload
		return map[string]any{"suspended": len(ids)}, nil
	})

	hooks.Register("test.user.reset_password", func(ctx *hooks.Context, resource string, ids []string, payload map[string]any) (map[string]any, error) {
		memberHookCalled = true
		memberPayload = payload
		return map[string]any{"status": "reset-sent", "id": ids[0]}, nil
	})

	hooks.Register("test.user.import_data", func(ctx *hooks.Context, resource string, ids []string, payload map[string]any) (map[string]any, error) {
		collectionHookCalled = true
		collectionPayload = payload
		return map[string]any{"imported": true}, nil
	})

	store := storage.NewMemoryStore()
	h := NewResourceHandler(store, store, reg, nil)

	// 1. Test Batch Action
	{
		w := httptest.NewRecorder()
		body := `{"ids":["u1","u2"],"payload":{"reason":"spam"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/resources/user/batch/suspend", strings.NewReader(body))
		req.SetPathValue("name", "user")
		req.SetPathValue("action", "suspend")
		h.HandleBatchAction(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("batch action status = %d, want 200", w.Code)
		}
		if !batchHookCalled {
			t.Errorf("batch hook was not called")
		}
		if len(batchIDs) != 2 || batchIDs[0] != "u1" || batchIDs[1] != "u2" {
			t.Errorf("incorrect batch IDs: %v", batchIDs)
		}
		if batchPayload["reason"] != "spam" {
			t.Errorf("incorrect batch payload: %v", batchPayload)
		}

		var res map[string]any
		json.Unmarshal(w.Body.Bytes(), &res)
		if res["suspended"] != float64(2) {
			t.Errorf("unexpected response: %+v", res)
		}
	}

	// 2. Test Member Action
	{
		w := httptest.NewRecorder()
		body := `{"notify":true}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/resources/user/u1/actions/reset_password", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("name", "user")
		req.SetPathValue("id", "u1")
		req.SetPathValue("action", "reset_password")
		h.HandleMemberAction(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("member action status = %d, want 200", w.Code)
		}
		if !memberHookCalled {
			t.Errorf("member hook was not called")
		}
		if memberPayload["notify"] != true {
			t.Errorf("incorrect member payload: %v", memberPayload)
		}

		var res map[string]any
		json.Unmarshal(w.Body.Bytes(), &res)
		if res["status"] != "reset-sent" || res["id"] != "u1" {
			t.Errorf("unexpected response: %+v", res)
		}
	}

	// 3. Test Collection Action
	{
		w := httptest.NewRecorder()
		body := `{"source":"s3://bucket/data.csv"}`
		req := httptest.NewRequest(http.MethodPost, "/api/admin/resources/user/collection/import_data", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("name", "user")
		req.SetPathValue("action", "import_data")
		h.HandleCollectionAction(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("collection action status = %d, want 200", w.Code)
		}
		if !collectionHookCalled {
			t.Errorf("collection hook was not called")
		}
		if collectionPayload["source"] != "s3://bucket/data.csv" {
			t.Errorf("incorrect collection payload: %v", collectionPayload)
		}

		var res map[string]any
		json.Unmarshal(w.Body.Bytes(), &res)
		if res["imported"] != true {
			t.Errorf("unexpected response: %+v", res)
		}
	}
}

func TestResourceHandler_ExportCSV(t *testing.T) {
	userManifest := newResourceManifest(t, "user", "mobile", "mobile")
	reg := &manifest.Registry{
		Resources: []*manifest.Manifest{userManifest},
	}
	store := storage.NewMemoryStore()
	ctx := context.Background()
	_, _ = store.Create(ctx, "user", map[string]any{"key": "val1"})
	_, _ = store.Create(ctx, "user", map[string]any{"key": "val2"})

	h := NewResourceHandler(store, store, reg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/resources/user/export.csv", nil)
	req.SetPathValue("name", "user")
	w := httptest.NewRecorder()
	h.ExportCSV(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/csv" {
		t.Errorf("expected Content-Type text/csv, got %q", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "key") || !strings.Contains(body, "val1") || !strings.Contains(body, "val2") {
		t.Errorf("expected body to contain headers and values, got:\n%s", body)
	}
}

func TestResourceHandler_RBACPolicy(t *testing.T) {
	userManifest := newResourceManifest(t, "user", "mobile", "mobile")
	var node yaml.Node
	yaml.Unmarshal([]byte(`
resource: user
policy:
  read: admin
  write: superadmin
`), &node)
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = *node.Content[0]
	}
	adminResManifest := &manifest.Manifest{
		Kind:     "AdminResource",
		Metadata: manifest.Metadata{Name: "user_admin"},
		Spec:     node,
	}

	reg := &manifest.Registry{
		Resources:      []*manifest.Manifest{userManifest},
		AdminResources: []*manifest.Manifest{adminResManifest},
	}
	store := storage.NewMemoryStore()
	h := NewResourceHandler(store, store, reg, nil)

	// Helper to create request with context role
	reqWithRole := func(method, path string, role string) *http.Request {
		req := httptest.NewRequest(method, path, nil)
		ctx := context.WithValue(req.Context(), "admin_role", role)
		return req.WithContext(ctx)
	}

	// 1. Operator role: Read policy is admin, so Operator (weight 10) is denied read access
	{
		req := reqWithRole(http.MethodGet, "/api/admin/resources/user", "operator")
		req.SetPathValue("name", "user")
		w := httptest.NewRecorder()
		h.GetRecords(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for operator read, got %d", w.Code)
		}
	}

	// 2. Admin role: Read policy is admin, so Admin (weight 20) is allowed read access
	{
		req := reqWithRole(http.MethodGet, "/api/admin/resources/user", "admin")
		req.SetPathValue("name", "user")
		w := httptest.NewRecorder()
		h.GetRecords(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK for admin read, got %d", w.Code)
		}
	}

	// 3. Admin role: Write policy is superadmin, so Admin (weight 20) is denied write access
	{
		req := reqWithRole(http.MethodDelete, "/api/admin/resources/user/1", "admin")
		req.SetPathValue("name", "user")
		req.SetPathValue("id", "1")
		w := httptest.NewRecorder()
		h.DeleteRecord(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for admin write, got %d", w.Code)
		}
	}

	// 4. Superadmin role: Write policy is superadmin, so Superadmin (weight 30) is allowed write access
	{
		req := reqWithRole(http.MethodDelete, "/api/admin/resources/user/1", "superadmin")
		req.SetPathValue("name", "user")
		req.SetPathValue("id", "1")
		w := httptest.NewRecorder()
		h.DeleteRecord(w, req)
		// Since ID 1 doesn't exist, we expect a 404 (indicating it bypassed the authorization check!)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found for superadmin write, got %d", w.Code)
		}
	}
}
