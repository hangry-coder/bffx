package cli

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/hangry-coder/bffx/pkg/buildprofile"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

func handleMigratePackaging(root string, args []string) {
	dryRun := false
	apply := false
	rollback := false
	target := buildprofile.ModeMinimal
	jsonOut := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			printMigratePackagingHelp()
			return
		case "--dry-run", "--plan":
			dryRun = true
		case "--apply":
			apply = true
		case "--rollback":
			rollback = true
		case "--json":
			jsonOut = true
		case "--to":
			if i+1 >= len(args) {
				log.Fatal("usage: --to full|minimal")
			}
			target = args[i+1]
			i++
		default:
			switch args[i] {
			case "plan", "preflight":
				dryRun = true
			case "apply":
				apply = true
			case "rollback":
				rollback = true
			default:
				log.Fatalf("unknown packaging migrate argument: %q (try --help)", args[i])
			}
		}
	}

	if rollback {
		if err := buildprofile.RollbackMigration(root); err != nil {
			log.Fatalf("rollback failed: %v", err)
		}
		fmt.Println("Packaging migration rolled back. Run: bffx sync && bffx doctor")
		return
	}

	if !dryRun && !apply {
		dryRun = true
	}

	reg, err := manifest.LoadAll(root)
	if err != nil {
		log.Fatalf("load manifests: %v", err)
	}

	if dryRun && !apply {
		pf := buildprofile.Preflight(reg, target)
		if !pf.OK {
			if jsonOut {
				emitJSON(pf)
			} else {
				fmt.Printf("Preflight: current=%s target=%s ok=false\n", pf.CurrentMode, pf.TargetMode)
				for _, b := range pf.Blockers {
					fmt.Printf("  blocker: %s\n", b)
				}
			}
			os.Exit(1)
		}

		plan, err := buildprofile.PlanMigration(reg, target)
		if err != nil {
			log.Fatalf("plan failed: %v", err)
		}
		if jsonOut {
			emitJSON(plan)
			return
		}
		fmt.Printf("Preflight: current=%s target=%s ok=true\n", pf.CurrentMode, pf.TargetMode)
		for _, w := range pf.Warnings {
			fmt.Printf("  warn: %s\n", w)
		}
		printMigrationPlan(plan)
		return
	}

	plan, err := buildprofile.ApplyMigration(root, reg, target, false)
	if err != nil {
		log.Fatalf("apply failed: %v", err)
	}
	if jsonOut {
		emitJSON(plan)
		return
	}
	fmt.Println("Packaging migration applied.")
	printMigrationPlan(plan)
	fmt.Println("Next: bffx sync && bffx update framework --vendor-only && bffx doctor")
}

func printMigrationPlan(plan *buildprofile.MigrationPlan) {
	if plan == nil {
		return
	}
	fmt.Printf("Migration: %s -> %s\n", plan.FromMode, plan.ToMode)
	for _, c := range plan.Changes {
		fmt.Printf("  - %s\n", c)
	}
	if len(plan.PackageDiff.Added) > 0 || len(plan.PackageDiff.Removed) > 0 {
		fmt.Println("  package diff:")
		for _, p := range plan.PackageDiff.Added {
			fmt.Printf("    + %s\n", p)
		}
		for _, p := range plan.PackageDiff.Removed {
			fmt.Printf("    - %s\n", p)
		}
	}
	if len(plan.ExcludedDiff.Added) > 0 || len(plan.ExcludedDiff.Removed) > 0 {
		fmt.Println("  excluded tooling diff:")
		for _, p := range plan.ExcludedDiff.Added {
			fmt.Printf("    + %s\n", p)
		}
		for _, p := range plan.ExcludedDiff.Removed {
			fmt.Printf("    - %s\n", p)
		}
	}
}

func printMigratePackagingHelp() {
	fmt.Println("Usage: bffx migrate packaging [plan|apply|rollback] [--root DIR]")
	fmt.Println("  Guided full <-> minimal packaging migration (Phase 8).")
	fmt.Println("  plan / --dry-run   Preflight + dry-run diff (default action when no subcommand)")
	fmt.Println("  apply / --apply    Backup project.yaml + profile, switch mode, refresh profile")
	fmt.Println("  rollback           Restore from last migration backup")
	fmt.Println("  --to full|minimal  Target mode (default: minimal)")
	fmt.Println("  --json             Machine-readable output")
}

func emitJSON(v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Fatalf("json encode: %v", err)
	}
	fmt.Println(string(b))
}
