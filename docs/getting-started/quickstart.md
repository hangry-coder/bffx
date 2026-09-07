# Getting Started with BFFX

BFFX gives you a powerful API, Async Worker, and Declarative Database Engine, right out of the box.

## Requirements
- Go 1.26+
- Docker (optional, but requested for deploy)

## Installation

### macOS/Linux
```bash
brew tap bffx-io/tap
brew install bffx
```

*Alternatively, download the binaries directly from [GitHub Releases](https://github.com/bffx-io/bffx/releases).*

---

## 🛡 Security Hardening (Handshake)
BFFX v2.1+ requires a mandatory handshake for mobile clients.

1. **Set your App Secret**: Define `BFFX_APP_SECRET` in your environment.
2. **Configure Client**: Your mobile app must send the `X-App-Secret` header with this value.
3. **Whitelist IPs**: (Optional) Restrict API/Admin access in `project.yaml` via `security.allowedIPs`.

---

## 🚀 Creating your first Backend

**1. Scaffold your Project**
```bash
bffx new super-app   # creates ./super-app/ with v2 layout (default store: sqlite)
cd super-app
```

**2. Generate a Database Resource**
Add an `Article` resource to the project:
```bash
bffx generate resource Article title:string content:string published:bool
```

**3. Build and Start the DEV Server**
```bash
go mod init my-app
bffx sync    # writes .bffx/build-profile.json, graph, openapi, registry
bffx dev
```
🎉 Your backend is running at `http://localhost:8080`.

For a **smaller vendored framework footprint**, scaffold with `bffx new super-app --minimal` or migrate later with `bffx migrate packaging` (see [Build Profile Contract](../core-concepts/build_profile_contract.md)).

**SQL stores (sqlite/postgres):** On first `bffx dev`, the database is created/reconciled automatically. To adopt **versioned migrations** (and avoid false “schema drift” warnings on later `bffx sync` runs), run once after the DB exists:

```bash
bffx migrate init
```

**API reference (dev only):** Log into the admin panel at `http://localhost:8080/admin` and open **API Reference** (not public `/api/docs`).

### ⚙️ Non-Interactive Scaffolding with YAML Config
For CI/CD pipelines or reproducible scaffolding, you can configure your project using a YAML config file and the `--config` flag:

```bash
bffx new --config=bffx-project-config.yaml
```

**`bffx-project-config.yaml` Template:**
```yaml
# Project metadata
name: my-automated-app
archetype: fintech      # Options: fintech, commerce, dictation, superapp (optional)
layout: v2              # Project structure layout (v2 or legacy)
minimal: true           # Skip sample resource/screen files

# Admin Panel bootstrap
admin:
  enabled: true
  email: "admin@myconfigapp.com"
  password: "supersecretadminpass"

# Pluggable battery presets/modes
batteries:
  store: sqlite         # Database store battery: sqlite, postgres
  cache: memory         # Caching battery: memory, redis

# Auto-populated environment variables (written to .env)
env:
  DATABASE_URL: "sqlite://app.db"
  MY_CUSTOM_SECRET: "top-secret-val"
```

---

## ⚡️ Quick Start with Vertical Archetypes

BFFX includes **vertical archetypes** to accelerate development for standard app genres. Instead of starting from a blank canvas, you can choose a pre-configured architecture with pre-wired database stores, background workers, AI pipelines, and dashboard screens.

To list all available presets:
```bash
bffx new --list-archetypes
```

### Supported Archetypes

| Archetype Alias | Description | Auth Strategy | Store Mode | Background Worker | Redis Required | Included AI Pipeline |
|---|---|---|---|---|---|---|
| **`fintech`** | Secure transactions & Plaid integrations | `mandatory` | `postgres` | Enabled | Yes | `ReceiptScan` (Ingestion) |
| **`commerce`** | E-commerce with Stripe integrations | `optional` | `postgres` | Enabled | Yes | — |
| **`dictation`** | Meeting transcriber & audio upload | `mandatory` | `postgres` | Enabled | Yes | `AudioUpload` (Ingestion) |
| **`superapp`** | RAG-ready general-purpose superapp | `mandatory` | `postgres` | Enabled | Yes | `Support` (Chatbot) |

### Creating a Project from an Archetype

You can create a project using either **Positional Fallback** or the explicit **`--archetype`** flag:

```bash
# 1. Positional Fallback (Project name matches archetype)
bffx new fintech

# 2. Custom project name with explicit archetype flag
bffx new my-bank --archetype=fintech
```
This scaffolds your chosen preset instantly, sets up layout directories, runs database reconciliation, and builds the post-scaffold template ready to run.

For details on all 10 presets, their specific configurations, and custom overriding, refer to the [Vertical Archetypes Reference Guide](../integrations/archetypes.md).

---

## 🧩 Working with Hooks

BFFX follows a "Declarative Core, Imperative Hooks" pattern. 

1. **Scaffold a Manifest**: Run `bffx generate action MyAction`. This creates `bffx/actions/myaction.yaml`.
2. **Implement Logic**: The `sync` command automatically creates a stub for you in `hooks/myaction.go`. Add your Go code there:
   ```go
   package hooks
   
   func HandleMyAction(ctx *handlers.ActionContext, w http.ResponseWriter, r *http.Request) {
       // Your logic here
   }
   ```
3. **Automatic Wiring**: Run `bffx dev`. The compiler automatically links your hook into the orchestrator runtime. No manual registration required.

---

## 🤖 AI-Native: MCP Support

BFFX includes a built-in Model Context Protocol (MCP) server that allows AI agents to inspect and evolve your project.

To start the MCP server over stdio:
```bash
bffx mcp serve
```

Agents can then list manifests, inspect the project graph, or even call tools like `resource.create` to scaffold your backend automatically.

---

## 🔔 Notifications & Addons

BFFX supports a modular Addon system. To enable push notifications:

1. **Enable the Addon**:
   ```bash
   bffx add notifications
   ```
2. **Register Devices**: Use the auto-generated `POST /api/v1/devices/register` endpoint from your mobile app.
3. **Send Notifications**: Call `NotifyUser` from any Go Hook or Python Skill:
   ```go
   // Go Hook Example
   notifications.NotifyUser(userID, "Your meal was analyzed!")
   ```

---

## 🚢 Deploying to Cloud

Want to go to production?
```bash
bffx deploy init
bffx deploy cloud --fly
```
This generates your `Dockerfile` and IaC files (like `fly.toml`) instantly, ready for cloud deployments!

When you run **`bffx deploy init`** from a checkout that contains the Bffx **`pkg/`** tree (or with **`BFFX_ROOT`** set to that path), the CLI also **vendors** the framework into **`.bffx/core`** and writes **`.bffx/framework_version`** and **`.bffx/framework_root`**. Later, refresh the embedded core with **`bffx update framework --vendor-only`** and validate with **`bffx doctor`**.

- **Schema / codegen drift** (new fields, resources): **`bffx upgrade`** — not the same as updating the vendored framework.
- **CLI reference** for all flags: [cli.md](../reference/cli.md#diagnostics-and-project-updates).
