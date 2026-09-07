package e2e

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCoreProductionMatrix(t *testing.T) {
	personas := []PersonaConfig{
		PersonaEnterpriseHeavy(),
		PersonaStandardStartup(),
		PersonaNoSQLNative(),
		PersonaLightweightMonolith(),
	}

	mutations := AllMutations()

	for _, p := range personas {
		if !p.IsAvailable() {
			t.Logf("Skipping persona %s (not available)", p.Name)
			continue
		}

		for _, m := range mutations {
			t.Run(fmt.Sprintf("%s/%s", p.Name, m.Name), func(t *testing.T) {
				ts := NewTestServer(t, p, m)
				
				// Assertions will go here
				// 1. Handshake Integrity
				// 2. Resource Creation & Auto-Ownership
				// 3. RLS Blocking
				// 4. Anonymous Provisioning
				
				runCoreE2EAssertions(t, ts)
			})
		}
	}
}

func runCoreE2EAssertions(t *testing.T, ts *TestServer) {
	// 1. Handshake Integrity - Should fail without secret
	resp, _ := ts.GET("/api/v1/users", map[string]string{})
	if resp.StatusCode != 403 {
		t.Errorf("Expected 403 without App Secret, got %d", resp.StatusCode)
	}

	headers := map[string]string{"X-App-Secret": "test-app-secret-at-least-32-chars-long-!!!"}
	
	// 2. Anonymous Provisioning - Should work with X-Device-ID
	headers["X-Device-ID"] = "test-device-uuid"
	resp, err := ts.GET("/api/v1/users", headers)
	if err != nil {
		t.Fatalf("Failed to GET users: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected 200 with App Secret & Device ID, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 3. Resource Creation & Auto-Ownership
	payload := map[string]any{
		"name":  "User 1",
		"email": "user1@example.com",
	}
	resp, err = ts.POST("/api/v1/users", payload, headers)
	if err != nil {
		t.Fatalf("Failed to POST user: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Errorf("Expected 201, got %d", resp.StatusCode)
	}
	var user1 map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&user1); err != nil {
		t.Fatalf("Failed to decode user response: %v", err)
	}
	resp.Body.Close()

	user1ID, ok := user1["id"].(string)
	if !ok {
		t.Fatalf("User response missing id or id is not a string: %v", user1)
	}

	// 4. RLS Blocking - Second user should NOT see User 1
	headers2 := map[string]string{
		"X-App-Secret": "test-app-secret-at-least-32-chars-long-!!!",
		"X-Device-ID":  "device-2",
	}
	
	resp, _ = ts.GET("/api/v1/users/"+user1ID, headers2)
	if resp.StatusCode != 403 && resp.StatusCode != 404 {
		t.Errorf("Expected 403 or 404 for other owner's record, got %d", resp.StatusCode)
	}
}
