package cli

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var updateHelpGolden = flag.Bool("update", false, "rewrite golden files")

func TestCLIHelpGolden(t *testing.T) {
	// Build the bffx binary
	tmpDir, err := os.MkdirTemp("", "bffx-cli-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	bffxBin := filepath.Join(tmpDir, "bffx")
	cmdBuild := exec.Command("go", "build", "-o", bffxBin, "../../cmd/bffx")
	if output, err := cmdBuild.CombinedOutput(); err != nil {
		t.Fatalf("failed to build bffx: %v\n%s", err, string(output))
	}

	tests := []struct {
		name string
		args []string
	}{
		{"root", []string{"--help"}},
		{"sync", []string{"sync", "--help"}},
		{"dev", []string{"dev", "--help"}},
		{"migrate", []string{"migrate", "--help"}},
		{"db", []string{"db", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(bffxBin, tt.args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			
			// Some flags packages print help to stderr (like standard flag)
			// others to stdout. We'll combine them for the golden.
			_ = cmd.Run()
			
			got := stdout.String() + stderr.String()
			// Normalize paths or environment specific strings if any
			got = strings.ReplaceAll(got, tmpDir, "/tmp/bffx")

			goldenPath := filepath.Join("testdata", "golden", "help", tt.name+".txt")
			if *updateHelpGolden {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(goldenPath, []byte(got), 0644); err != nil {
					t.Fatal(err)
				}
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("failed to read golden file: %v. Run with -update to generate.", err)
			}

			if got != string(want) {
				t.Errorf("CLI help for %v changed. Diff:\n--- want\n+++ got\n%s", tt.args, got)
			}
		})
	}
}
