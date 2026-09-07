package cli

import (
	"fmt"
	"log"

	"github.com/hangry-coder/bffx/pkg/app"
	"github.com/hangry-coder/bffx/pkg/deploy"
)

func HandleDeploy(args []string) {
	root := "."
	kamal := false
	var actArgs []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--root" && i+1 < len(args) {
			root = args[i+1]
			i++
		} else if args[i] == "--kamal" {
			kamal = true
		} else {
			actArgs = append(actArgs, args[i])
		}
	}
	args = actArgs

	_ = app.LoadEnv(root)

	if len(args) < 1 {
		printDeployUsage()
		return
	}
	switch args[0] {
	case "init":
		workspaceRoot := deploy.DetectWorkspaceRoot(root)

		opts := &deploy.InitOptions{
			Kamal: kamal,
		}
		if err := deploy.Init(root, workspaceRoot, opts); err != nil {
			log.Fatalf("deploy init failed: %v", err)
		}

	case "cloud":
		if len(args) < 2 {
			fmt.Println("usage: bffx deploy cloud [ --fly | --railway | --gcp ]")
			return
		}
		if err := deploy.Cloud(root, args[1]); err != nil {
			log.Fatalf("deploy cloud failed: %v", err)
		}
	case "status":
		deploy.Status(root)
	case "ship":
		if err := deploy.Ship(root); err != nil {
			log.Fatalf("deploy ship failed: %v", err)
		}
	case "rollback":
		if err := deploy.Rollback(root); err != nil {
			log.Fatalf("deploy rollback failed: %v", err)
		}
	default:
		printDeployUsage()
	}
}

func printDeployUsage() {
	fmt.Println(`usage: bffx deploy <command> [flags]

Commands:
  init       Generate Dockerfile, compose files, Caddy config, and optional Kamal config
  ship       Build (local), push image, and deploy using BFFX_DEPLOY_TAG / history
  rollback   Revert remote stack to the previous immutable image tag
  status     Show deploy artifacts and last known tag
  cloud      Generate provider-specific stubs (fly | railway | gcp)

Flags (init):
  --kamal    Also write config/deploy.yml for Kamal

Examples:
  bffx deploy init
  bffx deploy init --kamal
  bffx deploy ship
  bffx deploy rollback
  bffx deploy cloud fly`)
}
