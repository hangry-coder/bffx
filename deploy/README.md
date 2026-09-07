# 🚀 1-Click Deployment Guide for BFFX

Deploy your BFFX backend to production in minutes using your preferred cloud platform or container runner.

---

## ⚡ Deployment Options

| Provider | Best For | Storage Support | 1-Click Ready |
| :--- | :--- | :--- | :---: |
| **Fly.io** | Global edge, lowest latency, SQLite volume | Embedded SQLite & Postgres | ✅ |
| **Railway** | Quickest setup, native PostgreSQL & Redis | Postgres / Managed Redis | ✅ |
| **Render** | Managed services, automated Git deploys | Persistent Disk & Postgres | ✅ |
| **Docker Compose** | Self-hosted VPS (Hetzner, DigitalOcean, AWS EC2) | Postgres + Redis + Caddy | ✅ |
| **Kamal** | Zero-downtime container deploys to bare metal | Any Docker host | ✅ |

---

## 1. Deploy to Fly.io (Recommended for SQLite / Edge)

Fly.io runs BFFX close to your users and supports persistent NVMe storage for SQLite.

### Quick Start with CLI:
```bash
# 1. Ensure flyctl is installed
fly auth login

# 2. Run the 1-click deployment script
./deploy/fly/deploy.sh
```

Or manually:
```bash
cp deploy/fly/fly.toml ./fly.toml
fly volumes create bffx_data --region iad --size 1
fly deploy
```

---

## 2. Deploy to Railway

Railway deploys directly from your GitHub repository with automatic HTTPS and zero configuration.

[![Deploy on Railway](https://railway.app/button.svg)](https://railway.app/new)

1. Click **Deploy on Railway** or link your repository in the Railway dashboard.
2. Under **Variables**, add:
   - `PORT=8080`
   - `BFFX_ENV=production`
   - `BFFX_JWT_SECRET=<secure-random-32-character-secret>`
3. (Optional) Add a PostgreSQL or Redis plugin in Railway with 1 click; BFFX automatically resolves `DATABASE_URL` and `REDIS_URL`.

---

## 3. Deploy to Render

Render offers managed web services with integrated persistent disks and automated Git branch deploys.

1. Connect your GitHub repository to Render as a **Blueprint Instance**.
2. Render reads [`deploy/render/render.yaml`](file:///Users/jawad/Work/Bffx/deploy/render/render.yaml) automatically.
3. Your web service with persistent storage mounts at `/app/.bffx/data` will be provisioned immediately.

---

## 4. Self-Hosted VPS with Docker Compose

For deploying on your own server (Hetzner, OVH, DigitalOcean droplet, Linode):

```bash
# Start the full stack (BFFX + Postgres 16 + Redis 7)
docker compose -f deploy/docker/docker-compose.prod.yml up -d
```

Check health status:
```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

---

## 5. Kamal (Bare Metal / VPS Orchestration)

To deploy using 37signals' Kamal:
```bash
bffx deploy init --kamal
kamal setup
kamal deploy
```
