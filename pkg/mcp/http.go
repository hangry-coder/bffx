package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/logger"
)

// ServeHTTP starts the MCP server over HTTP.
func (s *Server) ServeHTTP(port int, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("cannot start MCP server over HTTP without a token: BFFX_MCP_TOKEN must be configured")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /mcp", func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") || !middleware.SecretsEqual(strings.TrimPrefix(authHeader, "Bearer "), token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.sendError(w, nil, -32700, "Parse error: "+err.Error())
			return
		}

		resp := s.handleRequest(&req)
		if resp != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}
	})

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	logger.Info("mcp server listening on http://%s/mcp", addr)
	return http.ListenAndServe(addr, mux)
}
