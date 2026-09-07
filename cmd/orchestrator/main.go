package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/hangry-coder/bffx/pkg/app"
)

func main() {
	port := 8080
	if p := os.Getenv("PORT"); p != "" {
		_, _ = fmt.Sscanf(p, "%d", &port)
	}
	if err := app.RunServer(context.Background(), "examples/notes", port, nil, nil); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}