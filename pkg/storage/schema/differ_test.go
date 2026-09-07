package schema

import (
	"testing"
)

func TestDiff(t *testing.T) {
	current := &SchemaSnapshot{
		Tables: map[string]TableDef{
			"user": {
				Name: "user",
				Columns: map[string]ColumnDef{
					"id":    {Name: "id", Type: "text"},
					"email": {Name: "email", Type: "text", Unique: true},
				},
			},
			"old_table": {
				Name: "old_table",
				Columns: map[string]ColumnDef{
					"id": {Name: "id", Type: "text"},
				},
			},
		},
	}

	desired := []TableDef{
		{
			Name: "user",
			Columns: map[string]ColumnDef{
				"id":    {Name: "id", Type: "text"},
				"email": {Name: "email", Type: "text", Unique: false}, // Unique removed
				"age":   {Name: "age", Type: "bigint"},               // New column
			},
		},
		{
			Name: "post",
			Columns: map[string]ColumnDef{
				"id":    {Name: "id", Type: "text"},
				"title": {Name: "title", Type: "text"},
			},
		},
	}

	ops := Diff(desired, current)

	expectedOps := map[string]bool{
		"create_table:post":  false,
		"drop_table:old_table": false,
		"add_column:user.age": false,
		"drop_index:user.email": false,
	}

	for _, op := range ops {
		key := ""
		if op.Column != "" {
			key = op.Kind + ":" + op.Table + "." + op.Column
		} else {
			key = op.Kind + ":" + op.Table
		}
		
		if _, ok := expectedOps[key]; ok {
			expectedOps[key] = true
		} else {
			t.Errorf("unexpected op: %s", key)
		}

		if op.Kind == "drop_table" || op.Kind == "drop_column" {
			if !op.Destructive {
				t.Errorf("op %s should be marked as destructive", key)
			}
		}
	}

	for key, found := range expectedOps {
		if !found {
			t.Errorf("missing expected op: %s", key)
		}
	}
}
