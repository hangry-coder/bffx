package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

func TestServeHTTP_RequiresToken(t *testing.T) {
	s := NewServer(".", &manifest.Registry{})
	err := s.ServeHTTP(0, "")
	if err == nil {
		t.Fatal("expected error when starting without a token, got nil")
	}
	if !strings.Contains(err.Error(), "BFFX_MCP_TOKEN must be configured") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestServeHTTP_AuthAndRequests(t *testing.T) {
	// Find a free port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	s := NewServer(".", &manifest.Registry{})
	token := "secure-test-token-at-least-32-chars-long"

	// Start the server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.ServeHTTP(port, token)
	}()

	// Wait for server to start
	url := fmt.Sprintf("http://127.0.0.1:%d/mcp", port)
	var resp *http.Response
	for i := 0; i < 50; i++ {
		req, _ := http.NewRequest("POST", url, strings.NewReader(`{}`))
		resp, err = http.DefaultClient.Do(req)
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("failed to connect to test MCP server: %v", err)
	}
	defer resp.Body.Close()

	// 1. Without Token -> 401 Unauthorized
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", resp.StatusCode)
	}

	// 2. With wrong token -> 401 Unauthorized
	req, _ := http.NewRequest("POST", url, strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer wrong-token")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong token, got %d", resp2.StatusCode)
	}

	// 3. With correct token -> 200 OK + valid JSON-RPC
	reqPayload := Request{JSONRPC: "2.0", ID: 1, Method: "initialize"}
	body, _ := json.Marshal(reqPayload)
	req, _ = http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	resp3, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp3.Body.Close()

	if resp3.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK with valid token, got %d", resp3.StatusCode)
	}
	var mcpResp Response
	if err := json.NewDecoder(resp3.Body).Decode(&mcpResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if mcpResp.Error != nil {
		t.Errorf("expected no error, got %+v", mcpResp.Error)
	}
}
