package deploy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hangry-coder/bffx/pkg/buildprofile"
	"github.com/hangry-coder/bffx/pkg/docsbundle"
	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/version"
)

// InitOptions controls deploy.Init behavior (nil = defaults).
type InitOptions struct {
	// DryRun logs planned writes and skips mutating the filesystem.
	DryRun bool
	// Kamal generates a Kamal config/deploy.yml configuration.
	Kamal bool
}

func Init(root string, workspaceRoot string, opts *InitOptions) error {
	o := InitOptions{}
	if opts != nil {
		o = *opts
	}

	reg, err := manifest.LoadAll(root)
	if err != nil {
		return fmt.Errorf("failed to load manifests: %w", err)
	}

	var project manifest.ProjectSpec
	reg.Project.UnmarshalSpec(&project)

	// Vendor the framework core to make the project standalone
	if workspaceRoot != "" {
		vo := &VendorOptions{DryRun: o.DryRun}
		if err := Vendor(root, workspaceRoot, vo); err != nil {
			logger.Error("Vendoring failed: %v. Proceeding with shared reference.", err)
		}
	}

	cmdMain := "cmd/orchestrator/main.go"
	runtimeCopy := "COPY --from=builder --chown=bffx:bffx /app/bffx /app/bffx"
	composeManifestVol := "      - ./bffx:/app/bffx:ro"
	if project.Layout == "v2" {
		cmdMain = "cmd/api/main.go"
		runtimeCopy = strings.Join([]string{
			"COPY --from=builder --chown=bffx:bffx /app/bffx /app/bffx",
			"COPY --from=builder --chown=bffx:bffx /app/internal /app/internal",
			"COPY --from=builder --chown=bffx:bffx /app/assets /app/assets",
			"COPY --from=builder --chown=bffx:bffx /app/config /app/config",
			"COPY --from=builder --chown=bffx:bffx /app/db /app/db",
		}, "\n")
		composeManifestVol = strings.Join([]string{
			"      - ./bffx:/app/bffx:ro",
			"      - ./internal/features:/app/internal/features:ro",
		}, "\n")
	}

	dockerfile := fmt.Sprintf(`# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Cache Go module downloads before copying full sources (replace => ./.bffx/core)
COPY go.mod go.sum ./
COPY .bffx ./.bffx
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

    # Cache Go build artifacts
    RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /bffx-server %s

# Production stage
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata wget su-exec
RUN addgroup -g 1001 bffx && adduser -u 1001 -G bffx -s /bin/sh -D bffx

WORKDIR /app
RUN chown bffx:bffx /app
COPY --from=builder --chown=bffx:bffx /bffx-server /app/bffx-server
COPY --from=builder --chown=bffx:bffx /app/.bffx /app/.bffx
%s
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=5s --start-period=60s --retries=8 \
  CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["/app/bffx-server"]
`, cmdMain, runtimeCopy)
	// ... rest of the logic for docker-compose ...

	workerDockerfile := `FROM python:3.11-slim

WORKDIR /app

# Install system dependencies if needed
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt* ./
RUN if [ -f requirements.txt ]; then pip install --no-cache-dir -r requirements.txt; fi

# Install bffx if needed, or assume it's in the path/copied
# For now, we assume the worker just needs the skills and functions
COPY . .

# Set environment variables
ENV PYTHONUNBUFFERED=1
ENV REDIS_URL=redis://redis:6379

CMD ["python", "worker/main.py"]
`

	datastoreService := ""
	apiEnv := []string{
		"BFFX_STORE_MODE=" + project.Store.Mode,
		"REDIS_URL=redis://redis:6379",
	}

	dependsOnYaml := "\n    depends_on:\n      redis:\n        condition: service_healthy"

	if project.Store.Mode == "postgres" {
		datastoreService = `
  datastore:
    image: postgres:16-alpine
    environment:
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=postgres
      - POSTGRES_DB=bffx
    volumes:
      - pgdata:/var/lib/postgresql/data
`
		apiEnv = append(apiEnv, "DATABASE_URL=postgres://postgres:postgres@datastore:5432/bffx?sslmode=disable")
		dependsOnYaml += "\n      datastore:\n        condition: service_started"
	} else if project.Store.Mode == "pocketbase" {
		datastoreService = `
  datastore:
    image: ghcr.io/pocketbase/pocketbase:latest
    ports:
      - "8090:8080"
    volumes:
      - pbdata:/pb_data
    command: serve --http=0.0.0.0:8080
`
		dependsOnYaml += "\n      datastore:\n        condition: service_started"
	}

	envYaml := ""
	for _, e := range apiEnv {
		envYaml += "\n      - " + e
	}

	workerService := ""
	if project.Runtime.Worker.Enabled {
		workerService = `
  worker:
    build:
      context: .
      dockerfile: worker/Dockerfile
    env_file: .env
    environment:
      - REDIS_URL=redis://redis:6379
    depends_on:
      redis:
        condition: service_healthy
      orchestrator:
        condition: service_healthy
    restart: unless-stopped`
	}

	volumesYaml := "  app_data:\n  redis_data:"
	if project.Store.Mode == "postgres" {
		volumesYaml += "\n  pgdata:"
	} else if project.Store.Mode == "pocketbase" {
		volumesYaml += "\n  pbdata:"
	}

	// Standard docker-compose.yml (Local Staging)
	dockerCompose := fmt.Sprintf(`services:
  orchestrator:
    build:
      context: .
      dockerfile: Dockerfile
    image: ${PROJECT_NAME:-bffx-app}:latest
    ports:
      - "8080:8080"
    env_file: .env
    environment:%s%s
    volumes:
      - app_data:/app/.bffx/data
%s
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/health"]
      interval: 15s
      timeout: 5s
      retries: 8
      start_period: 60s

  redis:
    image: redis:7-alpine
    restart: unless-stopped
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 3s
      retries: 3
%s%s

volumes:
%s
`, envYaml, dependsOnYaml, composeManifestVol, datastoreService, workerService, volumesYaml)

	// docker-compose.staging.yml
	dockerStaging := `services:
  orchestrator:
    ports:
      - "8081:8080"
    environment:
      - BFFX_ENV=staging
`

	// docker-compose.prod.yml
	dockerProd := `services:
  orchestrator:
    image: ${BFFX_DEPLOY_IMAGE:-ghcr.io/user/app}:${BFFX_DEPLOY_TAG:-latest}
    build: !reset [] # Disable build on prod
    # Exposing port 8080 directly on host is insecure. Use the Caddy sidecar below for SSL.
    # ports:
    #   - "8080:8080"
    expose:
      - "8080"
    env_file: .env
    environment:
      - BFFX_ENV=production
      - PORT=8080
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3
      start_period: 10s

  # Production Reverse Proxy with TLS termination using Caddy.
  # Update Caddyfile with your actual production domain.
  caddy:
    image: caddy:2.7-alpine
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
      - caddy_data:/data
      - caddy_config:/config
    depends_on:
      orchestrator:
        condition: service_healthy

volumes:
  caddy_data:
  caddy_config:
`

	caddyfile := `# Production Caddyfile for SSL termination.
# Replace your-domain.com with your actual production domain.
# Or use :80 for testing HTTP locally/staging.

your-domain.com {
    reverse_proxy orchestrator:8080

    # Production security headers
    header {
        # Disable FLoC tracking
        Permissions-Policy "interest-cohort=()"
        # Enable HSTS (HTTP Strict Transport Security)
        Strict-Transport-Security "max-age=63072000; includeSubDomains; preload"
        # Prevent Clickjacking
        X-Frame-Options "DENY"
        # Prevent MIME-sniffing
        X-Content-Type-Options "nosniff"
        # XSS Protection
        X-XSS-Protection "1; mode=block"
        # Referrer Policy
        Referrer-Policy "strict-origin-when-cross-origin"
    }
}
`

	if project.Runtime.Worker.Enabled {
		workerDir := filepath.Join(root, "worker")
		if !o.DryRun {
			if err := os.MkdirAll(workerDir, 0o755); err != nil {
				return err
			}
		} else {
			logger.Info("[dry-run] would mkdir %s", workerDir)
		}
	}

	dockerEntrypoint := `#!/bin/sh
set -e
# Named volumes for SQLite mount as root-owned; ensure bffx can write before dropping privs.
mkdir -p /app/.bffx/data
if [ "$(id -u)" = "0" ]; then
	chown -R bffx:bffx /app/.bffx/data
	exec su-exec bffx "$@"
fi
exec "$@"
`

	dockerIgnore := `
.git
.gitignore
bin/
*.test
tests/
specs/
*.db
*.db-shm
*.db-wal
.bffx/*.db
.env
.env.*
.DS_Store
gen/
`

	writes := []struct {
		rel     string
		mode    fs.FileMode
		content []byte
	}{
		{"Dockerfile", 0o644, []byte(dockerfile)},
		{"docker-entrypoint.sh", 0o755, []byte(dockerEntrypoint)},
		{"docker-compose.yml", 0o644, []byte(dockerCompose)},
		{"docker-compose.staging.yml", 0o644, []byte(dockerStaging)},
		{"docker-compose.prod.yml", 0o644, []byte(dockerProd)},
		{"Caddyfile", 0o644, []byte(caddyfile)},
		{".dockerignore", 0o644, []byte(dockerIgnore)},
	}

	if project.Runtime.Worker.Enabled {
		writes = append(writes, struct {
			rel     string
			mode    fs.FileMode
			content []byte
		}{filepath.Join("worker", "Dockerfile"), 0o644, []byte(workerDockerfile)})

		workerMain := `import time
import os

print("BFFX Python Worker starting...")
print(f"REDIS_URL: {os.getenv('REDIS_URL', 'not set')}")

while True:
    time.sleep(10)
`
		if _, err := os.Stat(filepath.Join(root, "worker", "main.py")); os.IsNotExist(err) {
			writes = append(writes, struct {
				rel     string
				mode    fs.FileMode
				content []byte
			}{filepath.Join("worker", "main.py"), 0o644, []byte(workerMain)})
		}
	}
	if o.Kamal {
		kamalDeploy := fmt.Sprintf(`# config/deploy.yml
# Kamal deployment configuration for BFFX
# For docs, see: https://kamal-deploy.org/

service: %s
image: ghcr.io/user/%s

servers:
  web:
    - 192.168.1.1 # Replace with your actual VPS IP address

registry:
  username: your-github-username
  password:
    - KAMAL_REGISTRY_PASSWORD

port: 8080

env:
  clear:
    BFFX_ENV: production
    PORT: 8080
  secret:
    - BFFX_JWT_SECRET
    - BFFX_ADMIN_SESSION_KEY
    - BFFX_APP_SECRET
`, reg.Project.Metadata.Name, reg.Project.Metadata.Name)

		if project.Store.Mode == "sqlite" {
			kamalDeploy += `
volumes:
  - bffx_data:/app/.bffx/data
`
		}

		kamalDeploy += `
builder:
  arch: amd64
`
		writes = append(writes, struct {
			rel     string
			mode    fs.FileMode
			content []byte
		}{filepath.Join("config", "deploy.yml"), 0o644, []byte(kamalDeploy)})
	}

	for _, w := range writes {
		if err := writeGeneratedFile(root, w.rel, w.mode, w.content, o.DryRun); err != nil {
			return err
		}
	}

	setupScript := `#!/bin/bash
set -e

echo "📦 Installing Docker..."
apt-get update -qq
apt-get install -y ca-certificates curl gnupg lsb-release
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /usr/share/keyrings/docker.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker.gpg] \
  https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" > /etc/apt/sources.list.d/docker.list
apt-get update -qq
apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

echo "📁 Creating app directory..."
mkdir -p /var/www/bffx-app

echo "✅ Droplet ready! Configure GitHub Secrets and run: bffx deploy ship"
`
	scriptsDir := filepath.Join(root, "scripts")
	if !o.DryRun {
		if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
			return err
		}
	} else {
		logger.Info("[dry-run] would mkdir %s", scriptsDir)
	}
	if err := writeGeneratedFile(root, filepath.Join("scripts", "setup-droplet.sh"), 0o755, []byte(setupScript), o.DryRun); err != nil {
		return err
	}

	if o.DryRun {
		logger.Info("[dry-run] deploy init: no files were written")
	} else {
		logger.Info("Generated Dockerfile, docker-compose variants, .dockerignore, and scripts/setup-droplet.sh")
	}
	return nil
}

