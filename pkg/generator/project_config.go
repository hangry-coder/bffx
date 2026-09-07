package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func generateConfigs(projectDir, name string, opts ProjectOptions) error {
	streamingEnabled := "true"
	if !opts.StreamingEnabled {
		streamingEnabled = "false"
	}

	// project.yaml
	projectLines := []string{
		"apiVersion: bffx.io/v1alpha1",
		"kind: Project",
		"metadata:",
		"  name: " + name,
		"spec:",
		"  runtime:",
		"    api:",
		"      language: go",
		"      port: 8080",
		"    worker:",
		"      language: python",
		"      enabled: true",
		"    redis:",
		"      enabled: true",
		"    streaming:",
		"      enabled: " + streamingEnabled,
	}
	projectLines = append(projectLines, storeSpecYAMLLines(opts.StoreMode)...)

	if opts.WithTelemetry {
		projectLines = append(projectLines,
			"  telemetryStore:",
			"    mode: postgres",
			"    url: postgres://bffx:bffx@127.0.0.1:5432/bffx_telemetry?sslmode=disable",
		)
	}

	projectLines = append(projectLines,
		"  defaults:",
		"    auth: builtin",
		"    files: true",
		"    jobs: true",
		"    builders: true",
		"  app:",
		"    authStrategy: "+func() string {
			if opts.AuthStrategy == "" {
				return "optional"
			}
			return opts.AuthStrategy
		}(),
		"    namespace: com.example."+name,
		"    apiPrefix: /api/v1",
		"    permissions:",
		"      - { type: notifications, required: true, justification: \"Required to send you daily goal reminders and security alerts.\" }",
		"      - { type: location, required: false, justification: \"Used to show you nearby fitness centers.\" }",
		"    ui:",
		"      welcome_title: \"Welcome to "+name+"!\"",
		"      primary_action: \"Get Started\"",
		"    menu:",
		"      - { label: \"Home\", icon: \"home\", path: \"/home\" }",
		"      - { label: \"Settings\", icon: \"settings\", path: \"/settings\" }",
		"  admin:",
		"    enabled: " + func() string {
			if opts.AdminEnabled {
				return "true"
			}
			return "false"
		}(),
		"  layout: "+string(opts.Layout),
	)

	if opts.Minimal {
		projectLines = append(projectLines, "  packaging:", "    mode: minimal")
	}

	if opts.Batteries != nil {
		projectLines = append(projectLines, "  batteries:")
		if opts.Batteries.Auth != "" {
			projectLines = append(projectLines, "    auth: "+opts.Batteries.Auth)
		}
		if opts.Batteries.Store != "" {
			projectLines = append(projectLines, "    store: "+opts.Batteries.Store)
		}
		if opts.Batteries.Cache != "" {
			projectLines = append(projectLines, "    cache: "+opts.Batteries.Cache)
		}
		if opts.Batteries.Blob != "" {
			projectLines = append(projectLines, "    blob: "+opts.Batteries.Blob)
		}
		if opts.Batteries.Analytics != "" {
			projectLines = append(projectLines, "    analytics: "+opts.Batteries.Analytics)
		}
		if opts.Batteries.Observability != "" {
			projectLines = append(projectLines, "    observability: "+opts.Batteries.Observability)
		}
		if opts.Batteries.Flags != "" {
			projectLines = append(projectLines, "    flags: "+opts.Batteries.Flags)
		}
		if opts.Batteries.Vlm != "" {
			projectLines = append(projectLines, "    vlm: "+opts.Batteries.Vlm)
		}
	}

	projectYaml := strings.Join(projectLines, "\n")
	projectYamlPath := "bffx/project.yaml"
	if opts.Layout == LayoutV2 {
		// In V2, we still use bffx/project.yaml as the core manifest for now
		// but we ensure the directory exists.
		os.MkdirAll(filepath.Join(projectDir, "bffx"), 0o755)
	}

	if err := os.WriteFile(filepath.Join(projectDir, projectYamlPath), []byte(projectYaml), 0o644); err != nil {
		return err
	}

	// .env
	envLines := defaultEnvLinesForStore(opts.StoreMode)
	for k, v := range opts.Env {
		envLines = append(envLines, fmt.Sprintf("%s=%s", k, v))
	}
	env := strings.Join(envLines, "\n")
	if err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte(env), 0o644); err != nil {
		return err
	}

	if len(opts.Env) > 0 {
		gitignore := strings.Join([]string{
			".env",
			".bffx/data/app.db",
			"node_modules/",
		}, "\n")
		if err := os.WriteFile(filepath.Join(projectDir, ".gitignore"), []byte(gitignore), 0o644); err != nil {
			return err
		}
	}

	if opts.Layout == LayoutV2 {
		devConfig := strings.Join([]string{
			"port: 8080",
			"logLevel: debug",
			"redis:",
			"  url: redis://localhost:6379",
			"env:",
			"  BFFX_ENV: development",
		}, "\n")
		prodConfig := strings.Join([]string{
			"port: 80",
			"logLevel: info",
			"redis:",
			"  url: redis://redis:6379",
			"env:",
			"  BFFX_ENV: production",
		}, "\n")

		if err := os.MkdirAll(filepath.Join(projectDir, "config"), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(projectDir, "config", "development.yaml"), []byte(devConfig), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(projectDir, "config", "production.yaml"), []byte(prodConfig), 0o644); err != nil {
			return err
		}
	}

	return nil
}

func generateDockerfiles(projectDir, name string, opts ProjectOptions) error {
	cmdPath := "./cmd/orchestrator/"
	if opts.Layout == LayoutV2 {
		cmdPath = "./cmd/api/"
	}

	// Dockerfile.api
	apiDockerfileLines := []string{
		"FROM golang:1.26-alpine AS builder",
		"WORKDIR /app",
		"ENV GOTOOLCHAIN=auto",
		"ENV CGO_ENABLED=0",
		"COPY . .",
		"RUN go mod tidy && go build -o bffx-server " + cmdPath,
		"",
		"FROM alpine:latest",
		"WORKDIR /app",
		"COPY --from=builder /app/bffx-server .",
		"COPY --from=builder /app/bffx ./bffx",
		"COPY --from=builder /app/.bffx ./.bffx",
	}

	if opts.Layout == LayoutV2 {
		apiDockerfileLines = append(apiDockerfileLines,
			"COPY --from=builder /app/assets ./assets",
			"COPY --from=builder /app/config ./config",
			"COPY --from=builder /app/db/migrations ./db/migrations",
		)
	} else {
		apiDockerfileLines = append(apiDockerfileLines,
			"COPY --from=builder /app/i18n ./i18n",
		)
	}

	apiDockerfileLines = append(apiDockerfileLines,
		"EXPOSE 8080",
		"CMD [\"./bffx-server\"]",
	)

	apiDockerfile := strings.Join(apiDockerfileLines, "\n")

	if err := os.WriteFile(filepath.Join(projectDir, "Dockerfile.api"), []byte(apiDockerfile), 0o644); err != nil {
		return err
	}

	// Dockerfile.worker
	workerDockerfile := strings.Join([]string{
		"FROM python:3.11-slim",
		"WORKDIR /app",
		"COPY requirements.txt* ./",
		"RUN if [ -f requirements.txt ]; then pip install -r requirements.txt; fi",
		"COPY . .",
		"CMD [\"python\", \"worker/main.py\"]",
	}, "\n")
	if err := os.WriteFile(filepath.Join(projectDir, "Dockerfile.worker"), []byte(workerDockerfile), 0o644); err != nil {
		return err
	}

	return nil
}
