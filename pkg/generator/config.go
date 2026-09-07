package generator

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type UserConfig struct {
	Name      string                `yaml:"name"`
	Archetype string                `yaml:"archetype"`
	Layout    string                `yaml:"layout"`
	Minimal   bool                  `yaml:"minimal"`
	Admin     *AdminConfig          `yaml:"admin"`
	Batteries *UserBatteryOverrides `yaml:"batteries"`
	Env       map[string]string     `yaml:"env"`
}

type AdminConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Email    string `yaml:"email"`
	Password string `yaml:"password"`
}

type UserBatteryOverrides struct {
	Auth          string `yaml:"auth"`
	Store         string `yaml:"store"`
	Cache         string `yaml:"cache"`
	Blob          string `yaml:"blob"`
	Analytics     string `yaml:"analytics"`
	Observability string `yaml:"observability"`
	Flags         string `yaml:"flags"`
	Vlm           string `yaml:"vlm"`
}

// LoadUserConfig reads and parses the UserConfig from a YAML file.
func LoadUserConfig(path string) (*UserConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read user config file: %w", err)
	}

	var cfg UserConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse user config yaml: %w", err)
	}

	return &cfg, nil
}
