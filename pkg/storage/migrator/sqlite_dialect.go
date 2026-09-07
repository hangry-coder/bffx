package migrator

import (
	"github.com/hangry-coder/bffx/pkg/storage/schema"
	"fmt"
	"strings"
)

type SQLiteDialect struct{}

func (d *SQLiteDialect) GenerateCreateTable(t schema.TableDef) string {
	var cols []string
	for _, c := range t.Columns {
		col := fmt.Sprintf("%s %s", c.Name, mapType(c.Type, false))
		if c.Unique {
			col += " UNIQUE"
		}
		cols = append(cols, col)
	}

	sql := fmt.Sprintf("CREATE TABLE %s (\n  %s\n);", t.Name, strings.Join(cols, ",\n  "))
	return sql
}

func (d *SQLiteDialect) GenerateDropTable(name string) string {
	return fmt.Sprintf("DROP TABLE %s;", name)
}

func (d *SQLiteDialect) GenerateAddColumn(table, col string, def schema.ColumnDef) string {
	sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, col, mapType(def.Type, false))
	if def.Unique {
		sql += " UNIQUE"
	}
	return sql + ";"
}

func (d *SQLiteDialect) GenerateDropColumn(table, col string) string {
	return fmt.Sprintf("-- SQLite does not support DROP COLUMN in older versions.\n-- ALTER TABLE %s DROP COLUMN %s;", table, col)
}

func (d *SQLiteDialect) GenerateAlterColumn(table, col string, oldType, newType string) string {
	return fmt.Sprintf("-- SQLite does not support ALTER COLUMN TYPE.\n-- ALTER TABLE %s ALTER COLUMN %s TYPE %s;", table, col, mapType(newType, false))
}

func (d *SQLiteDialect) GenerateCreateIndex(table, col string, unique bool) string {
	prefix := "INDEX"
	if unique {
		prefix = "UNIQUE INDEX"
	}
	return fmt.Sprintf("CREATE %s idx_%s_%s ON %s(%s);", prefix, table, col, table, col)
}

func (d *SQLiteDialect) GenerateDropIndex(table, col string) string {
	return fmt.Sprintf("DROP INDEX idx_%s_%s;", table, col)
}
