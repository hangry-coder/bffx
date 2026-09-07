package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// TestSecurityAuditBaseline focuses on verifying current vulnerabilities identified in audits.
// As we implement v2.6 fixes, these tests will transition from "documenting failure" to "enforcing security".
func TestSecurityAuditBaseline(t *testing.T) {
	RunEphemeralTest(t, E2ESpec{
		Name: "security-audit-baseline",
		ScaffoldArgs: []string{
			"--admin-email", "admin@example.com",
			"--admin-password", "admin123",
		},
		Port: 8089,
		HostTests: func(t *testing.T, port int) {
			baseUrl := fmt.Sprintf("http://localhost:%d", port)

			// 1. JWT Fail-Closed (Currently might fail/return anonymous access)
			t.Run("JWT_InvalidToken_401", func(t *testing.T) {
				req, _ := http.NewRequest("GET", baseUrl+"/api/v1/screens/home", nil)
				req.Header.Set("Authorization", "Bearer invalid-token-here")
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatalf("Request failed: %v", err)
				}
				// Currently, the audit says this might silently downgrade to anonymous access (200 OK)
				// Once fixed, this should be 401.
				if resp.StatusCode != http.StatusUnauthorized {
					t.Errorf("Expected 401 Unauthorized for invalid JWT, got %d", resp.StatusCode)
				}
			})

			// 2. Sensitive Field Stripping (Check if signup returns password hash)
			t.Run("Signup_Response_NoPassword", func(t *testing.T) {
				signupURL := baseUrl + "/api/v1/auth/signup"
				userPayload := `{"email": "audit-test@example.com", "password": "password123", "name": "Audit User"}`
				resp, err := http.Post(signupURL, "application/json", bytes.NewBufferString(userPayload))
				if err != nil || resp.StatusCode != http.StatusCreated {
					t.Fatalf("Signup failed: %v status: %d", err, resp.StatusCode)
				}
				
				var result map[string]any
				json.NewDecoder(resp.Body).Decode(&result)
				
				if _, exists := result["password"]; exists {
					t.Error("Signup response contains 'password' field - SECURITY RISK")
				}
				if _, exists := result["password_hash"]; exists {
					t.Error("Signup response contains 'password_hash' field - SECURITY RISK")
				}
			})

			// 3. Admin Session Integrity (Predictable session cookie)
			t.Run("Admin_Session_ForgedCookie_Blocked", func(t *testing.T) {
				req, _ := http.NewRequest("GET", baseUrl+"/admin/api/admin/resources", nil)
				// Audit says currently ANY cookie with the right name might pass
				req.AddCookie(&http.Cookie{Name: "bffx_admin_session", Value: "forged-session-value"})
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatalf("Request failed: %v", err)
				}
				if resp.StatusCode != http.StatusUnauthorized {
					t.Errorf("Expected 401 Unauthorized for forged admin cookie, got %d", resp.StatusCode)
				}
			})

			// 4. Jobs API Auth (Unauthenticated dispatch)
			t.Run("Jobs_UnauthenticatedDispatch_Blocked", func(t *testing.T) {
				dispatchURL := baseUrl + "/api/v1/jobs/dispatch"
				payload := `{"kind": "task", "name": "test-job", "input": {}}`
				resp, err := http.Post(dispatchURL, "application/json", bytes.NewBufferString(payload))
				if err != nil {
					t.Fatalf("Request failed: %v", err)
				}
				if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden {
					t.Errorf("Expected 401/403 for unauthenticated job dispatch, got %d", resp.StatusCode)
				}
			})
		},
	})
}

// TestRowLevelSecurity verifies that users can only access their own data.
func TestRowLevelSecurity(t *testing.T) {
	RunEphemeralTest(t, E2ESpec{
		Name: "security-row-level",
		ScaffoldArgs: []string{
			"--admin-email", "admin@example.com",
			"--admin-password", "admin1234567890",
		},
		Port: 8092,
		HostTests: func(t *testing.T, port int) {
			baseUrl := fmt.Sprintf("http://localhost:%d", port)

			// 1. Sign up User A
			signupURL := baseUrl + "/api/v1/auth/signup"
			userAPayload := `{"email": "userA@example.com", "password": "password123", "name": "User A"}`
			respA, err := http.Post(signupURL, "application/json", bytes.NewBufferString(userAPayload))
			if err != nil || respA.StatusCode != http.StatusCreated {
				t.Fatalf("User A signup failed: %v status: %d", err, respA.StatusCode)
			}
			var resultA map[string]any
			json.NewDecoder(respA.Body).Decode(&resultA)
			tokenA, _ := resultA["token"].(string)
			userAObj, _ := resultA["user"].(map[string]any)
			userAID, _ := userAObj["id"].(string)

			// 2. Sign up User B
			userBPayload := `{"email": "userB@example.com", "password": "password123", "name": "User B"}`
			respB, err := http.Post(signupURL, "application/json", bytes.NewBufferString(userBPayload))
			if err != nil || respB.StatusCode != http.StatusCreated {
				t.Fatalf("User B signup failed: %v status: %d", err, respB.StatusCode)
			}
			var resultB map[string]any
			json.NewDecoder(respB.Body).Decode(&resultB)
			tokenB, _ := resultB["token"].(string)
			userBObj, _ := resultB["user"].(map[string]any)
			userBID, _ := userBObj["id"].(string)

			// 3. User A GET /api/v1/users/{User_A_ID} -> should be 200 OK
			reqAOwn, _ := http.NewRequest("GET", baseUrl+"/api/v1/users/"+userAID, nil)
			reqAOwn.Header.Set("Authorization", "Bearer "+tokenA)
			respAOwn, err := http.DefaultClient.Do(reqAOwn)
			if err != nil || respAOwn.StatusCode != http.StatusOK {
				t.Errorf("User A fetching own profile failed: %v status: %d", err, respAOwn.StatusCode)
			}

			// 4. User A GET /api/v1/users/{User_B_ID} -> should be 403 Forbidden (policy is owner)
			reqAOther, _ := http.NewRequest("GET", baseUrl+"/api/v1/users/"+userBID, nil)
			reqAOther.Header.Set("Authorization", "Bearer "+tokenA)
			respAOther, err := http.DefaultClient.Do(reqAOther)
			if err != nil || respAOther.StatusCode != http.StatusForbidden {
				t.Errorf("Expected 403 Forbidden for User A accessing User B's profile, got %d (err: %v)", respAOther.StatusCode, err)
			}

			// 5. User B GET /api/v1/users/{User_B_ID} -> should be 200 OK
			reqBOwn, _ := http.NewRequest("GET", baseUrl+"/api/v1/users/"+userBID, nil)
			reqBOwn.Header.Set("Authorization", "Bearer "+tokenB)
			respBOwn, err := http.DefaultClient.Do(reqBOwn)
			if err != nil || respBOwn.StatusCode != http.StatusOK {
				t.Errorf("User B fetching own profile failed: %v status: %d", err, respBOwn.StatusCode)
			}
		},
	})
}
