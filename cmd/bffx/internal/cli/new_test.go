package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIListArchetypes(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	// Run HandleNew with list archetypes flag
	HandleNew([]string{"--list-archetypes"})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read captured stdout: %v", err)
	}
	output := buf.String()

	// Assertions
	if !strings.Contains(output, "Available BFFX Vertical Archetypes") {
		t.Errorf("expected header to be printed, got: %s", output)
	}
	if !strings.Contains(output, "fintech") {
		t.Errorf("expected fintech archetype to be listed, got: %s", output)
	}
	if !strings.Contains(output, "superapp") {
		t.Errorf("expected superapp archetype to be listed, got: %s", output)
	}
}

func TestCLIDualModeScaffolding(t *testing.T) {
	tmpDir := t.TempDir()

	// Run HandleNew using positional fallback mode: "fintech"
	// Should scaffold project "fintech" under tmpDir/fintech
	HandleNew([]string{
		"fintech",
		"--root", tmpDir,
		"--non-interactive",
	})

	projectDir := filepath.Join(tmpDir, "fintech")
	manifestPath := filepath.Join(projectDir, "bffx", "project.yaml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Fatalf("expected project.yaml to exist at %s", manifestPath)
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read project.yaml: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "name: fintech") {
		t.Errorf("expected project name to be 'fintech', got content: %s", content)
	}

	// Verify that database config exists (either postgres or sqlite depending on template)
	if !strings.Contains(content, "postgres") && !strings.Contains(content, "sqlite") {
		t.Errorf("expected database config to be generated, got content: %s", content)
	}

	// Verify that pipeline was scaffolded in v2 feature layout
	pipelinePath := filepath.Join(projectDir, "internal", "features", "finance", "manifests", "receiptscan_pipeline.yaml")
	if _, err := os.Stat(pipelinePath); os.IsNotExist(err) {
		t.Fatalf("expected post-hook pipeline to be scaffolded at %s", pipelinePath)
	}
}

func TestCLIPipelineOverride(t *testing.T) {
	tmpDir := t.TempDir()

	// Run HandleNew for superapp with a --pipeline=rag override
	HandleNew([]string{
		"superapp-rag",
		"--archetype", "superapp",
		"--pipeline", "rag",
		"--root", tmpDir,
		"--non-interactive",
	})

	projectDir := filepath.Join(tmpDir, "superapp-rag")
	pipelinePath := filepath.Join(projectDir, "internal", "features", "support", "manifests", "support_pipeline.yaml")
	if _, err := os.Stat(pipelinePath); os.IsNotExist(err) {
		t.Fatalf("expected pipeline to exist at %s", pipelinePath)
	}

	data, err := os.ReadFile(pipelinePath)
	if err != nil {
		t.Fatalf("failed to read pipeline.yaml: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "type: rag") {
		t.Errorf("expected pipeline type to be overridden to 'rag', got content: %s", content)
	}
}

func TestCLIConfigDrivenScaffolding(t *testing.T) {
	tmpDir := t.TempDir()

	configYaml := `
name: my-config-app
archetype: fintech
layout: v2
minimal: true

admin:
  enabled: true
  email: "admin@myconfigapp.com"
  password: "supersecretadminpass"

batteries:
  store: sqlite
  cache: memory

env:
  DATABASE_URL: "sqlite://app.db"
  MY_CUSTOM_SECRET: "top-secret-val"
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configYaml), 0o644); err != nil {
		t.Fatalf("failed to write config.yaml: %v", err)
	}

	// Run HandleNew with --config flag
	HandleNew([]string{
		"--config", configPath,
		"--root", tmpDir,
	})

	projectDir := filepath.Join(tmpDir, "my-config-app")
	
	// 1. Verify project exists
	manifestPath := filepath.Join(projectDir, "bffx", "project.yaml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Fatalf("expected project.yaml to exist at %s", manifestPath)
	}

	// 2. Verify .env file content
	envPath := filepath.Join(projectDir, ".env")
	envData, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("failed to read .env: %v", err)
	}
	envContent := string(envData)
	if !strings.Contains(envContent, "MY_CUSTOM_SECRET=top-secret-val") {
		t.Errorf("expected custom secret in .env, got: %s", envContent)
	}
	if !strings.Contains(envContent, "DATABASE_URL=sqlite://app.db") {
		t.Errorf("expected DATABASE_URL in .env, got: %s", envContent)
	}

	// 3. Verify .gitignore exists and contains .env
	gitIgnorePath := filepath.Join(projectDir, ".gitignore")
	gitIgnoreData, err := os.ReadFile(gitIgnorePath)
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}
	if !strings.Contains(string(gitIgnoreData), ".env") {
		t.Errorf("expected .gitignore to ignore .env, got: %s", string(gitIgnoreData))
	}
}
