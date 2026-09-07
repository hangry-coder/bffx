package schema

import (
	"reflect"
)

// Diff compares desired state against current state and returns a slice of DiffOp
func Diff(desired []TableDef, current *SchemaSnapshot) []DiffOp {
	var ops []DiffOp

	desiredMap := make(map[string]TableDef)
	for _, t := range desired {
		desiredMap[t.Name] = t
	}

	currentMap := make(map[string]TableDef)
	if current != nil {
		for _, t := range current.Tables {
			currentMap[t.Name] = t
		}
	}

	// 1. Find added tables and compare existing ones
	for name, dT := range desiredMap {
		cT, exists := currentMap[name]
		if !exists {
			ops = append(ops, DiffOp{
				Kind:    "create_table",
				Table:   name,
				Details: map[string]any{"table": dT},
			})
			continue // Skip individual column ops for a brand new table
		}

		// Compare columns
		for colName, dC := range dT.Columns {
			cC, colExists := cT.Columns[colName]
			if !colExists {
				ops = append(ops, DiffOp{
					Kind:    "add_column",
					Table:   name,
					Column:  colName,
					NewType: dC.Type,
				})
			} else if dC.Type != cC.Type {
				ops = append(ops, DiffOp{
					Kind:        "alter_column",
					Table:       name,
					Column:      colName,
					OldType:     cC.Type,
					NewType:     dC.Type,
					Destructive: true, // Type changes are generally destructive
				})
			}
		}

		// Find removed columns in existing tables
		if exists {
			for colName := range cT.Columns {
				if _, desiredExists := dT.Columns[colName]; !desiredExists {
					ops = append(ops, DiffOp{
						Kind:        "drop_column",
						Table:       name,
						Column:      colName,
						Destructive: true,
					})
				}
			}
		}

		// Tree transitions are not yet supported by the migrator's writer;
		// emitting them silently produced empty SQL. Surface them as a clear
		// error path until the writer/dialects implement them. Today this
		// means: change `tree: true/false` on an existing resource requires a
		// manual migration. Brand-new tree resources are still handled via
		// the create_table op and the resource's parent_id column.
		if dT.IsTree != cT.IsTree {
			ops = append(ops, DiffOp{
				Kind:        "tree_transition_unsupported",
				Table:       name,
				Destructive: true,
				Details:     map[string]any{"desired_tree": dT.IsTree, "current_tree": cT.IsTree},
			})
		}

		
		// Handle Indexes (Simplified for now: Unique fields)
		// In a real system we'd compare the Indexes slice.
		// For now, let's just use the Unique flag in ColumnDef.
		for colName, dC := range dT.Columns {
			cC, colExists := cT.Columns[colName]
			if dC.Unique && (!colExists || !cC.Unique) {
				ops = append(ops, DiffOp{
					Kind:   "create_index",
					Table:  name,
					Column: colName,
					Details: map[string]any{"unique": true},
				})
			} else if !dC.Unique && colExists && cC.Unique {
				ops = append(ops, DiffOp{
					Kind:   "drop_index",
					Table:  name,
					Column: colName,
				})
			}
		}
	}

	// 2. Find removed tables
	for name := range currentMap {
		if _, exists := desiredMap[name]; !exists {
			ops = append(ops, DiffOp{
				Kind:        "drop_table",
				Table:       name,
				Destructive: true,
			})
		}
	}

	return ops
}

// DeepEqualSchema is a helper for testing
func DeepEqualSchema(a, b []TableDef) bool {
	if len(a) != len(b) {
		return false
	}
	// Simplified check for now
	return reflect.DeepEqual(a, b)
}
