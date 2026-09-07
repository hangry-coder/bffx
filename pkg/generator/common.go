package generator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"gopkg.in/yaml.v3"
)

func GenerateBuilder(root, name, group string, layout LayoutType) error {
	if name == "" {
		return errors.New("builder name is required")
	}

	paths := GetPaths(root, group, layout)
	if err := os.MkdirAll(paths.Builders, 0o755); err != nil {
		return fmt.Errorf("create builders directory: %w", err)
	}

	filePath := filepath.Join(paths.Builders, strings.ToLower(name)+".yaml")
	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("builder manifest already exists: %s\nHint: Use 'bffx upgrade' to modify existing builders.", filePath)
	}

	manifest := strings.Join([]string{
		"apiVersion: bffx.io/v1alpha1",
		"kind: Builder",
		"metadata:",
		"  name: " + name,
		"spec:",
		"  route:",
		"    method: GET",
		"    path: /api/v1/" + strings.ToLower(name),
		"    auth: required",
		"  sources:",
		"    - app",
		"    - currentUser",
		"  output:",
		"    app.name: project.name",
		"    auth.user: currentUser",
		"",
	}, "\n")

	if err := os.WriteFile(filePath, []byte(manifest), 0o644); err != nil {
		return fmt.Errorf("write builder manifest: %w", err)
	}
	return nil
}

func GenerateStream(root, name, group string, layout LayoutType) error {
	if name == "" {
		return errors.New("stream name is required")
	}

	paths := GetPaths(root, group, layout)
	if err := os.MkdirAll(paths.Manifests, 0o755); err != nil {
		return fmt.Errorf("create manifests directory: %w", err)
	}

	filePath := filepath.Join(paths.Manifests, strings.ToLower(name)+".yaml")
	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("stream manifest already exists: %s", filePath)
	}

	manifest := strings.Join([]string{
		"apiVersion: bffx.io/v1alpha1",
		"kind: Stream",
		"metadata:",
		"  name: " + name,
		"spec:",
		"  route:",
		"    path: /api/v1/streams/" + strings.ToLower(name),
		"    auth: required",
		"  channels:",
		"    - bffx:events:*", // Default to all events or a specific one
		"",
	}, "\n")

	return os.WriteFile(filePath, []byte(manifest), 0o644)
}

func GenerateTemplate(root, name, group string, layout LayoutType) error {
	if name == "" {
		return errors.New("template name is required")
	}

	paths := GetPaths(root, group, layout)
	if err := os.MkdirAll(paths.Templates, 0o755); err != nil {
		return fmt.Errorf("create templates directory: %w", err)
	}

	filePath := filepath.Join(paths.Templates, strings.ToLower(name)+".yaml")
	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("template manifest already exists: %s", filePath)
	}

	manifest := strings.Join([]string{
		"apiVersion: bffx.io/v1alpha1",
		"kind: Template",
		"metadata:",
		"  name: " + name,
		"spec:",
		"  subject: \"Hello {{.Name}}!\"",
		"  body: |",
		"    Hi {{.Name}},",
		"    ",
		"    This is a generated template for " + name + ".",
		"",
	}, "\n")

	return os.WriteFile(filePath, []byte(manifest), 0o644)
}

func GenerateCronJob(root, name, group string, layout LayoutType) error {
	if name == "" {
		return errors.New("cronjob name is required")
	}

	paths := GetPaths(root, group, layout)
	if err := os.MkdirAll(paths.CronJobs, 0o755); err != nil {
		return fmt.Errorf("create cronjobs directory: %w", err)
	}

	filePath := filepath.Join(paths.CronJobs, strings.ToLower(name)+".yaml")
	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("cronjob manifest already exists: %s", filePath)
	}

	manifest := strings.Join([]string{
		"apiVersion: bffx.io/v1alpha1",
		"kind: CronJob",
		"metadata:",
		"  name: " + name,
		"spec:",
		"  schedule: \"0 0 * * *\"", // Daily at midnight
		"  action: " + name + "Action",
		"  description: \"Automatically generated cronjob for " + name + "\"",
		"",
	}, "\n")

	return os.WriteFile(filePath, []byte(manifest), 0o644)
}

