package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hangry-coder/bffx/pkg/auth"
)

func generateCoreResources(projectDir, name string, opts ProjectOptions) error {
	// User resource
	fields, _ := ParseFields([]string{"email:string", "password:string", "name:string", "role:string", "status:string", "device_id:string", "phone:string", "whatsapp:string", "is_verified:bool", "otp_code:string", "google_sub:string", "apple_sub:string"})
	if err := GenerateResource(projectDir, "User", fields, ResourceOptions{Layout: opts.Layout, WithHooks: true, Group: "system", ReadPolicy: "owner", WritePolicy: "authenticated"}); err != nil {
		return fmt.Errorf("generate User resource: %w", err)
	}

	// Inject Granular Hooks into User manifest
	paths := GetPaths(projectDir, "system", opts.Layout)
	userPath := filepath.Join(paths.Manifests, "user.yaml")
	userManifestHooks := strings.Join([]string{
		"  hooks:",
		"    afterCreate:",
		"      - { action: SendOTP, mode: async }",
		"    afterUpdate:",
		"      - { action: CheckVerification, mode: sync }",
		"      - { action: SendWelcome, mode: async }",
	}, "\n")
	userData, _ := os.ReadFile(userPath)
	userContent := strings.Replace(string(userData), "  hooks: {}", userManifestHooks, 1)
	if err := os.WriteFile(userPath, []byte(userContent), 0o644); err != nil {
		return err
	}

	if err := GenerateBlueprint(projectDir, "User", fields, ResourceOptions{Layout: opts.Layout, Group: "system"}); err != nil {
		return fmt.Errorf("generate User blueprint: %w", err)
	}
	if err := GenerateResourceSpec(projectDir, "User", fields, ResourceOptions{Layout: opts.Layout, Group: "system"}); err != nil {
		return fmt.Errorf("generate User spec: %w", err)
	}

	// Admin Dashboard
	if opts.AdminEnabled {
		adminFields, _ := ParseFields([]string{"email:string:unique", "password:string", "role:string"})
		if err := GenerateResource(projectDir, "AdminUser", adminFields, ResourceOptions{Layout: opts.Layout, Group: "admin", ReadPolicy: "admin", WritePolicy: "admin", SkipBlueprint: true}); err != nil {
			return fmt.Errorf("generate AdminUser resource: %w", err)
		}

		adminHash, _ := auth.HashPassword(opts.AdminPassword)
		adminYaml := strings.Join([]string{
			"apiVersion: bffx.io/v1alpha1",
			"kind: Blueprint",
			"metadata:",
			"  name: InitialAdmin",
			"spec:",
			"  resource: AdminUser",
			"  count: 1",
			"  defaults:",
			"    email: " + opts.AdminEmail,
			"    password: \"" + adminHash + "\"",
			"    role: superadmin",
		}, "\n")
		paths = GetPaths(projectDir, "admin", opts.Layout)
		os.MkdirAll(paths.Blueprints, 0o755)
		if err := os.WriteFile(filepath.Join(paths.Blueprints, "admin.yaml"), []byte(adminYaml), 0o644); err != nil {
			return err
		}
	}

	// Monetization
	if opts.WithMonetization {
		subFields, _ := ParseFields([]string{"user_id:string", "plan_id:string", "status:string", "expires_at:string"})
		if err := GenerateResource(projectDir, "Subscription", subFields, ResourceOptions{Layout: opts.Layout, Group: "system"}); err != nil {
			return fmt.Errorf("generate Subscription resource: %w", err)
		}
		if err := GenerateBlueprint(projectDir, "Subscription", subFields, ResourceOptions{Layout: opts.Layout, Group: "system"}); err != nil {
			return fmt.Errorf("generate Subscription blueprint: %w", err)
		}

		planFields, _ := ParseFields([]string{"name:string", "price:float", "interval:string", "description:string"})
		if err := GenerateResource(projectDir, "Plan", planFields, ResourceOptions{Layout: opts.Layout, Group: "system"}); err != nil {
			return fmt.Errorf("generate Plan resource: %w", err)
		}
		if err := GenerateBlueprint(projectDir, "Plan", planFields, ResourceOptions{Layout: opts.Layout, Group: "system"}); err != nil {
			return fmt.Errorf("generate Plan blueprint: %w", err)
		}

		entFields, _ := ParseFields([]string{"user_id:string", "key:string", "granted:bool"})
		if err := GenerateResource(projectDir, "Entitlement", entFields, ResourceOptions{Layout: opts.Layout, Group: "system"}); err != nil {
			return fmt.Errorf("generate Entitlement resource: %w", err)
		}
		if err := GenerateBlueprint(projectDir, "Entitlement", entFields, ResourceOptions{Layout: opts.Layout, Group: "system"}); err != nil {
			return fmt.Errorf("generate Entitlement blueprint: %w", err)
		}
	}

	// Flags
	if opts.WithFlags {
		flagFields, _ := ParseFields([]string{"key:string", "enabled:bool", "rules:string", "description:string"})
		if err := GenerateResource(projectDir, "FeatureFlag", flagFields, ResourceOptions{Layout: opts.Layout, Group: "system"}); err != nil {
			return fmt.Errorf("generate FeatureFlag resource: %w", err)
		}
		if err := GenerateBlueprint(projectDir, "FeatureFlag", flagFields, ResourceOptions{Layout: opts.Layout, Group: "system"}); err != nil {
			return fmt.Errorf("generate FeatureFlag blueprint: %w", err)
		}

		rolloutFields, _ := ParseFields([]string{"flag_id:string", "percentage:int", "release_id:string"})
		if err := GenerateResource(projectDir, "Rollout", rolloutFields, ResourceOptions{Layout: opts.Layout, Group: "system"}); err != nil {
			return fmt.Errorf("generate Rollout resource: %w", err)
		}
		if err := GenerateBlueprint(projectDir, "Rollout", rolloutFields, ResourceOptions{Layout: opts.Layout, Group: "system"}); err != nil {
			return fmt.Errorf("generate Rollout blueprint: %w", err)
		}
	}

	// Telemetry
	if opts.WithTelemetry {
		telemetryFields, _ := ParseFields([]string{"user_id:string", "event_type:string", "screen_name:string", "action_name:string", "properties:string"})
		if err := GenerateResource(projectDir, "TelemetryEvent", telemetryFields, ResourceOptions{Layout: opts.Layout, Group: "system"}); err != nil {
			return fmt.Errorf("generate TelemetryEvent resource: %w", err)
		}

		paths = GetPaths(projectDir, "system", opts.Layout)
		telemetryPath := filepath.Join(paths.Manifests, "telemetryevent.yaml")
		telData, _ := os.ReadFile(telemetryPath)
		telContent := strings.Replace(string(telData), "spec:", "spec:\n  telemetry: true", 1)
		if err := os.WriteFile(telemetryPath, []byte(telContent), 0o644); err != nil {
			return err
		}

		if err := GenerateBlueprint(projectDir, "TelemetryEvent", telemetryFields, ResourceOptions{Layout: opts.Layout, Group: "system"}); err != nil {
			return fmt.Errorf("generate TelemetryEvent blueprint: %w", err)
		}

		bpPath := filepath.Join(paths.Blueprints, "telemetryevent.yaml")
		bpData, _ := os.ReadFile(bpPath)
		bpContent := strings.Replace(string(bpData), "spec:", "spec:\n  count: 20", 1)
		if err := os.WriteFile(bpPath, []byte(bpContent), 0o644); err != nil {
			return err
		}

		if err := GenerateResourceSpec(projectDir, "TelemetryEvent", telemetryFields, ResourceOptions{Layout: opts.Layout, Group: "system"}); err != nil {
			return fmt.Errorf("generate TelemetryEvent spec: %w", err)
		}
	}

	// Ads
	if opts.WithAds {
		adFields, _ := ParseFields([]string{"provider:string", "unit_id:string", "type:string", "enabled:bool"})
		if err := GenerateResource(projectDir, "AdUnit", adFields, ResourceOptions{Layout: opts.Layout, Group: "system"}); err != nil {
			return fmt.Errorf("generate AdUnit resource: %w", err)
		}
		if err := GenerateBlueprint(projectDir, "AdUnit", adFields, ResourceOptions{Layout: opts.Layout, Group: "system"}); err != nil {
			return fmt.Errorf("generate AdUnit blueprint: %w", err)
		}
	}

	// AuditLog Resource (Security)
	auditFields, _ := ParseFields([]string{"actor_id:string", "actor_type:string", "action:string", "resource_kind:string", "resource_id:string", "payload:string", "ip_address:string"})
	if err := GenerateResource(projectDir, "AuditLog", auditFields, ResourceOptions{Layout: opts.Layout, Group: "system", ReadPolicy: "admin", WritePolicy: "admin"}); err != nil {
		return fmt.Errorf("generate AuditLog resource: %w", err)
	}

	// Always generate AppString for i18n
	appStringFields, _ := ParseFields([]string{"key:string", "locale:string", "value:string"})
	if err := GenerateResource(projectDir, "AppString", appStringFields, ResourceOptions{Layout: opts.Layout, Group: "mobile", ReadPolicy: "public", WritePolicy: "admin"}); err != nil {
		return fmt.Errorf("generate AppString resource: %w", err)
	}

	// Seed some default AppStrings
	appStringYaml := strings.Join([]string{
		"apiVersion: bffx.io/v1alpha1",
		"kind: Blueprint",
		"metadata:",
		"  name: DefaultAppStrings",
		"spec:",
		"  resource: AppString",
		"  count: 3",
		"  defaults:",
		"    locale: en",
		"  items:",
		"    - { key: \"login_btn\", value: \"Login to Your Account\" }",
		"    - { key: \"home_tab\", value: \"Home\" }",
		"    - { key: \"settings_tab\", value: \"Settings\" }",
	}, "\n")
	paths = GetPaths(projectDir, "mobile", opts.Layout)
	os.MkdirAll(paths.Blueprints, 0o755)
	if err := os.WriteFile(filepath.Join(paths.Blueprints, "appstrings.yaml"), []byte(appStringYaml), 0o644); err != nil {
		return err
	}

	// Add AppConfig for Mobile Meta Info (Icons, Colors, etc.)
	appConfigFields, _ := ParseFields([]string{"key:string", "value:string", "type:string", "description:string"})
	if err := GenerateResource(projectDir, "AppConfig", appConfigFields, ResourceOptions{Layout: opts.Layout, Group: "mobile", ReadPolicy: "public", WritePolicy: "admin"}); err != nil {
		return fmt.Errorf("generate AppConfig resource: %w", err)
	}

	// Add AppPermission for Phase 2
	appPermissionFields, _ := ParseFields([]string{"device_id:string", "location_granted:bool", "notifications_granted:bool", "push_granted:bool"})
	if err := GenerateResource(projectDir, "AppPermission", appPermissionFields, ResourceOptions{Layout: opts.Layout, Group: "mobile", ReadPolicy: "owner", WritePolicy: "owner"}); err != nil {
		return fmt.Errorf("generate AppPermission resource: %w", err)
	}

	// Scaffold example SessionCleanup CronJob manifest for v2 slices
	if opts.Layout == LayoutV2 {
		if err := GenerateCronJob(projectDir, "SessionCleanup", "system", opts.Layout); err != nil {
			return fmt.Errorf("generate example SessionCleanup cronjob: %w", err)
		}
	}

	return nil
}
