package middleware

import (
	"net/http"
	"strings"

	apierrors "github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/Masterminds/semver/v3"
)

// lifecycleExemptPath returns true for routes that are not mobile/API clients
// (browser admin UI, health, metrics, interactive docs).
func lifecycleExemptPath(path string) bool {
	switch path {
	case "/health", "/metrics":
		return true
	case "/admin":
		return true
	}
	return strings.HasPrefix(path, "/admin/") ||
		strings.HasPrefix(path, "/api/admin")
}

// ClientVersion checks for the X-BFFX-Client-Version header and adds headers
// to signal if an upgrade is recommended or required based on LifecycleSpec.
func ClientVersion(spec manifest.LifecycleSpec) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if lifecycleExemptPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			versionStr := r.Header.Get("X-BFFX-Client-Version")
			platform := r.Header.Get("X-BFFX-Platform")

			if versionStr == "" {
				if spec.Semver.Enforce {
					apierrors.Write(w, apierrors.New(
						http.StatusBadRequest,
						"X-BFFX-Client-Version header is required",
						"missing_client_version",
					))
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			v, err := semver.NewVersion(versionStr)
			if err != nil {
				if spec.Semver.Enforce {
					apierrors.Write(w, apierrors.New(
						http.StatusBadRequest,
						"Invalid SemVer in X-BFFX-Client-Version: "+versionStr,
						"invalid_client_version",
					))
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// Check platform-specific overrides or global defaults
			minVersionStr := ""
			suggestVersionStr := ""

			if p, ok := spec.Platforms[platform]; ok {
				minVersionStr = p.MinVersion
				suggestVersionStr = p.SuggestVersion
			} else if p, ok := spec.Platforms["default"]; ok {
				minVersionStr = p.MinVersion
				suggestVersionStr = p.SuggestVersion
			}

			if minVersionStr != "" {
				minV, err := semver.NewVersion(minVersionStr)
				if err == nil && v.LessThan(minV) {
					w.Header().Set("X-BFFX-Upgrade-Required", "true")
					apierrors.Write(w, apierrors.New(
						http.StatusUpgradeRequired,
						"Client version "+versionStr+" is below minimum "+minVersionStr,
						"client_upgrade_required",
					))
					return
				}
			}

			if suggestVersionStr != "" {
				suggestV, err := semver.NewVersion(suggestVersionStr)
				if err == nil && v.LessThan(suggestV) {
					w.Header().Set("X-BFFX-Upgrade-Suggested", "true")
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