func GenerateAction(root, name, group string, layout LayoutType) error {
	if name == "" {
		return errors.New("action name is required")
	}

	paths := GetPaths(root, group, layout)
	if err := os.MkdirAll(paths.Actions, 0o755); err != nil {
		return fmt.Errorf("create actions directory: %w", err)
	}

	filePath := filepath.Join(paths.Actions, strings.ToLower(name)+".yaml")
	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("action manifest already exists: %s\nHint: Use 'bffx upgrade' to modify existing actions.", filePath)
	}

	manifest := strings.Join([]string{
		"apiVersion: bffx.io/v1alpha1",
		"kind: Action",
		"metadata:",
		"  name: " + name,
		"spec:",
		"  route:",
		"    method: POST",
		"    path: /api/v1/actions/" + strings.ToLower(name),
		"    auth: required",
		"",
	}, "\n")

	if err := os.WriteFile(filePath, []byte(manifest), 0o644); err != nil {
		return fmt.Errorf("write action manifest: %w", err)
	}

	// Create Go hook stub
	os.MkdirAll(paths.Hooks, 0o755)
	goStub := fmt.Sprintf("package hooks\n\nimport (\n\t\"github.com/hangry-coder/bffx/pkg/api/handlers\"\n\t\"net/http\"\n)\n\nfunc Handle%s(ctx *handlers.ActionContext, w http.ResponseWriter, r *http.Request) {\n    // Implement custom action logic here\n}\n", name)
	os.WriteFile(filepath.Join(paths.Hooks, strings.ToLower(name)+".go"), []byte(goStub), 0o644)

	if err := EnsureAdminManifestForAction(root, name, layout); err != nil {
		return fmt.Errorf("generate admin action manifest: %w", err)
	}

	return nil
}

