package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hangry-coder/bffx/pkg/logger"
)

func generateBasics(projectDir, name string, opts ProjectOptions) error {
	// dummy worker scripts
	os.MkdirAll(filepath.Join(projectDir, "worker/skills"), 0o755)
	os.MkdirAll(filepath.Join(projectDir, "worker/functions"), 0o755)
	if err := os.WriteFile(filepath.Join(projectDir, "worker/skills/echo.py"), []byte("def run(input):\n    return input\n"), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(projectDir, "worker/functions/ping.py"), []byte("def run():\n    return 'pong'\n"), 0o644); err != nil {
		return err
	}

	// README.md
	readme := strings.Join([]string{
		"# 🚀 " + name + " (BFFX Powered)",
		"",
		"Congratulations! Your project is born with **Full Batteries Included**.",
		"",
		"## 🚀 Quick Start",
		"```bash",
		"bffx dev",
		"```",
		"",
		"---",
		"## 📜 License & Legal",
		"This project is generated using **BFFX**. The generated code is yours to use, modify, and distribute under your chosen license. Please note that BFFX framework core components are subject to their own respective OSS licenses. See [BFFX GitHub](https://github.com/hangry-coder/bffx) for details.",
	}, "\n")
	if err := os.WriteFile(filepath.Join(projectDir, "README.md"), []byte(readme), 0o644); err != nil {
		return err
	}

	// PR Template
	prTemplate := strings.Join([]string{
		"# Description",
		"Please include a summary of the change and which issue is fixed. Please also include relevant motivation and context.",
		"",
		"## Type of change",
		"- [ ] Bug fix (non-breaking change which fixes an issue)",
		"- [ ] New feature (non-breaking change which adds functionality)",
		"- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)",
		"- [ ] Documentation update",
		"",
		"## How Has This Been Tested?",
		"Please describe the tests that you ran to verify your changes.",
		"",
		"## Checklist:",
		"- [ ] My code follows the style guidelines of this project",
		"- [ ] I have performed a self-review of my own code",
		"- [ ] I have commented my code, particularly in hard-to-understand areas",
		"- [ ] My changes generate no new warnings",
		"- [ ] I have added tests that prove my fix is effective or that my feature works",
	}, "\n")
	os.MkdirAll(filepath.Join(projectDir, ".github"), 0o755)
	if err := os.WriteFile(filepath.Join(projectDir, ".github/pull_request_template.md"), []byte(prTemplate), 0o644); err != nil {
		return err
	}

	// tests
	if err := GenerateTests(projectDir, name); err != nil {
		return fmt.Errorf("generate tests: %w", err)
	}

	if err := GenerateGoMod(projectDir, name); err != nil {
		return err
	}

	if err := GenerateOrchestratorMain(projectDir, false); err != nil {
		return err
	}

	// Hooks (Batteries Included)
	userHooks := strings.Join([]string{
		"package hooks",
		"",
		"import (",
		"	\"github.com/hangry-coder/bffx/pkg/api/errors\"",
		"	\"github.com/hangry-coder/bffx/pkg/api/handlers\"",
		"	\"encoding/json\"",
		"	\"net/http\"",
		")",
		"",
		"// @bffx:hook",
		"func SendOTP(ctx *handlers.ActionContext, payload map[string]any) error {",
		"	emailAddr, _ := payload[\"email\"].(string)",
		"	userID, _ := payload[\"id\"].(string)",
		"	code, hashed, _, err := ctx.OTP.Generate()",
		"	if err != nil {",
		"		return err",
		"	}",
		"	ctx.Store.Update(ctx.Context, \"User\", userID, map[string]any{\"otp_code\": hashed})",
		"	return ctx.Comm.EmailMgr.Send(ctx.Context, \"default\", emailAddr, \"Verify\", \"Code: \"+code)",
		"}",
		"",
		"// @bffx:hook",
		"func CheckVerification(ctx *handlers.ActionContext, payload map[string]any) error {",
		"	return nil",
		"}",
		"",
		"// @bffx:hook",
		"func SendWelcome(ctx *handlers.ActionContext, payload map[string]any) error {",
		"	return nil",
		"}",
		"",
		"func HandleConvertGuestToUser(ctx *handlers.ActionContext, w http.ResponseWriter, r *http.Request) {",
		"	errors.WriteJSON(w, http.StatusOK, map[string]any{\"status\": \"converted\"})",
		"}",
		"",
		"func HandleVerifyOTP(ctx *handlers.ActionContext, w http.ResponseWriter, r *http.Request) {",
		"	var payload struct { Email string; Code string }",
		"	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {",
		"		errors.Write(w, errors.ErrBadRequest)",
		"		return",
		"	}",
		"	user, err := ctx.Store.GetByField(ctx.Context, \"User\", \"email\", payload.Email)",
		"	if err != nil {",
		"		errors.Write(w, errors.ErrNotFound)",
		"		return",
		"	}",
		"	hashedCode, _ := user[\"otp_code\"].(string)",
		"	if !ctx.OTP.Verify(hashedCode, payload.Code) {",
		"		errors.Write(w, errors.ErrUnauthorized)",
		"		return",
		"	}",
		"	ctx.Store.Update(ctx.Context, \"User\", user[\"id\"].(string), map[string]any{\"otp_code\": \"\", \"is_verified\": true})",
		"	errors.WriteJSON(w, http.StatusOK, map[string]any{\"status\": \"verified\"})",
		"}",
	}, "\n")
	paths := GetPaths(projectDir, "system", opts.Layout)
	os.MkdirAll(paths.Hooks, 0o755)
	if err := os.WriteFile(filepath.Join(paths.Hooks, "user.go"), []byte(userHooks), 0o644); err != nil {
		return err
	}

	return nil
}

