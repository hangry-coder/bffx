package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/hangry-coder/bffx/pkg/deploy"
	"github.com/hangry-coder/bffx/pkg/doctor"
	"github.com/hangry-coder/bffx/pkg/generator"
	"github.com/hangry-coder/bffx/pkg/generator/archetypes"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
)

func HandleNew(args []string) {
	for _, arg := range args {
		if arg == "--list-archetypes" {
			reg := archetypes.GetRegistry()
			fmt.Println("🚀 Available BFFX Vertical Archetypes:")
			fmt.Println("--------------------------------------------------------------------------------")
			fmt.Printf("%-15s | %-10s | %-8s | %s\n", "Archetype", "Layout", "Auth", "Description")
			fmt.Println("--------------------------------------------------------------------------------")
			for _, arch := range reg.Archetypes {
				fmt.Printf("%-15s | %-10s | %-8s | %s\n", arch.Name, arch.Layout, arch.AuthStrategy, arch.Description)
			}
			fmt.Println("--------------------------------------------------------------------------------")
			return
		}
	}

	var name string
	var archetypeAlias string
	var pipelineOverride string

	opts := generator.ProjectOptions{
		Layout:           generator.LayoutV2,
		StoreMode:        "",
		StreamingEnabled: true,
		AdminEnabled:     true,
		AdminEmail:       "admin@example.com",
	}

	nonInteractive := false
	devVendor := false
	parentDir := "."

	// 1. Look for config file and load it
	var configPath string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--config=") {
			configPath = strings.TrimPrefix(arg, "--config=")
		} else if arg == "--config" && i+1 < len(args) {
			configPath = args[i+1]
		}
	}

	if configPath != "" {
		fmt.Printf("🚀 Loading %s...\n", configPath)
		cfg, err := generator.LoadUserConfig(configPath)
		if err != nil {
			log.Fatalf("failed to load config: %v", err)
		}
		nonInteractive = true
		if cfg.Name != "" {
			name = cfg.Name
		}
		if cfg.Archetype != "" {
			archetypeAlias = cfg.Archetype
		}
		if cfg.Layout != "" {
			opts.Layout = generator.LayoutType(cfg.Layout)
		}
		if cfg.Minimal {
			opts.Minimal = true
		}
		if cfg.Admin != nil {
			opts.AdminEnabled = cfg.Admin.Enabled
			opts.AdminEmail = cfg.Admin.Email
			opts.AdminPassword = cfg.Admin.Password
		}
		if cfg.Batteries != nil {
			opts.Batteries = cfg.Batteries
			if cfg.Batteries.Store != "" {
				opts.StoreMode = cfg.Batteries.Store
			}
		}
		if cfg.Env != nil {
			opts.Env = cfg.Env
		}
	}

	// 2. Parse CLI args to allow overrides (CLI flags > config file > defaults)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--config=") || arg == "--config" {
			if arg == "--config" {
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "--archetype=") {
			archetypeAlias = strings.TrimPrefix(arg, "--archetype=")
			nonInteractive = true
		} else if arg == "--archetype" && i+1 < len(args) {
			archetypeAlias = args[i+1]
			nonInteractive = true
			i++
		} else if strings.HasPrefix(arg, "--pipeline=") {
			pipelineOverride = strings.TrimPrefix(arg, "--pipeline=")
			nonInteractive = true
		} else if arg == "--pipeline" && i+1 < len(args) {
			pipelineOverride = args[i+1]
			nonInteractive = true
			i++
		} else {
			switch arg {
			case "--root", "--parent-dir":
				if i+1 < len(args) {
					parentDir = args[i+1]
					i++
				}
			case "--store":
				if i+1 < len(args) {
					opts.StoreMode = args[i+1]
					if opts.Batteries == nil {
						opts.Batteries = &generator.UserBatteryOverrides{}
					}
					opts.Batteries.Store = args[i+1]
					i++
				}
			case "--no-admin":
				opts.AdminEnabled = false
			case "--non-interactive":
				nonInteractive = true
			case "--admin-email":
				if i+1 < len(args) {
					opts.AdminEmail = args[i+1]
					opts.AdminEnabled = true
					i++
				}
			case "--admin-password":
				if i+1 < len(args) {
					opts.AdminPassword = args[i+1]
					i++
				}
			case "--with-telemetry":
				opts.WithTelemetry = true
			case "--with-monetization":
				opts.WithMonetization = true
			case "--with-flags":
				opts.WithFlags = true
			case "--with-ads":
				opts.WithAds = true
			case "--minimal":
				opts.Minimal = true
				nonInteractive = true
			case "--auth-strategy":
				if i+1 < len(args) {
					opts.AuthStrategy = args[i+1]
					i++
				}
			case "--layout":
				if i+1 < len(args) {
					opts.Layout = generator.LayoutType(args[i+1])
					i++
				}
			case "--dev-vendor":
				devVendor = true
			default:
				if name == "" && !strings.HasPrefix(arg, "-") {
					name = arg
				}
			}
		}
	}

	if name != "" && archetypeAlias == "" {
		if _, ok := archetypes.LookupArchetype(name); ok {
			archetypeAlias = name
			nonInteractive = true
		}
	}

	if pipelineOverride != "" {
		opts.PipelineType = pipelineOverride
	}

	if !nonInteractive {
		// Always run interactive wizard for key choices
		fmt.Println("🚀 Welcome to the BFFX Project Wizard!")
		fmt.Println("--------------------------------------")

		if name == "" {
			fmt.Print("Project Name: ")
			fmt.Scanln(&name)
			if name == "" {
				log.Fatal("Project name is required")
			}
		} else {
			fmt.Printf("Project Name: %s\n", name)
		}

		fmt.Print("Disable admin panel? (y/N): ")
		var disableChoice string
		fmt.Scanln(&disableChoice)
		if strings.ToLower(disableChoice) == "y" {
			opts.AdminEnabled = false
		} else {
			opts.AdminEnabled = true
			fmt.Print("Admin Email [admin@example.com]: ")
			fmt.Scanln(&opts.AdminEmail)
			if opts.AdminEmail == "" {
				opts.AdminEmail = "admin@example.com"
			}

			fmt.Print("Admin Password (leave empty to auto-generate): ")
			fmt.Scanln(&opts.AdminPassword)
			if opts.AdminPassword != "" {
				fmt.Print("Confirm Password: ")
				var confirm string
				fmt.Scanln(&confirm)
				if opts.AdminPassword != confirm {
					log.Fatal("Passwords do not match")
				}
			}
		}

		fmt.Println("\nSelect Addons (y/n):")
		fmt.Print("1. Subscriptions, Entitlements & Plans? ")
		var mChoice string
		fmt.Scanln(&mChoice)
		opts.WithMonetization = strings.ToLower(mChoice) == "y"

		fmt.Print("2. Feature Flags & Rollouts? ")
		var fChoice string
		fmt.Scanln(&fChoice)
		opts.WithFlags = strings.ToLower(fChoice) == "y"

		fmt.Print("3. Telemetry & Analytics? ")
		var tChoice string
		fmt.Scanln(&tChoice)
		opts.WithTelemetry = strings.ToLower(tChoice) == "y"

		fmt.Print("4. Ads Ecosystem? ")
		var aChoice string
		fmt.Scanln(&aChoice)
		opts.WithAds = strings.ToLower(aChoice) == "y"

		fmt.Print("5. Auth Strategy (mandatory/optional/anonymous) [optional]: ")
		var authChoice string
		fmt.Scanln(&authChoice)
		if authChoice != "" {
			opts.AuthStrategy = authChoice
		}
	}

	var generatedPassword string
	if opts.AdminEnabled && opts.AdminPassword == "" {
		generatedPassword = generateRandomPassword()
		opts.AdminPassword = generatedPassword
	}

	if archetypeAlias != "" {
		if err := generator.ScaffoldArchetype(parentDir, name, archetypeAlias, opts); err != nil {
			log.Fatalf("new project failed (archetype): %v", err)
		}
	} else {
		if err := generator.ScaffoldNewProject(parentDir, name, opts); err != nil {
			log.Fatalf("new project failed: %v", err)
		}
	}

	// Auto-seed initial resources (like AdminUser) for sqlite projects only.
	projectDir := filepath.Join(parentDir, name)
	reg, err := manifest.LoadAll(projectDir)
	if err == nil && reg != nil && reg.Project != nil {
		spec := reg.ProjectSpec()
		if spec.Store.Mode == "" || spec.Store.Mode == "sqlite" {
			dbPath := filepath.Join(projectDir, ".bffx", "data", "app.db")
			if spec.Store.Path != "" && !filepath.IsAbs(spec.Store.Path) {
				dbPath = filepath.Join(projectDir, spec.Store.Path)
			}
			store, err := storage.NewSQLiteStore(dbPath)
			if err == nil {
				store.Reconcile(context.Background(), reg)
				storage.SeedRegistry(reg, store)
			}
		}
	}

	// Find framework root (where pkg/ lives)
	workspaceRoot := ""
	curr, _ := os.Getwd()
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(filepath.Join(curr, "pkg")); err == nil {
			workspaceRoot = curr
			break
		}
		curr = filepath.Dir(curr)
	}

	if devVendor || workspaceRoot != "" {
		if workspaceRoot != "" {
			fmt.Println("🛠️ Dev Vendor: Embedding local framework core...")
			if err := deploy.Vendor(projectDir, workspaceRoot, nil); err != nil {
				log.Fatalf("dev-vendor failed: %v", err)
			}
		} else if devVendor {
			log.Fatal("dev-vendor flag specified but framework root (pkg/ directory) could not be found in parent directories")
		}
	}

	// Inject environment variables into the current process context
	for k, v := range opts.Env {
		os.Setenv(k, v)
	}

	// Run scoped doctor check
	fmt.Printf("\n🔍 Testing Battery Connectivity (bffx doctor)...\n")
	results, err := doctor.CheckBatteriesOnly(projectDir)
	if err != nil {
		fmt.Printf("⚠️ Warning: Failed to run battery check: %v\n", err)
	} else {
		hasFailures := false
		for _, r := range results {
			icon := "✅"
			if r.Status == "fail" {
				icon = "❌"
				hasFailures = true
			} else if r.Status == "warn" {
				icon = "⚠️"
			}
			fmt.Printf("%s Battery: %s: %s\n", icon, r.Name, r.Message)
		}
		if hasFailures {
			fmt.Printf("\n⚠️ Warning: Some batteries failed validation. Please review your credentials in %s/.env\n", name)
		} else {
			fmt.Printf("\nAll batteries connected and verified. You're ready to start work!\n")
		}
	}

	fmt.Printf("\n✨ Successfully scaffolded %s!\n", name)
	if generatedPassword != "" {
		fmt.Printf("🔑 Generated Admin Password: %s\n", generatedPassword)
		fmt.Println("⚠️  Make sure to save this password. It is hashed and stored in the database for the initial AdminUser.")
	}
}

func generateRandomPassword() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