func writeGeneratedFile(root, rel string, mode fs.FileMode, content []byte, dryRun bool) error {
	path := filepath.Join(root, rel)
	if dryRun {
		logger.Info("[dry-run] would write %s (%d bytes, mode %o)", path, len(content), mode)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, mode)
}

func loadEnv(root string) error {
	path := filepath.Join(root, ".env")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
	return scanner.Err()
}

type DeployEntry struct {
	SHA  string    `json:"sha"`
	Tag  string    `json:"tag"`
	Time time.Time `json:"time"`
}

type DeployHistory struct {
	Deploys []DeployEntry `json:"deploys"`
}

func loadDeployHistory(root string) (*DeployHistory, error) {
	historyPath := filepath.Join(root, ".bffx", "deploy-history.json")
	data, err := os.ReadFile(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &DeployHistory{Deploys: []DeployEntry{}}, nil
		}
		return nil, err
	}
	var history DeployHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}
	return &history, nil
}

func saveDeployHistory(root string, history *DeployHistory) error {
	historyDir := filepath.Join(root, ".bffx")
	if err := os.MkdirAll(historyDir, 0o755); err != nil {
		return err
	}
	historyPath := filepath.Join(historyDir, "deploy-history.json")
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(historyPath, data, 0o644)
}

func recordDeploy(root string, sha string, tag string) error {
	history, err := loadDeployHistory(root)
	if err != nil {
		return err
	}

	newEntry := DeployEntry{
		SHA:  sha,
		Tag:  tag,
		Time: time.Now(),
	}
	history.Deploys = append(history.Deploys, newEntry)

	if len(history.Deploys) > 10 {
		history.Deploys = history.Deploys[len(history.Deploys)-10:]
	}

	return saveDeployHistory(root, history)
}

