package router

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/runtimecontracts"
)

// Compile-time assertions that *Router satisfies shared runtime contracts.
var (
	_ runtimecontracts.ScreenRunner     = (*Router)(nil)
	_ runtimecontracts.CacheInvalidator = (*Router)(nil)
)

// RunScreen executes a Screen in-process with admin impersonation. It is
// the entry point for the admin panel's "Run as user" feature and lives in
// the router because it reuses resolveSources/resolveOutput.
//
// The runner deliberately bypasses the auth wrapper since the caller is
// already an authenticated admin; impersonation is expressed via the
// supplied opts (UserID, Locale, ExtraClaims). For any Screen whose
// `requires_auth: required`, this means the admin will see the response a
// user with the supplied UserID would have seen — including 401 semantics
// when UserID is empty.
func (r *Router) RunScreen(ctx context.Context, name string, opts runtimecontracts.RunOpts) (runtimecontracts.RunResult, error) {
	if r == nil || r.reg == nil {
		return runtimecontracts.RunResult{}, errors.New("router not initialized")
	}

	// Locate the screen manifest.
	var screen *manifest.Manifest
	for _, m := range r.reg.Screens {
		if m.Metadata.Name == name {
			screen = m
			break
		}
	}
	if screen == nil {
		return runtimecontracts.RunResult{}, fmt.Errorf("screen %q not found", name)
	}

	var spec manifest.ScreenSpec
	if err := screen.UnmarshalSpec(&spec); err != nil {
		return runtimecontracts.RunResult{}, fmt.Errorf("decode screen spec: %w", err)
	}

	// Honour `requires_auth: required` semantics even on the admin path
	// so the runner accurately reflects production behavior.
	if spec.RequiresAuth == "required" && opts.UserID == "" {
		return runtimecontracts.RunResult{
			Errors: []string{"screen requires authentication; provide user_id"},
		}, nil
	}

	// Surface kill switch state so admins can see what the impersonated
	// user would see. We do not synthesize the 503 envelope here because
	// the runner returns structured data; the UI renders the warning.
	if r.killSwitch != nil {
		if ks, err := r.killSwitch.Get(ctx, name, ""); err == nil && ks != nil && ks.IsActiveNow() {
			return runtimecontracts.RunResult{
				Output: map[string]any{
					"killed":     true,
					"reason":     ks.Reason,
					"expires_at": ks.ExpiresAt,
				},
			}, nil
		}
	}

	// Synthesize a fake *http.Request so the existing source resolvers
	// (which read from headers/path) work unchanged.
	method := spec.Route.Method
	if method == "" {
		method = http.MethodGet
	}
	rawURL := spec.Route.Path
	if rawURL == "" {
		rawURL = r.reg.ApiPrefix + "/screens/" + name
	}
	if q := opts.Query.Encode(); q != "" {
		if u, err := url.Parse(rawURL); err == nil {
			u.RawQuery = q
			rawURL = u.String()
		}
	}
	req := httptest.NewRequest(method, rawURL, nil)
	for k, vs := range opts.Headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	// Build the synthetic auth context.
	runCtx := ctx
	claims := map[string]any{}
	if opts.UserID != "" {
		claims["sub"] = opts.UserID
	}
	for k, v := range opts.ExtraClaims {
		claims[k] = v
	}
	if len(claims) > 0 {
		runCtx = middleware.WithClaims(runCtx, claims)
	}
	if opts.Locale != "" {
		runCtx = middleware.WithLocale(runCtx, opts.Locale)
	}
	if dev := opts.Headers.Get("X-Device-ID"); dev != "" {
		runCtx = middleware.WithDeviceID(runCtx, dev)
	}

	req = req.WithContext(runCtx)

	// Resolve sources & output using the same pipeline as the real route.
	sourceData, errs := r.resolveSources(req, spec.Sources, spec.PartialSuccess)
	if len(errs) > 0 && !spec.PartialSuccess {
		errStrs := make([]string, len(errs))
		for i, e := range errs {
			errStrs[i] = e.Error()
		}
		return runtimecontracts.RunResult{Errors: errStrs}, nil
	}
	output := r.resolveOutput(runCtx, sourceData, spec.Output)

	result := runtimecontracts.RunResult{Output: output}
	if len(errs) > 0 {
		for _, e := range errs {
			result.Errors = append(result.Errors, e.Error())
		}
	}
	return result, nil
}

