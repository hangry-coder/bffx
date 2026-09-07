# Notes example (BFFX reference app)

End-to-end demo: **auth**, **Note CRUD**, **screen aggregation** (`NoteList`), **ShareNote** action (FCM to recipients), **hooks** (title validation + analytics + push on create), and **presigned uploads** (when blob env is configured).

## Prerequisites

- Go **1.26.2** (see root `go.mod`).
- From the **repository root**, build the CLI once:

```bash
go build -o bffx ./cmd/bffx
export PATH="$PWD:$PATH"
```

## Run

```bash
cd examples/notes
bffx sync
go run ./cmd/api
```

The API listens on **`:8080`** by default (`PORT` overrides).

### Cold start flow (under 15 minutes)

1. **Signup** — `POST /api/v1/auth/signup` with `email`, `password` (≥8 chars), `name`.
2. **Login** — `POST /api/v1/auth/login` with the same `email` / `password`; keep the `token` header as `Authorization: Bearer …` for the next calls.
3. **Screen** — `GET /api/v1/screens/note-list` returns `user` + `notes` (aggregated).
4. **Create note** — `POST /api/v1/notes` JSON `{"title":"Hello","body":"…"}` (title max 200 chars; enforced in `BeforeCreateNote`).
5. **Share** — `POST /api/v1/notes/share` JSON `{"note_id":"<id>","recipient_user_ids":["<other-user-id>"],"title":"optional"}` — delivers FCM to devices registered for those users (no-op if no `Device` rows / tokens).
6. **Presigned upload** — when the framework blob layer is configured (`BFFX_UPLOAD_*` / S3-compatible), `POST /api/v1/uploads/presign` with a key under your user prefix; store returned `attachment_key` on the note. See [`docs/operations.md`](../../docs/operations.md).

### Smart Caching & SWR
This example leverages BFFX Smart Caching. The `note-list` screen is configured with:
- **TTL**: 60 seconds.
- **Tags**: `resource:Note`.
- **Headers**: `X-BFFX-Cache: HIT|MISS|BYPASS`.

**Workflow:**
1. `GET /api/v1/screens/note-list` → returns `MISS` + `ETag`.
2. Subsequent calls → return `HIT` + `ETag` (from Redis).
3. `POST /api/v1/notes` (Create) → automatically purges `resource:Note` tag.
4. Next `GET` → returns `MISS` (fresh data).

Mobile clients should use the generated `getWithSWR` methods to show cached data instantly while revalidating in the background.

### Tests (CI)

From repo root:

```bash
./scripts/ci/example-notes.sh
```

Or:

```bash
cd examples/notes && bffx sync && go test ./...
```
