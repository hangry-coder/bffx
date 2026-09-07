# Guide: REST-Only Mobile Integration

BFFX is highly versatile. While it ships with a powerful Server-Driven UI (SDUI) framework for screens and fragments, **mobile and frontend teams are 100% free to adopt BFFX purely as an API and CRUD engine**, ignoring screen-building logic altogether.

This guide outlines how to interact with BFFX as a traditional REST backend using standard client generators or plain HTTP libraries.

---

## 1. Accessing the OpenAPI Spec & Endpoints

BFFX automatically generates and serves standard REST endpoints for all resources defined in your `bffx/resources/` directory.

### CRUD Action Routing Taxonomy
For a resource named `Task`, BFFX exposes the following standard routes under your configured `apiPrefix` (e.g., `/api/v1`):

| Endpoint | Method | Action | Auth/Policy | Description |
|----------|--------|--------|-------------|-------------|
| `/api/v1/tasks` | `GET` | List | Configured | Paginated query (supports `limit` and `offset`). |
| `/api/v1/tasks/{id}` | `GET` | Get | Configured | Fetch a single record by UUID. |
| `/api/v1/tasks` | `POST` | Create | Configured | Create a new record. Passes JSON body. |
| `/api/v1/tasks/{id}` | `PUT`/`PATCH` | Update | Configured | Modify an existing record. |
| `/api/v1/tasks/{id}` | `DELETE` | Delete | Configured | Delete a record. |

---

## 2. Authentication & Headers

Standard HTTP client integrations (like `dio` or `http` in Dart/Flutter, or `Axios` in React Native/Web) must include the following headers for secure requests:

### Required Headers

```http
Authorization: Bearer <your_jwt_access_token>
X-BFFX-App-Secret: <your_configured_app_secret>
Content-Type: application/json
```

### Guest & Anonymous Authentication Flow
If your project supports guest sessions, you can retrieve an anonymous JWT session simply by calling:

```http
POST /api/v1/auth/anonymous
Content-Type: application/json

{
  "device_id": "unique-client-device-uuid"
}
```

The response returns an access token and a refresh token, allowing you to authenticate immediately:

```json
{
  "access_token": "ey...",
  "refresh_token": "...",
  "expires_in": 3600
}
```

---

## 3. Custom Actions & Business Logic

If you declare a custom action (e.g., `ShareNote.yaml` or a payment flow), BFFX mounts it as a POST route.

### Example Action Request
If you have an action named `ShareNote` under `bffx/actions/`, call it as follows:

```http
POST /api/v1/actions/share-note
Authorization: Bearer <token>
Content-Type: application/json

{
  "note_id": "4a4b4c-...",
  "recipient_user_ids": ["user-123", "user-456"]
}
```

---

## 4. Bypassing Server-Driven UI

In the generated Flutter client code, standard actions are available directly as simple methods. You can skip importing or using `BFFXScreen` or `BFFXFragment` components and simply invoke database and action methods directly on the core client:

```dart
// Instantiate the BFFX client
final client = BFFXClient(baseUrl: 'https://api.myapp.dev');

// Perform CRUD actions directly
final notes = await client.request('/api/v1/notes', method: 'GET');
final newNote = await client.request(
  '/api/v1/notes', 
  method: 'POST',
  body: {
    'title': 'My awesome REST note',
    'content': 'This note bypasses SDUI!'
  }
);
```

By standardizing on raw JSON exchange, any platform (Swift/iOS, Kotlin/Android, React/Next.js) can build fully custom presentation layers without adopting opinionated server-driven design paradigms.
