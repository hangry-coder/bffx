package buildprofile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var errMissingProject = errors.New("build profile requires a Project manifest")

// Path returns the canonical build profile path under a project root.
func Path(root string) string {
	return filepath.Join(root, ".bffx", Filename)
}

// Load reads .bffx/build-profile.json from root.
func Load(root string) (*Profile, error) {
	b, err := os.ReadFile(Path(root))
	if err != nil {
		return nil, err
	}
	var prof Profile
	if err := json.Unmarshal(b, &prof); err != nil {
		return nil, fmt.Errorf("parse %s: %w", Filename, err)
	}
	return &prof, nil
}

// Write marshals and writes the profile to .bffx/build-profile.json.
func Write(root string, prof *Profile) error {
	if prof == nil {
		return fmt.Errorf("nil profile")
	}
	outDir := filepath.Join(root, ".bffx")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(prof, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(Path(root), b, 0o644)
}
