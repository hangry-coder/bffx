# 🚀 BFFX CLI Reference

The `bffx` CLI is your primary interface for managing the development lifecycle and infrastructure of your backend.

---
*   [🚀 CLI Reference](cli.md)
*   [🏗 Architecture Guide](../core-concepts/architecture.md)
---

## 🛠 Project Lifecycle

### `bffx new NAME [flags]`
Scaffolds a new project at **`./NAME/`** under the parent directory (default parent: current working directory). Uses the **v2** vertical-slice layout (`internal/features/`, `cmd/api/`, `db/migrations/`) unless `--layout legacy` is set. Standardized on **Go 1.26**. Primary store defaults to **sqlite** (zero Docker deps).

| Flag | Description |
|------|-------------|
| **`--minimal`** | Sets `spec.packaging.mode: minimal`; omits sample onboarding screens. |
| **`--no-admin`** | Disables admin panel (`admin.enabled: false`; skips `bffx/admin/`). |
| **`--layout legacy`** | Legacy tree (`bffx/resources/`, `cmd/orchestrator/`) instead of v2. |
| **`--store MODE`** | Primary store: `sqlite` (default), `postgres`, `mongo`, `memory`, or `pocketbase`. Writes matching `spec.store` and `.env` entries. |
| **`--parent-dir PATH`** | Parent directory for the project folder (default `.` → `./NAME/`). |
| **`--root PATH`** | Alias for **`--parent-dir`** on `bffx new` only (not the same as `--root` on `bffx dev` / `bffx sync`, which point at the project directory). |
| **`--non-interactive`** | Skip the wizard; use flags / config only. |
| **`--config FILE`** | Load options from YAML (see archetype templates). |
| **`--auth-strategy`** | `mandatory`, `optional`, or `anonymous`. |
| **`--with-monetization`**, **`--with-flags`**, **`--with-telemetry`**, **`--with-ads`** | Enable addon scaffolds. |
| **`--admin-email`**, **`--admin-password`** | Initial admin user (admin enabled by default). |

**Examples:**

```bash
bffx new myapp
bffx new backend --store postgres --with-flags --non-interactive
bffx new myapp --parent-dir /tmp/projects   # → /tmp/projects/myapp/
```

### `bffx dev [--root PATH] [--watch]`
Starts the development environment:
1. Performs a `sync` in the project directory (`--root` is resolved to an **absolute** path).
2. Compiles your project's orchestrator with **`GOWORK=off`** so a **parent** `go.work` (e.g. the Bffx monorepo) cannot force the wrong main module when building `./cmd/orchestrator` (legacy) or `./cmd/api` (v2 layout).
3. Starts the server on port `8080`.
4. In development, open **API Reference** from the admin panel at `/admin` (authenticated; not served at public `/api/docs`).

**`--watch`**: After the first start, watches `bffx/**/*.yaml`, `hooks/**/*.go`, `internal/features/**/*.go`, and `cmd/**` for changes; debounces (~500ms), re-runs `sync`, rebuilds, and restarts the child process. Default is off for backwards compatibility.

Use **`bffx dev --help`** (or `-h`) to print usage **without** running sync or starting the server. The same pattern applies to **`bffx sync`**, **`bffx migrate`**, and **`bffx db`** subcommands.

### `bffx sync [--root PATH] [--dry-run]`
Synchronizes and compiles manifest configurations:
1. Validates all YAML blueprints and resource schemas in `bffx/` or `internal/features/`.
2. Writes **`.bffx/build-profile.json`** (capabilities, packaging mode, vendoring package list).
3. Writes `.bffx/graph.json`, `.bffx/graph.hash`, `.bffx/diagnostics.json`, `.bffx/openapi.json`.
4. Generates Protobuf and Go/gRPC stubs when wire is enabled (`buf generate`).
5. Emits orchestrator registry and typed model artifacts.
6. Detects database schema drift and notifies you of pending migrations.

Profile emission happens **before** store initialization, so sync can write the build profile even when the database is temporarily unavailable.