func resolveRollbackTarget(root string) (*DeployEntry, error) {
	history, err := loadDeployHistory(root)
	if err != nil {
		return nil, err
	}

	n := len(history.Deploys)
	if n < 2 {
		return nil, fmt.Errorf("no previous deploy found in history for rollback (need at least 2 entries, got %d)", n)
	}

	rollbackTarget := history.Deploys[n-2]
	history.Deploys = history.Deploys[:n-1]
	if err := saveDeployHistory(root, history); err != nil {
		return nil, err
	}

	return &rollbackTarget, nil
}

func Ship(root string) error {
	if err := loadEnv(root); err != nil {
		return fmt.Errorf("failed to load local .env: %w", err)
	}

	host := os.Getenv("BFFX_DEPLOY_HOST")
	user := os.Getenv("BFFX_DEPLOY_USER")
	path := os.Getenv("BFFX_DEPLOY_PATH")
	image := os.Getenv("BFFX_DEPLOY_IMAGE")

	if host == "" || user == "" || path == "" || image == "" {
		return fmt.Errorf("missing deployment config in .env (BFFX_DEPLOY_HOST, BFFX_DEPLOY_USER, BFFX_DEPLOY_PATH, BFFX_DEPLOY_IMAGE)")
	}

	sha := getGitSHA(root)
	tag := fmt.Sprintf("%s:%s", image, sha)
	latest := fmt.Sprintf("%s:latest", image)

	logger.Info("🚀 Building image %s...", tag)

	// 1. Build image locally with BuildKit
	buildCmd := exec.Command("docker", "build", "-t", tag, "-t", latest, ".")
	buildCmd.Env = append(os.Environ(), "DOCKER_BUILDKIT=1")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}

	// 2. Push image to registry
	logger.Info("📤 Pushing images to registry...")
	for _, t := range []string{tag, latest} {
		pushCmd := exec.Command("docker", "push", t)
		pushCmd.Stdout = os.Stdout
		pushCmd.Stderr = os.Stderr
		if err := pushCmd.Run(); err != nil {
			return fmt.Errorf("docker push %s failed: %w", t, err)
		}
	}

	// 3. Update remote server
	logger.Info("📡 Updating remote server %s...", host)

	// RSYNC compose, caddy, and env files
	rsyncArgs := []string{"-avz", "--exclude", ".git", filepath.Join(root, "docker-compose.yml"), filepath.Join(root, "docker-compose.prod.yml"), filepath.Join(root, ".env")}
	if _, err := os.Stat(filepath.Join(root, "Caddyfile")); err == nil {
		rsyncArgs = append(rsyncArgs, filepath.Join(root, "Caddyfile"))
	}
	rsyncArgs = append(rsyncArgs, fmt.Sprintf("%s@%s:%s", user, host, path))
	rsyncCmd := exec.Command("rsync", rsyncArgs...)
	if err := rsyncCmd.Run(); err != nil {
		return fmt.Errorf("rsync failed: %w", err)
	}

	// Update local deploy history
	if err := recordDeploy(root, sha, tag); err != nil {
		return fmt.Errorf("failed to record deploy history: %w", err)
	}

	// Create remote .bffx directory
	mkdirCmd := exec.Command("ssh", fmt.Sprintf("%s@%s", user, host), fmt.Sprintf("mkdir -p %s/.bffx", path))
	if err := mkdirCmd.Run(); err != nil {
		return fmt.Errorf("failed to create remote .bffx directory: %w", err)
	}

	// Copy deploy-history.json to remote
	scpCmd := exec.Command("scp", filepath.Join(root, ".bffx", "deploy-history.json"), fmt.Sprintf("%s@%s:%s/.bffx/deploy-history.json", user, host, path))
	if err := scpCmd.Run(); err != nil {
		return fmt.Errorf("failed to copy deploy history to remote: %w", err)
	}

	// Set BFFX_DEPLOY_TAG inside remote .env
	updateEnvCmd := fmt.Sprintf("cd %s && touch .env && grep -q '^BFFX_DEPLOY_TAG=' .env && sed -i 's/^BFFX_DEPLOY_TAG=.*/BFFX_DEPLOY_TAG=%s/' .env || echo 'BFFX_DEPLOY_TAG=%s' >> .env", path, sha, sha)
	sshEnvCmd := exec.Command("ssh", fmt.Sprintf("%s@%s", user, host), updateEnvCmd)
	if err := sshEnvCmd.Run(); err != nil {
		return fmt.Errorf("failed to update remote .env with BFFX_DEPLOY_TAG: %w", err)
	}

	// Remote Pull and Up
	deployCmd := fmt.Sprintf("cd %s && docker compose -f docker-compose.yml -f docker-compose.prod.yml pull && docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d", path)
	sshCmd := exec.Command("ssh", fmt.Sprintf("%s@%s", user, host), deployCmd)
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr
	if err := sshCmd.Run(); err != nil {
		return fmt.Errorf("remote deploy failed: %w", err)
	}

	// Save last deploy tag for backwards compatibility
	_ = os.WriteFile(filepath.Join(root, ".bffx", "last_deploy.txt"), []byte(tag), 0o644)

	logger.Info("✅ Shipment successful! Your app is live at http://%s", host)
	return nil
}

