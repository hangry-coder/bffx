# BFFX Deployment Decisions & Overview

BFFX applications are packaged as lightweight, production-ready container images with minimal runtime overhead. You can deploy BFFX on virtually any infrastructure that supports OCI containers (Docker).

This guide helps you choose the best deployment methodology based on your project's scaling requirements, budget, and operational experience.

---

## Deployment Options at a Glance

| Deployment Target | Best For | Architecture | Rollback Mechanism | Pros / Cons |
| :--- | :--- | :--- | :--- | :--- |
| **Docker Compose + SSH** | Simple, low-cost VPS setups | Single host, SQLite or external DB | `bffx deploy rollback` | + Zero overhead, very fast deploys.<br>- No built-in load balancer/rolling updates. |
| **[Kamal](./kamal.md)** | Production VPS deployments | Docker-based zero-downtime rolling deploys | `kamal rollback` | + Automated, zero-downtime, no cloud lock-in.<br>- Requires Ruby on operator machine. |
| **[Coolify](./coolify.md)** | Self-hosted PaaS on a VPS | Full control PaaS panel | 1-Click Dashboard rollback | + UI-driven dashboard, simple DB provision.<br>- Panel adds minor RAM overhead to VPS. |
| **Fly.io / Railway** | Edge databases & low-ops hosting | Managed containers | Built-in CLI rollback | + Global edge placement, managed DBs.<br>- Bandwidth and SQLite storage costs. |
| **Kubernetes (K8s)** | Enterprise & high availability | Scale-out replicas | Helm / kubectl rollback | + Auto-scaling, maximum control.<br>- Massive operational complexity. |

---

## How to Choose: Decision Tree

```
Are you deploying to a virtual private server (VPS, e.g., DigitalOcean, Hetzner, AWS EC2)?
  ├── YES: Do you want zero-downtime rolling updates?
  │      ├── YES: Use Kamal (Recommended for production VPS).
  │      └── NO: Do you prefer a web-based management dashboard?
  │             ├── YES: Use Coolify.
  │             └── NO: Use `bffx deploy ship` (Simplest VPS deploy).
  │
  └── NO: Do you want a fully-managed Platform-as-a-Service (PaaS)?
         ├── YES: Use Fly.io, Railway, or Render.
         └── NO: For enterprise scale-out with Kubernetes, use official container images.
```

---

## Common Deployment Principles for BFFX

### 1. Build Once, Promote Everywhere
Never build container images directly on production hosts. Compile your production image in a CI pipeline (e.g. GitHub Actions) or on your operator laptop, tag it with an immutable tag (e.g., git commit SHA), push it to an OCI registry (e.g., GitHub Packages / GHCR), and pull that pinned tag on your host.

### 2. Environment Configuration
Store all configuration in environment variables. Avoid baking secrets into your container images. BFFX relies on the following production variables:
* `BFFX_ENV=production`
* `BFFX_JWT_SECRET` (Minimum 32 characters)
* `BFFX_ADMIN_SESSION_KEY` (Minimum 32 characters)
* `BFFX_APP_SECRET` (Minimum 16 characters)
* `DATABASE_URL` (For Postgres mode)

### 3. SQLite Volumes
If you are deploying with SQLite, you **must** mount a persistent volume at `/app/.bffx/data/` (where `app.db` resides). Mounting a volume over `/app/.bffx/` directly is an anti-pattern as it masks the baked-in core framework runtime binaries.

---

## `bffx deploy` commands

| Command | Status | Notes |
| --- | --- | --- |
| `bffx deploy init` | **Supported** | Dockerfile, compose, Caddy, optional `--kamal` |
| `bffx deploy ship` | **Supported** | Local build + push + remote compose with `BFFX_DEPLOY_TAG` |
| `bffx deploy rollback` | **Supported** | Uses deploy history file |
| `bffx deploy status` | **Supported** | Artifact presence + last tag |
| `bffx deploy cloud fly` | **Stub** | Generates starter config; verify before production |
| `bffx deploy cloud railway` | **Stub** | Same |
| `bffx deploy cloud gcp` | **Stub** | Same |

For production VPS, prefer **`deploy init` + `deploy ship`** or [Kamal](./kamal.md) until cloud stubs are marked stable in release notes.

---

## Detailed Guides

* [Deploying with Kamal](./kamal.md)
* [Deploying with Coolify](./coolify.md)
