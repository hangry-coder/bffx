package router

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"strings"
)

// crudCollectionHTTPPath returns the collection segment base path /api/v1/{segment}.
func crudCollectionHTTPPath(spec manifest.ResourceSpec, resourceName string) string {
	p := strings.TrimSpace(spec.Routes.CollectionPath)
	p = strings.Trim(p, "/")
	if p != "" {
		return "/api/v1/" + p
	}
	name := strings.ToLower(strings.TrimSpace(resourceName))
	return "/api/v1/" + simplePlural(name)
}

func simplePlural(name string) string {
	if name == "" {
		return name
	}
	// Minimal pluralization for REST conventions; use routes.collectionPath for irregular nouns.
	return name + "s"
}

func policyReadIsOwner(readPolicy any) bool {
	s, ok := readPolicy.(string)
	return ok && strings.EqualFold(strings.TrimSpace(s), "owner")
}