func Rollback(root string) error {
	if err := loadEnv(root); err != nil {
		return fmt.Errorf("failed to load local .env: %w", err)
	}

	host := os.Getenv("BFFX_DEPLOY_HOST")
	user := os.Getenv("BFFX_DEPLOY_USER")
	path := os.Getenv("BFFX_DEPLOY_PATH")

	if host == "" || user == "" || path == "" {
		return fmt.Errorf("missing deployment config in .env")
	}

	// Pull remote deploy-history.json to local machine first to ensure consistency
	scpCmd := exec.Command("scp", fmt.Sprintf("%s@%s:%s/.bffx/deploy-history.json", user, host, path), filepath.Join(root, ".bffx", "deploy-history.json"))
	_ = scpCmd.Run() // Fallback to local history if remote is missing

	rollbackTarget, err := resolveRollbackTarget(root)
	if err != nil {
		return err
	}

	logger.Info("⏪ Rolling back to SHA: %s (tag: %s)...", rollbackTarget.SHA, rollbackTarget.Tag)

	// Copy updated deploy-history.json back to remote
	scpPushCmd := exec.Command("scp", filepath.Join(root, ".bffx", "deploy-history.json"), fmt.Sprintf("%s@%s:%s/.bffx/deploy-history.json", user, host, path))
	if err := scpPushCmd.Run(); err != nil {
		return fmt.Errorf("failed to copy updated deploy history to remote: %w", err)
	}

	// Update BFFX_DEPLOY_TAG in remote .env to rollbackTarget.SHA
	updateEnvCmd := fmt.Sprintf("cd %s && touch .env && grep -q '^BFFX_DEPLOY_TAG=' .env && sed -i 's/^BFFX_DEPLOY_TAG=.*/BFFX_DEPLOY_TAG=%s/' .env || echo 'BFFX_DEPLOY_TAG=%s' >> .env", path, rollbackTarget.SHA, rollbackTarget.SHA)
	sshEnvCmd := exec.Command("ssh", fmt.Sprintf("%s@%s", user, host), updateEnvCmd)
	if err := sshEnvCmd.Run(); err != nil {
		return fmt.Errorf("failed to update remote .env with rollback target: %w", err)
	}

	deployCmd := fmt.Sprintf("cd %s && docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d", path)
	sshCmd := exec.Command("ssh", fmt.Sprintf("%s@%s", user, host), deployCmd)
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr
	if err := sshCmd.Run(); err != nil {
		return fmt.Errorf("remote rollback failed: %w", err)
	}

	logger.Info("✅ Rollback successful! App is now running SHA: %s", rollbackTarget.SHA)
	return nil
}

