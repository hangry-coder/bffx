package cli

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/hangry-coder/bffx/pkg/app"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/mcp"
)

func HandleMCP(args []string) {
	if len(args) < 1 || args[0] != "serve" {
		fmt.Println("usage: bffx mcp serve [--transport stdio|http] [--port 8081] [--token <token>]")
		os.Exit(1)
	}

	app.LoadEnv(".")

	transport := "stdio"
	port := 8081
	token := os.Getenv("BFFX_MCP_TOKEN")
	root := "."

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--transport":
			if i+1 < len(args) {
				transport = args[i+1]
				i++
			}
		case "--port":
			if i+1 < len(args) {
				p, err := strconv.Atoi(args[i+1])
				if err == nil {
					port = p
				}
				i++
			}
		case "--token":
			if i+1 < len(args) {
				token = args[i+1]
				i++
			}
		case "--root":
			if i+1 < len(args) {
				root = args[i+1]
				i++
			}
		}
	}

	reg, err := manifest.LoadAll(root)
	if err != nil {
		log.Fatalf("failed to load manifests: %v", err)
	}

	server := mcp.NewServer(root, reg)

	if transport == "stdio" {
		server.ServeStdio()
	} else if transport == "http" {
		if err := server.ServeHTTP(port, token); err != nil {
			log.Fatalf("mcp http server failed: %v", err)
		}
	} else {
		log.Fatalf("unknown transport %q", transport)
	}
}
