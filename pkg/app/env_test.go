package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnv(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	
	content := `
# This is a comment
DB_URL=postgres://localhost:5432
API_KEY=12345
  # Indented comment
  SPACED_KEY =  spaced_value  
INVALID_LINE
`
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp .env: %v", err)
	}

	// Set an existing var to check override behavior
	os.Setenv("ALREADY_SET", "original")
	os.WriteFile(envPath, []byte(content+"\nALREADY_SET=new"), 0644)

	if err := LoadEnv(tempDir); err != nil {
		t.Fatalf("LoadEnv failed: %v", err)
	}

	tests := []struct {
		key   string
		want  string
	}{
		{"DB_URL", "postgres://localhost:5432"},
		{"API_KEY", "12345"},
		{"SPACED_KEY", "spaced_value"},
		{"ALREADY_SET", "original"},
	}

	for _, tt := range tests {
		got := os.Getenv(tt.key)
		if got != tt.want {
			t.Errorf("os.Getenv(%q) = %q; want %q", tt.key, got, tt.want)
		}
	}
}
