package handlers

import (
	"github.com/hangry-coder/bffx/pkg/comm"
	"github.com/hangry-coder/bffx/pkg/comm/email"
	"github.com/hangry-coder/bffx/pkg/featureflags"
	"github.com/hangry-coder/bffx/pkg/featureflags/providers"
	"github.com/hangry-coder/bffx/pkg/i18n"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestCommonHandler(t *testing.T) {
	s := storage.NewMemoryStore()
	bundle := i18n.NewBundle("en")

	var projSpec yaml.Node
	yaml.Unmarshal([]byte(`app: { name: TestApp, auth_strategy: optional }`), &projSpec)

	reg := &manifest.Registry{
		Project: &manifest.Manifest{Metadata: manifest.Metadata{Name: "TestApp"}, Spec: projSpec},
	}

	fs := featureflags.NewFlagService(reg, providers.NewBffxProvider(s, reg))
	h := NewCommonHandler(reg, fs, nil, bundle, s, nil)

	t.Run("Bootstrap", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/bootstrap", nil)
		rr := httptest.NewRecorder()
		h.Bootstrap(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var res map[string]any
		json.Unmarshal(rr.Body.Bytes(), &res)

		assert.Equal(t, "TestApp", res["app"].(map[string]any)["name"])
		assert.Equal(t, "optional", res["auth"].(map[string]any)["requirement"])
		assert.Contains(t, res, "navigation")
		assert.Contains(t, res, "features")
	})
}

func TestActionContext_UserID_and_T(t *testing.T) {
	ctx := context.Background()
	bundle := i18n.NewBundle("en")
	ac := &ActionContext{Bundle: bundle, Context: ctx, User: map[string]any{"id": "from-user"}}
	assert.Equal(t, "from-user", ac.UserID())
	ac2 := &ActionContext{Bundle: bundle, Context: ctx, Claims: map[string]any{"sub": "from-claims"}}
	assert.Equal(t, "from-claims", ac2.UserID())
	ac3 := &ActionContext{Bundle: bundle, Context: ctx}
	assert.Equal(t, "", ac3.UserID())
	assert.Equal(t, "missing.key", ac3.T("missing.key"))
}

func TestActionContext_SendEmailTemplate(t *testing.T) {
	// Create a temporary project root
	tmp := t.TempDir()

	// Create email templates directory inside tmp
	dir := filepath.Join(tmp, "assets", "emails")
	err := os.MkdirAll(dir, 0o755)
	assert.NoError(t, err)

	subjectTemplate := `Subject: Welcome {{.Name}}`
	htmlTemplate := `<p>Hello {{.Name}}</p>`
	txtTemplate := `Hello {{.Name}}`

	err = os.WriteFile(filepath.Join(dir, "welcome.subject"), []byte(subjectTemplate), 0o644)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "welcome.html"), []byte(htmlTemplate), 0o644)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "welcome.txt"), []byte(txtTemplate), 0o644)
	assert.NoError(t, err)

	// Temporarily switch working directory to tmp so that "." resolves to tmp
	oldWd, err := os.Getwd()
	assert.NoError(t, err)
	err = os.Chdir(tmp)
	assert.NoError(t, err)
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	// Mock email provider
	mgr := email.NewManager()
	mgr.RegisterProvider("default", &mockEmailProvider{})

	// Create comm Hub
	hub := comm.NewHub(nil, nil, mgr, nil)

	ac := &ActionContext{
		Comm:    hub,
		Context: context.Background(),
	}

	// Test sending existing template
	err = ac.SendEmailTemplate("test@example.com", "welcome", map[string]string{"Name": "John"})
	assert.NoError(t, err)

	// Test sending missing template
	err = ac.SendEmailTemplate("test@example.com", "missing", map[string]string{"Name": "John"})
	assert.Error(t, err)
}

type mockEmailProvider struct{}

func (m *mockEmailProvider) Send(ctx context.Context, to, subject, body string) error {
	return nil
}
