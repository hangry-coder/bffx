package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

func (h *ResourceHandler) findAdminResourceSpec(name string) *manifest.AdminResourceSpec {
	graph, err := manifest.BuildAdminGraph(h.reg)
	if err != nil || graph == nil {
		return nil
	}
	for i := range graph.Resources {
		if strings.EqualFold(graph.Resources[i].Resource, name) {
			return &graph.Resources[i]
		}
	}
	return nil
}

func (h *ResourceHandler) findAssociation(spec *manifest.AdminResourceSpec, name string) *manifest.AdminAssociation {
	if spec == nil {
		return nil
	}
	for i := range spec.Associations {
		if strings.EqualFold(spec.Associations[i].Name, name) {
			return &spec.Associations[i]
		}
	}
	return nil
}

func (h *ResourceHandler) GetAssociations(w http.ResponseWriter, r *http.Request) {
	parent := r.PathValue("name")
	id := r.PathValue("id")
	assocName := r.PathValue("association")

	if allowed, _ := h.authorize(r, parent, "read"); !allowed {
		http.Error(w, "Forbidden: Insufficient permissions", http.StatusForbidden)
		return
	}

	parentSpec := h.findAdminResourceSpec(parent)
	assoc := h.findAssociation(parentSpec, assocName)
	if assoc == nil {
		http.Error(w, "association not found", http.StatusNotFound)
		return
	}

	if _, ok := h.reg.GetResource(assoc.Resource); !ok {
		http.Error(w, "unknown child resource", http.StatusBadRequest)
		return
	}

	store := h.store
	if m, ok := h.reg.GetResource(assoc.Resource); ok {
		var rs manifest.ResourceSpec
		m.UnmarshalSpec(&rs)
		if rs.Telemetry {
			store = h.telemetry
		}
	}

	perPage := 25
	if assoc.PerPage > 0 {
		perPage = assoc.PerPage
	} else if parentSpec != nil && parentSpec.Index.PerPage > 0 {
		perPage = parentSpec.Index.PerPage
	}
	page := 1
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}

	qb := store.Query(r.Context(), assoc.Resource).Where(assoc.ForeignKey, "=", id)
	if parts := strings.Fields(assoc.Sort); len(parts) >= 2 {
		desc := strings.EqualFold(parts[1], "desc")
		qb = qb.OrderBy(parts[0], desc)
	}
	offset := (page - 1) * perPage
	rows, err := qb.Limit(perPage).Offset(offset).Execute(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"rows":     rows,
		"page":     page,
		"per_page": perPage,
		"total":    len(rows),
	})
}
