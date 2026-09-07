package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/hangry-coder/bffx/pkg/docsbundle"
)

// ListGuideDocs returns the bundled guide index for the admin panel.
func (h *DocsHandler) ListGuideDocs(w http.ResponseWriter, r *http.Request) {
	if !h.isEnabled() {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	_ = docsbundle.EnsureBundled(h.reg.Root)

	idxPath := filepath.Join(docsbundle.ProjectDocsDir(h.reg.Root), "index.json")
	data, err := os.ReadFile(idxPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(docsbundle.Index{Entries: []docsbundle.IndexEntry{}})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// GetGuideDoc returns one markdown guide file from .bffx/docs/.
func (h *DocsHandler) GetGuideDoc(w http.ResponseWriter, r *http.Request) {
	if !h.isEnabled() {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	rel := strings.TrimPrefix(r.PathValue("path"), "/")
	rel = filepath.ToSlash(rel)
	if rel == "" || strings.Contains(rel, "..") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	full := filepath.Join(docsbundle.ProjectDocsDir(h.reg.Root), filepath.FromSlash(rel))
	abs, err := filepath.Abs(full)
	if err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	base, err := filepath.Abs(docsbundle.ProjectDocsDir(h.reg.Root))
	if err != nil || !strings.HasPrefix(abs, base+string(filepath.Separator)) && abs != base {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"path":     rel,
		"title":    docsbundleTitle(string(data), rel),
		"markdown": string(data),
	})
}

func docsbundleTitle(body, rel string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return rel
}
