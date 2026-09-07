# Action Authorization Contract

In BFFX, security is divided into two distinct responsibilities: **declarative entry-point protection (Authentication)** and **runtime logic execution (Authorization)**. Understanding the distinction and contract between **Resource Policies** and **Action Hooks** is crucial to securing your BFF.

---

## 1. Resource Policies (The Gatekeeper)

Resource manifests govern database read/write access using a declarative model. 

```yaml
apiVersion: v1
kind: Resource
metadata:
  name: Note
spec:
  fields:
    - name: title
      type: string
  policy:
    read: owner
    write: owner
```

### The Contract
- **Automatic Enforcement**: BFFX automatically hooks into database queries and CRUD controllers to ensure that a requesting user can only perform queries allowed by the declared `policy` (e.g., `owner`, `public`, `admin`).
- **Low Overhead**: Developers do not need to write query filter modifications or authorization logic for simple CRUD. BFFX restricts queries at the SQL/datastore layer.

---

## 2. Action Authentication (The Mux Level)

Action manifests expose custom business logic via HTTP endpoints. They define access at the routing level.

```yaml
apiVersion: v1
kind: Action
metadata:
  name: SendSensitiveReport
spec:
  route:
    method: POST
    path: /api/v1/reports/sensitive
    auth: required
```

### The Contract
- **Authentication, Not Authorization**: The `route.auth` property controls whether BFFX accepts or rejects requests based on session/token validity. 
  - `required`: Request must have a valid JWT. Unauthenticated requests are rejected with `401 Unauthorized` before reaching any custom hook or handler.
  - `optional`: The request is routed; if a valid token is present, context is populated. If not, the request proceeds anonymously.
  - `public`: Bypass all credential checking.

---

## 3. The Danger of Public Actions & The Sensitive Name Heuristic

Exposing endpoints to the public internet is sometimes required (e.g., `login`, `register`, `oauth_callback`). However, leaving state-changing actions publicly accessible without verifying permissions inside the logic hook introduces critical security vulnerabilities.

### The BFFX Lint Guardrail
To prevent accidental exposures of administrative or mutation actions, the `bffx lint` engine scans action manifests. If an Action has `auth: public` and matches a sensitive pattern in its name, the linter emits a warning.

#### Sensitive Keywords Scanned
- **Mutations & Destruction**: `create`, `update`, `delete`, `destroy`, `remove`, `purge`, `write`, `set`, `reset`
- **Access Control**: `grant`, `revoke`, `password`, `role`, `permission`, `auth`
- **High Privilege & Finance**: `admin`, `payment`, `payout`, `billing`, `invoice`, `sensitive`
- **Identities**: `user`, `profile`

> [!WARNING]
> **Lint Warning Example:**
> `[WARNING] delete_user: Action 'delete_user' has public auth but has a sensitive name pattern. Consider protecting it or explicitly verifying authorization inside the action hooks.`

---

## 4. Best Practices Checklist for Custom Actions

When building custom actions, follow this hierarchy to ensure safety:

1. **Default to Private**: Always configure `auth: required` for custom endpoints unless there is an explicit requirement for the public internet to trigger them.
2. **Context-Aware Validation**: Inside your Action hooks (Go or Python), inspect the context for user information before performing any business logic:
   ```go
   func HandleMyAction(ctx context.Context, input map[string]any) (map[string]any, error) {
       // Safely retrieve caller identity from BFFX context
       user, ok := auth.UserFromContext(ctx)
       if !ok {
           return nil, errors.New("unauthorized")
       }
       
       // Explicit business-logic check
       if user.Role != "admin" {
           return nil, errors.New("forbidden")
       }
       
       // Execute action logic...
       return map[string]any{"status": "success"}, nil
   }
   ```
3. **Never Trust Client Input**: Do not pass user IDs or roles in request payloads (`input`) to authorize the user. Always resolve them from the validated authentication context (`ctx`).
