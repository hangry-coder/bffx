package deploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeployInit_WritesExpectedFiles(t *testing.T) {
	src := filepath.Join("testdata", "minimal_project")
	tmp := t.TempDir()
	if err := copyDirTree(src, tmp); err != nil {
		t.Fatal(err)
	}
	if err := Init(tmp, "", &InitOptions{DryRun: false}); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"Dockerfile",
		"docker-compose.yml",
		".dockerignore",
		filepath.Join("worker", "Dockerfile"),
	} {
		p := filepath.Join(tmp, rel)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}
	df, err := os.ReadFile(filepath.Join(tmp, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if len(df) < 200 {
		t.Fatalf("Dockerfile unexpectedly short (%d bytes)", len(df))
	}
	s := string(df)
	if !strings.Contains(s, "FROM golang") || !strings.Contains(s, "CMD") {
		t.Fatal("Dockerfile missing expected markers")
	}
}

func TestDeployInit_OmitsWorkerWhenDisabled(t *testing.T) {
	src := filepath.Join("testdata", "minimal_project")
	tmp := t.TempDir()
	if err := copyDirTree(src, tmp); err != nil {
		t.Fatal(err)
	}

	// Override project.yaml to set worker.enabled: false
	projectYAML := `apiVersion: bffx.io/v1alpha1
kind: Project
metadata:
  name: DeployInitGolden
spec:
  runtime:
    api: { language: go, port: 8080 }
    worker: { language: python, enabled: false }
    redis: { enabled: false }
  store:
    mode: sqlite
    path: .bffx/app.db
  defaults:
    auth: builtin
    files: true
    jobs: false
    builders: true
  app:
    namespace: com.example.deployinit
    apiPrefix: /api/v1
`
	if err := os.WriteFile(filepath.Join(tmp, "bffx", "project.yaml"), []byte(projectYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Init(tmp, "", &InitOptions{DryRun: false}); err != nil {
		t.Fatal(err)
	}

	// 1. worker/Dockerfile should NOT exist
	p := filepath.Join(tmp, "worker", "Dockerfile")
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Errorf("worker/Dockerfile should not exist when worker is disabled")
	}

	// 2. worker/main.py should NOT exist
	pMain := filepath.Join(tmp, "worker", "main.py")
	if _, err := os.Stat(pMain); !os.IsNotExist(err) {
		t.Errorf("worker/main.py should not exist when worker is disabled")
	}

	// 3. docker-compose.yml should NOT contain "worker:" service
	composeBytes, err := os.ReadFile(filepath.Join(tmp, "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	composeStr := string(composeBytes)
	if strings.Contains(composeStr, "worker:") {
		t.Errorf("docker-compose.yml should not define worker service when disabled, got:\n%s", composeStr)
	}
}

func TestDeployInit_WritesKamalConfig(t *testing.T) {
	src := filepath.Join("testdata", "minimal_project")
	tmp := t.TempDir()
	if err := copyDirTree(src, tmp); err != nil {
		t.Fatal(err)
	}

	if err := Init(tmp, "", &InitOptions{Kamal: true}); err != nil {
		t.Fatal(err)
	}

	// config/deploy.yml should exist
	p := filepath.Join(tmp, "config", "deploy.yml")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("config/deploy.yml should be written when Kamal option is true: %v", err)
	}

	content, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)

	if !strings.Contains(s, "service: DeployInitGolden") {
		t.Errorf("expected service name DeployInitGolden in Kamal config, got:\n%s", s)
	}
	if !strings.Contains(s, "volumes:") || !strings.Contains(s, "bffx_data:/app/.bffx/data") {
		t.Errorf("expected sqlite volume config in Kamal config, got:\n%s", s)
	}
}

func copyDirTree(srcRoot, dstRoot string) error {
	return filepath.WalkDir(srcRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil || rel == "." {
			return err
		}
		dst := filepath.Join(dstRoot, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
}
