package e2e

import (
	"github.com/hangry-coder/bffx/pkg/app"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type TestServer struct {
	URL     string
	Port    int
	BaseDir string
	cancel  context.CancelFunc
}

func NewTestServer(t *testing.T, persona PersonaConfig, mutation AuthMutation) *TestServer {
	if !persona.IsAvailable() {
		t.Skipf("Persona %s not available (missing env)", persona.Name)
	}

	tmpDir, err := os.MkdirTemp("", "bffx-e2e-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Set mandatory secrets for non-memory personas
	t.Setenv("BFFX_JWT_SECRET", "test-jwt-secret-at-least-32-chars-long-!!!")
	t.Setenv("BFFX_APP_SECRET", "test-app-secret-at-least-32-chars-long-!!!")

	// Prepare project.yaml
	yaml := persona.YAML
	if mutation.Strategy != "" {
		// Simple injection for test purposes
		yaml += fmt.Sprintf("\n  defaults:\n    auth: builtin\n  app:\n    authStrategy: %s\n", mutation.Strategy)
	}
	
	// Add App Secret to project.yaml as well for the handshake middleware
	yaml += "\n  security:\n    appSecret: \"test-app-secret-at-least-32-chars-long-!!!\"\n"
	
	// Replace env vars in YAML
	yaml = os.ExpandEnv(yaml)

	os.MkdirAll(filepath.Join(tmpDir, "bffx"), 0755)
	err = os.WriteFile(filepath.Join(tmpDir, "bffx", "project.yaml"), []byte(yaml), 0644)
	if err != nil {
		t.Fatalf("Failed to write project.yaml: %v", err)
	}

	// Find free port
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	ctx, cancel := context.WithCancel(context.Background())
	ts := &TestServer{
		URL:     fmt.Sprintf("http://127.0.0.1:%d", port),
		Port:    port,
		BaseDir: tmpDir,
		cancel:  cancel,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.RunServer(ctx, tmpDir, port, nil, nil)
	}()

	// Wait for health
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("Server exited early with error: %v", err)
			}
		default:
		}

		resp, err := http.Get(ts.URL + "/health")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			lastErr = nil
			break
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("unexpected status: %d", resp.StatusCode)
			resp.Body.Close()
		}
		time.Sleep(200 * time.Millisecond)
	}

	if lastErr != nil {
		t.Errorf("Persona %s failed health check: %v", persona.Name, lastErr)
	}

	t.Cleanup(func() {
		ts.Close()
		os.RemoveAll(tmpDir)
	})

	return ts
}

func (ts *TestServer) Close() {
	ts.cancel()
	// In a real scenario, we might need a more graceful way to kill the server
	// if it doesn't respond to context cancellation.
}

func (ts *TestServer) GET(path string, headers map[string]string) (*http.Response, error) {
	req, _ := http.NewRequest("GET", ts.URL+path, nil)
	
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Inject App Secret if configured and headers is nil (not explicitly provided)
	if secret := os.Getenv("BFFX_APP_SECRET"); secret != "" && headers == nil {
		req.Header.Set("X-App-Secret", secret)
	}

	return http.DefaultClient.Do(req)
}

func (ts *TestServer) POST(path string, body any, headers map[string]string) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}
	req, _ := http.NewRequest("POST", ts.URL+path, bodyReader)
	req.Header.Set("Content-Type", "application/json")

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Inject App Secret if configured and headers is nil (not explicitly provided)
	if secret := os.Getenv("BFFX_APP_SECRET"); secret != "" && headers == nil {
		req.Header.Set("X-App-Secret", secret)
	}

	return http.DefaultClient.Do(req)
}
