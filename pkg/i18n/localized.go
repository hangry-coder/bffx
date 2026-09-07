package i18n

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// LocalizedString is a map of locale codes to their respective translations.
// Example: {"en": "Hello", "es": "Hola"}
type LocalizedString map[string]string

// Get returns the translation for the specified locale.
// It falls back to "en" if the locale is not found, or returns an empty string.
func (l LocalizedString) Get(locale string) string {
	if l == nil {
		return ""
	}
	if val, ok := l[locale]; ok {
		return val
	}
	// Fallback to en
	return l["en"]
}

// Value implements the driver.Valuer interface for SQL compatibility.
func (l LocalizedString) Value() (driver.Value, error) {
	if l == nil {
		return nil, nil
	}
	return json.Marshal(l)
}

// Scan implements the sql.Scanner interface for SQL compatibility.
func (l *LocalizedString) Scan(value interface{}) error {
	if value == nil {
		*l = nil
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, l)
}
