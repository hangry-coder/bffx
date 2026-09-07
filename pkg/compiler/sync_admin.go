package compiler

import (
	"github.com/hangry-coder/bffx/pkg/admin/navprefs"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

func buildAdminGraph(reg *manifest.Registry) (*manifest.AdminGraph, error) {
	return manifest.BuildAdminGraph(reg)
}

func loadAdminNavPrefs(root string) manifest.NavPreferences {
	p := navprefs.Load(root)
	return manifest.NavPreferences{
		Resources: p.Resources,
		Features:  p.Features,
	}
}