func GenerateGoMod(projectDir, name string) error {
	goMod := strings.Join([]string{
		"module " + name,
		"",
		"go 1.26",
		"",
	}, "\n")
	return os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goMod), 0o644)
}

func GenerateOrchestratorMain(projectDir string, dryRun bool) error {
	cmdDirName := "orchestrator"
	if _, err := os.Stat(filepath.Join(projectDir, "cmd", "api")); err == nil {
		cmdDirName = "api"
	}

	regPath := filepath.Join(projectDir, "cmd", cmdDirName, "registry.gen.go")
	_, regErr := os.Stat(regPath)
	runServerCall := "if err := app.RunServer(context.Background(), \".\", port, nil, nil); err != nil {"
	if regErr == nil {
		// bffx sync emits registry.gen.go with ActionHandlers / HookHandlers — preserve wiring.
		runServerCall = "if err := app.RunServer(context.Background(), \".\", port, ActionHandlers, HookHandlers); err != nil {"
	}

	orchestratorMain := strings.Join([]string{
		"package main",
		"",
		"import (",
		"	\"github.com/hangry-coder/bffx/pkg/app\"",
		"	\"context\"",
		"	\"fmt\"",
		"	\"log\"",
		"	\"os\"",
		")",
		"",
		"func main() {",
		"	port := 8080",
		"	if p := os.Getenv(\"PORT\"); p != \"\" {",
		"		_, _ = fmt.Sscanf(p, \"%d\", &port)",
		"	}",
		"	" + runServerCall,
		"		log.Fatalf(\"server failed: %v\", err)",
		"	}",
		"}",
	}, "\n")
	outPath := filepath.Join(projectDir, "cmd", cmdDirName, "main.go")
	if dryRun {
		logger.Info("[dry-run] would write %s (%d bytes)", outPath, len(orchestratorMain))
		return nil
	}
	dir := filepath.Join(projectDir, "cmd", cmdDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(outPath, []byte(orchestratorMain), 0o644)
}
