package doctor

import (
	"os"
	"strings"

	"github.com/hangry-coder/bffx/pkg/manifest"
)

func archetypeDoctorChecks(root string, reg *manifest.Registry) []Result {
	var results []Result
	if reg == nil || reg.Project == nil {
		return results
	}

	var spec manifest.ProjectSpec
	if err := reg.Project.UnmarshalSpec(&spec); err != nil {
		return results
	}

	// 1. VLM Key check
	if spec.Batteries.Vlm == "gemini" {
		if os.Getenv("GEMINI_API_KEY") == "" {
			results = append(results, Result{
				Name:    "Archetype: Gemini Key",
				Status:  "warn",
				Message: "Gemini VLM is configured but GEMINI_API_KEY is not set in the environment.",
			})
		} else {
			results = append(results, Result{
				Name:    "Archetype: Gemini Key",
				Status:  "ok",
				Message: "GEMINI_API_KEY is set in the environment",
			})
		}
	}

	// 2. Look for archetype name/context from project name or settings
	projName := strings.ToLower(reg.Project.Metadata.Name)
	if strings.Contains(projName, "fintech") || spec.Store.Mode == "postgres" && spec.App.AuthStrategy == "mandatory" {
		// Fintech check (e.g. Plaid)
		if os.Getenv("PLAID_CLIENT_ID") == "" || os.Getenv("PLAID_SECRET") == "" {
			results = append(results, Result{
				Name:    "Archetype: Plaid Integration",
				Status:  "warn",
				Message: "Fintech archetype detected but Plaid credentials (PLAID_CLIENT_ID/PLAID_SECRET) are missing from the environment.",
			})
		}
	}

	if strings.Contains(projName, "commerce") {
		// Commerce check (Stripe)
		if os.Getenv("STRIPE_SECRET_KEY") == "" {
			results = append(results, Result{
				Name:    "Archetype: Stripe Integration",
				Status:  "warn",
				Message: "Commerce archetype detected but Stripe credentials (STRIPE_SECRET_KEY) are missing from the environment.",
			})
		}
	}

	return results
}
