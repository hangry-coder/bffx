package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"gopkg.in/yaml.v3"
)

// ConfigOverlay represents the structure of the config/<env>.yaml files.
type ConfigOverlay struct {
	Port     int               `yaml:"port"`
	LogLevel string            `yaml:"logLevel"`
	Redis    struct {
		Url string `yaml:"url"`
	} `yaml:"redis"`
	Env map[string]string `yaml:"env"`
}

func (c *ConfigOverlay) ApplyTo(spec *manifest.ProjectSpec) {
	if c == nil {
		return
	}
	if c.Port > 0 {
		spec.Runtime.Api.Port = c.Port
	}
	if c.Redis.Url != "" {
		spec.Runtime.Redis.Url = c.Redis.Url
		spec.Runtime.Redis.Enabled = true
	}
}

var systemEnv = make(map[string]bool)

func init() {
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		if len(pair) > 0 {
			systemEnv[pair[0]] = true
		}
	}
}

// LoadConfigOverlay reads config/<env>.yaml and applies it as an overlay.
func LoadConfigOverlay(root string) (*ConfigOverlay, error) {
	env := os.Getenv("BFFX_ENV")
	if env == "" {
		env = "development"
	}

	configPath := filepath.Join(root, "config", env+".yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read config overlay: %w", err)
	}

	var overlay ConfigOverlay
	if err := yaml.Unmarshal(data, &overlay); err != nil {
		return nil, fmt.Errorf("parse config overlay: %w", err)
	}

	// Apply structured fields
	if overlay.Port > 0 {
		setEnvOverride("PORT", fmt.Sprintf("%d", overlay.Port))
		setEnvOverride("BFFX_PORT", fmt.Sprintf("%d", overlay.Port))
	}
	if overlay.LogLevel != "" {
		setEnvOverride("LOG_LEVEL", overlay.LogLevel)
	}
	if overlay.Redis.Url != "" {
		setEnvOverride("BFFX_REDIS_URL", overlay.Redis.Url)
	}

	for k, v := range overlay.Env {
		setEnvOverride(k, v)
	}

	return &overlay, nil
}

func setEnvOverride(key, value string) {
	// Only set if not in the original system environment
	if !systemEnv[key] {
		os.Setenv(key, value)
	}
}

