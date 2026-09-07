package validation

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"fmt"
	"strings"
)

type Validator struct{}

func NewValidator() *Validator {
	return &Validator{}
}

// AllowedPayload keeps only keys declared on the resource manifest (strong params).
func AllowedPayload(spec *manifest.ResourceSpec, payload map[string]any) map[string]any {
	if payload == nil {
		return map[string]any{}
	}
	out := make(map[string]any)
	for _, f := range spec.Fields {
		if val, ok := payload[f.Name]; ok {
			out[f.Name] = val
		}
	}
	return out
}

func (v *Validator) Validate(spec *manifest.ResourceSpec, payload map[string]any, isUpdate bool) error {
	for _, field := range spec.Fields {
		val, ok := payload[field.Name]
		if field.Required && !ok && !isUpdate {
			return fmt.Errorf("field %s is required", field.Name)
		}

		if ok && val != nil {
			if err := v.checkType(field.Type, val); err != nil {
				return fmt.Errorf("field %s: %w", field.Name, err)
			}
		}
	}
	return nil
}

func (v *Validator) checkType(expected string, actual any) error {
	switch strings.ToLower(expected) {
	case "string":
		if _, ok := actual.(string); !ok {
			return fmt.Errorf("expected string, got %T", actual)
		}
	case "int", "integer":
		// JSON numbers are often float64
		switch actual.(type) {
		case int, int64, float64:
			return nil
		default:
			return fmt.Errorf("expected integer, got %T", actual)
		}
	case "float", "number":
		switch actual.(type) {
		case float32, float64, int, int64:
			return nil
		default:
			return fmt.Errorf("expected number, got %T", actual)
		}
	case "bool", "boolean":
		if _, ok := actual.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", actual)
		}
	}
	return nil
}
