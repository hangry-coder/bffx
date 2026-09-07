package cli

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hangry-coder/bffx/pkg/compiler"
	"github.com/hangry-coder/bffx/pkg/logger"
)

func HandleBuild(args []string) {
	root := "."
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-h" || arg == "--help" || arg == "help" {
			fmt.Println("Usage: bffx build [--root DIR]")
			fmt.Println("  Run compiler sync and build ./cmd/orchestrator or ./cmd/api.")
			return
		}
		if arg == "--root" && i+1 < len(args) {
			root = args[i+1]
			i++
		}
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		log.Fatalf("invalid --root %q: %v", root, err)
	}
	root = rootAbs

	logger.Info("Syncing manifests...")
	if _, err := compiler.Sync(root, false); err != nil {
		log.Fatalf("sync failed: %v", err)
	}

	logger.Info("Building orchestrator...")
	entry := "./cmd/orchestrator"
	if _, err := os.Stat(filepath.Join(root, "cmd", "api")); err == nil {
		entry = "./cmd/api"
	}
	buildCmd := exec.Command("go", "build", "-o", ".bffx/orchestrator", entry)
	buildCmd.Dir = root
	buildCmd.Env = compiler.GoBuildEnv()
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		log.Fatalf("Build failed: %v", err)
	}
	logger.Info("Build successful! Binary located at .bffx/orchestrator")
}
