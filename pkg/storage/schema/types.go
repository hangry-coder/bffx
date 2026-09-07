package schema

// ColumnDef defines a column in a table
type ColumnDef struct {
	Name       string `json:"name"`
	Type       string `json:"type"` // Canonical BFFX type: text, bigint, real, boolean, jsonb, timestamp
	Unique     bool   `json:"unique,omitempty"`
	ForeignKey string `json:"foreignKey,omitempty"` // target table, empty if none
}

// TableDef defines a table in the schema
type TableDef struct {
	Name    string               `json:"name"`
	Columns map[string]ColumnDef `json:"columns"`
	Indexes []string             `json:"indexes,omitempty"`
	IsTree  bool                 `json:"isTree,omitempty"`
}

// SchemaSnapshot represents the state of the schema at a given version
type SchemaSnapshot struct {
	Version string               `json:"version"`
	Tables  map[string]TableDef  `json:"tables"`
}

// DiffOp represents a single operation to be performed on the schema
type DiffOp struct {
	Kind        string         `json:"kind"` // "create_table", "add_column", "drop_column", "alter_column", "create_index", "drop_index", "create_tree", "drop_tree"
	Table       string         `json:"table"`
	Column      string         `json:"column,omitempty"`
	OldType     string         `json:"oldType,omitempty"`
	NewType     string         `json:"newType,omitempty"`
	Destructive bool           `json:"destructive"`
	Details     map[string]any `json:"details,omitempty"`
}
