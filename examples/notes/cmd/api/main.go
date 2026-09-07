package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/hangry-coder/bffx/pkg/app"
)

func main() {
	port := 8080
	if p := os.Getenv("PORT"); p != "" {
		_, _ = fmt.Sscanf(p, "%d", &port)
	}

	// Register custom API endpoint using our cookbook pattern
	app.CustomRouteRegistrar = func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api/v1/app/ping", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok","message":"pong"}`))
		})
	}

	if err := app.RunServer(context.Background(), ".", port, ActionHandlers, HookHandlers); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}