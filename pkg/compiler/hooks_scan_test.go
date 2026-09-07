package compiler

import (
	"strings"
	"testing"
)

func TestIsRouterHookFuncSignature(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{`func SendOTP(ctx *handlers.ActionContext, payload map[string]any) error {`, true},
		{`func BeforeCreateNote(ctx *handlers.ActionContext, payload map[string]any) error {`, true},
		{`func AfterCreateNote(ctx *handlers.ActionContext, result map[string]any) error {`, true},
		{`func BeforeReceiptScan(ctx *handlers.ActionContext, payload map[string]any) error {`, true},
		{`func HandleShareNote(ctx *handlers.ActionContext, w http.ResponseWriter, r *http.Request) {`, false},
		{`func UpsertWidgetSnapshot(ctx context.Context, store storage.Store, userID, snapshotType string, payload map[string]any) (map[string]any, error) {`, false},
		{`func (h *Helper) Run(ctx *handlers.ActionContext, payload map[string]any) error {`, false},
	}
	for _, tc := range tests {
		if got := isRouterHookFuncSignature(tc.line); got != tc.want {
			t.Errorf("isRouterHookFuncSignature(%q) = %v, want %v", tc.line, got, tc.want)
		}
	}
}

func TestShouldRegisterPayloadHook(t *testing.T) {
	hook := hookFuncDecl{
		Name:      "SendOTP",
		Signature: `func SendOTP(ctx *handlers.ActionContext, payload map[string]any) error {`,
	}
	if !shouldRegisterPayloadHook(hook) {
		t.Fatal("expected SendOTP to register")
	}

	util := hookFuncDecl{
		Name:      "UpsertWidgetSnapshot",
		Signature: `func UpsertWidgetSnapshot(ctx context.Context, store storage.Store, userID string, payload map[string]any) (map[string]any, error) {`,
	}
	if shouldRegisterPayloadHook(util) {
		t.Fatal("expected utility helper to be skipped")
	}

	skipped := hookFuncDecl{
		Name:      "SendOTP",
		Comments:  "// @bffx:skip-hook",
		Signature: `func SendOTP(ctx *handlers.ActionContext, payload map[string]any) error {`,
	}
	if shouldRegisterPayloadHook(skipped) {
		t.Fatal("expected @bffx:skip-hook to exclude hook")
	}

	pipeline := hookFuncDecl{
		Name:      "BeforeReceiptScan",
		Comments:  "// @bffx:action",
		Signature: `func BeforeReceiptScan(ctx *handlers.ActionContext, payload map[string]any) error {`,
	}
	if !shouldRegisterPayloadHook(pipeline) {
		t.Fatal("expected annotated pipeline hook to register")
	}

	badAnnotation := hookFuncDecl{
		Name:      "NotAHook",
		Comments:  "// @bffx:hook",
		Signature: `func NotAHook(ctx context.Context) error {`,
	}
	if shouldRegisterPayloadHook(badAnnotation) {
		t.Fatal("expected invalid signature to be rejected even with @bffx:hook")
	}

	unexported := hookFuncDecl{
		Name:      "sendOTPToUser",
		Signature: `func sendOTPToUser(ctx *handlers.ActionContext, user map[string]any) error {`,
	}
	if shouldRegisterPayloadHook(unexported) {
		t.Fatal("expected unexported helper to be skipped")
	}
}

func TestParseHookDeclsFromSource(t *testing.T) {
	src := `package hooks

// SendOTP delivers a verification code.
func SendOTP(ctx *handlers.ActionContext, payload map[string]any) error {
	return nil
}

// @bffx:skip-hook
// UpsertWidgetSnapshot stores widget state for other hooks.
func UpsertWidgetSnapshot(ctx context.Context, store storage.Store, userID string, payload map[string]any) (map[string]any, error) {
	return nil, nil
}
`
	decls := parseHookDeclsFromSource(src)
	if len(decls) != 2 {
		t.Fatalf("expected 2 decls, got %d", len(decls))
	}
	if decls[0].Name != "SendOTP" || !strings.Contains(decls[0].Comments, "SendOTP delivers") {
		t.Fatalf("unexpected first decl: %+v", decls[0])
	}
	if decls[1].Name != "UpsertWidgetSnapshot" || !strings.Contains(decls[1].Comments, "@bffx:skip-hook") {
		t.Fatalf("unexpected second decl: %+v", decls[1])
	}
}
