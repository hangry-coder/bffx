# 🗺 Database Migration Guide

This guide explains how to transition your existing BFFX project to the new versioned migration system.

## 1. Update your Bffx CLI
First, ensure you are using the latest version of the framework.
```bash
# In the Bffx framework repository
make build
# Ensure the binary in bin/ is accessible or move it to your path
```

## 2. Update your project's framework core
In your existing project (e.g., `my-backend`), refresh the vendored core to get the new migration logic:
```bash
cd my-backend
bffx update framework --vendor-only
```

**Admin UI (embedded SPA):** vendoring copies `pkg/admin/ui-v2/dist/` only (not `node_modules` or `src/`). If `dist/` is missing in the Bffx checkout, run `scripts/build-admin-ui.sh` before `update framework`, or let the vendor step build it when `npm` is available.

## 3. Initialize Migrations
Since your project already has a database, you need to tell BFFX about your current state so it doesn't try to "re-create" existing tables.

### Recommended: `bffx migrate init`
From your project root:
```bash
bffx migrate init
```
This:
1. Generates `migrations/<timestamp>_initial_schema.up.sql` (and a matching `.down.sql`) from your current manifests.
2. Writes `migrations/schema.json` as the baseline snapshot.
3. Creates the `schema_migrations` tracking table and stamps the baseline as already-applied, so existing tables are not re-created.

After `init`, the orchestrator will use the migrator on every boot. Until you run `init`, the legacy declarative reconcile path stays in effect, so existing projects keep working unchanged.

### Manual fallback (if `init` cannot reach your DB)
1. Run `bffx migrate plan "initial_schema"`.
2. **DO NOT run `bffx migrate apply` yet** if your DB already contains those tables.
3. Mark the migration as applied:
   ```bash
   # SQLite
   sqlite3 .bffx/app.db "CREATE TABLE IF NOT EXISTS schema_migrations (version BIGINT PRIMARY KEY, dirty INTEGER NOT NULL);"
   sqlite3 .bffx/app.db "INSERT OR REPLACE INTO schema_migrations(version, dirty) VALUES (YYYYMMDDHHMMSS, 0);"

   # Postgres
   psql "$DATABASE_URL" -c "CREATE TABLE IF NOT EXISTS schema_migrations (version BIGINT PRIMARY KEY, dirty BOOLEAN NOT NULL); INSERT INTO schema_migrations(version, dirty) VALUES (YYYYMMDDHHMMSS, false) ON CONFLICT (version) DO NOTHING;"
   ```
   *(Replace YYYYMMDDHHMMSS with the timestamp prefix of your generated `*.up.sql`.)*

## 4. Normal Workflow
From now on, whenever you change a manifest in `bffx/`:
1. Run `bffx sync` (it will warn you about drift).
2. Run `bffx migrate plan "my_change_description"`.
3. Review the generated `.up.sql` in `migrations/`.
4. Run `bffx migrate apply`.

## 5. Production
In production, your app will now refuse to start if there are pending migrations. You must explicitly run:
```bash
bffx migrate apply
```
before starting your server.

## 6. System-Table Prefixing & Admin V2 Upgrades (Release v0.1.2)

### 6.1 Prefixed System Tables Migration
BFFX Release v0.1.2 introduces table prefixing (`bffx_*`) for core identity and admin tables to separate system records from your application domain.

During the compatibility window, the framework implements a dual-read fallback:
- **Write Path:** Writes to the new `bffx_*` tables.
- **Read Path:** Tries `bffx_*` first, falling back to legacy tables (`adminuser`, `user`, etc.) if missing.

#### How to Migrate Downstream Projects:
1. Rebuild your `bffx` CLI and run `bffx sync` in your project folder to generate the updated registry.
2. Run `bffx migrate plan "prefix_system_tables"` to generate the SQL scripts for copying data.
3. Review the generated `.up.sql` to verify safety.
4. Run `bffx migrate apply` to apply the migrations.
5. In production, keep compatibility mode enabled by default. If you want to enforce strict table checks immediately, set:
   ```env
   BFFX_SYSTEM_TABLES_STRICT=true
   ```

### 6.2 Modernized React V2 Admin Panel
The Admin Web Console has been modernized with a high-fidelity React interface (V2) served directly from the Go binary.

* **Default (V2 UI):** The modernized React V2 Admin Panel is now served by default.
* **Opting-Out (Legacy V1 UI):** If you need to rollback to the legacy HTML/JS admin dashboard, set the environment variable `BFFX_ADMIN_UI_V2=false` (or in `.env` file).

---

## 7. Packaging migration (full ↔ minimal)

This is **separate from database migrations** (sections 1–5). It switches how much of the BFFX framework is vendored into `.bffx/core` and updates the canonical build profile.

### When to use

- You want a **smaller vendored core** (minimal mode excludes CLI tooling packages).
- You scaffolded with **`bffx new --minimal`** and need rollback instructions.
- You are cutting over an existing backend after shadow profile validation.

### Workflow

```bash
bffx migrate packaging plan
bffx migrate packaging apply --to minimal
bffx sync
bffx update framework --vendor-only
bffx doctor

# Rollback if needed:
bffx migrate packaging rollback
```

### Artifacts

| File | Purpose |
|------|---------|
| `.bffx/build-profile.json` | Capabilities + package list (emitted by every `bffx sync`) |
| `.bffx/packaging-migration.json` | Rollback pointer from last apply |
| `.bffx/backups/packaging-migration/<timestamp>/` | Backup of `project.yaml` and profile |

See [Build Profile Contract](../core-concepts/build_profile_contract.md) and [CLI reference](../reference/cli.md).
