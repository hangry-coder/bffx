package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/hangry-coder/bffx/pkg/admin/navprefs"
	"github.com/hangry-coder/bffx/pkg/manifest"
)

func manifestNavPrefs(p navprefs.Preferences) manifest.NavPreferences {
	return manifest.NavPreferences{
		Resources: p.Resources,
		Features:  p.Features,
	}
}

func (h *ResourceHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	var graph *manifest.AdminGraph
	path := filepath.Join(h.reg.Root, ".bffx", "admin-graph.json")
	if bytes, err := os.ReadFile(path); err == nil {
		json.Unmarshal(bytes, &graph)
	}

	prefs := navprefs.Load(h.reg.Root)
	mp := manifestNavPrefs(prefs)

	if graph == nil {
		var err error
		graph, err = manifest.BuildAdminGraph(h.reg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		manifest.ApplyNavPreferences(graph, mp)
		manifest.ReconcileAdminMenu(graph, h.reg, mp)
	} else {
		manifest.ApplyNavPreferences(graph, mp)
		manifest.ReconcileAdminMenu(graph, h.reg, mp)
	}

	if os.Getenv("BFFX_ENV") == "production" {
		graph.Site.Features.ApiDocs = false
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(graph)
}
