package router

import (
	"github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"net/http"
	"strings"
)

func (r *Router) withPolicy(m *manifest.Manifest, action string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var spec manifest.ResourceSpec
		if err := m.UnmarshalSpec(&spec); err != nil {
			next(w, req)
			return
		}

		policy := spec.Policy.Read
		if action == "write" {
			policy = spec.Policy.Write
		}

		if policy == nil || policy == "" || policy == "public" {
			next(w, req)
			return
		}

		claims := middleware.GetClaims(req.Context())
		id := req.PathValue("id")

		if id == "" {
			// Collection routes (LIST / CREATE): no row to pass into the policy engine.
			if policyStringEquals(policy, "owner") {
				if claims == nil {
					errors.WriteError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
					return
				}
				next(w, req)
				return
			}

			if !r.policyEngine.Evaluate(policy, claims, nil) {
				status := http.StatusForbidden
				errMsg := "forbidden"
				if claims == nil && isAuthenticatedOnlyPolicy(policy) {
					status = http.StatusUnauthorized
					errMsg = "unauthorized"
				}
				errors.WriteError(w, status, errMsg, strings.ToLower(errMsg))
				return
			}
			next(w, req)
			return
		}

		item, err := r.store.Get(req.Context(), m.Metadata.Name, id)
		if err != nil {
			errors.Write(w, errors.ErrNotFound)
			return
		}

		if !r.policyEngine.Evaluate(policy, claims, item) {
			errors.Write(w, errors.ErrForbidden)
			return
		}

		next(w, req)
	}
}

func policyStringEquals(p any, want string) bool {
	s, ok := p.(string)
	if !ok {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(s), want)
}

func isAuthenticatedOnlyPolicy(p any) bool {
	return policyStringEquals(p, "authenticated")
}
