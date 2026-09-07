package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func generateOnboarding(projectDir, name string, opts ProjectOptions) error {
	if opts.Minimal {
		return nil
	}

	// Common screens for all cases. Home is created in generateUI.
	if err := GenerateScreen(projectDir, "IntroCarousel", ScreenOptions{Layout: opts.Layout, NavType: "onboarding", Group: "mobile"}); err != nil {
		return fmt.Errorf("generate IntroCarousel screen: %w", err)
	}
	if err := GenerateScreen(projectDir, "PermissionRequest", ScreenOptions{Layout: opts.Layout, NavType: "onboarding", Group: "mobile"}); err != nil {
		return fmt.Errorf("generate PermissionRequest screen: %w", err)
	}

	// Seed Intro Config
	introSeed := strings.Join([]string{
		"apiVersion: bffx.io/v1alpha1",
		"kind: Blueprint",
		"metadata:",
		"  name: IntroConfig",
		"spec:",
		"  resource: AppConfig",
		"  items:",
		"    - { key: \"onboarding.intro.title_1\", value: \"Welcome to " + name + "\" }",
		"    - { key: \"onboarding.intro.subtitle_1\", value: \"Your tagline goes here\" }",
		"    - { key: \"onboarding.intro.title_2\", value: \"Feature Two\" }",
		"    - { key: \"onboarding.intro.title_3\", value: \"Feature Three\" }",
	}, "\n")
	paths := GetPaths(projectDir, "mobile", opts.Layout)
	os.MkdirAll(paths.Blueprints, 0o755)
	if err := os.WriteFile(filepath.Join(paths.Blueprints, "onboarding_intro.yaml"), []byte(introSeed), 0o644); err != nil {
		return err
	}

	authStrategy := opts.AuthStrategy
	if authStrategy == "" {
		authStrategy = "optional"
	}

	switch authStrategy {
	case "anonymous":
		// Case A: Anonymous Use
		deviceFields, _ := ParseFields([]string{"platform:string", "os_version:string", "is_onboarded:bool"})
		if err := GenerateResource(projectDir, "Device", deviceFields, ResourceOptions{Layout: opts.Layout, Group: "mobile", ReadPolicy: "public", WritePolicy: "authenticated"}); err != nil {
			return fmt.Errorf("generate Device resource: %w", err)
		}

	case "optional":
		// Case B: Optional Login
		deviceFields, _ := ParseFields([]string{"platform:string", "is_onboarded:bool", "is_guest:bool"})
		if err := GenerateResource(projectDir, "Device", deviceFields, ResourceOptions{Layout: opts.Layout, Group: "mobile", ReadPolicy: "public", WritePolicy: "authenticated"}); err != nil {
			return fmt.Errorf("generate Device resource: %w", err)
		}

		// Screens
		if err := GenerateScreen(projectDir, "AuthGateway", ScreenOptions{Layout: opts.Layout, NavType: "onboarding", Group: "mobile"}); err != nil {
			return fmt.Errorf("generate AuthGateway screen: %w", err)
		}
		if err := GenerateScreen(projectDir, "QuickPref", ScreenOptions{Layout: opts.Layout, NavType: "onboarding", Group: "mobile"}); err != nil {
			return fmt.Errorf("generate QuickPref screen: %w", err)
		}
		if err := GenerateScreen(projectDir, "Signup", ScreenOptions{Layout: opts.Layout, NavType: "auth_flow", Group: "mobile"}); err != nil {
			return fmt.Errorf("generate Signup screen: %w", err)
		}
		if err := GenerateScreen(projectDir, "Login", ScreenOptions{Layout: opts.Layout, NavType: "auth_flow", Group: "mobile"}); err != nil {
			return fmt.Errorf("generate Login screen: %w", err)
		}
		if err := GenerateScreen(projectDir, "UserProfile", ScreenOptions{Layout: opts.Layout, NavType: "folded", Icon: "person", Group: "mobile"}); err != nil {
			return fmt.Errorf("generate UserProfile screen: %w", err)
		}

		// Custom Action: ConvertGuestToUser
		convertAction := strings.Join([]string{
			"apiVersion: bffx.io/v1alpha1",
			"kind: Action",
			"metadata:",
			"  name: ConvertGuestToUser",
			"spec:",
			"  group: mobile",
			"  route:",
			"    method: POST",
			"    path: /api/v1/onboarding/convert-guest",
			"    auth: optional",
		}, "\n")
		paths = GetPaths(projectDir, "mobile", opts.Layout)
		os.MkdirAll(paths.Actions, 0o755)
		if err := os.WriteFile(filepath.Join(paths.Actions, "ConvertGuestToUser.yaml"), []byte(convertAction), 0o644); err != nil {
			return err
		}
		if opts.Layout == LayoutV2 {
			os.MkdirAll(paths.Hooks, 0o755)
			convertStub := strings.Join([]string{
				"package hooks",
				"",
				"import (",
				"	\"github.com/hangry-coder/bffx/pkg/api/handlers\"",
				"	\"net/http\"",
				")",
				"",
				"// HandleConvertGuestToUser handles guest to registered user conversion.",
				"func HandleConvertGuestToUser(ctx *handlers.ActionContext, w http.ResponseWriter, r *http.Request) {",
				"	w.WriteHeader(http.StatusOK)",
				"}",
			}, "\n")
			if err := os.WriteFile(filepath.Join(paths.Hooks, "mobile.go"), []byte(convertStub), 0o644); err != nil {
				return err
			}
		}

	case "mandatory":
		// Case C: Must Login
		surveyFields, _ := ParseFields([]string{"user_id:string", "question:string", "answer:string"})
		if err := GenerateResource(projectDir, "OnboardingSurvey", surveyFields, ResourceOptions{Layout: opts.Layout, Group: "mobile", ReadPolicy: "owner", WritePolicy: "owner"}); err != nil {
			return fmt.Errorf("generate OnboardingSurvey resource: %w", err)
		}

		// Extend User schema
		sysPaths := GetPaths(projectDir, "system", opts.Layout)
		userPath := filepath.Join(sysPaths.Manifests, "user.yaml")
		userData, _ := os.ReadFile(userPath)
		extraFields := "    - { name: bio, type: string }\n    - { name: profile_pic_url, type: string }\n    - { name: is_onboarded, type: bool }"
		userContent := strings.Replace(string(userData), "    - { name: whatsapp, type: string }", "    - { name: whatsapp, type: string }\n"+extraFields, 1)
		if err := os.WriteFile(userPath, []byte(userContent), 0o644); err != nil {
			return err
		}

		// Screens
		if err := GenerateScreen(projectDir, "AuthGateway", ScreenOptions{Layout: opts.Layout, NavType: "onboarding", Group: "mobile"}); err != nil {
			return fmt.Errorf("generate AuthGateway screen: %w", err)
		}
		if err := GenerateScreen(projectDir, "Signup", ScreenOptions{Layout: opts.Layout, NavType: "auth_flow", Group: "mobile"}); err != nil {
			return fmt.Errorf("generate Signup screen: %w", err)
		}
		if err := GenerateScreen(projectDir, "Login", ScreenOptions{Layout: opts.Layout, NavType: "auth_flow", Group: "mobile"}); err != nil {
			return fmt.Errorf("generate Login screen: %w", err)
		}
		if err := GenerateScreen(projectDir, "PersonalizationStep", ScreenOptions{Layout: opts.Layout, NavType: "onboarding", Group: "mobile"}); err != nil {
			return fmt.Errorf("generate PersonalizationStep screen: %w", err)
		}
		if err := GenerateScreen(projectDir, "UserProfile", ScreenOptions{Layout: opts.Layout, NavType: "folded", Icon: "person", Group: "mobile"}); err != nil {
			return fmt.Errorf("generate UserProfile screen: %w", err)
		}

		// Custom Action: CompleteProfile
		completeAction := strings.Join([]string{
			"apiVersion: bffx.io/v1alpha1",
			"kind: Action",
			"metadata:",
			"  name: CompleteProfile",
			"spec:",
			"  group: mobile",
			"  route:",
			"    method: POST",
			"    path: /api/v1/onboarding/complete-profile",
			"    auth: required",
		}, "\n")
		paths = GetPaths(projectDir, "mobile", opts.Layout)
		os.MkdirAll(paths.Actions, 0o755)
		if err := os.WriteFile(filepath.Join(paths.Actions, "CompleteProfile.yaml"), []byte(completeAction), 0o644); err != nil {
			return err
		}
		if opts.Layout == LayoutV2 {
			os.MkdirAll(paths.Hooks, 0o755)
			completeStub := strings.Join([]string{
				"package hooks",
				"",
				"import (",
				"	\"github.com/hangry-coder/bffx/pkg/api/handlers\"",
				"	\"net/http\"",
				")",
				"",
				"// HandleCompleteProfile handles complete profile onboarding actions.",
				"func HandleCompleteProfile(ctx *handlers.ActionContext, w http.ResponseWriter, r *http.Request) {",
				"	w.WriteHeader(http.StatusOK)",
				"}",
			}, "\n")
			if err := os.WriteFile(filepath.Join(paths.Hooks, "mobile.go"), []byte(completeStub), 0o644); err != nil {
				return err
			}
		} else {
			os.MkdirAll(paths.Hooks, 0o755)
			completeStub := strings.Join([]string{
				"package hooks",
				"",
				"import (",
				"	\"github.com/hangry-coder/bffx/pkg/api/handlers\"",
				"	\"net/http\"",
				")",
				"",
				"// HandleCompleteProfile handles complete profile onboarding actions.",
				"func HandleCompleteProfile(ctx *handlers.ActionContext, w http.ResponseWriter, r *http.Request) {",
				"	w.WriteHeader(http.StatusOK)",
				"}",
			}, "\n")
			if err := os.WriteFile(filepath.Join(paths.Hooks, "complete_profile.go"), []byte(completeStub), 0o644); err != nil {
				return err
			}
		}
	}

	return nil
}
