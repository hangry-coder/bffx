package doctor

import (
	"fmt"
	"os"
	"strings"

	"github.com/hangry-coder/bffx/pkg/buildprofile"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

func lintBuildProfile(root string, reg *manifest.Registry) []Result {
	var results []Result
	if reg == nil {
		return results
	}

	stored, loadErr := buildprofile.Load(root)
	spec := reg.ProjectSpec()
	minimalProject := spec != nil && buildprofile.ModeFromSpec(spec) == buildprofile.ModeMinimal

	if loadErr != nil {
		if os.IsNotExist(loadErr) {
			status := "warn"
			msg := ".bffx/build-profile.json missing — run bffx sync to emit the canonical build profile"
			if minimalProject {
				status = "fail"
				msg = ".bffx/build-profile.json required for packaging.mode=minimal — run bffx sync"
			}
			results = append(results, Result{
				Name:    "Build Profile",
				Status:  status,
				Message: msg,
			})
		} else {
			results = append(results, Result{
				Name:    "Build Profile",
				Status:  "fail",
				Message: fmt.Sprintf("Could not read build profile: %v", loadErr),
			})
		}
		return results
	}

	graphHash := readGraphHash(root)
	derived, err := buildprofile.Derive(reg, buildprofile.DeriveOptions{GraphHash: graphHash})
	if err != nil {
		results = append(results, Result{
			Name:    "Build Profile",
			Status:  "fail",
			Message: fmt.Sprintf("Could not derive build profile from manifests: %v", err),
		})
		return results
	}

	for _, issue := range buildprofile.DriftIssues(stored, derived) {
		results = append(results, Result{
			Name:    "Build Profile: Drift",
			Status:  "fail",
			Message: issue.Message,
		})
	}

	for _, issue := range buildprofile.ValidateConsistency(stored, reg) {
		results = append(results, Result{
			Name:    "Build Profile: Consistency",
			Status:  "fail",
			Message: issue.Message,
		})
	}

	modeMsg := fmt.Sprintf("mode=%s capabilities[ai=%t game=%t featureflags=%t admin=%t] packages=%d",
		stored.Mode,
		stored.Capabilities.AI,
		stored.Capabilities.Game,
		stored.Capabilities.FeatureFlags,
		stored.Capabilities.Admin,
		len(stored.Packages),
	)
	if stored.Mode == buildprofile.ModeMinimal {
		modeMsg += fmt.Sprintf(" excludedTooling=%d", len(stored.ExcludedTooling))
	}

	if len(results) == 0 {
		results = append(results, Result{
			Name:    "Build Profile",
			Status:  "ok",
			Message: modeMsg,
		})
	} else {
		results = append(results, Result{
			Name:    "Build Profile: Summary",
			Status:  "warn",
			Message: modeMsg,
		})
	}

	return results
}

func lintPackagingMigration(root string, reg *manifest.Registry) []Result {
	var results []Result
	if reg == nil {
		return results
	}

	rec, err := buildprofile.LoadMigrationRecord(root)
	if err != nil {
		results = append(results, Result{
			Name:    "Packaging Migration",
			Status:  "fail",
			Message: fmt.Sprintf("Could not read migration record: %v", err),
		})
		return results
	}
	if rec == nil {
		return results
	}

	current := buildprofile.ModeFromSpec(reg.ProjectSpec())
	if rec.ToMode == current {
		results = append(results, Result{
			Name:    "Packaging Migration: Rollback",
			Status:  "warn",
			Message: fmt.Sprintf("Last migration %s→%s at %s; rollback via: bffx migrate packaging rollback", rec.FromMode, rec.ToMode, rec.MigratedAt),
		})
	} else {
		results = append(results, Result{
			Name:    "Packaging Migration: Rollback",
			Status:  "warn",
			Message: fmt.Sprintf("Migration record (%s→%s) does not match current mode=%q; review .bffx/%s", rec.FromMode, rec.ToMode, current, buildprofile.MigrationRecordFile),
		})
	}
	return results
}

func readGraphHash(root string) string {
	b, err := os.ReadFile(root + "/.bffx/graph.hash")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
