package migrator

import (
	"github.com/hangry-coder/bffx/pkg/storage/schema"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// GenerateMigration generates timestamped .up.sql and .down.sql files and updates the snapshot
func GenerateMigration(ops []schema.DiffOp, d Dialect, dir string, name string, snap *schema.SchemaSnapshot) error {
	if len(ops) == 0 {
		return nil
	}

	timestamp := time.Now().Format("20060102150405")
	baseName := fmt.Sprintf("%s_%s", timestamp, name)

	var upSql, downSql string

	for _, op := range ops {
		up, down := generateSQL(op, d)
		upSql += up + "\n"
		downSql = down + "\n" + downSql // Reverse order for down migrations
	}

	// Write UP migration
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	upPath := filepath.Join(dir, baseName+".up.sql")
	if err := os.WriteFile(upPath, []byte(upSql), 0644); err != nil {
		return err
	}

	// Write DOWN migration
	downPath := filepath.Join(dir, baseName+".down.sql")
	if err := os.WriteFile(downPath, []byte(downSql), 0644); err != nil {
		return err
	}

	// Update snapshot
	applyOpsToSnapshot(ops, snap)
	snap.Version = timestamp
	
	snapshotPath := filepath.Join(dir, "schema.json")
	return schema.SaveSnapshot(snapshotPath, snap)
}

func generateSQL(op schema.DiffOp, d Dialect) (string, string) {
	var up, down string

	switch op.Kind {
	case "create_table":
		table := op.Details["table"].(schema.TableDef)
		up = d.GenerateCreateTable(table)
		down = d.GenerateDropTable(op.Table)
	case "drop_table":
		// This is tricky because we need the old table def for the DOWN migration.
		// For now, we'll just put a comment.
		up = d.GenerateDropTable(op.Table)
		down = fmt.Sprintf("-- TODO: Restore table %s", op.Table)
	case "add_column":
		colDef := schema.ColumnDef{Name: op.Column, Type: op.NewType}
		up = d.GenerateAddColumn(op.Table, op.Column, colDef)
		down = d.GenerateDropColumn(op.Table, op.Column)
	case "drop_column":
		up = d.GenerateDropColumn(op.Table, op.Column)
		down = fmt.Sprintf("-- TODO: Restore column %s.%s", op.Table, op.Column)
	case "alter_column":
		up = d.GenerateAlterColumn(op.Table, op.Column, op.OldType, op.NewType)
		down = d.GenerateAlterColumn(op.Table, op.Column, op.NewType, op.OldType)
	case "create_index":
		unique := false
		if u, ok := op.Details["unique"].(bool); ok {
			unique = u
		}
		up = d.GenerateCreateIndex(op.Table, op.Column, unique)
		down = d.GenerateDropIndex(op.Table, op.Column)
	case "drop_index":
		up = d.GenerateDropIndex(op.Table, op.Column)
		down = d.GenerateCreateIndex(op.Table, op.Column, false) // Assumption
	case "tree_transition_unsupported":
		up = fmt.Sprintf("-- ERROR: changing the `tree` flag on resource %q is not yet supported by the migrator.\n"+
			"-- Write the schema change manually (parent_id column + closure table) and edit the version row in schema_migrations to skip this op.\n"+
			"SELECT 1 WHERE 1 = 0;", op.Table)
		down = up
	default:
		up = fmt.Sprintf("-- WARNING: unhandled migration op %q for table %s; no-op generated.", op.Kind, op.Table)
		down = up
	}

	if op.Destructive {
		down = "-- WARNING: This migration is destructive.\n" + down
	}

	return up, down
}

func applyOpsToSnapshot(ops []schema.DiffOp, snap *schema.SchemaSnapshot) {
	if snap.Tables == nil {
		snap.Tables = make(map[string]schema.TableDef)
	}

	for _, op := range ops {
		switch op.Kind {
		case "create_table":
			table := op.Details["table"].(schema.TableDef)
			snap.Tables[op.Table] = table
		case "drop_table":
			delete(snap.Tables, op.Table)
		case "add_column":
			table := snap.Tables[op.Table]
			if table.Columns == nil {
				table.Columns = make(map[string]schema.ColumnDef)
			}
			table.Columns[op.Column] = schema.ColumnDef{Name: op.Column, Type: op.NewType}
			snap.Tables[op.Table] = table
		case "drop_column":
			table := snap.Tables[op.Table]
			delete(table.Columns, op.Column)
			snap.Tables[op.Table] = table
		case "alter_column":
			table := snap.Tables[op.Table]
			col := table.Columns[op.Column]
			col.Type = op.NewType
			table.Columns[op.Column] = col
			snap.Tables[op.Table] = table
		case "create_index":
			table := snap.Tables[op.Table]
			col := table.Columns[op.Column]
			col.Unique = true
			table.Columns[op.Column] = col
			snap.Tables[op.Table] = table
		case "drop_index":
			table := snap.Tables[op.Table]
			col := table.Columns[op.Column]
			col.Unique = false
			table.Columns[op.Column] = col
			snap.Tables[op.Table] = table
		case "tree_transition_unsupported":
			// Intentionally not mutating the snapshot: the operator is expected
			// to land the schema change manually and re-stamp the snapshot.
		}
	}
}
