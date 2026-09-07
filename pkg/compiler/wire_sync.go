package compiler

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hangry-coder/bffx/pkg/batteries/wire/proto"
	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

// SyncWire generates all proto definitions and buf configuration schemas
// when project.yaml has runtime.wire.enabled: true.
func SyncWire(root string, reg *manifest.Registry, projectSpec *manifest.ProjectSpec) error {
	if projectSpec == nil || !projectSpec.Runtime.Wire.Enabled {
		return nil
	}

	outDir := filepath.Join(root, ".bffx")
	protoDir := filepath.Join(outDir, "proto", "bffx", "v1")
	if err := os.MkdirAll(protoDir, 0o755); err != nil {
		return fmt.Errorf("make proto dir: %w", err)
	}

	// 1. Generate proto content
	wirePackage := projectSpec.Runtime.Wire.Package
	if wirePackage == "" {
		wirePackage = "bffx.v1"
	}
	protoText := proto.BuildProto(reg, wirePackage)

	// 2. Write proto file
	protoFile := filepath.Join(protoDir, "service.proto")
	if err := os.WriteFile(protoFile, []byte(protoText), 0o644); err != nil {
		return fmt.Errorf("write service.proto: %w", err)
	}

	// 3. Write buf.yaml
	bufYamlText := `version: v1
build:
  excludes:
    - core
deps:
  - buf.build/googleapis/googleapis
lint:
  use:
    - DEFAULT
`
	bufYamlFile := filepath.Join(outDir, "buf.yaml")
	if err := os.WriteFile(bufYamlFile, []byte(bufYamlText), 0o644); err != nil {
		return fmt.Errorf("write buf.yaml: %w", err)
	}

	// 4. Write buf.gen.yaml (for standard Go code generation)
	bufGenYamlText := `version: v1
plugins:
  - plugin: go
    out: gen/go
    opt:
      - paths=source_relative
  - plugin: go-grpc
    out: gen/go
    opt:
      - paths=source_relative
`
	bufGenYamlFile := filepath.Join(outDir, "buf.gen.yaml")
	if err := os.WriteFile(bufGenYamlFile, []byte(bufGenYamlText), 0o644); err != nil {
		return fmt.Errorf("write buf.gen.yaml: %w", err)
	}

	// 5. Auto-run `buf generate`
	pathEnv := os.Getenv("PATH")
	homeDir, _ := os.UserHomeDir()
	goBin := filepath.Join(homeDir, "go", "bin")
	brewBin := "/opt/homebrew/bin"
	newPath := fmt.Sprintf("%s:%s:%s", goBin, brewBin, pathEnv)

	cmd := exec.Command("buf", "generate")
	cmd.Dir = outDir
	cmd.Env = append(os.Environ(), "PATH="+newPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.Warn("Failed to auto-run `buf generate` (ensure `buf` and plugins are in PATH): %v\nOutput: %s", err, string(output))
	} else {
		logger.Info("Successfully generated protobuf Go/gRPC stubs using `buf generate`")
	}

	return nil
}
