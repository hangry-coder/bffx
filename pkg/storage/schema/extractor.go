package schema

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"strings"
)

// FromRegistry converts a manifest Registry into a slice of TableDef
func FromRegistry(reg *manifest.Registry) []TableDef {
	var tables []TableDef

	for _, r := range reg.Resources {
		var spec manifest.ResourceSpec
		if err := r.UnmarshalSpec(&spec); err != nil {
			continue
		}

		table := TableDef{
			Name:    ResolveTable(r.Metadata.Name),
			Columns: make(map[string]ColumnDef),
			IsTree:  manifest.TreeEnabled(spec.Tree),
		}

		// Add standard fields
		table.Columns["id"] = ColumnDef{Name: "id", Type: "text"}
		table.Columns["created_at"] = ColumnDef{Name: "created_at", Type: "text"} // Standard timestamps are text in bffx for now (RFC3339)
		table.Columns["updated_at"] = ColumnDef{Name: "updated_at", Type: "text"}
		table.Columns["created_by"] = ColumnDef{Name: "created_by", Type: "text"}

		// Add fields from spec
		for _, f := range spec.Fields {
			colName := strings.ToLower(f.Name)
			if _, exists := table.Columns[colName]; exists {
				continue
			}

			table.Columns[colName] = ColumnDef{
				Name:       colName,
				Type:       mapType(f.Type),
				Unique:     f.Unique,
				ForeignKey: f.Target,
			}
		}

		// Handle Tree Logic (parent_id)
		if manifest.TreeEnabled(spec.Tree) {
			table.Columns["parent_id"] = ColumnDef{
				Name:       "parent_id",
				Type:       "text", // References ID which is text
				ForeignKey: table.Name,
			}
		}

		tables = append(tables, table)
	}

	return tables
}

func mapType(t string) string {
	switch t {
	case "int":
		return "bigint"
	case "float":
		return "real"
	case "bool":
		return "boolean"
	case "date":
		return "timestamp"
	case "json":
		return "jsonb"
	default:
		return "text"
	}
}