*   **`--dry-run`**: Validates configuration files without writing generated stubs or files to disk.

### `bffx routes list [--root DIR] [--json] [--filter KIND]`
Lists registered HTTP routes from loaded manifests plus built-in system/auth/CRUD paths.

- **`--json`**: Machine-readable array (`method`, `path`, `kind`, `name`, `auth`).
- **`--filter`**: Substring or kind filter (`action`, `crud`, `screen`, `builder`, `builtin`, `stream`, `cronjob`, …).

Useful for debugging path drift (compare with mobile `BFFXConfig.baseUrl` + relative paths).

### `bffx migrate layout [--root DIR] [--dry-run] [--apply]`
Migrates a **legacy** project tree toward the optional **v2 vertical-slice** layout (`internal/features/<name>/manifests/`, `db/migrations/`, etc.). Run **`--dry-run`** first; **`--apply`** performs file moves. Does not rename `cmd/orchestrator` → `cmd/api` unless documented in the migration log — review before shipping.

### `bffx migrate packaging [plan|apply|rollback] [--root DIR] [--to full|minimal] [--json]`
Guided **full ↔ minimal** packaging migration (build profile source of truth):

| Subcommand | Action |
|------------|--------|
| **`plan`** (default) | Preflight + dry-run diff (project.yaml, profile packages, excluded tooling) |
| **`apply`** | Backup `project.yaml` + `.bffx/build-profile.json`, switch `spec.packaging.mode`, refresh profile, write `.bffx/packaging-migration.json` rollback pointer |
| **`rollback`** | Restore from last backup |

After **`apply`**, run **`bffx sync`**, **`bffx update framework --vendor-only`**, and **`bffx doctor`**.

---

## Diagnostics and project updates

These commands are often confused; they do different jobs:

| Command | Purpose |
|---------|---------|
| **`bffx doctor`** | Read-only health report for **your project** (manifests, secrets, storage, tooling, **vendored framework** metadata). |
| **`bffx upgrade`** | **Schema / manifest sync** (`compiler.Sync`): regenerates artifacts when resources change. **Not** a full Bffx `pkg/` refresh. |
| **`bffx update framework`** | Refreshes **generated shell + Docker templates + vendored** `bffx` core (when a framework checkout is found). See below. |
| **`bffx update`** (no subcommand) | Prints how to update the **CLI binary** (package manager) vs **`bffx update framework`** for **projects**. |

### `bffx version` / `bffx --version` / `bffx -v`
Prints the CLI build identity and the embedded **framework release** constant (`bffx/pkg/version`). Release builds may override the CLI string via `-ldflags -X main.Version=...`.

### `bffx doctor [--root DIR]`
Runs **`doctor.Check`** from the project root (default `.`). Loads `.env` via **`app.LoadEnv`**.

**Always reported (when applicable):**

- **Manifest stability** — `bffx/` manifests load; resource count.
- **Resource uniqueness** — duplicate resource names.
- **Security** — `BFFX_JWT_SECRET`, `BFFX_WORKER_SECRET`, optional `BFFX_APP_SECRET` length (stricter in production).
- **Storage** — store can be opened for the configured mode.
- **Docker** — `docker` on `PATH`.
- **Go** — `go` on `PATH`; warns if older than **go1.26**.
- **Batteries** — resolved `auth`, `store`, `cache`, `blob`, `analytics`, `observability`, `flags`, `vlm`, `nutrition` with connectivity checks where applicable.
- **Build profile** — `.bffx/build-profile.json` presence, drift vs manifests (`profileHash` / `graphHash`), internal consistency (pipelines ⇒ AI, etc.). **Fails** if `packaging.mode=minimal` but profile is missing.
- **Packaging migration** — warns when `.bffx/packaging-migration.json` rollback is available or record/mode mismatch.
- **Extension taxonomy** — deprecated `batteries.nutrition`, canonical flag providers.
- **Observability contract** — canonical `batteries.observability` naming.
- **Route contracts** — warns when Action/Builder paths omit `app.apiPrefix` (e.g. `/api/v1`).

