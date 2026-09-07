package manifest

import (
	"fmt"
	"strings"
)

// TreeEnabled reports whether hierarchical (closure-table) storage is enabled.
// YAML may use bool or legacy strings: "ownership" (true), "none"/"" (false).
func TreeEnabled(tree any) bool {
	switch v := tree.(type) {
	case bool:
		return v
	case string:
		s := strings.ToLower(strings.TrimSpace(v))
		switch s {
		case "ownership", "true", "yes", "1":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

// TreeLintMessage returns a non-empty warning when `tree` is a string outside the known set.
func TreeLintMessage(tree any) string {
	s, ok := tree.(string)
	if !ok {
		return ""
	}
	t := strings.ToLower(strings.TrimSpace(s))
	switch t {
	case "", "none", "false", "no", "0", "ownership", "true", "yes", "1":
		return ""
	default:
		return fmt.Sprintf("unknown tree value %q (use true/false or ownership/none)", strings.TrimSpace(s))
	}
}
