package router

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProductionCode_DoesNotImportAdmin ensures public API packages do not
// depend on pkg/admin at compile time (Wave 6 admin/public boundary).
func TestProductionCode_DoesNotImportAdmin(t *testing.T) {
	t.Helper()
	root := filepath.Join("..", "..", "..") // repo root from pkg/api/router
	for _, pkgDir := range []string{
		filepath.Join(root, "pkg", "api", "router"),
		filepath.Join(root, "pkg", "api", "middleware"),
	} {
		if err := assertNoAdminImportsInDir(t, pkgDir); err != nil {
			t.Fatalf("%s: %v", pkgDir, err)
		}
	}
}

func assertNoAdminImportsInDir(t *testing.T, dir string) error {
	t.Helper()
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(p, "/pkg/admin") {
				t.Errorf("%s imports admin package %s", path, p)
			}
		}
		return nil
	})
}
