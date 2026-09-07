package email

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenderFileTemplates(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "assets", "emails")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	html := `<p>Hello {{.Name}}</p>`
	if err := os.WriteFile(filepath.Join(dir, "welcome.html"), []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}
	_, body, _, err := RenderFileTemplates(tmp, "welcome", map[string]string{"Name": "Ada"})
	if err != nil {
		t.Fatal(err)
	}
	if body != "<p>Hello Ada</p>" {
		t.Fatalf("body: %q", body)
	}
}
