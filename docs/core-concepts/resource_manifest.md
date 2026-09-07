# 💎 BFFX Resource Configuration Reference (`resources/`)

Resource manifests define your application's data models, API routes, and business logic triggers (hooks). Every `.yaml` file in `bffx/resources/` represents a database table and a set of API endpoints.

---

## 🔝 Top-Level Fields

| Field | Description | Example |
| :--- | :--- | :--- |
| `apiVersion` | The version of the BFFX manifest schema. | `bffx.io/v1alpha1` |
| `kind` | Must be `Resource` for these manifests. | `Resource` |
| `metadata.name` | The PascalCase name of the resource. | `ConsentGrant` |
| `spec.group` | The UI/Admin grouping for this resource. | `app`, `system`, `mobile` |
| `spec.system` | If `true`, this is a framework-managed resource. | `true` / `false` |

---

## 🛠 `spec.fields` & Auto-Injected Fields
Defines the schema of your resource. 

**IMPORTANT**: BFFX automatically injects and manages the following fields for EVERY resource. You do not need to define them in your manifest:
*   **`created_at`**: ISO8601 timestamp of when the record was created.
*   **`updated_at`**: ISO8601 timestamp of the last modification.
*   **`created_by`**: The UUID of the user who created the record (automatically provisioned for anonymous users via `X-Device-ID`).

| Property | Description | Types |
| :--- | :--- | :--- |
| `name` | The field name (snake_case suggested). | `id`, `user_id`, `status` |
| `type` | The data type. | `string`, `int`, `float`, `bool`, `json` |
| `required`| Whether the field must be present on CREATE. | `true` / `false` |
| `default` | The default value if not provided. | `"active"`, `0`, `false` |
| `bucket` | (Optional) Configures the field for file uploads. | `"images"`, `"documents"` |

---

## 🛡 `spec.policy`
Controls who can access the generated API routes.

*   **`read`**: Permission required to GET or LIST records.
*   **`write`**: Permission required to POST, PATCH, or DELETE records.

**Standard Values:**
*   **`owner` (Default)**: Only the user who created the record can access it. Recommended for user data.
*   `public`: Anyone can access (no token required).
*   `authenticated`: Any user with a valid JWT.
*   `system`: Only internal worker processes or admin keys.

---

## 🪝 `spec.hooks`
Toggle individual Go hooks. If enabled, `bffx sync` will look for matching functions in your `hooks/` directory.

```yaml
hooks:
  beforeCreate: true   # Run logic before saving to DB (e.g., validation, scoring)
  afterCreate: true    # Run logic after saving (e.g., sending emails, triggering AI)
  beforeUpdate: true
  afterUpdate: true
  beforeDelete: true
```

---

## 🚀 Advanced Features

*   **`stream: true`**: Automatically publishes changes to the project's event bus. Useful for real-time dashboards.
*   **`tree: true`**: Enables Parent/Child relationship logic (DAGs) for things like Skill Trees or folder structures.

---

## 📝 Example: `ConsentGrant.yaml`
```yaml
apiVersion: bffx.io/v1alpha1
kind: Resource
metadata: 
  name: ConsentGrant
spec:
  group: app
  fields:
    - { name: scope, type: string, required: true } # e.g. "health:read"
    - { name: status, type: string, default: "active" }
  policy: 
    read: owner
    write: owner
  routes:
    crud: true
```

## 📝 Example: `AuditEvent.yaml`
```yaml
apiVersion: bffx.io/v1alpha1
kind: Resource
metadata:
  name: AuditEvent
spec:
  fields:
    - { name: action, type: string, required: true }
    - { name: metadata, type: json }
  policy:
    read: owner
    write: system # Users check their logs; only framework/hooks can write
  routes:
    crud: true
  stream: true # Push audit events to real-time monitors
```

---

## 🚀 Custom Action Caching & Safety (`cache_ttl`)

Custom `Action` manifests (defining custom Go hooks at `bffx/actions/MyAction.yaml`) support route-level response caching via `cache_ttl`.

```yaml
apiVersion: bffx.io/v1alpha1
kind: Action
metadata:
  name: GetStaticCatalog
spec:
  route:
    method: GET
    path: /api/v1/catalog
    cache_ttl: 300 # Cache GET responses for 5 minutes (300 seconds)
```

### ⚠️ Dynamic GET Cache Safety Rules
To prevent serving stale state or exposing data privacy leaks, the BFFX engine and `bffx doctor` enforce strict safety guidelines on action caching:

1. **Mutable Routes are Forbidden from Caching**:
   - Any Action with a mutable HTTP method (`POST`, `PUT`, `PATCH`, `DELETE`) **must not** define `cache_ttl`.
   - The BFFX linter (`bffx doctor`) treats caching on mutable actions as a **critical lint failure** (`❌`).
2. **GET Actions Warn on Caching**:
   - Custom business logic `Action` GETs are stateful and dynamic by nature.
   - Any Action with method `GET` and `cache_ttl > 0` triggers a **doctor warning** (`⚠️`) to advise developer caution. Ensure that dynamic user state is not cached under a shared key.
3. **State Mutation Invalidation**:
   - If an action response is cached, you must invalidate it when state changes using cache tags or programmatic invalidation.

## 🔁 Action Idempotency Modes

Mutation Actions can opt into stricter replay guarantees:

```yaml
apiVersion: bffx.io/v1alpha1
kind: Action
metadata:
  name: ChargeCustomer
spec:
  route:
    method: POST
    path: /api/v1/billing/charge
    auth: required
    idempotency_mode: durable # ""/"cache" (default) or "durable"
```

- `cache` (or omitted): uses the configured idempotency store behavior.
- `durable`: requires a durable idempotency backend and a client `X-Idempotency-Key`; otherwise the route fails fast.
