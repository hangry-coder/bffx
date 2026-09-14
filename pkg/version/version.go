// Package version holds the framework release identifier stamped into generated
// projects and reported by the CLI (unless overridden at link time via -X main.Version=...).
package version

import "os"

// FrameworkVersion is written to <project>/.bffx/framework_version when the
// embedded core is refreshed. Bump this when cutting a framework release.
const FrameworkVersion = "0.1.3-beta"

// FrameworkModule is the canonical Go module path for the remote bffx framework repository.
const FrameworkModule = "github.com/hangry-coder/bffx"

// FrameworkModulePath returns the framework module path, allowing override via BFFX_FRAMEWORK_MODULE env var.
func FrameworkModulePath() string {
	if m := os.Getenv("BFFX_FRAMEWORK_MODULE"); m != "" {
		return m
	}
	return FrameworkModule
}