func getGitSHA(root string) string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "latest"
	}
	return strings.TrimSpace(string(out))
}

func Cloud(root, provider string) error {

	var config string
	var filename string

	switch strings.ToLower(provider) {
	case "fly", "--fly":
		filename = "fly.toml"
		config = `app = "bffx-app"
primary_region = "iad"

[build]
  dockerfile = "Dockerfile"

[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = true
  auto_start_machines = true
  min_machines_running = 1
`
	case "railway", "--railway":
		filename = "railway.json"
		config = `{
  "$schema": "https://railway.app/railway.schema.json",
  "build": {
    "builder": "DOCKERFILE"
  },
  "deploy": {
    "numReplicas": 1,
    "restartPolicyType": "ON_FAILURE"
  }
}`
	case "gcp", "--gcp":
		filename = "app.yaml"
		config = `runtime: custom
env: flex
`
	default:
		return fmt.Errorf("unknown provider %s; valid options: fly, railway, gcp", provider)
	}

	deployInstructions := `# Deployment Checklist

## Security Context
> [!WARNING]
> DO NOT bake secrets into the container image.

1. **Environment Variables**:
   Set ` + "`BFFX_JWT_SECRET`" + ` and ` + "`BFFX_WORKER_SECRET`" + ` in your cloud provider's dashboard.
2. **Database**:
   If using SQLite, ensure your platform supports persistent volumes.
3. **Queue**:
   If using jobs, provision a managed Redis instance and set ` + "`REDIS_URL`" + `.
`

	if err := os.WriteFile(filepath.Join(root, filename), []byte(config), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "DEPLOY.md"), []byte(deployInstructions), 0o644); err != nil {
		return err
	}

	logger.Info("Generated %s and DEPLOY.md", filename)
	return nil
}

