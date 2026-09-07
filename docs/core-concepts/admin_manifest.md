# 🖥️ BFFX Admin Manifest Reference

The BFFX Admin Panel is a manifest-driven, out-of-the-box administrative panel. Instead of writing custom HTML, CSS, or React components for every CRUD interface, you define declarative YAML manifests for admin kinds.

**Manifest locations:**

| Layout | Admin manifests directory |
|--------|---------------------------|
| **v2** (default) | `internal/features/admin/manifests/` |
| **legacy** | `bffx/admin/` |

At sync time (`bffx sync`), BFFX compiles these manifests into `.bffx/admin-graph.json`. The embedded React SPA reads the graph from `GET /api/admin/config` and renders navigation from `spec.menu` on `AdminSite` (with sensible defaults when `menu` is omitted).

---

## 🔝 Top-Level Manifest Kinds

The Admin Manifest system is divided into four main kinds:
1. **`AdminSite`**: Configures global site metadata, branding, and core features.
2. **`AdminDashboard`**: Declares home screen widgets (metrics, charts, tables).
3. **`AdminResource`**: Overrides the default CRUD table, search filters, scopes, forms, and custom action hooks for database resources.
4. **`AdminPage`**: Renders custom non-CRUD pages (e.g., reports, custom forms, iframe embeds).

---

## 🌐 `AdminSite` — Global Branding & Navigation

Create this file as `internal/features/admin/manifests/site.yaml` (v2) or `bffx/admin/site.yaml` (legacy) to customize branding, themes, and global navigation.

```yaml
apiVersion: bffx.io/v1alpha1
kind: AdminSite
metadata:
  name: default
spec:
  title: "Admin Console"
  theme: system                        # light | dark | system
  default_per_page: 25
  export:
    csv_enabled: true
  features:
    app_features: true                 # Toggle BFFX native app features tree
    feature_flags: true                # Toggle dedicated feature flags UI
    jobs: true                         # Toggle background jobs SSE runner
    liveops: true                      # Toggle live telemetry charts
    api_docs: true                     # Embed interactive API Reference (dev only)
  session:
    dev_unlimited: true                # development default: long-lived cookie
    max_age: 8h                        # production absolute cap (ignored when dev_unlimited in dev)
    idle_timeout: 30m                  # SPA idle → POST /logout (production default if unset)
  menu:
    - label: Dashboard
      page: dashboard
      priority: 0
    - label: Feature Flags
      page: feature-flags
      priority: 5
    - label: Users
      page: users
      priority: 8
    - label: Monitoring
      priority: 50
      children:
        - label: Performance
          page: performance
          priority: 0
    - label: Resources
      priority: 80                     # children: pinned resources + Other Resources (injected)
    - label: App Features
      priority: 70                       # children: pinned screens + Other Features (injected)
    - label: Settings
      priority: 90                       # App Settings + Docs subgroup (injected)
```

**Default menu scaffold:** `bffx new` / `bffx add admin` writes the skeleton above into `default.yaml`. The **Main** label in the sidebar is UI chrome only (not a manifest row). Omit `menu` entirely to let BFFX synthesize the same structure at compile time.

**Navigation note:** When `spec.menu` is set, the SPA renders those entries plus dynamic injections (Resources, App Features, Settings/Docs). `AdminResource.spec.menu.parent` must match a top-level menu item `label` (e.g. `Platform`, `Fasting`). The framework maps parent `app` onto label `Operations` for older scaffolds.

**Session:** Admin cookies use signed payload `v2|email|issued|expires|idle`. Configure via `spec.session` or env `BFFX_ADMIN_SESSION_MAX_AGE`, `BFFX_ADMIN_SESSION_IDLE`, `BFFX_ADMIN_SESSION_DEV_UNLIMITED`. `GET /api/admin/config` exposes `site.session_resolved` for the SPA idle timer.

**Developer docs (non-production):** When `features.api_docs` is enabled and `BFFX_ENV` is not `production`, Settings includes a **Docs** group with **API Reference** (Swagger) and **BFFX Guide** (markdown from `.bffx/docs/`, bundled on `bffx update framework --vendor-only`).

---

## 📊 `AdminDashboard` — Home Screen Widgets

Configure widgets for your admin panel home screen in `bffx/admin/dashboard.yaml`.

