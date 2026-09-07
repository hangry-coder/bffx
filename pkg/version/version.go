// Package version holds the framework release identifier stamped into generated
// projects and reported by the CLI (unless overridden at link time via -X main.Version=...).
package version

// FrameworkVersion is written to <project>/.bffx/framework_version when the
// embedded core is refreshed. Bump this when cutting a framework release.
const FrameworkVersion = "0.1.3-beta"
