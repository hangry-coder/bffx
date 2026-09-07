# 🚂 Rails ActiveAdmin to BFFX Admin Migration Guide

If you are coming from the Ruby on Rails ecosystem, you are likely familiar with **ActiveAdmin**. ActiveAdmin utilizes a Ruby DSL to register models, define index tables, build filter sidebars, customize forms, and write custom member or batch actions.

BFFX's Admin Manifest system replicates the developer convenience of ActiveAdmin in Go, but swaps the runtime Ruby evaluation with **declarative YAML manifests** (`bffx/admin/*.yaml`) compiled at build time, and uses a pre-built React SPA for the user interface.

---

## 🔍 Key Architectural Differences

| Concept | Rails ActiveAdmin | BFFX Admin Panel |
| :--- | :--- | :--- |
| **Configuration** | Ruby DSL blocks (`app/admin/`) | YAML manifests (`bffx/admin/`) |
| **Logic Execution** | Server-side Ruby execution | Server-side Go Hook endpoints |
| **UI Rendering** | Server-rendered HTML (via Arbre) | Client-rendered React SPA (dist/ embed) |
| **Asset Pipeline** | Webpacker / Sprockets per project | No node_modules or assets in your backend |
| **Database** | ActiveRecord | BFFX unified `storage.Store` (SQLite/PG) |

---

## 🗺️ DSL Mapping Cheat Sheet

Here is how common ActiveAdmin Ruby constructs translate directly to BFFX YAML manifests:

| ActiveAdmin (Ruby) | BFFX Admin (YAML) | Notes |
| :--- | :--- | :--- |
| `ActiveAdmin.register Post` | `kind: AdminResource` + `spec.resource: Post` | Registers the model with the admin site. |
| `menu parent: "Blog", priority: 1` | `spec.menu.parent: Blog`, `spec.menu.priority: 1` | Sidebar grouping and sorting. |
| `index do column :title; actions end` | `spec.index.columns: [title]` | Selects columns for the index table. |
| `filter :title, as: :string` | `spec.filters: [{ field: title, as: string }]` | Sidebar filter parameters. |
| `scope :published` | `spec.scopes: [{ name: published, where: { ... } }]` | Quick pre-filter tabs above table. |
| `show do attributes_table end` | `spec.show.attributes: [...]` | Detail page field list. |
| `form do f.inputs end` | `spec.form.inputs: [...]` | Form builder inputs. |
| `batch_action :destroy` | `spec.batch_actions: [{ name: destroy, ... }]` | Operations targeting selected checkboxes. |
| `member_action :approve` | `spec.member_actions: [{ name: approve, ... }]` | Action targeting a single record ID. |
| `register_page "Overview"` | `kind: AdminPage` | Non-CRUD pages with widget layouts. |
| `controller do ... end` | Go Hooks namespace (e.g. `admin.post.approve`) | Code execution backend logic. |

---

## 🔄 Side-by-Side Example

### 1. Rails ActiveAdmin (`app/admin/users.rb`)
```ruby
ActiveAdmin.register User do
  menu parent: "Operations", priority: 10
  
  index do
    selectable_column
    column :email
    column :name
    column :status
    column :created_at
    actions
  end

  scope :all, default: true
  scope :active, -> { where(status: "active") }

  filter :email
  filter :status, as: :select, collection: ["active", "suspended"]

  form do |f|
    f.inputs "Details" do
      f.input :email
      f.input :name
      f.input :status, as: :select, collection: ["active", "suspended"]
    end
    f.actions
  end

  member_action :suspend, method: :put do
    resource.update!(status: "suspended")
    redirect_to admin_user_path(resource), notice: "Suspended!"
  end
end
```

### 2. BFFX YAML Manifest (`bffx/admin/user.yaml`)
```yaml
apiVersion: bffx.io/v1alpha1
kind: AdminResource
metadata:
  name: UserAdmin
spec:
  resource: User
  enabled: true
  menu:
    parent: "Operations"
    priority: 10
  index:
    selectable: true
    columns:
      - email
      - name
      - status
      - created_at
    scopes:
      - name: all
        default: true
      - name: active
        where:
          status: active
    filters:
      - field: email
        as: string
      - field: status
        as: select
        options: [active, suspended]
  form:
    inputs:
      - field: email
        as: email
      - field: name
        as: string
      - field: status
        as: select
        options: [active, suspended]
  member_actions:
    - name: suspend
      label: Suspend
      hook: admin.user.suspend
```

---

## 🪝 Migrating Controller Blocks to Go Hooks

In Rails ActiveAdmin, custom member actions are defined using inline Ruby blocks:
```ruby
member_action :approve, method: :put do
  resource.approve!
  redirect_to admin_post_path(resource), notice: "Approved!"
end
```

In BFFX, custom behaviors are written as type-safe Go Hooks in your Go project:

1. **Declare the action in YAML:**
   ```yaml
   member_actions:
     - name: approve
       label: Approve
       hook: admin.post.approve
   ```

2. **Implement the Go Hook (in `hooks/admin_post.go` or similar):**
   ```go
   package hooks

   import (
       "context"
       "github.com/hangry-coder/bffx/pkg/storage"
   )

   // HandleAdminPostApprove is invoked when the approve action is triggered.
   func HandleAdminPostApprove(ctx context.Context, id string, store storage.Store) error {
       // Update record state
       err := store.Update(ctx, "Post", id, map[string]any{
           "status": "approved",
       })
       return err
   }
   ```

---

## 🧱 Migrating Arbre (Arbitrary HTML) to Pages

ActiveAdmin uses **Arbre** to build dashboards and custom views programmatically using Ruby code. Because BFFX uses a pre-built SPA, raw HTML cannot be dynamically injected at runtime. Instead, you migrate Arbre pages to **`AdminPage`** manifests:

1. **For analytical views:** Use `AdminPage` with standard metric cards, tables, and chart widgets.
2. **For arbitrary legacy UIs:** Use `AdminPage` with the `iframe` widget pointing to your dedicated micro-frontend or external business tools (e.g. Retool, Metabase).
   ```yaml
   widgets:
     - type: iframe
       url: "https://your-internal-tool.com/embed"
       height: 500
   ```
