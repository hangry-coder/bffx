package pipelines

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hangry-coder/bffx/pkg/auth"
	"github.com/hangry-coder/bffx/tests/archetypes"
)

func findFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestArchetypePipelines(t *testing.T) {
	tests := []struct {
		alias        string
		pipelineName string
		isAsync      bool
		isChatbot    bool
		payload      string
		contentType  string
		expectedCode int
	}{
		{
			alias:        "subscriptions",
			pipelineName: "InboxParse",
			isAsync:      false,
			isChatbot:    false,
			payload:      "raw-email-body-text",
			contentType:  "text/plain",
			expectedCode: 200,
		},
		{
			alias:        "commerce",
			pipelineName: "ProductScan",
			isAsync:      false,
			isChatbot:    false,
			payload:      "raw-product-image-bytes",
			contentType:  "application/octet-stream",
			expectedCode: 200,
		},
		{
			alias:        "fintech",
			pipelineName: "ReceiptScan",
			isAsync:      true,
			isChatbot:    false,
			payload:      "raw-receipt-image-bytes",
			contentType:  "application/octet-stream",
			expectedCode: 200,
		},
		{
			alias:        "dictation",
			pipelineName: "AudioUpload",
			isAsync:      true,
			isChatbot:    false,
			payload:      "raw-audio-file-bytes",
			contentType:  "application/octet-stream",
			expectedCode: 200,
		},
		{
			alias:        "superapp",
			pipelineName: "Support",
			isAsync:      false,
			isChatbot:    true,
			payload:      `{"session_id": "test-session", "message": "hello"}`,
			contentType:  "application/json",
			expectedCode: 200,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.alias, func(t *testing.T) {
			tmpDir := t.TempDir()
			projectDir := archetypes.SetupArchetypeProject(t, tmpDir, tc.alias)
			archetypes.SyncArchetypeProject(t, projectDir)
			archetypes.BuildArchetypeProject(t, projectDir, "v2")

			port := findFreePort(t)
			addr := fmt.Sprintf("http://127.0.0.1:%d", port)

			binaryPath := filepath.Join(projectDir, ".bffx", "orchestrator.test.bin")
			cmd := exec.Command(binaryPath)
			cmd.Dir = projectDir

			// Inject mock JWT secret and mock VLM response into background server env
			mockVLMJSON := `{"text": "mock-response-data", "status": "ok"}`
			cmd.Env = append(os.Environ(),
				fmt.Sprintf("PORT=%d", port),
				"BFFX_JWT_SECRET=test-secret-key-12345-very-secure-must-be-32-chars-long",
				"BFFX_VLM_MOCK_RESPONSE="+mockVLMJSON,
				"DATABASE_URL=test.db",
				"CLERK_JWKS_URL=https://example.com/.well-known/jwks.json",
				"WORKOS_API_KEY=mock-workos-key",
				"WORKOS_CLIENT_ID=mock-workos-client-id",
			)

			// Capture background logs in case of failure
			var logBuf bytes.Buffer
			cmd.Stdout = &logBuf
			cmd.Stderr = &logBuf

			err := cmd.Start()
			if err != nil {
				t.Fatalf("failed to start orchestrator background process: %v", err)
			}
			defer func() {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				if t.Failed() {
					t.Logf("Orchestrator logs:\n%s", logBuf.String())
				}
			}()

			// Wait for server to become healthy (up to 5 seconds)
			healthy := false
			client := &http.Client{Timeout: 500 * time.Millisecond}
			var lastErr error
			for i := 0; i < 50; i++ {
				resp, err := client.Get(addr + "/health")
				if err == nil && resp.StatusCode == http.StatusOK {
					healthy = true
					resp.Body.Close()
					break
				}
				if err != nil {
					lastErr = err
				} else {
					lastErr = fmt.Errorf("status code %d", resp.StatusCode)
					resp.Body.Close()
				}
				time.Sleep(100 * time.Millisecond)
			}

			if !healthy {
				t.Fatalf("orchestrator server failed to become healthy on port %d: %v\nLogs:\n%s", port, lastErr, logBuf.String())
			}

			// Construct valid authentication token for the request
			jwtSvc := auth.NewJWTService("test-secret-key-12345-very-secure-must-be-32-chars-long")
			token, err := jwtSvc.GenerateToken("test-user", "user", "", "", false, time.Hour)
			if err != nil {
				t.Fatalf("failed to generate auth token: %v", err)
			}

			// Send POST request to the pipeline endpoint
			pipelinePath := "/api/v1/pipelines/" + strings.ToLower(tc.pipelineName)
			req, err := http.NewRequest(http.MethodPost, addr+pipelinePath, strings.NewReader(tc.payload))
			if err != nil {
				t.Fatalf("failed to create pipeline request: %v", err)
			}
			req.Header.Set("Content-Type", tc.contentType)
			req.Header.Set("Authorization", "Bearer "+token)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("pipeline request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.expectedCode {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("expected status code %d, got %d. Body: %s", tc.expectedCode, resp.StatusCode, string(body))
			}

			// If chatbot, assert SSE streaming format
			if tc.isChatbot {
				contentType := resp.Header.Get("Content-Type")
				if !strings.Contains(contentType, "text/event-stream") {
					t.Fatalf("expected content type to contain text/event-stream, got: %s", contentType)
				}

				bodyBytes, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("failed to read SSE body: %v", err)
				}
				bodyStr := string(bodyBytes)
				if !strings.Contains(bodyStr, "event: done") {
					t.Fatalf("expected SSE stream to contain 'event: done', got: %s", bodyStr)
				}
			}
		})
	}
}

// Seed random number generator
func init() {
	rand.Seed(time.Now().UnixNano())
}
