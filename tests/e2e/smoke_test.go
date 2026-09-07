package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
)

func TestSmokeHappyPath(t *testing.T) {
	RunEphemeralTest(t, E2ESpec{
		Name: "smoke-app",
		ScaffoldArgs: []string{
			"--admin-email", "admin@example.com",
			"--admin-password", "admin123",
			"--with-telemetry",
		},
		Port: 8084,
		HostTests: func(t *testing.T, port int) {
			baseUrl := fmt.Sprintf("http://localhost:%d", port)
			
			t.Log("Verifying User Signup on Host...")
			signupURL := baseUrl + "/api/v1/auth/signup"
			userPayload := `{"email": "tester@example.com", "password": "password123", "name": "Test User"}`
			resp, err := http.Post(signupURL, "application/json", bytes.NewBufferString(userPayload))
			if err != nil || resp.StatusCode != http.StatusCreated {
				var body string
				if resp != nil {
					b, _ := io.ReadAll(resp.Body)
					body = string(b)
				}
				t.Fatalf("Signup failed: %v status: %d body: %s", err, resp.StatusCode, body)
			}

			t.Log("Verifying User Login on Host...")
			loginURL := baseUrl + "/api/v1/auth/login"
			loginPayload := `{"email": "tester@example.com", "password": "password123"}`
			resp, err = http.Post(loginURL, "application/json", bytes.NewBufferString(loginPayload))
			if err != nil || resp.StatusCode != http.StatusOK {
				t.Fatalf("Login failed: %v status: %d", err, resp.StatusCode)
			}

			var loginResult struct {
				Token string `json:"token"`
			}
			json.NewDecoder(resp.Body).Decode(&loginResult)
			if loginResult.Token == "" {
				t.Fatal("Login did not return a JWT token")
			}

			t.Log("Verifying HomeScreen Builder on Host...")
			req, _ := http.NewRequest("GET", baseUrl+"/api/v1/screens/home", nil)
			req.Header.Set("Authorization", "Bearer "+loginResult.Token)
			resp, err = http.DefaultClient.Do(req)
			if err != nil || resp.StatusCode != http.StatusOK {
				t.Fatalf("GET /screens/home failed: %v status: %d", err, resp.StatusCode)
			}
			t.Log("✨ Host Smoke Success")
		},
		DockerTests: func(t *testing.T, port int) {
			baseUrl := fmt.Sprintf("http://localhost:%d", port)
			// Simple check that it's alive and responding
			resp, err := http.Get(baseUrl + "/health")
			if err != nil || resp.StatusCode != http.StatusOK {
				t.Fatalf("Health check failed in docker: %v", err)
			}
			t.Log("✨ Docker Smoke Success")
		},
	})
}