**Framework / vendoring** (when `go.mod` exists in the project root):

- If **`replace bffx => ./.bffx/core`** is absent: **ok** — embedded-core checks skipped (typical for the Bffx framework repo itself).
- If present: validates **`.bffx/core/pkg`**, **`.bffx/framework_version`** vs this CLI’s framework constant, **`.bffx/framework_root`** (path must contain `pkg/`), whether a **framework checkout** is discoverable for `bffx update framework` (`BFFX_ROOT`, recorded root, or walking up from the project dir / cwd), and (when **`authStrategy`** is `optional` or `mandatory`) that vendored **`auth.go`** still contains **anonymous session** support.

Exit code **1** if any check is **fail** (e.g. broken vendored layout).

### Phase 0–8 architecture CI scripts

| Script | CI trigger | Purpose |
|--------|------------|---------|
| `./scripts/ci/packaging-matrix.sh` | Every PR | Full vs minimal scaffold, migration unit tests, sync/build gates |
| `./scripts/ci/architecture-baseline.sh` | Nightly / manual | Sync/build/binary-size baseline for reference fixtures |

These scripts maintain safe migration paths while refactoring core architecture.

### `bffx upgrade [--root DIR]`
Alias for **schema drift detection + sync**: runs **`compiler.Sync`** and prints resource/field changes. Use after editing manifests (add/remove fields, resources). Does **not** copy a new framework `pkg/` tree into **`.bffx/core`**.

### `bffx update framework [DIR] [--root DIR] [--vendor-only] [--dry-run] [--force]`
Keeps a **generated backend** aligned with the Bffx **framework snapshot** you have locally (or pointed at by **`BFFX_ROOT`**).

**What it does (full update, default):**

- Requires a **clean git** working tree (unless **`--dry-run`**).
- Backs up **`cmd/orchestrator/main.go`**, **`Dockerfile`**, **`docker-compose.yml`** to **`*.bak`** (skipped in dry-run).
- Regenerates **`cmd/orchestrator/main.go`** (if **`cmd/orchestrator/registry.gen.go`** exists, keeps **`ActionHandlers` / `HookHandlers`** wiring).
- Runs **`bffx deploy init`** logic: refreshes Docker/compose assets and, when a framework root is found, **vendors** `pkg/` + root `go.mod`/`go.sum` into **`.bffx/core`** and stamps **`.bffx/framework_version`** + **`.bffx/framework_root`**.
- Refreshes generated **test** scaffolding.

**If the stamped `.bffx/framework_version` already matches this CLI’s framework constant** and **`--force` is not set**, the command **skips** regenerating **`main.go`**, Docker/compose, and tests — but still **re-vendors `pkg/`** into **`.bffx/core`** when a framework checkout is found (the stamp is coarse; code may still be stale). With **`--force`**, those skipped files are regenerated too. If no checkout is found, it exits after logging (use **`BFFX_ROOT`** / **`.bffx/framework_root`** or **`--force`** as needed).

**`--vendor-only`**: only re-run **vendoring** into **`.bffx/core`** (and stamps), without touching **`main.go`** or Docker assets. When **`spec.packaging.mode: minimal`** (or `.bffx/build-profile.json` says `minimal`), vendoring copies the profile's runtime **`packages`** list instead of the entire `pkg/` tree. Runs whenever a framework checkout is found (not blocked by a matching version stamp).

**`--dry-run`**: logs what would happen; **no** file writes, backups, or **`go mod tidy`**. Allowed on a **dirty** git tree.

**Finding the framework checkout** (for vendoring): in order — **`BFFX_ROOT`**, **`.bffx/framework_root`**, walk parents of **`--root` / project** until a directory with **`pkg/`** appears, then the same from **cwd**. Run from inside a generated app that lives under your Bffx clone, or set **`BFFX_ROOT`** to the framework repo path.

### `bffx update` (no arguments, or unknown subcommand)
Prints instructions for updating the **installed CLI** (e.g. Homebrew) vs **`bffx update framework`** for **projects**.

