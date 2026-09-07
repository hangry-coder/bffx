package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/hangry-coder/bffx/pkg/audit"
	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
)

type ResourceHandler struct {
	store     storage.Store
	telemetry storage.Store
	reg       *manifest.Registry
	auditor   *audit.Auditor
}

func NewResourceHandler(store, telemetry storage.Store, reg *manifest.Registry, auditor *audit.Auditor) *ResourceHandler {
	return &ResourceHandler{store: store, telemetry: telemetry, reg: reg, auditor: auditor}
}

// reservedAdminResources are resources that have a dedicated admin UI surface
// elsewhere in the panel, so they MUST NOT appear in the generic "Other
// Resources" CRUD list. Surfacing them in two places confused operators (see
// "Feature Flag in MAIN nav empty while the table shows under Other
// Resources" report) and lets stale CRUD edits diverge from the canonical
// backing store (`bffx_feature_flag`, the audit pipeline, the incident
// workflow, etc.).
//
// Keep the set narrow and case-insensitive. Anything prefixed with `bffx_`
// is always reserved.
var reservedAdminResources = map[string]struct{}{
	"feature_flag":  {},
	"feature_flags": {},
	"featureflag":   {},
	"featureflags":  {},
	"audit_log":     {},
	"audit_logs":    {},
	"auditlog":      {},
	"auditlogs":     {},
	"incident":      {},
	"incidents":     {},
	"admin_user":    {},
	"admin_users":   {},
	"adminuser":     {},
	"adminusers":    {},
}

func isReservedAdminResource(name string) bool {
	n := strings.ToLower(name)
	if strings.HasPrefix(n, "bffx_") {
		return true
	}
	_, ok := reservedAdminResources[n]
	return ok
}

func (h *ResourceHandler) authorize(r *http.Request, resourceName string, requiredType string) (bool, string) {
	userRole, _ := r.Context().Value("admin_role").(string)

	var resourceSpec *manifest.AdminResourceSpec
	graph, err := manifest.BuildAdminGraph(h.reg)
	if err == nil && graph != nil {
		for i := range graph.Resources {
			if strings.EqualFold(graph.Resources[i].Resource, resourceName) {
				resourceSpec = &graph.Resources[i]
				break
			}
		}
	}

	requiredRole := ""
	if resourceSpec != nil {
		if requiredType == "write" {
			requiredRole = resourceSpec.Policy.Write
		} else {
			requiredRole = resourceSpec.Policy.Read
		}
	}

	roleWeight := func(role string) int {
		switch strings.ToLower(role) {
		case "superuser", "superadmin":
			return 30
		case "admin":
			return 20
		case "operator":
			return 10
		default:
			return 0
		}
	}

	if requiredRole == "" {
		return true, userRole
	}

	return roleWeight(userRole) >= roleWeight(requiredRole), userRole
}

