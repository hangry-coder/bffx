package buildprofile

import (
	"fmt"
	"strings"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

// Issue describes a build-profile consistency problem.
type Issue struct {
	Code    string
	Message string
}

func (i Issue) String() string {
	return i.Message
}

// ValidateConsistency checks internal profile/manifest alignment.
func ValidateConsistency(prof *Profile, reg *manifest.Registry) []Issue {
	if prof == nil || reg == nil {
		return nil
	}
	var issues []Issue

	if prof.Counts.Pipelines > 0 && !prof.Capabilities.AI {
		issues = append(issues, Issue{
			Code:    "pipelines_without_ai",
			Message: fmt.Sprintf("profile lists %d pipelines but capabilities.ai=false", prof.Counts.Pipelines),
		})
	}
	if prof.Counts.LiveOps > 0 && !prof.Capabilities.Game {
		issues = append(issues, Issue{
			Code:    "liveops_without_game",
			Message: fmt.Sprintf("profile lists %d liveops events but capabilities.game=false", prof.Counts.LiveOps),
		})
	}
	if prof.Counts.Leaderboards > 0 && !prof.Capabilities.Game {
		issues = append(issues, Issue{
			Code:    "leaderboards_without_game",
			Message: fmt.Sprintf("profile lists %d leaderboards but capabilities.game=false", prof.Counts.Leaderboards),
		})
	}

	vlm := strings.TrimSpace(prof.Batteries.Vlm)
	if vlm != "" && vlm != "noop" && prof.Counts.Pipelines == 0 && !prof.Capabilities.AI {
		issues = append(issues, Issue{
			Code:    "vlm_without_ai",
			Message: fmt.Sprintf("batteries.vlm=%q implies AI surface but capabilities.ai=false", vlm),
		})
	}

	nutrition := strings.TrimSpace(prof.Batteries.Nutrition)
	if nutrition != "" && nutrition != "noop" && !prof.Capabilities.Nutrition {
		issues = append(issues, Issue{
			Code:    "nutrition_mismatch",
			Message: fmt.Sprintf("batteries.nutrition=%q but capabilities.nutrition=false", nutrition),
		})
	}

	if prof.Mode == ModeMinimal {
		for _, p := range ToolingPackages {
			for _, included := range prof.Packages {
				if included == p {
					issues = append(issues, Issue{
						Code:    "tooling_in_minimal_packages",
						Message: fmt.Sprintf("minimal profile must not include tooling package %q", p),
					})
				}
			}
		}
	}

	return issues
}

// DriftIssues compares a stored profile with a freshly derived profile.
func DriftIssues(stored, derived *Profile) []Issue {
	if stored == nil || derived == nil {
		return nil
	}
	var issues []Issue

	if stored.ProfileHash != "" && derived.ProfileHash != "" && stored.ProfileHash != derived.ProfileHash {
		issues = append(issues, Issue{
			Code:    "profile_drift",
			Message: "build-profile.json is stale; run bffx sync to refresh",
		})
	}
	if stored.GraphHash != "" && derived.GraphHash != "" && stored.GraphHash != derived.GraphHash {
		issues = append(issues, Issue{
			Code:    "graph_drift",
			Message: "build-profile graphHash does not match current graph.hash; run bffx sync",
		})
	}
	if stored.Mode != derived.Mode {
		issues = append(issues, Issue{
			Code:    "mode_drift",
			Message: fmt.Sprintf("profile mode=%q but manifests derive mode=%q", stored.Mode, derived.Mode),
		})
	}

	return issues
}