func GeneratePipeline(root, name, group, pipelineType, catalogName string, layout LayoutType) error {
	if name == "" {
		return errors.New("pipeline name is required")
	}
	if pipelineType == "" {
		pipelineType = "ingestion"
	}

	paths := GetPaths(root, group, layout)
	if err := os.MkdirAll(paths.Manifests, 0o755); err != nil {
		return fmt.Errorf("create manifests directory: %w", err)
	}

	// 1. Write the YAML Manifest stub
	manifestPath := filepath.Join(paths.Manifests, strings.ToLower(name)+"_pipeline.yaml")
	if _, err := os.Stat(manifestPath); err == nil {
		return fmt.Errorf("pipeline manifest already exists: %s", manifestPath)
	}

	var specLines []string
	specLines = append(specLines,
		"apiVersion: bffx.io/v1alpha1",
		"kind: Pipeline",
		"metadata:",
		"  name: " + name,
		"spec:",
		"  type: " + pipelineType,
		"  route:",
		"    method: POST",
		"    path: /api/v1/pipelines/" + strings.ToLower(name),
		"    auth: required",
		"  model_routing:",
		"    - gemini",
	)
	if catalogName != "" {
		specLines = append(specLines,
			"  catalog:",
			"    adapter: " + catalogName,
		)
	}
	specLines = append(specLines,
		"  settings:",
		"    compression_max_width: 800",
		"    cache_bypass: false",
		"  hooks:",
		"    beforePipeline:",
		"      - action: Before" + name,
		"    afterPipeline:",
		"      - action: After" + name,
		"",
	)

	if err := os.WriteFile(manifestPath, []byte(strings.Join(specLines, "\n")), 0o644); err != nil {
		return fmt.Errorf("write pipeline manifest: %w", err)
	}

	// 2. Create the internal feature pipeline package
	feature := group
	if feature == "" {
		feature = "app"
	}

	pipelineDir := filepath.Join(root, "internal", "features", feature, "pipelines", strings.ToLower(name))
	if err := os.MkdirAll(pipelineDir, 0o755); err != nil {
		return fmt.Errorf("create pipeline source directory: %w", err)
	}

	// Create pipeline.go
	pipelineGo := fmt.Sprintf(`package %s

import (
	"context"
)

// Pipeline represents the core logic for the %s pipeline.
type Pipeline struct {
	// Add your states, configuration, or clients here
}

func NewPipeline() *Pipeline {
	return &Pipeline{}
}

func (p *Pipeline) Run(ctx context.Context, input []byte) (any, error) {
	// Implement custom pipeline step orchestration
	return map[string]any{
		"message": "Hello from %s pipeline",
	}, nil
}
`, strings.ToLower(name), name, name)

	if err := os.WriteFile(filepath.Join(pipelineDir, "pipeline.go"), []byte(pipelineGo), 0o644); err != nil {
		return fmt.Errorf("write pipeline.go: %w", err)
	}

	// Create system_prompt.txt
	systemPrompt := fmt.Sprintf("You are the AI engine powering the %s pipeline.\nAnalyze the input carefully and respond with strict JSON structures.\n", name)
	if err := os.WriteFile(filepath.Join(pipelineDir, "system_prompt.txt"), []byte(systemPrompt), 0o644); err != nil {
		return fmt.Errorf("write system_prompt.txt: %w", err)
	}

	// Create adapters.go
	var adaptersGo string
	if catalogName == "openfoodfacts" {
		adaptersGo = fmt.Sprintf(`package %s

import (
	"context"
	"fmt"

	"github.com/hangry-coder/bffx/pkg/addons/catalog/nutrition"
)

// NutritionCatalogAdapter implements the orchestrator.Catalog interface using openfoodfacts.
type NutritionCatalogAdapter struct {
	catalog *nutrition.AddonCatalog
}

func NewNutritionCatalogAdapter() *NutritionCatalogAdapter {
	return &NutritionCatalogAdapter{
		catalog: nutrition.NewAddonCatalog{},
	}
}

func (a *NutritionCatalogAdapter) Resolve(ctx context.Context, query string, hints map[string]string) (any, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}
	return a.catalog.Resolve(ctx, query, hints)
}

func (a *NutritionCatalogAdapter) Source() string {
	return "openfoodfacts"
}
`, strings.ToLower(name))
	} else {
		adaptersGo = fmt.Sprintf(`package %s

import (
	"context"
	"fmt"
)

// GenericCatalogAdapter implements the orchestrator.Catalog interface.
type GenericCatalogAdapter struct{}

func NewGenericCatalogAdapter() *GenericCatalogAdapter {
	return &GenericCatalogAdapter{}
}

func (a *GenericCatalogAdapter) Resolve(ctx context.Context, query string, hints map[string]string) (any, error) {
	if query == "" {
		return nil, fmt.Errorf("query is empty")
	}
	return map[string]any{
		"resolved": true,
		"query":    query,
		"source":   "%s",
	}, nil
}

func (a *GenericCatalogAdapter) Source() string {
	if "%s" != "" {
		return "%s"
	}
	return "generic"
}
`, strings.ToLower(name), catalogName, catalogName, catalogName)
	}

	if err := os.WriteFile(filepath.Join(pipelineDir, "adapters.go"), []byte(adaptersGo), 0o644); err != nil {
		return fmt.Errorf("write adapters.go: %w", err)
	}

	// 3. Create Hook points in hooks/
	if err := os.MkdirAll(paths.Hooks, 0o755); err != nil {
		return fmt.Errorf("create hooks directory: %w", err)
	}

	hookGo := fmt.Sprintf(`package hooks

import (
	"fmt"

	"github.com/hangry-coder/bffx/pkg/api/handlers"
)

// Before%s executes custom preprocessing rules before the %s pipeline triggers.
// @bffx:hook
func Before%s(ctx *handlers.ActionContext, payload map[string]any) error {
	fmt.Printf("beforePipeline executed for %s\n")
	return nil
}

// After%s executes custom postprocessing rules after the %s pipeline finishes.
// @bffx:hook
func After%s(ctx *handlers.ActionContext, payload map[string]any) error {
	fmt.Printf("afterPipeline executed for %s\n")
	return nil
}
`, name, name, name, name, name, name, name, name)

	hookPath := filepath.Join(paths.Hooks, strings.ToLower(name)+".go")
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		if err := os.WriteFile(hookPath, []byte(hookGo), 0o644); err != nil {
			return fmt.Errorf("write hook: %w", err)
		}
	}

	return nil
}

