package buildprofile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

// DeriveOptions configures profile derivation.
type DeriveOptions struct {
	GraphHash string
}

// Derive builds a profile from the loaded manifest registry.
func Derive(reg *manifest.Registry, opts DeriveOptions) (*Profile, error) {
	if reg == nil || reg.Project == nil {
		return nil, errMissingProject
	}

	spec := reg.ProjectSpec()
	mode := normalizeMode(spec.Packaging.Mode)

	caps := deriveCapabilities(reg, spec)
	surfaces := deriveSurfaces(reg, caps)
	counts := ManifestCounts{
		Actions:      len(reg.Actions),
		Screens:      len(reg.Screens),
		Pipelines:    len(reg.Pipelines),
		LiveOps:      len(reg.LiveOpsEvents),
		Leaderboards: len(reg.Leaderboards),
	}

	prof := &Profile{
		APIVersion:      APIVersion,
		Project:         reg.Project.Metadata.Name,
		Mode:            mode,
		GraphHash:       strings.TrimSpace(opts.GraphHash),
		Capabilities:    caps,
		Surfaces:        surfaces,
		Counts:          counts,
		Batteries:       snapshotBatteries(spec),
		Packages:        ResolvePackages(mode, caps),
		ExcludedTooling: ExcludedToolingForMode(mode),
	}
	prof.ProfileHash = hashProfile(prof)
	return prof, nil
}

func normalizeMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case ModeMinimal:
		return ModeMinimal
	default:
		return ModeFull
	}
}

func deriveCapabilities(reg *manifest.Registry, spec *manifest.ProjectSpec) Capabilities {
	vlm := strings.TrimSpace(spec.Batteries.Vlm)
	if vlm == "" {
		vlm = "noop"
	}
	nutrition := strings.TrimSpace(spec.Batteries.Nutrition)

	hasPipelines := len(reg.Pipelines) > 0
	aiEnabled := hasPipelines || (vlm != "" && vlm != "noop")

	gameEnabled := len(reg.LiveOpsEvents) > 0 || len(reg.Leaderboards) > 0

	flagsProvider := strings.TrimSpace(spec.Batteries.Flags)
	if flagsProvider == "" {
		flagsProvider = strings.TrimSpace(spec.FeatureFlags.Provider)
	}

	streaming := spec.Runtime.Streaming.Enabled
	for _, s := range reg.Screens {
		var ss manifest.ScreenSpec
		if s.UnmarshalSpec(&ss) == nil && ss.Stream {
			streaming = true
			break
		}
	}

	monetization := spec.Monetization.Enabled

	return Capabilities{
		AI:           aiEnabled,
		Game:         gameEnabled,
		FeatureFlags: flagsProvider != "" || spec.FeatureFlags.Idempotency || spec.FeatureFlags.RefreshTokens != nil,
		Admin:        spec.Admin.Enabled,
		Billing:      monetization,
		Ads:          monetization,
		Nutrition:    nutrition != "" && nutrition != "noop",
		Wire:         spec.Runtime.Wire.Enabled,
		Streaming:    streaming,
		Worker:       spec.Runtime.Worker.Enabled || spec.Defaults.Jobs,
		I18n:         strings.TrimSpace(spec.Batteries.I18n) != "none",
	}
}

func deriveSurfaces(reg *manifest.Registry, caps Capabilities) RouterSurfaces {
	imports := []string{"github.com/hangry-coder/bffx/pkg/featureflags"}
	if caps.Billing {
		imports = append(imports, "github.com/hangry-coder/bffx/pkg/addons/billing")
	}
	if caps.Ads {
		imports = append(imports, "github.com/hangry-coder/bffx/pkg/addons/ads")
	}
	if caps.Nutrition {
		imports = append(imports, "github.com/hangry-coder/bffx/pkg/batteries/nutrition")
	}
	if caps.AI {
		imports = append(imports, "github.com/hangry-coder/bffx/pkg/ai/...")
	}
	if caps.Game {
		imports = append(imports,
			"github.com/hangry-coder/bffx/pkg/game/liveops",
			"github.com/hangry-coder/bffx/pkg/game/leaderboard",
		)
	}
	sort.Strings(imports)

	pipelineNames := namesFromManifests(reg.Pipelines)
	liveOpsNames := namesFromManifests(reg.LiveOpsEvents)
	leaderboardNames := namesFromManifests(reg.Leaderboards)

	return RouterSurfaces{
		AddonImports: imports,
		Pipelines:    pipelineNames,
		LiveOps:      liveOpsNames,
		Leaderboards: leaderboardNames,
	}
}

func namesFromManifests(items []*manifest.Manifest) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, len(items))
	for i, m := range items {
		out[i] = m.Metadata.Name
	}
	sort.Strings(out)
	return out
}

func snapshotBatteries(spec *manifest.ProjectSpec) BatteriesSnapshot {
	return BatteriesSnapshot{
		Flags:     strings.TrimSpace(spec.Batteries.Flags),
		Vlm:       strings.TrimSpace(spec.Batteries.Vlm),
		Nutrition: strings.TrimSpace(spec.Batteries.Nutrition),
	}
}

func hashProfile(prof *Profile) string {
	copy := *prof
	copy.ProfileHash = ""
	b, _ := json.Marshal(&copy)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
