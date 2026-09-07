package generator

import (
	"os"
	"path/filepath"
	"strings"
)

func generateI18n(projectDir, name string, opts ProjectOptions) error {
	enYaml := strings.Join([]string{
		"welcome_message: \"Welcome to " + name + "!\"",
		"error_unauthorized: \"You must be logged in to access this.\"",
		"error_not_found: \"The requested resource was not found.\"",
		"login_btn: \"Log In\"",
		"signup_btn: \"Sign Up\"",
		"home_tab: \"Dashboard\"",
		"settings_tab: \"Settings\"",
	}, "\n")

	i18nDir := filepath.Join(projectDir, "i18n")
	if opts.Layout == LayoutV2 {
		i18nDir = filepath.Join(projectDir, "assets", "i18n")
	}

	os.MkdirAll(i18nDir, 0o755)
	return os.WriteFile(filepath.Join(i18nDir, "en.yaml"), []byte(enYaml), 0o644)
}

func generateEmailTemplates(projectDir, name string, opts ProjectOptions) error {
	if opts.Layout != LayoutV2 {
		return nil
	}
	emailDir := filepath.Join(projectDir, "assets", "emails")
	if err := os.MkdirAll(emailDir, 0o755); err != nil {
		return err
	}
	welcomeHtml := `<!DOCTYPE html>
<html>
<head>
    <title>Welcome to {{.name}}</title>
</head>
<body>
    <h1>Welcome, {{.email}}!</h1>
    <p>We are thrilled to have you onboard.</p>
</body>
</html>`
	return os.WriteFile(filepath.Join(emailDir, "welcome.html"), []byte(welcomeHtml), 0o644)
}

func generateUI(projectDir, name string, opts ProjectOptions) error {
	homeAuth := "required"
	if opts.AuthStrategy == "optional" {
		homeAuth = "optional"
	}

	// Home Screen manifest (Unified: Nav + Data)
	homeYaml := strings.Join([]string{
		"apiVersion: bffx.io/v1alpha1",
		"kind: Screen",
		"metadata:",
		"  name: Home",
		"spec:",
		"  group: mobile",
		"  name: Home",
		"  nav_type: bottom",
		"  icon: home",
		"  order: 1",
		"  requires_auth: " + homeAuth,
		"  route:",
		"    method: GET",
		"    path: /api/v1/screens/home",
		"    auth: " + homeAuth,
		"  sources:",
		"    - currentUser",
		"  output:",
		"    user: currentUser",
		"",
	}, "\n")
	paths := GetPaths(projectDir, "mobile", opts.Layout)
	os.MkdirAll(paths.Screens, 0o755)
	if err := os.WriteFile(filepath.Join(paths.Screens, "home.yaml"), []byte(homeYaml), 0o644); err != nil {
		return err
	}

	if !opts.Minimal {
		// OTP Action
		otpYaml := strings.Join([]string{
			"apiVersion: bffx.io/v1alpha1",
			"kind: Action",
			"metadata:",
			"  name: VerifyOTP",
			"spec:",
			"  route:",
			"    method: POST",
			"    path: /api/v1/auth/verify",
			"    auth: optional",
			"",
		}, "\n")
		os.MkdirAll(paths.Actions, 0o755)
		if err := os.WriteFile(filepath.Join(paths.Actions, "verify.yaml"), []byte(otpYaml), 0o644); err != nil {
			return err
		}

		// ConnectedDevices screen manifest
		devicesYaml := strings.Join([]string{
			"apiVersion: bffx.io/v1alpha1",
			"kind: Screen",
			"metadata:",
			"  name: ConnectedDevices",
			"spec:",
			"  group: mobile",
			"  name: Devices",
			"  nav_type: folded",
			"  icon: devices",
			"  requires_auth: required",
			"  route:",
			"    method: GET",
			"    path: /api/v1/screens/settings/devices",
			"    auth: required",
			"  sources:",
			"    - { kind: Resource, name: Device, query: { user_id: currentUser.id } }",
			"  output:",
			"    devices: Device",
			"",
		}, "\n")
		os.MkdirAll(paths.Screens, 0o755)
		if err := os.WriteFile(filepath.Join(paths.Screens, "devices.yaml"), []byte(devicesYaml), 0o644); err != nil {
			return err
		}
	}

	// bootstrap builder
	bootstrapYaml := strings.Join([]string{
		"apiVersion: bffx.io/v1alpha1",
		"kind: Builder",
		"metadata:",
		"  name: Bootstrap",
		"spec:",
		"  route:",
		"    method: GET",
		"    path: /api/v1/app/bootstrap",
		"    auth: optional",
		"  sources:",
		"    - app",
		"    - currentUser",
		"    - i18n",
		"    - navigation",
		"    - onboarding",
		"    - { kind: Resource, name: AppConfig }",
		"  output:",
		"    app: app",
		"    auth.user: currentUser",
		"    translations: i18n",
		"    navigation: navigation",
		"    onboarding: onboarding",
		"    config: AppConfig",
		"",
	}, "\n")
	os.MkdirAll(paths.Builders, 0o755)
	if err := os.WriteFile(filepath.Join(paths.Builders, "bootstrap.yaml"), []byte(bootstrapYaml), 0o644); err != nil {
		return err
	}

	return nil
}
