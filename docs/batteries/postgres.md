# Store battery: Postgres (incl. Supabase)

**Supabase in v1 = managed Postgres only.** Set `batteries.store: postgres` (or `spec.store.mode: postgres`) and point `DATABASE_URL` / `spec.store.url` at the Supabase **pooler** connection string. Do not use Supabase Auth, Realtime, or Storage SDKs inside the BFF.

## Local swap

```yaml
# bffx/project.yaml
batteries:
  store: postgres
spec:
  store:
    mode: postgres
    url: ${DATABASE_URL}
```

```bash
export DATABASE_URL="postgres://user:pass@localhost:5432/myapp?sslmode=disable"
bffx migrate apply --root .
bffx doctor --root .
```

## Supabase notes

- Use the **transaction** pooler URL for server-side migrations when recommended by Supabase docs.
- **RLS** on Supabase applies to direct client access; the BFF uses a service role / server connection — document policies for any direct Supabase client usage separately.
- See also [supabase.md](../supabase.md).

## Verdict

**Beta** — Same `pkg/storage` Postgres driver as self-hosted Postgres; no Supabase-specific SDK dependency.