func (h *ResourceHandler) ListResources(w http.ResponseWriter, r *http.Request) {
	groups := make(map[string][]map[string]any)
	for _, m := range h.reg.Resources {
		var spec manifest.ResourceSpec
		m.UnmarshalSpec(&spec)

		// Skip resources that have a dedicated admin surface. Operators
		// should manage them via the canonical UI (e.g. Feature Flags,
		// Audit Log, Incidents), not via the generic CRUD list.
		if isReservedAdminResource(m.Metadata.Name) {
			continue
		}

		group := spec.Group
		if group == "" {
			if m.Feature != "" {
				group = "feature:" + m.Feature
			} else {
				group = "app"
			}
		}

		groups[group] = append(groups[group], map[string]any{
			"name":        m.Metadata.Name,
			"displayName": spec.DisplayName,
			"path":        strings.ToLower(m.Metadata.Name) + "s",
			"fields":      spec.Fields,
			"feature":     m.Feature,
		})

	}

	// Add Feature Flags as a virtual resource group
	if len(h.reg.FeatureFlags) > 0 {
		groups["configuration"] = append(groups["configuration"], map[string]any{
			"name": "Feature Flags",
			"path": "flags", // Special path
			"type": "virtual",
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(groups)
}

func (h *ResourceHandler) GetRecords(w http.ResponseWriter, r *http.Request) {
	resource := r.PathValue("name")
	if allowed, _ := h.authorize(r, resource, "read"); !allowed {
		http.Error(w, "Forbidden: Insufficient permissions", http.StatusForbidden)
		return
	}
	store := h.store
	var resourceSpec *manifest.AdminResourceSpec

	graph, err := manifest.BuildAdminGraph(h.reg)
	if err == nil && graph != nil {
		for i := range graph.Resources {
			if strings.EqualFold(graph.Resources[i].Resource, resource) {
				resourceSpec = &graph.Resources[i]
				break
			}
		}
	}

	// Use telemetry store if specified in the schema
	if m, ok := h.reg.GetResource(resource); ok {
		var spec manifest.ResourceSpec
		m.UnmarshalSpec(&spec)
		if spec.Telemetry {
			store = h.telemetry
		}
	}

	// Start query builder
	qb := store.Query(r.Context(), resource)

	// 1. Pagination parameters
	pageStr := r.URL.Query().Get("page")
	perPageStr := r.URL.Query().Get("per_page")

	page := 1
	if pageVal, err := strconv.Atoi(pageStr); err == nil && pageVal > 0 {
		page = pageVal
	}

	perPage := 25
	if resourceSpec != nil && resourceSpec.Index.PerPage > 0 {
		perPage = resourceSpec.Index.PerPage
	} else if graph != nil && graph.Site.DefaultPerPage > 0 {
		perPage = graph.Site.DefaultPerPage
	}
	if perPageVal, err := strconv.Atoi(perPageStr); err == nil && perPageVal > 0 {
		perPage = perPageVal
	}

	// 2. Scopes
	scopeName := r.URL.Query().Get("scope")
	if scopeName == "" && resourceSpec != nil {
		// Find default scope
		for _, s := range resourceSpec.Index.Scopes {
			if s.Default {
				scopeName = s.Name
				break
			}
		}
	}

	if scopeName != "" && scopeName != "all" && resourceSpec != nil {
		for _, s := range resourceSpec.Index.Scopes {
			if strings.EqualFold(s.Name, scopeName) {
				for k, v := range s.Where {
					qb = qb.Where(k, "=", v)
				}
				break
			}
		}
	}

	// 3. Filters
	// Allowed filter fields from spec
	allowedFilters := make(map[string]manifest.AdminResourceFilter)
	if resourceSpec != nil {
		for _, f := range resourceSpec.Index.Filters {
			allowedFilters[strings.ToLower(f.Field)] = f
		}
	}

	for k, vals := range r.URL.Query() {
		if !strings.HasPrefix(k, "filter_") {
			continue
		}
		if len(vals) == 0 || vals[0] == "" {
			continue
		}
		val := vals[0]

		// Extract field and operator from param name
		cleanName := strings.TrimPrefix(k, "filter_")

		var fieldName string
		var op string = "="
		var queryVal any = val

		if strings.HasSuffix(cleanName, "_contains") {
			fieldName = strings.TrimSuffix(cleanName, "_contains")
			op = "LIKE"
			queryVal = "%" + val + "%"
		} else if strings.HasSuffix(cleanName, "_eq") {
			fieldName = strings.TrimSuffix(cleanName, "_eq")
			op = "="
		} else if strings.HasSuffix(cleanName, "_gte") {
			fieldName = strings.TrimSuffix(cleanName, "_gte")
			op = ">="
		} else if strings.HasSuffix(cleanName, "_lte") {
			fieldName = strings.TrimSuffix(cleanName, "_lte")
			op = "<="
		} else {
			fieldName = cleanName
		}

		filterSpec, ok := allowedFilters[strings.ToLower(fieldName)]
		if !ok {
			// Ignore filter params not declared in manifest for safety
			continue
		}

		// Type-cast value for bool filters
		if filterSpec.As == "bool" || filterSpec.As == "boolean" {
			if val == "true" || val == "1" {
				queryVal = true
			} else if val == "false" || val == "0" {
				queryVal = false
			}
		}

		qb = qb.Where(filterSpec.Field, op, queryVal)
	}

	// Get total matching count
	totalCount, err := qb.Count(r.Context())
	if err != nil {
		totalCount = 0
	}

	// 4. Sorting
	sortStr := r.URL.Query().Get("sort")
	if sortStr == "" && resourceSpec != nil {
		sortStr = resourceSpec.Index.DefaultSort
	}

	if sortStr != "" {
		desc := false
		fieldName := sortStr
		if strings.HasSuffix(strings.ToLower(sortStr), "_desc") {
			fieldName = sortStr[:len(sortStr)-5]
			desc = true
		} else if strings.HasSuffix(strings.ToLower(sortStr), "_asc") {
			fieldName = sortStr[:len(sortStr)-4]
		} else if parts := strings.Split(sortStr, " "); len(parts) == 2 {
			fieldName = parts[0]
			if strings.EqualFold(parts[1], "desc") {
				desc = true
			}
		}
		qb = qb.OrderBy(fieldName, desc)
	}

	// 5. Pagination Limit and Offset
	offset := (page - 1) * perPage
	qb = qb.Limit(perPage).Offset(offset)

	// Execute query
	records, err := qb.Execute(r.Context())
	if err != nil {
		records = []map[string]any{}
	}

	// Write response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Total-Count", strconv.Itoa(totalCount))
	w.Header().Set("X-Page", strconv.Itoa(page))
	w.Header().Set("X-Per-Page", strconv.Itoa(perPage))
	w.Header().Set("Access-Control-Expose-Headers", "X-Total-Count, X-Page, X-Per-Page")

	if records == nil {
		records = []map[string]any{}
	}
	sanitizedRecords := make([]map[string]any, 0, len(records))
	for _, rec := range records {
		sanitizedRecords = append(sanitizedRecords, sanitizeRecord(rec, resourceSpec))
	}
	json.NewEncoder(w).Encode(sanitizedRecords)
}

func (h *ResourceHandler) GetRecord(w http.ResponseWriter, r *http.Request) {
	resource := r.PathValue("name")
	if allowed, _ := h.authorize(r, resource, "read"); !allowed {
		http.Error(w, "Forbidden: Insufficient permissions", http.StatusForbidden)
		return
	}
	id := r.PathValue("id")
	store := h.store
	var resourceSpec *manifest.AdminResourceSpec

	graph, err := manifest.BuildAdminGraph(h.reg)
	if err == nil && graph != nil {
		for i := range graph.Resources {
			if strings.EqualFold(graph.Resources[i].Resource, resource) {
				resourceSpec = &graph.Resources[i]
				break
			}
		}
	}

	// Use telemetry store if specified in the schema
	if m, ok := h.reg.GetResource(resource); ok {
		var spec manifest.ResourceSpec
		m.UnmarshalSpec(&spec)
		if spec.Telemetry {
			store = h.telemetry
		}
	}

	record, err := store.Get(r.Context(), resource, id)
	if err != nil || record == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sanitizeRecord(record, resourceSpec))
}

func (h *ResourceHandler) CreateRecord(w http.ResponseWriter, r *http.Request) {
	resource := r.PathValue("name")
	if allowed, _ := h.authorize(r, resource, "write"); !allowed {
		http.Error(w, "Forbidden: Insufficient permissions", http.StatusForbidden)
		return
	}
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.ToLower(resource) == "adminuser" || strings.ToLower(resource) == "user" {
		if raw, ok := payload["password"].(string); ok && raw != "" && !strings.HasPrefix(raw, "$2") {
			if hash, err := auth.HashPassword(raw); err == nil {
				payload["password"] = hash
			}
		}
	}
	result, err := h.store.Create(r.Context(), resource, payload)
	if err != nil || result == nil {
		http.Error(w, "failed to create", http.StatusInternalServerError)
		return
	}

	// Audit Logging
	if h.auditor != nil {
		actorID, _ := r.Context().Value("admin_id").(string)
		payloadStr, _ := json.Marshal(payload)
		resID, _ := result["id"].(string)
		h.auditor.Record(r.Context(), audit.AuditLog{
			ActorID:      actorID,
			ActorType:    "admin",
			Action:       "create",
			ResourceKind: resource,
			ResourceID:   resID,
			Payload:      string(payloadStr),
			IPAddress:    r.RemoteAddr,
		})
	}

	var resourceSpec *manifest.AdminResourceSpec
	graph, errGraph := manifest.BuildAdminGraph(h.reg)
	if errGraph == nil && graph != nil {
		for i := range graph.Resources {
			if strings.EqualFold(graph.Resources[i].Resource, resource) {
				resourceSpec = &graph.Resources[i]
				break
			}
		}
	}

	json.NewEncoder(w).Encode(sanitizeRecord(result, resourceSpec))
}

func (h *ResourceHandler) UpdateRecord(w http.ResponseWriter, r *http.Request) {
	resource := r.PathValue("name")
	if allowed, _ := h.authorize(r, resource, "write"); !allowed {
		http.Error(w, "Forbidden: Insufficient permissions", http.StatusForbidden)
		return
	}
	id := r.PathValue("id")
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.ToLower(resource) == "adminuser" || strings.ToLower(resource) == "user" {
		if val, exists := payload["password"]; exists {
			if raw, ok := val.(string); ok && raw != "" {
				if !strings.HasPrefix(raw, "$2") {
					if hash, err := auth.HashPassword(raw); err == nil {
						payload["password"] = hash
					}
				}
			} else {
				// delete empty/nil/non-string password to prevent overwriting with empty string
				delete(payload, "password")
			}
		}
	}
	result, err := h.store.Update(r.Context(), resource, id, payload)
	if err != nil || result == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Audit Logging
	if h.auditor != nil {
		actorID, _ := r.Context().Value("admin_id").(string)
		payloadStr, _ := json.Marshal(payload)
		h.auditor.Record(r.Context(), audit.AuditLog{
			ActorID:      actorID,
			ActorType:    "admin",
			Action:       "update",
			ResourceKind: resource,
			ResourceID:   id,
			Payload:      string(payloadStr),
			IPAddress:    r.RemoteAddr,
		})
	}

	var resourceSpec *manifest.AdminResourceSpec
	graph, errGraph := manifest.BuildAdminGraph(h.reg)
	if errGraph == nil && graph != nil {
		for i := range graph.Resources {
			if strings.EqualFold(graph.Resources[i].Resource, resource) {
				resourceSpec = &graph.Resources[i]
				break
			}
		}
	}

	json.NewEncoder(w).Encode(sanitizeRecord(result, resourceSpec))
}

func (h *ResourceHandler) DeleteRecord(w http.ResponseWriter, r *http.Request) {
	resource := r.PathValue("name")
	if allowed, _ := h.authorize(r, resource, "write"); !allowed {
		http.Error(w, "Forbidden: Insufficient permissions", http.StatusForbidden)
		return
	}
	id := r.PathValue("id")
	if err := h.store.Delete(r.Context(), resource, id); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Audit Logging
	if h.auditor != nil {
		actorID, _ := r.Context().Value("admin_id").(string)
		h.auditor.Record(r.Context(), audit.AuditLog{
			ActorID:      actorID,
			ActorType:    "admin",
			Action:       "delete",
			ResourceKind: resource,
			ResourceID:   id,
			IPAddress:    r.RemoteAddr,
		})
	}

	w.WriteHeader(http.StatusNoContent)
}
