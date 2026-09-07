package migrator

import (
	"github.com/hangry-coder/bffx/pkg/storage/schema"
)

// Dialect interface defines methods for generating SQL for different databases
type Dialect interface {
	GenerateCreateTable(table schema.TableDef) string
	GenerateDropTable(name string) string
	GenerateAddColumn(table, col string, def schema.ColumnDef) string
	GenerateDropColumn(table, col string) string
	GenerateAlterColumn(table, col string, oldType, newType string) string
	GenerateCreateIndex(table, col string, unique bool) string
	GenerateDropIndex(table, col string) string
}

func mapType(t string, isPostgres bool) string {
	switch t {
	case "text":
		return "TEXT"
	case "bigint":
		if isPostgres {
			return "BIGINT"
		}
		return "INTEGER"
	case "real":
		if isPostgres {
			return "DOUBLE PRECISION"
		}
		return "REAL"
	case "boolean":
		if isPostgres {
			return "BOOLEAN"
		}
		return "INTEGER"
	case "timestamp":
		if isPostgres {
			return "TIMESTAMPTZ"
		}
		return "TEXT"
	case "jsonb":
		if isPostgres {
			return "JSONB"
		}
		return "TEXT"
	default:
		return "TEXT"
	}
}
