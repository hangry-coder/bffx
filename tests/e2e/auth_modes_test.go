package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestAuthModeAnonymous(t *testing.T) {
	RunEphemeralTest(t, E2ESpec{
		Name:         "test-auth-anon",
		ScaffoldArgs: []string{"--auth-strategy", "anonymous"},
		Port:         8081,
		HostTests: func(t *testing.T, port int) {
			t.Log("Verifying Device-ID Auto-Registration on Host...")
			req, _ := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/api/v1/app/bootstrap", port), nil)
			req.Header.Set("X-Device-ID", "host-device-456")

			resp, err := http.DefaultClient.Do(req)
			if err != nil || resp.StatusCode != http.StatusOK {
				t.Fatalf("Bootstrap failed on host: %v", err)
			}

			var result struct {
				Auth struct {
					User struct{ ID string } `json:"user"`
				} `json:"auth"`
			}
			json.NewDecoder(resp.Body).Decode(&result)
			if result.Auth.User.ID == "" {
				t.Fatal("Expected anonymous user on host but got empty")
			}
			t.Logf("✨ Host Success: User %s created", result.Auth.User.ID)
		},
		DockerTests: func(t *testing.T, port int) {
			t.Log("Verifying Device-ID Auto-Registration in Docker...")
			req, _ := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/api/v1/app/bootstrap", port), nil)
			req.Header.Set("X-Device-ID", "docker-device-789")

			resp, err := http.DefaultClient.Do(req)
			if err != nil || resp.StatusCode != http.StatusOK {
				t.Fatalf("Bootstrap failed in docker: %v", err)
			}
			t.Log("✨ Docker Success")
		},
	})
}

func TestAuthModeOptional(t *testing.T) {
	RunEphemeralTest(t, E2ESpec{
		Name:         "test-auth-optional",
		ScaffoldArgs: []string{"--auth-strategy", "optional"},
		Port:         8085,
		HostTests: func(t *testing.T, port int) {
			// In optional mode, we should get 200 even without token
			resp, _ := http.Get(fmt.Sprintf("http://localhost:%d/api/v1/screens/home", port))
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Expected 200 on optional host, got %d", resp.StatusCode)
			}
			t.Log("✨ Host Success: Optional auth allowed guest access")
		},
		DockerTests: func(t *testing.T, port int) {
			resp, _ := http.Get(fmt.Sprintf("http://localhost:%d/api/v1/screens/home", port))
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Expected 200 on optional docker, got %d", resp.StatusCode)
			}
			t.Log("✨ Docker Success: Optional auth allowed guest access")
		},
	})
}

func TestAuthModeMandatory(t *testing.T) {
	RunEphemeralTest(t, E2ESpec{
		Name:         "test-auth-mandatory",
		ScaffoldArgs: []string{"--auth-strategy", "mandatory"},
		Port:         8082,
		HostTests: func(t *testing.T, port int) {
			resp, _ := http.Get(fmt.Sprintf("http://localhost:%d/api/v1/screens/home", port))
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("Expected 401 on host, got %d", resp.StatusCode)
			}
			t.Log("✨ Host Success")
		},
		DockerTests: func(t *testing.T, port int) {
			resp, _ := http.Get(fmt.Sprintf("http://localhost:%d/api/v1/screens/home", port))
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("Expected 401 in docker, got %d", resp.StatusCode)
			}
			t.Log("✨ Mandatory Auth Success")
		},
	})
}
