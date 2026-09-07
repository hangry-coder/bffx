package api

import (
	"github.com/hangry-coder/bffx/pkg/app"
	"github.com/hangry-coder/bffx/pkg/testing"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
	go_testing "testing"
)

func TestAppSecretHardening(t *go_testing.T) {
	testing.SetT(t)
	testing.Describe("App Secret Hardening", func() {
		os.Setenv("BFFX_APP_SECRET", "super-secret-1234567890")
		defer os.Unsetenv("BFFX_APP_SECRET")

		projectDir := "test-app-secret"
		os.MkdirAll(projectDir+"/bffx", 0755)
		os.WriteFile(projectDir+"/bffx/project.yaml", []byte(`
apiVersion: bffx.io/v1alpha1
kind: Project
metadata:
  name: test-app
spec:
  store:
    mode: memory
`), 0644)
		defer os.RemoveAll(projectDir)

		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		port := ln.Addr().(*net.TCPAddr).Port
		ln.Close()

		go func() {
			_ = app.RunServer(context.Background(), projectDir, port, nil, nil)
		}()

		base := fmt.Sprintf("http://127.0.0.1:%d", port)
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			res, err := http.Get(base + "/health")
			if err == nil && res.StatusCode == http.StatusOK {
				res.Body.Close()
				break
			}
			if res != nil {
				res.Body.Close()
			}
			time.Sleep(100 * time.Millisecond)
		}

		testing.It("should block requests without X-App-Secret", func() {
			res, err := http.Get(base + "/api/v1/hello")
			testing.Expect(err).ToEqual(nil)
			if res != nil {
				testing.Expect(res.StatusCode).ToEqual(http.StatusForbidden)
			} else {
				t.Error("Response is nil")
			}
		})

		testing.It("should block requests with WRONG X-App-Secret", func() {
			req, _ := http.NewRequest("GET", base+"/api/v1/hello", nil)
			req.Header.Set("X-App-Secret", "wrong-secret")
			res, err := http.DefaultClient.Do(req)
			testing.Expect(err).ToEqual(nil)
			if res != nil {
				testing.Expect(res.StatusCode).ToEqual(http.StatusForbidden)
			} else {
				t.Error("Response is nil")
			}
		})

		testing.It("should allow requests with CORRECT X-App-Secret", func() {
			req, _ := http.NewRequest("GET", base+"/api/v1/hello", nil)
			req.Header.Set("X-App-Secret", "super-secret-1234567890")
			res, err := http.DefaultClient.Do(req)
			testing.Expect(err).ToEqual(nil)
			if res != nil {
				testing.Expect(res.StatusCode).ToEqual(http.StatusOK)
			} else {
				t.Error("Response is nil")
			}
		})

		testing.It("should NOT block non-api routes (like health)", func() {
			res, err := http.Get(base + "/health")
			testing.Expect(err).ToEqual(nil)
			if res != nil {
				testing.Expect(res.StatusCode).ToEqual(http.StatusOK)
			} else {
				t.Error("Response is nil")
			}
		})
	})
}
