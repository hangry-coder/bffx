package cli

import (
	"log"
	"os"
	"os/exec"

	"github.com/hangry-coder/bffx/pkg/logger"
)

func HandleUp(args []string) {
	root := "."
	for i := 0; i < len(args); i++ {
		if args[i] == "--root" && i+1 < len(args) {
			root = args[i+1]
			i++
		}
	}

	logger.Info("Launching full stack with docker compose for %s...", root)
	cmd := exec.Command("docker", "compose", "up", "--build")

	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		log.Fatalf("docker-compose failed: %v", err)
	}
}
