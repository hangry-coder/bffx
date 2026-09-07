# Cookbook: Registering Custom HTTP Routes

BFFX is manifest-driven by default, generating the standard CRUD actions, validation schemas, and mobile SDK methods from your YAML specs. However, real-world backend applications often require bespoke, high-performance, or legacy HTTP endpoints.

BFFX supports this extensibility via the `app.CustomRouteRegistrar` hook. When you mount custom routes using this pattern, they are **automatically wrapped in the entire BFFX security and middleware chain** (CORS, Trace Propagation, Access Logs, JWT Context, Timeout, Rate Limiting, and Recovery).

---

## The Pattern

In your orchestrator's `main.go` entry point (normally located in `cmd/orchestrator/main.go`), you can set the `app.CustomRouteRegistrar` callback prior to calling `app.RunServer`.

```go
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

	// 1. Define custom routes using standard Go 1.22 routing pattern
	app.CustomRouteRegistrar = func(mux *http.ServeMux) {
		
		// GET endpoint
		mux.HandleFunc("GET /api/v1/app/ping", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok","message":"pong"}`))
		})

		// POST endpoint with body parsing
		mux.HandleFunc("POST /api/v1/app/feedback", func(w http.ResponseWriter, r *http.Request) {
			// Handle custom body processing...
			w.WriteHeader(http.StatusAccepted)
		})
	}

	// 2. Start the core orchestrator server
	if err := app.RunServer(context.Background(), ".", port, ActionHandlers, HookHandlers); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
```

---

## Interacting with Middleware & Context

Because custom routes are mounted under the main router multiplexer, they automatically participate in all middleware context decorations:

### 1. Retrieving Authenticated User ID
BFFX automatically parses JWT credentials. You can retrieve the authenticated user's ID inside your custom handler:

```go
import "github.com/hangry-coder/bffx/pkg/api/middleware"

func myHandler(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	// Proceed with user-specific context...
}
```

### 2. Trace Propagation & Logging
Custom handlers benefit from full request-scoped logging. If your handler triggers logs, they will be stamped with the correct request trace ID:

```go
import "github.com/hangry-coder/bffx/pkg/logger"

func myHandler(w http.ResponseWriter, r *http.Request) {
	logger.Info("Received custom route request on trace ID: %s", middleware.GetTraceID(r.Context()))
}
```

---

## Flutter Integration

When generating client SDKs via `bffx sync`, standard actions and screens are parsed and exported automatically. For bespoke custom routes, you can simply write an extension or wrapper over the default client to call your endpoints cleanly:

```dart
// Custom extension in lib/bffx_client.dart
extension CustomRoutes on BFFXClient {
  Future<Map<String, dynamic>> pingServer() async {
    final res = await request('/api/v1/app/ping', method: 'GET');
    if (res.statusCode != 200) {
      throw Exception('Ping failed');
    }
    return jsonDecode(res.body);
  }
}
```
