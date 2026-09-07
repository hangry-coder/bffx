package worker

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

func TestCronSchedulerLoadFromRegistry(t *testing.T) {
	tmp := t.TempDir()
	bffxDir := filepath.Join(tmp, "bffx")
	dir := filepath.Join(bffxDir, "cronjobs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	projectYAML := `apiVersion: bffx.io/v1alpha1
kind: Project
metadata:
  name: cron-test
spec:
  store:
    mode: sqlite
    path: .bffx/data/app.db
`
	if err := os.WriteFile(filepath.Join(bffxDir, "project.yaml"), []byte(projectYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	yaml := `apiVersion: bffx.io/v1alpha1
kind: CronJob
metadata:
  name: hourly_cleanup
spec:
  schedule: "0 * * * *"
  action: cleanup
`
	if err := os.WriteFile(filepath.Join(dir, "hourly_cleanup.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	reg, err := manifest.LoadAll(tmp)
	if err != nil {
		t.Fatal(err)
	}
	s := NewCronScheduler()
	if err := s.LoadFromRegistry(reg); err != nil {
		t.Fatal(err)
	}
	jobs := s.Jobs()
	if len(jobs) != 1 || jobs[0].Action != "cleanup" {
		t.Fatalf("jobs: %+v", jobs)
	}
	if err := s.Start(context.Background(), func(job CronJob) error { return nil }); err != nil {
		t.Fatal(err)
	}
}
