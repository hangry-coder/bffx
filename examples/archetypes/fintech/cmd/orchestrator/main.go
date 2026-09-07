package main

import (
	"github.com/hangry-coder/bffx/pkg/app"
	"context"
	"fmt"
	"log"
	"os"
)

func main() {
	port := 8080
	if p := os.Getenv("PORT"); p != "" {
		_, _ = fmt.Sscanf(p, "%d", &port)
	}
	if err := app.RunServer(context.Background(), ".", port, nil, nil); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}