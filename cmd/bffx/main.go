package main

import (
	"fmt"
	"os"

	"github.com/hangry-coder/bffx/cmd/bffx/internal/cli"
	"github.com/hangry-coder/bffx/pkg/logger"
	"github.com/hangry-coder/bffx/pkg/version"
)

// Version is the CLI build identity (overridden by goreleaser: -X main.Version=...).
var Version = version.FrameworkVersion

func main() {
	logger.SetLevel(logger.ParseLevel(os.Getenv("LOG_LEVEL")))
	cli.Version = Version

	if len(os.Args) < 2 {
		cli.Usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Printf("bffx %s (framework %s)\n", Version, version.FrameworkVersion)
		return
	case "help", "--help", "-h":
		cli.Usage()
		return
	case "sync":
		cli.HandleSync(os.Args[2:])
	case "new":
		cli.HandleNew(os.Args[2:])
	case "dev":
		cli.HandleDev(os.Args[2:])
	case "build":
		cli.HandleBuild(os.Args[2:])
	case "routes":
		cli.HandleRoutes(os.Args[2:])
	case "generate", "gen", "g":
		cli.HandleGenerate(os.Args[2:])
	case "env":
		cli.HandleEnv(os.Args[2:])
	case "mcp":
		cli.HandleMCP(os.Args[2:])
	case "doctor":
		cli.HandleDoctor(os.Args[2:])
	case "upgrade":
		cli.HandleUpgrade(os.Args[2:])
	case "deploy":
		cli.HandleDeploy(os.Args[2:])
	case "up":
		cli.HandleUp(os.Args[2:])
	case "add":
		cli.HandleAdd(os.Args[2:])
	case "test":
		cli.HandleTest(os.Args[2:])
	case "update":
		cli.HandleUpdate(os.Args[2:])
	case "lint":
		cli.HandleLint(os.Args[2:])
	case "seed":
		cli.HandleSeed(os.Args[2:])
	case "migrate":
		cli.HandleMigrate(os.Args[2:])
	case "db":
		cli.HandleDB(os.Args[2:])
	case "check":
		cli.HandleCheck(os.Args[2:])
	default:
		cli.Usage()
		os.Exit(1)
	}
}
