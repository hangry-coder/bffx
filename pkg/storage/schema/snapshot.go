package schema

import (
	"encoding/json"
	"os"
)

// LoadSnapshot loads a SchemaSnapshot from a JSON file
func LoadSnapshot(path string) (*SchemaSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SchemaSnapshot{Tables: make(map[string]TableDef)}, nil
		}
		return nil, err
	}

	var snap SchemaSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, err
	}

	if snap.Tables == nil {
		snap.Tables = make(map[string]TableDef)
	}

	return &snap, nil
}

// SaveSnapshot saves a SchemaSnapshot to a JSON file
func SaveSnapshot(path string, snap *SchemaSnapshot) error {
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
