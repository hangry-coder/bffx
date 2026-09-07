package e2e

import (
	"testing"
)

func TestPersonaBootAllLocal(t *testing.T) {
	personas := LocalPersonas()
	mutation := AuthMutation{Name: "None", Strategy: ""}

	for _, p := range personas {
		t.Run(p.Name, func(t *testing.T) {
			ts := NewTestServer(t, p, mutation)
			
			resp, err := ts.GET("/health", nil)
			if err != nil {
				t.Fatalf("Failed to GET /health: %v", err)
			}
			if resp.StatusCode != 200 {
				t.Errorf("Expected 200, got %d", resp.StatusCode)
			}
			resp.Body.Close()
		})
	}
}