func hasPkgDir(dir string) bool {
	if dir == "" {
		return false
	}
	st, err := os.Stat(filepath.Join(dir, "pkg"))
	return err == nil && st.IsDir()
}

// DetectWorkspaceRoot returns the Bffx framework checkout (a directory containing pkg/),
// or empty if none was found. Honors BFFX_ROOT, then .bffx/framework_root inside projectDir,
// then walks upward from projectDir (so commands work from a generated project root), then
// from the current working directory.
func DetectWorkspaceRoot(projectDir string) string {
	if w := os.Getenv("BFFX_ROOT"); w != "" {
		if hasPkgDir(w) {
			return filepath.Clean(w)
		}
		logger.Warn("BFFX_ROOT is set but %q does not contain pkg/", w)
	}

	if projectDir != "" {
		rootFile := filepath.Join(projectDir, ".bffx", "framework_root")
		if b, err := os.ReadFile(rootFile); err == nil {
			line := strings.TrimSpace(strings.Split(string(b), "\n")[0])
			if line != "" && hasPkgDir(line) {
				return filepath.Clean(line)
			}
		}

		for curr := filepath.Clean(projectDir); curr != "" && curr != "."; {
			if hasPkgDir(curr) {
				return curr
			}
			parent := filepath.Dir(curr)
			if parent == curr {
				break
			}
			curr = parent
		}
	}

	if wd, err := os.Getwd(); err == nil {
		if hasPkgDir(wd) {
			return filepath.Clean(wd)
		}
		curr := wd
		for i := 0; i < 16; i++ {
			if hasPkgDir(curr) {
				return filepath.Clean(curr)
			}
			parent := filepath.Dir(curr)
			if parent == curr {
				break
			}
			curr = parent
		}
	}
	return ""
}

