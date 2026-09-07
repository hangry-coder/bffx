# Deploying BFFX with Kamal

[Kamal](https://kamal-deploy.org/) is a Docker-first deployment tool developed by 37signals. It allows you to deploy web applications to any VPS (Ubuntu/Linux) with zero downtime, using a load balancer (Kamal Proxy) and a rolling upgrade strategy.

This guide walks you through setting up Kamal to deploy a BFFX application.

---

## Prerequisites

1. **Ruby**: Kamal is a Ruby gem. Install Ruby on your operator machine.
2. **Docker**: Running locally on your operator machine (for builds).
3. **Registry Account**: A container registry (e.g. GitHub Packages `ghcr.io` or Docker Hub).
4. **VPS**: A Linux virtual machine (e.g. DigitalOcean, Hetzner, AWS EC2) with SSH key access.

---

## 1. Initializing Kamal

If you haven't initialized deployment configs yet, you can initialize both Docker and Kamal configurations together using the BFFX CLI:

```bash
bffx deploy init --kamal
```

This will write `config/deploy.yml` in your project root, alongside the standard Dockerfile.

Alternatively, you can install Kamal locally and initialize it:

```bash
gem install kamal
kamal init
```

---

## 2. Configuration (`config/deploy.yml`)

Here is a standard, production-ready `config/deploy.yml` for a BFFX app:

```yaml
# config/deploy.yml
service: bffx-app
image: ghcr.io/your-username/bffx-app

# Host VPS details
servers:
  web:
    - 192.168.1.1 # Replace with your actual VPS IP address

# Container Registry authentication details
registry:
  username: your-github-username
  password:
    - KAMAL_REGISTRY_PASSWORD # Loaded from local .env

# Port where BFFX listens inside the container
port: 8080

# Environment variables injected into the container
env:
  clear:
    BFFX_ENV: production
    PORT: 8080
  secret:
    - BFFX_JWT_SECRET
    - BFFX_ADMIN_SESSION_KEY
    - BFFX_APP_SECRET
    # - DATABASE_URL (uncomment if using Postgres/external database)

# Persistent volume mounts
volumes:
  - bffx_data:/app/.bffx/data # Persistent directory for SQLite db

# Build options
builder:
  arch: amd64
  # Use cache to speed up GHA or local compiles
  cache:
    type: registry
    to: ghcr.io/your-username/bffx-app-buildcache:latest
```

---

## 3. Managing Secrets

Do not hardcode production secrets in `deploy.yml`. Store them in your local `.env` file on your operator machine. Kamal automatically reads `.env` to populate secret bindings:

```bash
# .env
KAMAL_REGISTRY_PASSWORD=your-github-pat-token
BFFX_JWT_SECRET=strong-32-char-jwt-secret-key-goes-here
BFFX_ADMIN_SESSION_KEY=strong-32-char-session-signing-key
BFFX_APP_SECRET=strong-16-char-app-secret-key
```

---

## 4. Setup and Deployment

### Step 1: Initial VM Setup
Before shipping, tell Kamal to prepare the target server by installing Docker, setting up the network bridge, and starting the Kamal Proxy:

```bash
kamal setup
```

### Step 2: Running Migrations
If you have database schema updates, run migrations first on the host server:

```bash
kamal app exec -i "/app/bffx-server migrate up"
```

### Step 3: Deploying the Application
Run the deploy command to build, push, pull, and do a zero-downtime traffic handover:

```bash
kamal deploy
```

---

## 5. Rollback

If a deployment fails or introduces a critical bug, rolling back is immediate because Kamal keeps previous image tags on the server:

```bash
# Roll back to the previous running tag
kamal rollback
```

To rollback to a specific version or git SHA tag:

```bash
kamal rollback <git-sha-tag>
```
