package doctor

import (
	"fmt"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

func seedDoctorChecks(reg *manifest.Registry) []Result {
	var results []Result
	if reg == nil {
		return results
	}

	n := len(reg.Seeds)
	msg := fmt.Sprintf("%d Seed manifest(s) loaded", n)
	if n == 0 {
		results = append(results, Result{
			Name:    "Seed manifests",
			Status:  "warn",
			Message: msg + "; for large catalogs use internal/features/<feature>/seeds/*.yaml (kind: Seed)",
		})
		return results
	}

	results = append(results, Result{
		Name:    "Seed manifests",
		Status:  "ok",
		Message: msg + "; large catalogs: internal/features/<feature>/seeds/ with spec.order for FK-safe apply",
	})
	return results
}