```yaml
apiVersion: bffx.io/v1alpha1
kind: AdminDashboard
metadata:
  name: main
spec:
  widgets:
    - type: metric
      title: Active Users (24h)
      query:
        resource: User
        aggregate: count
        where:
          status: active
    - type: chart
      title: User Signups
      source: builtin                  # Built-in telemetry metrics
      metric: auth.signups
      range: 7d
    - type: table
      title: Recent Incidents
      resource: Incident
      limit: 5
      columns: [severity, message, created_at]
```

---

## 📦 `AdminResource` — Table & CRUD Customs

Whenever you scaffold a database model, BFFX generates an `AdminResource` manifest (e.g., `bffx/admin/user.yaml`) to customize its admin interface. If a resource has no matching `AdminResource` manifest, BFFX synthesizes sensible defaults automatically.

```yaml
apiVersion: bffx.io/v1alpha1
kind: AdminResource
metadata:
  name: UserAdmin
spec:
  resource: User                     # Links to kind: Resource name
  enabled: true                      # Set to false to hide completely from admin
  
  menu:
    label: Users
    parent: Operations               # Groups this under the "Operations" sidebar group
    priority: 10
    pin: true                        # Pin directly to the top of the sidebar

  index:
    per_page: 25
    default_sort: "created_at desc"
    selectable: true                 # Enables batch checkbox selection
    columns:
      - email
      - name
      - role
      - status
      - created_at
    scopes:                          # Scopes represent quick filter tabs above the table
      - name: all
        default: true
      - name: guests
        where:
          role: guest
      - name: active
        where:
          status: active
    filters:                         # Sidebar search options
      - field: email
        as: string
        label: Email
      - field: role
        as: select
        options: [guest, user, admin]
      - field: created_at
        as: date_range

  show:
    attributes:                      # Fields to render on detail page
      - email
      - name
      - role
      - created_at
    exclude: [password, otp_code]     # Protect sensitive fields

  form:
    exclude: [password, otp_code, role] # Fields to omit from create/edit forms
    inputs:
      - field: email
        as: email
      - field: name
        as: string
      - field: status
        as: select
        options: [active, suspended]

  batch_actions:                     # Multi-select operations
    - name: suspend
      label: Suspend Selected
      confirm: "Are you sure you want to suspend these users?"
      hook: admin.user.batchSuspend   # Maps to a Go hook: admin.user.batchSuspend

  associations:                      # Parent detail → child tables
    - name: devices
      resource: Device
      foreign_key: user_id
      per_page: 25
  belongs_to:                        # Child index/detail → parent links (auto-inferred from field.target)
    - field: user_id
      resource: User
  member_actions:                    # Single record operations
    - name: send_reset
      label: Send Reset Email
      hook: admin.user.sendReset
      only: [show]                   # Display only on the record detail page

  policy:                            # RBAC overrides
    read: admin                      # Minimum role to read/list
    write: superadmin                # Minimum role to create/update/delete/execute actions
```

---

## 📄 `AdminPage` — Custom Layouts & Embeds

Use `AdminPage` manifests to define arbitrary custom pages populated with widgets, charts, and escape hatches.

```yaml
apiVersion: bffx.io/v1alpha1
kind: AdminPage
metadata:
  name: analytics
spec:
  menu:
    label: Analytics
    parent: Reports
    priority: 1
  layout: two_column
  widgets:
    - type: markdown
      content_key: admin.analytics.intro # Resolves localized strings
    - type: iframe
      url: "https://metabase.example.com/embed/dashboard/1"
      height: 600
```

---

## 🪝 Go Action Hooks

Rather than writing client-side Javascript, all custom admin behaviors are driven server-side using **Go Hooks**. 
When a user triggers a batch, member, or collection action in the UI, the backend router executes the Go function registered under that hook name (e.g. `admin.user.batchSuspend`).

---

## 🎨 Field Widget Registry (`as` mapping)

The frontend generic renderer renders appropriate HTML inputs and view widgets according to the `as` attribute in the manifest:

| Widget `as` | Index Column View | Filter Input | Form Field | Show Details |
| :--- | :--- | :--- | :--- | :--- |
| `string` | Plain Text | Contains Text | Text Input | Text |
| `email` | Plain Text | Contains Text | Email Input | Mailto Link |
| `select` | Badge | Select Dropdown | Select Dropdown | Badge |
| `bool` | Checkmark/Cross | Checkboxes | Checkbox | Toggle Switch |
| `date` / `datetime` | Formatted Date | Date Range Picker | Datepicker | Formatted Date |
| `json` | Truncated Preview | — | JSON Editor | Interactive Tree |
| `relation` | Clickable Link | Select Dropdown | Searchable Select | Clickable Link |
| `password` | Hidden | — | Password (Create only) | Hidden |