func Status(root string) {
	fmt.Println("--- Deployment Status ---")
	if _, err := os.Stat(filepath.Join(root, "Dockerfile")); err == nil {
		fmt.Println("Dockerfile: ✅ Present")
	} else {
		fmt.Println("Dockerfile: ❌ Missing (Run: bffx deploy init)")
	}

	if _, err := os.Stat(filepath.Join(root, "docker-compose.yml")); err == nil {
		fmt.Println("docker-compose.yml: ✅ Present")
	} else {
		fmt.Println("docker-compose.yml: ❌ Missing")
	}

	hasCloud := false
	for _, f := range []string{"fly.toml", "railway.json", "app.yaml"} {
		if _, err := os.Stat(filepath.Join(root, f)); err == nil {
			fmt.Printf("Cloud Target: ✅ %s\n", f)
			hasCloud = true
			break
		}
	}
	if !hasCloud {
		fmt.Println("Cloud Target: ❌ Missing (Run: bffx deploy cloud [provider])")
	}
}

// VendorOptions controls Vendor (nil = defaults).
type VendorOptions struct {
	DryRun bool
}

// StampFrameworkMetadata records which framework snapshot and source tree were applied.
func StampFrameworkMetadata(projectRoot, workspaceRoot string) error {
	bffxMeta := filepath.Join(projectRoot, ".bffx")
	if err := os.MkdirAll(bffxMeta, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(bffxMeta, "framework_version"), []byte(strings.TrimSpace(version.FrameworkVersion)+"\n"), 0o644); err != nil {
		return err
	}
	abs, err := filepath.Abs(workspaceRoot)
	if err != nil {
		abs = workspaceRoot
	}
	if err := os.WriteFile(filepath.Join(bffxMeta, "framework_root"), []byte(strings.TrimSpace(abs)+"\n"), 0o644); err != nil {
		return err
	}
	return nil
}

