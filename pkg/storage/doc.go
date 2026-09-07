// Package storage provides the primary persistence abstraction for BFFX.
//
// Interface Contract:
//   - All methods MUST accept context.Context as the first argument.
//   - Methods return (result, error) rather than (result, bool).
//   - Return errors.ErrNotFound (see bffx/pkg/errors) for missing records.
//
// Drivers:
//   - SQLite: Primary driver for local dev and edge (pkg/storage/sqlite.go).
//   - Postgres: Enterprise driver (pkg/storage/postgres.go).
//   - MongoDB: Optional driver (pkg/storage/mongo.go); requires go build -tags=experimental_mongo.
//   - PocketBase: Optional driver (pkg/storage/pocketbase.go); requires -tags=experimental_pocketbase.
//   - Memory: For testing and ephemeral state (pkg/storage/memory.go).
//
// RouterStore:
//   Implements dual-routing where primary data goes to a SQL store and
//   telemetry/logs go to a secondary store (e.g. Mongo/S3).
package storage
