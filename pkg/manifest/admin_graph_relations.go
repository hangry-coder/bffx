package manifest

import "strings"

func inferBelongsTo(fields []ResourceField) []AdminBelongsTo {
	var out []AdminBelongsTo
	seen := make(map[string]struct{})
	for _, f := range fields {
		if f.Target == "" {
			continue
		}
		if !strings.HasSuffix(f.Name, "_id") && f.Name != "id" {
			continue
		}
		if _, ok := seen[f.Name]; ok {
			continue
		}
		seen[f.Name] = struct{}{}
		out = append(out, AdminBelongsTo{
			Field:    f.Name,
			Resource: f.Target,
		})
	}
	return out
}

func mergeBelongsTo(explicit, inferred []AdminBelongsTo) []AdminBelongsTo {
	if len(explicit) == 0 {
		return inferred
	}
	byField := make(map[string]AdminBelongsTo, len(explicit)+len(inferred))
	for _, b := range inferred {
		byField[strings.ToLower(b.Field)] = b
	}
	for _, b := range explicit {
		byField[strings.ToLower(b.Field)] = b
	}
	out := make([]AdminBelongsTo, 0, len(byField))
	for _, b := range byField {
		out = append(out, b)
	}
	return out
}