func GenerateService(root, name, group, serviceType string, layout LayoutType) error {
	if name == "" {
		return errors.New("service name is required")
	}
	if serviceType == "" {
		serviceType = "rest"
	}

	paths := GetPaths(root, group, layout)
	if err := os.MkdirAll(paths.Manifests, 0o755); err != nil {
		return fmt.Errorf("create manifests directory: %w", err)
	}

	filePath := filepath.Join(paths.Manifests, strings.ToLower(name)+".yaml")
	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("service manifest already exists: %s\nHint: Use 'bffx upgrade' to modify existing services.", filePath)
	}

	manifest := strings.Join([]string{
		"apiVersion: bffx.io/v1alpha1",
		"kind: Service",
		"metadata:",
		"  name: " + name,
		"spec:",
		"  type: " + serviceType,
		"  endpoint: https://api.example.com",
		"  timeout: 5s",
		"",
	}, "\n")

	if err := os.WriteFile(filePath, []byte(manifest), 0o644); err != nil {
		return fmt.Errorf("write service manifest: %w", err)
	}

	return nil
}

func GenerateAddon(root, name string, layout LayoutType) error {
	switch name {
	case "notifications":
		// Scaffold Device resource for notifications
		fields := []Field{
			{Name: "token", Type: "string"},
			{Name: "platform", Type: "string"},
			{Name: "providerKey", Type: "string"},
		}
		if err := GenerateResource(root, "Device", fields, ResourceOptions{Layout: layout, WithHooks: true, Group: "mobile"}); err != nil {
			return err
		}

		fmt.Println("Scaffolded Device resource for notifications addon")
	case "flags":
		// FeatureFlag: name, enabled, rules
		if err := GenerateResource(root, "FeatureFlag", []Field{
			{Name: "key", Type: "string"},
			{Name: "enabled", Type: "bool"},
			{Name: "rules", Type: "string"}, // JSON targeting
			{Name: "description", Type: "string"},
		}, ResourceOptions{Layout: layout, WithHooks: true, Group: "system"}); err != nil {
			return err
		}
		// Rollout: flag_id, percentage, release_id
		if err := GenerateResource(root, "Rollout", []Field{
			{Name: "flag_id", Type: "string"},
			{Name: "percentage", Type: "int"},
			{Name: "release_id", Type: "string"},
		}, ResourceOptions{Layout: layout, WithHooks: true, Group: "system"}); err != nil {
			return err
		}
		// Release: version, status
		if err := GenerateResource(root, "Release", []Field{
			{Name: "version", Type: "string"},
			{Name: "status", Type: "string"},
		}, ResourceOptions{Layout: layout, WithHooks: true, Group: "system"}); err != nil {
			return err
		}
		fmt.Println("Scaffolded Feature Flags, Rollouts, and Releases resources")
	case "subscriptions":
		// Subscription: user_id, plan_id, status, expires_at
		if err := GenerateResource(root, "Subscription", []Field{
			{Name: "user_id", Type: "string"},
			{Name: "plan_id", Type: "string"},
			{Name: "status", Type: "string"},
			{Name: "expires_at", Type: "string"},
		}, ResourceOptions{Layout: layout, WithHooks: true, Group: "system"}); err != nil {
			return err
		}
		// Entitlement: user_id, key, granted
		if err := GenerateResource(root, "Entitlement", []Field{
			{Name: "user_id", Type: "string"},
			{Name: "key", Type: "string"},
			{Name: "granted", Type: "bool"},
		}, ResourceOptions{Layout: layout, WithHooks: true, Group: "system"}); err != nil {
			return err
		}

		fmt.Println("Scaffolded Subscription and Entitlement resources")
	default:
		return fmt.Errorf("unknown addon: %s", name)
	}
	return nil
}

