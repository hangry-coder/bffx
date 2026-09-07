package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

func (h *ResourceHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
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

	if m, ok := h.reg.GetResource(resource); ok {
		var spec manifest.ResourceSpec
		m.UnmarshalSpec(&spec)
		if spec.Telemetry {
			store = h.telemetry
		}
	}

	qb := store.Query(r.Context(), resource)

	scopeName := r.URL.Query().Get("scope")
	if scopeName == "" && resourceSpec != nil {
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
			continue
		}

		if filterSpec.As == "bool" || filterSpec.As == "boolean" {
			if val == "true" || val == "1" {
				queryVal = true
			} else if val == "false" || val == "0" {
				queryVal = false
			}
		}

		qb = qb.Where(filterSpec.Field, op, queryVal)
	}

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

	records, err := qb.Execute(r.Context())
	if err != nil {
		records = []map[string]any{}
	}

	var fields []string
	if resourceSpec != nil && len(resourceSpec.Index.Columns) > 0 {
		fields = resourceSpec.Index.Columns
	} else if m, ok := h.reg.GetResource(resource); ok {
		var spec manifest.ResourceSpec
		m.UnmarshalSpec(&spec)
		for _, f := range spec.Fields {
			fields = append(fields, f.Name)
		}
	} else {
		if len(records) > 0 {
			for k := range records[0] {
				fields = append(fields, k)
			}
		}
	}

	var finalFields []string
	for _, f := range fields {
		if isSensitiveField(f) {
			continue
		}
		if resourceSpec != nil {
			excluded := false
			for _, excl := range resourceSpec.Show.Exclude {
				if strings.EqualFold(excl, f) {
					excluded = true
					break
				}
			}
			for _, excl := range resourceSpec.Form.Exclude {
				if strings.EqualFold(excl, f) {
					excluded = true
					break
				}
			}
			if excluded {
				continue
			}
		}
		finalFields = append(finalFields, f)
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-export.csv", resource))

	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write(finalFields); err != nil {
		return
	}

	for _, rec := range records {
		row := make([]string, len(finalFields))
		for i, field := range finalFields {
			val := rec[field]
			if val == nil {
				row[i] = ""
			} else {
				switch v := val.(type) {
				case string:
					row[i] = v
				case []byte:
					row[i] = string(v)
				case bool:
					row[i] = strconv.FormatBool(v)
				case int:
					row[i] = strconv.Itoa(v)
				case int64:
					row[i] = strconv.FormatInt(v, 10)
				case float64:
					row[i] = strconv.FormatFloat(v, 'f', -1, 64)
				default:
					if bytes, err := json.Marshal(v); err == nil {
						row[i] = string(bytes)
					} else {
						row[i] = fmt.Sprint(v)
					}
				}
			}
		}
		if err := writer.Write(row); err != nil {
			return
		}
	}
}