---

## 🚢 Deployment Tiers

BFFX supports three tiers of environment parity:

| Tier | Command | Environment | Use Case |
|------|---------|-------------|----------|
| **1. Local Dev** | `bffx dev` | Native Host | Fast feature development & hot reloading. |
| **2. Local Staging** | `bffx up` | Docker Compose | Verifying container networking & worker logic. |
| **3. Production** | `bffx deploy ship` | Remote VPS | Pushing versioned images to a production Droplet. |

### `bffx up [--root PATH]`
The **Local Staging** command. 
*   **Action**: Runs `docker compose up --build`.
*   **Result**: Launches your API (on port 8080), Redis, and Python workers in a containerized environment identical to production.
*   **Health Checks**: Automatically waits for Redis to be healthy before starting the API.

---

### `bffx deploy status`
Shows the current deployment state (which manifests/configs exist).

---

## 🛰 Production Deployment

### `bffx deploy init [--root DIR]`
Transforms your project into a **Standalone Package**.
*   **Dockerization**: Generates an optimized `Dockerfile` with BuildKit caching and multi-tier `docker-compose.yml` variants.
*   **Scripts**: Generates `scripts/setup-droplet.sh` for one-click VPS provisioning.
*   **Vendoring**: When the CLI can resolve a **Bffx framework checkout** (see **`BFFX_ROOT`** and **`bffx update framework`**), copies **`pkg/`** and the framework’s root **`go.mod` / `go.sum`** into **`.bffx/core`** and ensures **`replace bffx => ./.bffx/core`** in the project **`go.mod`**, then runs **`go mod tidy`** in the project. Stamps **`.bffx/framework_version`** and **`.bffx/framework_root`**.

Run from the **Bffx repo** (or set **`BFFX_ROOT`**) when initializing a standalone app so **`.bffx/core`** is populated.

