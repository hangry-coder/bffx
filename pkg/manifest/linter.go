package manifest

import (
	"fmt"
	"strings"
)

type LinterError struct {
	ManifestName string
	Message      string
	Severity     string // "error" or "warning"
}

func (e LinterError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", strings.ToUpper(e.Severity), e.ManifestName, e.Message)
}

// LintRegistry runs lint with default options (strict=false).
func LintRegistry(reg *Registry) []LinterError {
	return LintRegistryWithOptions(reg, false)
}

// LintRegistryWithOptions validates manifests; strict upgrades selected warnings to errors.
func LintRegistryWithOptions(reg *Registry, strict bool) []LinterError {
	var errs []LinterError

	spec := reg.ProjectSpec()
	if spec != nil && reg.Project != nil {
		errs = append(errs, lintProject(reg.Project.Metadata.Name, spec)...)
	}

	for _, m := range reg.Resources {
		var spec ResourceSpec
		if err := m.UnmarshalSpec(&spec); err == nil {
			errs = append(errs, lintResource(m.Metadata.Name, &spec, strict)...)
		}
	}

	for _, m := range reg.Actions {
		var spec ActionSpec
		if err := m.UnmarshalSpec(&spec); err == nil {
			errs = append(errs, lintAction(m.Metadata.Name, &spec, strict)...)
		}
	}

	return errs
}

var sensitiveKeywords = []string{
	"delete", "update", "create", "admin", "payment", "payout",
	"billing", "invoice", "destroy", "remove", "purge", "write",
	"set", "reset", "grant", "revoke", "password", "role",
	"permission", "user", "auth", "profile", "sensitive",
}

func lintAction(name string, spec *ActionSpec, strict bool) []LinterError {
	var errs []LinterError

	authStr := strings.ToLower(strings.TrimSpace(spec.Route.Auth))
	if authStr == "public" {
		lowerName := strings.ToLower(name)
		isSensitive := false
		for _, kw := range sensitiveKeywords {
			if strings.Contains(lowerName, kw) {
				isSensitive = true
				break
			}
		}

		if isSensitive {
			sev := "warning"
			if strict {
				sev = "error"
			}
			errs = append(errs, LinterError{
				ManifestName: name,
				Message:      fmt.Sprintf("Action '%s' has public auth but has a sensitive name pattern. Consider protecting it or explicitly verifying authorization inside the action hooks.", name),
				Severity:     sev,
			})
		}
	}

	return errs
}


func lintProject(name string, spec *ProjectSpec) []LinterError {
	var errs []LinterError

	if spec.Store.Mode == "" {
		errs = append(errs, LinterError{
			ManifestName: name,
			Message:      "Missing required field 'store.mode' in Project manifest",
			Severity:     "error",
		})
	}

	if spec.App.AuthStrategy == "" {
		errs = append(errs, LinterError{
			ManifestName: name,
			Message:      "Missing 'app.authStrategy'; defaulting to 'optional'",
			Severity:     "warning",
		})
	}

	return errs
}

var piiFields = []string{"email", "phone", "address", "ssn", "credit_card", "password", "secret", "token", "key"}

// policyKeyword extracts a simple string keyword policy for lint purposes.
// If rule uses composite YAML (maps/dicts/lists), returns composite=true and skips keyword lint.
func policyKeyword(rule any) (keyword string, composite bool) {
	switch v := rule.(type) {
	case string:
		return strings.ToLower(strings.TrimSpace(v)), false
	case nil:
		return "", false
	default:
		return "", true
	}
}

func lintResource(name string, spec *ResourceSpec, strict bool) []LinterError {
	var errs []LinterError

	if msg := TreeLintMessage(spec.Tree); msg != "" {
		sev := "warning"
		if strict {
			sev = "error"
		}
		errs = append(errs, LinterError{
			ManifestName: name,
			Message:      msg,
			Severity:     sev,
		})
	}

	readKw, readComposite := policyKeyword(spec.Policy.Read)

	publicLike := false
	if !readComposite && readKw == "public" {
		publicLike = true
	}

	if publicLike {
		for _, field := range spec.Fields {
			fieldName := strings.ToLower(field.Name)
			for _, pii := range piiFields {
				if strings.Contains(fieldName, pii) {
					sev := "warning"
					if strict {
						sev = "error"
					}
					errs = append(errs, LinterError{
						ManifestName: name,
						Message:      fmt.Sprintf("PII field '%s' exposed in public resource", field.Name),
						Severity:     sev,
					})
					break
				}
			}
		}
	}

	return errs
}