func Vendor(root string, workspaceRoot string, opts *VendorOptions) error {
	o := VendorOptions{}
	if opts != nil {
		o = *opts
	}

	vendorDir := filepath.Join(root, ".bffx", "core")
	srcPkg := filepath.Join(workspaceRoot, "pkg")

	profile := loadVendorProfile(root)
	pkgPaths := buildprofile.ResolvePackages(profile.Mode, profile.Capabilities)

	if o.DryRun {
		logger.Info("[dry-run] would mkdir %s", vendorDir)
		if len(pkgPaths) == 0 {
			logger.Info("[dry-run] would copy tree %s -> %s/pkg (full mode)", srcPkg, vendorDir)
		} else {
			logger.Info("[dry-run] would copy %d selective pkg paths (mode=%s)", len(pkgPaths), profile.Mode)
			for _, p := range pkgPaths {
				logger.Info("[dry-run]   %s", p)
			}
		}
		logger.Info("[dry-run] would copy go.mod/go.sum from %s into %s", workspaceRoot, vendorDir)
		logger.Info("[dry-run] would ensure replace bffx => ./.bffx/core in project go.mod")
		logger.Info("[dry-run] would run go mod tidy in %s", root)
		logger.Info("[dry-run] would write .bffx/framework_version and .bffx/framework_root")
		return nil
	}

	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		return err
	}

	if err := EnsureAdminUIDist(workspaceRoot, profile); err != nil {
		return err
	}

	dstPkg := filepath.Join(vendorDir, "pkg")
	if err := os.RemoveAll(dstPkg); err != nil {
		return err
	}
	if err := os.MkdirAll(dstPkg, 0o755); err != nil {
		return err
	}

	if len(pkgPaths) == 0 {
		if err := copyDir(srcPkg, dstPkg); err != nil {
			return err
		}
	} else {
		for _, rel := range pkgPaths {
			src := filepath.Join(workspaceRoot, rel)
			relUnderPkg := strings.TrimPrefix(rel, "pkg/")
			dst := filepath.Join(dstPkg, relUnderPkg)
			if err := copyDir(src, dst); err != nil {
				return fmt.Errorf("vendor %s: %w", rel, err)
			}
		}
		logger.Info("Minimal vendoring: copied %d pkg paths (excluded %d tooling packages)", len(pkgPaths), len(profile.ExcludedTooling))
	}

	if err := VerifyVendoredAdminDist(vendorDir, profile); err != nil {
		return err
	}

	for _, f := range []string{"go.mod", "go.sum"} {
		src := filepath.Join(workspaceRoot, f)
		dst := filepath.Join(vendorDir, f)
		if _, err := os.Stat(src); err == nil {
			if b, err := os.ReadFile(src); err == nil {
				if err := os.WriteFile(dst, b, 0o644); err != nil {
					return err
				}
			}
		}
	}

	projectMod := filepath.Join(root, "go.mod")
	if b, err := os.ReadFile(projectMod); err == nil {
		content := string(b)
		if !strings.Contains(content, "replace github.com/hangry-coder/bffx") {
			content += "\nrequire github.com/hangry-coder/bffx v0.0.0\n"
			content += "replace github.com/hangry-coder/bffx => ./.bffx/core\n"
			if err := os.WriteFile(projectMod, []byte(content), 0o644); err != nil {
				return err
			}
		}
	}

	if err := StampFrameworkMetadata(root, workspaceRoot); err != nil {
		return err
	}

	if err := docsbundle.Bundle(root, workspaceRoot); err != nil {
		logger.Warn("Bundling admin guide docs failed: %v", err)
	} else {
		logger.Info("Bundled BFFX guide docs to .bffx/docs/")
	}

	logger.Info("Project vendored successfully to .bffx/core")

	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		logger.Warn("Failed to run go mod tidy: %v\n%s", err, out)
	}

	return nil
}

func loadVendorProfile(root string) buildprofile.Profile {
	if prof, err := buildprofile.Load(root); err == nil && prof != nil {
		return *prof
	}
	reg, err := manifest.LoadAll(root)
	if err != nil || reg == nil {
		return buildprofile.Profile{Mode: buildprofile.ModeFull}
	}
	derived, err := buildprofile.Derive(reg, buildprofile.DeriveOptions{})
	if err != nil || derived == nil {
		return buildprofile.Profile{Mode: buildprofile.ModeFull}
	}
	return *derived
}

// copyDir copies a directory tree without vendor excludes (tests / golden fixtures).
func copyDir(src, dst string) error {
	return copyDirFiltered(src, dst)
}
