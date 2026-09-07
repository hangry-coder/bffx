package i18n

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type Bundle struct {
	mu           sync.RWMutex
	translations map[string]map[string]string // locale -> key -> value
	defaultLocale string
}

func NewBundle(defaultLocale string) *Bundle {
	if defaultLocale == "" {
		defaultLocale = "en"
	}
	return &Bundle{
		translations:  make(map[string]map[string]string),
		defaultLocale: defaultLocale,
	}
}

// LoadFromDir loads all .yaml files in the given directory.
// Filenames should match locale (e.g. en.yaml, es.yaml).
func (b *Bundle) LoadFromDir(dir string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, f := range files {
		if !f.IsDir() && (strings.HasSuffix(f.Name(), ".yaml") || strings.HasSuffix(f.Name(), ".yml")) {
			locale := strings.TrimSuffix(strings.TrimSuffix(f.Name(), ".yaml"), ".yml")
			path := filepath.Join(dir, f.Name())
			
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			var data map[string]string
			if err := yaml.Unmarshal(content, &data); err != nil {
				return fmt.Errorf("error parsing %s: %w", f.Name(), err)
			}

			if b.translations[locale] == nil {
				b.translations[locale] = make(map[string]string)
			}
			for k, v := range data {
				b.translations[locale][k] = v
			}
		}
	}
	return nil
}

// DiscoverAndLoad scans the project root for translations in various locations:
// 1. i18n/ (Legacy)
// 2. assets/i18n/ (V2 Global)
// 3. internal/features/**/i18n/ (V2 Feature-specific)
func (b *Bundle) DiscoverAndLoad(root string) error {
	// 1. Legacy
	if err := b.LoadFromDir(filepath.Join(root, "i18n")); err != nil {
		return err
	}

	// 2. V2 Global
	if err := b.LoadFromDir(filepath.Join(root, "assets", "i18n")); err != nil {
		return err
	}

	// 3. V2 Features
	featuresDir := filepath.Join(root, "internal", "features")
	if entries, err := os.ReadDir(featuresDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				featureI18nDir := filepath.Join(featuresDir, entry.Name(), "i18n")
				if err := b.LoadFromDir(featureI18nDir); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// Translate returns the translation for the key in the given locale.
// Fallback logic: locale -> defaultLocale -> key.
func (b *Bundle) Translate(locale, key string, overrides map[string]string) string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// 1. Check dynamic overrides first (e.g. from DB)
	if val, ok := overrides[key]; ok {
		return val
	}

	// 2. Check current locale
	if b.translations[locale] != nil {
		if val, ok := b.translations[locale][key]; ok {
			return val
		}
	}

	// 3. Check default locale
	if b.translations[b.defaultLocale] != nil {
		if val, ok := b.translations[b.defaultLocale][key]; ok {
			return val
		}
	}

	return key
}

// GetMap returns a full map of translations for a locale, merged with overrides.
func (b *Bundle) GetMap(locale string, overrides map[string]string) map[string]string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make(map[string]string)

	// Load defaults
	if b.translations[b.defaultLocale] != nil {
		for k, v := range b.translations[b.defaultLocale] {
			result[k] = v
		}
	}

	// Load locale
	if locale != b.defaultLocale && b.translations[locale] != nil {
		for k, v := range b.translations[locale] {
			result[k] = v
		}
	}

	// Load overrides
	for k, v := range overrides {
		result[k] = v
	}

	return result
}