- `bffx ship`: Shortcut for `deploy ship`.
- `bffx doctor [--root DIR]`: Project health, security, storage, tooling, and **framework / vendoring** diagnostics (see [Diagnostics and project updates](#diagnostics-and-project-updates)).
- `bffx lint [--root DIR] [--strict]`: Lint manifests and code for best practices. Use `--strict` for production audits (PII/Security).
- `bffx mcp`: Start the Model Context Protocol (MCP) server for AI-native development.

### `bffx deploy ship`
The "Zero-Ops" deployment command for custom Linux VPS (e.g., DigitalOcean Droplets).
*   **Action**: Builds a versioned Docker image (Git SHA), pushes to GHCR, and triggers a remote `docker compose pull && up` on the VPS.
*   **Zero Downtime**: Uses Docker's image swap mechanism for near-instant updates.
*   **Rollback**: Automatically saves the previous tag for easy revert.

### `bffx deploy rollback`
Instantly reverts the production environment to the previously deployed image tag.

### `bffx deploy cloud [provider]`
Generates cloud-native configuration files for `--fly`, `--railway`, or `--gcp`.

---

## ☁️ VPS Setup Guide (DigitalOcean Droplet)

BFFX is optimized for the **$6/mo DigitalOcean Droplet**.

1.  **Create Droplet**: Ubuntu 22.04 (Basic, 1 vCPU, 1GB RAM).
2.  **Provision**: Run the generated setup script:
    ```bash
    scp scripts/setup-droplet.sh root@YOUR_IP:/tmp/
    ssh root@YOUR_IP "bash /tmp/setup-droplet.sh"
    ```
3.  **Sync Files**: Upload your prod config and environment:
    ```bash
    rsync -avz docker-compose.prod.yml .env root@YOUR_IP:/var/www/bffx-app/
    ```
4.  **Ship**: Run `bffx deploy ship` to go live.

### Required Environment Variables
Set these in your local `.env` for `bffx deploy ship` to work:
*   `BFFX_DEPLOY_HOST`: Droplet IP.
*   `BFFX_DEPLOY_USER`: SSH user (usually `root`).
*   `BFFX_DEPLOY_PATH`: Target directory (e.g., `/var/www/bffx-app`).
*   `BFFX_DEPLOY_IMAGE`: Your GHCR image path (e.g., `ghcr.io/username/myapp`).

---

## 🤖 CI/CD Automation

### `bffx generate cicd`
Generates a production-ready **GitHub Actions** workflow (`.github/workflows/deploy.yml`).
*   **Logic**: Runs E2E tests on every PR. On merge to `main`, it builds the Docker image (with GHA caching), pushes to GHCR, and deploys to your VPS.

---

## 🗄 Database Migrations

BFFX uses a versioned migration system to manage SQL database schemas. This ensures production reliability and allows team review of schema changes.

Existing projects fall back to the legacy declarative reconcile until you opt in by running `bffx migrate init`.

### `bffx migrate init`
One-time bootstrap for an existing project. Generates a baseline `*.up.sql` / `*.down.sql` + `schema.json` from your current manifests and stamps the baseline as already-applied in the `schema_migrations` table so existing tables are not re-created.

### `bffx migrate plan [NAME]`
Compares your current manifests (`bffx/`) against the last known schema snapshot (`migrations/schema.json`) and generates a new migration.
*   **Artifacts**: Creates `YYYYMMDDHHMMSS_name.up.sql` and `YYYYMMDDHHMMSS_name.down.sql`.
*   **Safety**: Automatically adds `-- WARNING: Destructive migration` if it detects column/table drops or type changes.
*   **Snapshot**: Updates `migrations/schema.json` to reflect the new desired state.

### `bffx migrate apply`
Runs all pending `up` migrations against the database.
*   **Tracking**: Uses a `schema_migrations` table to track applied versions.
*   **Automation**: In development mode (`BFFX_ENV!=production`), this is called automatically on server boot if a `migrations/` directory is present. In production the orchestrator refuses to start if drift is detected.

### `bffx migrate status`
Shows the currently applied migration version and whether the database is in a "dirty" state (failed migration).

### `bffx migrate diff`
Previews the pending schema operations without generating files or updating snapshots.

---

## 🛡 Security & Auth

### `bffx env check`
Validates your `.env` for required secrets.

### Device Binding
Fingerprints sessions using `X-Device-ID`. Middleware rejects requests if the `User-Agent` doesn't match the token.

---

## 💎 Generators

*   `bffx generate resource NAME field:type ... [--no-admin-manifest]`: Adds a new manifest.
*   `bffx generate scaffold NAME field:type ... [--no-admin-manifest]`: Creates resource + policy + builder.
*   `bffx generate screen NAME [--nav bottom|folded|onboarding] [--icon NAME] [--order INT] [--no-admin-manifest]`: Mobile screen manifest.
*   `bffx generate action NAME [--no-admin-manifest]`: Scaffolds action manifest.
*   `bffx generate cicd`: Scaffolds GitHub Actions deployment pipeline.
*   `bffx generate worker NAME`: Scaffolds a background job worker.
*   `bffx generate pipeline TYPE --name NAME --feature FEATURE [--catalog=ADAPTER]`: Scaffolds ingestion/chatbot pipeline manifests, adapters, and hooks (v2 layout).
*   `bffx generate client [flutter|kotlin|csharp|typescript]`: Generates type-safe client SDKs.

**`--no-admin-manifest`**: Prevents the generator from auto-scaffolding the companion `AdminResource` or matching admin spec stub in `bffx/admin/` when the admin panel is enabled.

### `bffx add [MODULE]`
Scaffolds complex features:
*   **`admin`**: Built-in Admin Web UI at `/admin`. Sets `spec.admin.enabled: true` in `project.yaml`, scaffolds `site.yaml` and `dashboard.yaml`, and automatically **backfills** default `AdminResource` stubs for all existing resources, screens, and actions.
*   **`flags`**: Feature Flags & Rollouts.
*   **`subscriptions`**: Subscription tracking & Entitlements.
*   **`notifications`**: Push notification device management.
