# BFFX Production Operations Runbook

This operations runbook provides administrators, DevOps engineers, and system operators with clear, executable procedures to deploy, manage, and scale the BFFX framework in production environments.

For a complete reference of active system environment flags, see the **[BFFX Environment Configuration](file:///Users/jawad/Work/Bffx/docs/environment.md)** guide.

---

## 1. Environment & Secrets Management

In a production environment, BFFX requires explicit environment variable declarations to activate secure, fail-closed operation. Never run production instances with default or auto-generated fallback configurations.

### 1.1 Critical Configuration Block
Export the following environment variables in your deployment container or container orchestration system (e.g., Kubernetes Secrets, AWS ECS Task Definition):

```bash
# Enable production hardening (fail-closed CORS, strict HTTPS headers)
export BFFX_ENV="production"

# Cryptographically secure secrets (minimum 32-character high-entropy strings)
export BFFX_ADMIN_SECRET="5d2be0d77a06a644917a42188ab6bc84285e6878b1cf6565"
export BFFX_JWT_SECRET="a237f394c8e7bd231dfb192809de99fa2818bb819c98bc61b"
export BFFX_ADMIN_SESSION_KEY="c8c5c7d81a9ba2e6840d21a52e9abcb71a39281a8bcf82c"

# Restrict browser client access to trusted domains only
export BFFX_CORS_ALLOW_ORIGINS="https://app.yourcompany.com"

# Network routing & Rate Limiting client attribution
export BFFX_TRUST_X_FORWARDED_FOR="true"
```

> [!IMPORTANT]
> Keep `BFFX_SCHEMA_SINGLETON` unset or set to `true` to ensure locking on schemas if running multiple horizontal processes.

---

## 2. CLI Deployment Workflows

BFFX includes automated tools to initialize, package, and ship your application to container platforms.

### 2.1 Initializing Deploy Configuration
To generate standard multi-stage production Dockerfiles and platform configurations:

```bash
bffx deploy init
```

To initialize deployment configurations with support for third-party orchestrators like Kamal, run:

```bash
bffx deploy init --kamal
```
This generates `config/deploy.yml` with predefined service naming and volume mounts matching your project's datastore mode.

### 2.2 Volume Mount Rules (Important)
When deploying with Docker Compose, ensure your SQLite storage volume is mounted to `/app/.bffx/data/` (not to `/app/.bffx/` directly):

```yaml
# Correct SQLite volume layout:
volumes:
  - app_data:/app/.bffx/data
```
Mounting directly to `/app/.bffx/` is an anti-pattern as it masks the baked-in core framework runtime binaries located in `/app/.bffx/core/`.

### 2.3 Shipping to Production
To compile your assets, build the production-ready Docker image (tagged with git commit SHA and `:latest`), push it to your OCI registry, and deploy it to the remote server:

```bash
bffx deploy ship
```
This writes the active commit tag history and pushes the pinned tag configuration to the server environment.

### 2.4 Rolling Back Releases
If a release causes issues, roll back the remote stack to the previously deployed image version:

```bash
bffx deploy rollback
```
The command automatically reads the deployment history file and restarts the Docker Compose stack using the previous immutable tag configuration.

---

## 3. Database Migrations & Versioning

In production, **never** rely on declarative auto-reconciliation (`Reconcile()`) which triggers on startup and performs unsafe DDL changes. Instead, use versioned migrations.

### 3.1 Creating a Migration Plan
When you modify resource manifests under `bffx/resources/`, generate a SQL-based versioned migration file:

```bash
bffx migrate plan --name add_tasks_status
```
Review the planned SQL output in the generated file under `db/migrations/` before applying it.

### 3.2 Running the Migrations
Execute versioned migrations during your rolling deploy's pre-boot phase:

```bash
bffx migrate up
```

### 3.3 Verifying Migration Status
To check current schema version offsets and pending scripts:

```bash
bffx migrate status
```

---

## 4. Redis Operations & distributed Caching

Redis acts as the distributed communication spine for token revocation, rate limiting, and real-time state synchronization.

### 4.1 Configuring the Redis Spine
Wire Redis securely using the `rediss://` protocol:

```bash
export BFFX_REDIS_URL="rediss://default:your-secure-password@redis.internal.yourdomain.com:6379"
```

### 4.2 Recommended Redis Eviction Policy
To ensure Redis does not run out of memory when caching user sessions and tracking revoked tokens, configure the Redis server instance with:

```ini
maxmemory 2gb
maxmemory-policy volatile-lru
```

---

## 5. Production Backup & Recovery

Regularly execute database backup pipelines to prevent data loss.

### 5.1 Postgres SQL Backup
For Postgres installations, execute structured pg_dumps:

```bash
pg_dump -U bffx -h postgres.internal.yourdomain.com -d bffx -F c -b -v -f /mnt/backups/pg_bffx_$(date +%F_%H%M%S).dump
```

### 5.2 SQLite Online Backup
If running on SQLite for staging/isolated deployments, trigger a non-blocking online database copy:

```bash
bffx db backup --path /mnt/backups/sqlite/app_$(date +%F).db
```

---

## 6. Health Diagnostics & Automated Auditing

Run framework diagnostic routines as a post-deployment verification test or CI build gate.

### 6.1 Running Production Doctor Checks

The `doctor` CLI tool analyzes your project's manifests, configuration files, and active environment to identify security gaps and architectural risks. Always run the doctor check before shipping:

```bash
# Run online check (requires live database and redis connection):
bffx doctor --env production

# Run offline check (CI/CD friendly, bypasses socket connectivity checks):
bffx doctor --env production --offline
```

#### New Production-Only Audits
- **SQLite Prevention**: SQLite is flagged as a failure in production. You must configure `store.mode: postgres` to enable horizontal scaling and database high-availability.
- **Local File Blob Prevention**: Local filesystem storage is flagged as a failure in production. You must switch `batteries.blob` to `s3` or `minio` to prevent data loss when stateless containers restart.
- **Unresolved Environment Variables**: Any placeholder variables `${VAR}` defined in `project.yaml` or config files that are not resolved by active environment variables will fail the doctor check.
- **Plaintext Blueprint Seed Credentials**: Check for hardcoded plaintext literals in blueprints/seeds for credential fields like `password`, `secret`, `otp_code`, etc. In production, these must use environment variable expansion (e.g., `$ADMIN_PASSWORD`).
- **Resource Policy Omission**: Omitted policy blocks or insecure public write permissions on resources default to fail in production.

#### Remediation
If any check fails, the tool exits with code `1`, printing the exact manifest file, line details, and a clear remediation path.

---

## 7. Security Hardening & Edge Gateway

### 7.1 Force SSL and Security Headers
You can enforce HTTPS redirection and HSTS directly in the framework by enabling the `forceSSL` toggle in your project spec (`bffx/project.yaml`):

```yaml
apiVersion: v1
kind: Project
metadata:
  name: your-project
spec:
  app:
    forceSSL: true
```

When enabled:
1. All HTTP requests are redirected to HTTPS based on trusted proxy headers (`X-Forwarded-Proto`).
2. `Strict-Transport-Security` headers are automatically emitted in production mode.

### 7.2 Reverse Proxy Configuration (Caddy)
To terminate TLS securely upstream, the generator scaffolds a sidecar `caddy` reverse proxy service in `docker-compose.prod.yml` and generates a secure default `Caddyfile`.

Ensure the `Caddyfile` is updated with your correct production domain:
```caddy
your-domain.com {
    reverse_proxy orchestrator:8080
}
```
This automatically provisions Let's Encrypt certificates, manages TLS rotation, and injects optimal production security headers (X-Frame-Options, X-Content-Type-Options, etc.).