func GenerateConfig(root, key, value string) error {
	projectFile := filepath.Join(root, "bffx", "project.yaml")
	data, err := os.ReadFile(projectFile)
	if err != nil {
		return fmt.Errorf("failed to read project.yaml: %w", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to unmarshal project.yaml: %w", err)
	}


	// Traverse the keys (e.g. store.mode)
	parts := strings.Split(key, ".")
	curr := config
	for i, part := range parts {
		if i == len(parts)-1 {
			// Set value (coerce string "true"/"false" to bool if possible)
			if value == "true" {
				curr[part] = true
			} else if value == "false" {
				curr[part] = false
			} else {
				curr[part] = value
			}
			break
		}

		next, ok := curr[part].(map[string]interface{})
		if !ok {
			next = make(map[string]interface{})
			curr[part] = next
		}
		curr = next
	}

	out, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal project.yaml: %w", err)
	}

	if err := os.WriteFile(projectFile, out, 0o644); err != nil {
		return fmt.Errorf("failed to write project.yaml: %w", err)
	}

	fmt.Printf("updated %s to %s in project.yaml\n", key, value)
	return nil
}

func GenerateCICD(root string) error {
	workflowDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		return fmt.Errorf("create workflows directory: %w", err)
	}

	workflow := `name: Test & Deploy

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
  workflow_dispatch:
    inputs:
      action:
        description: 'Action to perform'
        required: true
        default: 'deploy'
        type: choice
        options:
          - deploy
          - rollback
      target_tag:
        description: 'Image tag for rollback'
        required: false
        type: string

env:
  IMAGE_NAME: ghcr.io/${{ github.repository_owner }}/bffx-app

jobs:
  test:
    name: Run Tests & Static Analysis
    runs-on: ubuntu-latest
    if: github.event.inputs.action != 'rollback'
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-go@v6
        with:
          go-version: '1.26'
          cache: true

      - name: Run Go Vet
        run: go vet ./...

      - name: Run golangci-lint
        uses: golangci-lint/golangci-lint-action@v6
        with:
          version: latest
          args: --timeout=5m

      - name: Run govulncheck (Vulnerability Scan)
        run: |
          go install golang.org/x/vuln/cmd/govulncheck@latest
          govulncheck ./...

      - name: Run Unit Tests
        run: go test -v ./...

      - name: Run E2E Tests
        run: go test -v ./tests/e2e/... -timeout 120s

  predeploy-gate:
    name: Pre-Deploy Production Gate
    needs: test
    runs-on: ubuntu-latest
    if: |
      github.ref == 'refs/heads/main' &&
      github.event.inputs.action != 'rollback' &&
      (github.event_name == 'push' || github.event_name == 'workflow_dispatch')
    steps:
      - uses: actions/checkout@v5

      - name: Validate deployment settings presence
        env:
          DEPLOY_HOST: ${{ secrets.DEPLOY_HOST }}
          DEPLOY_USER: ${{ secrets.DEPLOY_USER }}
          DEPLOY_PATH: ${{ secrets.DEPLOY_PATH }}
          DEPLOY_SSH_KEY: ${{ secrets.DEPLOY_SSH_KEY }}
        run: |
          set -euo pipefail
          required_vars=(DEPLOY_HOST DEPLOY_USER DEPLOY_PATH DEPLOY_SSH_KEY)
          for var in "${required_vars[@]}"; do
            if [ -z "${!var:-}" ]; then
              echo "::error::Missing deployment secret: $var"
              exit 1
            fi
          done

      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version: '1.26'
          cache: true

      - name: Align production secrets with bffx doctor
        env:
          DATABASE_URL: ${{ secrets.DATABASE_URL }}
          CLERK_SECRET_KEY: ${{ secrets.CLERK_SECRET_KEY }}
          CLERK_JWKS_URL: ${{ secrets.CLERK_JWKS_URL }}
          AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
          AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          AWS_REGION: ${{ secrets.AWS_REGION }}
          BFFX_S3_ENDPOINT: ${{ secrets.BFFX_S3_ENDPOINT }}
          BFFX_S3_BUCKET: ${{ secrets.BFFX_S3_BUCKET }}
          BFFX_POSTHOG_API_KEY: ${{ secrets.BFFX_POSTHOG_API_KEY }}
          BFFX_POSTHOG_HOST: ${{ secrets.BFFX_POSTHOG_HOST }}
          BFFX_SENTRY_DSN: ${{ secrets.BFFX_SENTRY_DSN }}
          BFFX_GEMINI_API_KEY: ${{ secrets.BFFX_GEMINI_API_KEY }}
          BFFX_REDIS_URL: ${{ secrets.BFFX_REDIS_URL }}
          BFFX_JWT_SECRET: ${{ secrets.BFFX_JWT_SECRET }}
          BFFX_WORKER_SECRET: ${{ secrets.BFFX_WORKER_SECRET }}
        run: |
          go run .bffx/core/cmd/bffx/main.go doctor --env production --offline

  deploy:
    name: Build & Deploy
    needs: [test, predeploy-gate]
    runs-on: ubuntu-latest
    if: |
      always() &&
      github.ref == 'refs/heads/main' &&
      (github.event.inputs.action == 'rollback' || 
       (needs.test.result == 'success' && needs.predeploy-gate.result == 'success'))
    permissions:
      contents: read
      packages: write

    steps:
      - uses: actions/checkout@v5

      - name: Set up Docker Buildx
        if: github.event.inputs.action != 'rollback'
        uses: docker/setup-buildx-action@v3

      - name: Log in to GitHub Container Registry
        if: github.event.inputs.action != 'rollback'
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and Push Docker Image
        if: github.event.inputs.action != 'rollback'
        uses: docker/build-push-action@v5
        with:
          context: .
          push: true
          tags: |
            ${{ env.IMAGE_NAME }}:latest
            ${{ env.IMAGE_NAME }}:${{ github.sha }}
          cache-from: type=gha
          cache-to: type=gha,mode=max

      - name: Deploy to Droplet
        uses: appleboy/ssh-action@v1.0.3
        env:
          DEPLOY_TAG: ${{ github.event.inputs.action == 'rollback' && github.event.inputs.target_tag || github.sha }}
        with:
          host: ${{ secrets.DEPLOY_HOST }}
          username: ${{ secrets.DEPLOY_USER }}
          key: ${{ secrets.DEPLOY_SSH_KEY }}
          envs: DEPLOY_TAG
          script: |
            cd ${{ secrets.DEPLOY_PATH }}
            echo "${{ secrets.GITHUB_TOKEN }}" | docker login ghcr.io -u ${{ github.actor }} --password-stdin
            BFFX_DEPLOY_TAG=$DEPLOY_TAG docker compose -f docker-compose.yml -f docker-compose.prod.yml pull
            BFFX_DEPLOY_TAG=$DEPLOY_TAG docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
            docker image prune -f

      - name: Post-Deploy Smoke Check
        run: |
          echo "Waiting for app to start..."
          sleep 15
          curl --fail --silent --show-error --connect-timeout 5 --max-time 15 http://${{ secrets.DEPLOY_HOST }}:8080/health || {
            echo "Smoke check failed: Application did not respond at http://${{ secrets.DEPLOY_HOST }}:8080/health"
            exit 1
          }
`

	return os.WriteFile(filepath.Join(workflowDir, "deploy.yml"), []byte(workflow), 0o644)
}
