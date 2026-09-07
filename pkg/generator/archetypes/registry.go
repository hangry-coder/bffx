package archetypes

import (
	_ "embed"
	"fmt"
	"gopkg.in/yaml.v3"
)

//go:embed registry.yaml
var registryData []byte

type Registry struct {
	Archetypes []Archetype `yaml:"archetypes"`
}

type Archetype struct {
	Name         string          `yaml:"name"`
	Description  string          `yaml:"description"`
	AuthStrategy string          `yaml:"auth_strategy"`
	Layout       string          `yaml:"layout"`
	Minimal      bool            `yaml:"minimal"`
	Batteries    BatteryConfig   `yaml:"batteries"`
	Runtime      RuntimeConfig   `yaml:"runtime"`
	Addons       AddonConfig     `yaml:"addons"`
	Pipeline     *PipelineConfig `yaml:"pipeline,omitempty"`
}

type BatteryConfig struct {
	Auth          string `yaml:"auth"`
	Store         string `yaml:"store"`
	Cache         string `yaml:"cache"`
	Blob          string `yaml:"blob"`
	Analytics     string `yaml:"analytics"`
	Observability string `yaml:"observability"`
	Flags         string `yaml:"flags"`
	I18n          string `yaml:"i18n"`
}

type RuntimeConfig struct {
	Redis          bool   `yaml:"redis"`
	Worker         bool   `yaml:"worker"`
	WorkerLanguage string `yaml:"worker_language"`
	Streaming      bool   `yaml:"streaming"`
}

type AddonConfig struct {
	Monetization bool `yaml:"monetization"`
	Flags        bool `yaml:"flags"`
	Telemetry    bool `yaml:"telemetry"`
	Ads          bool `yaml:"ads"`
}

type PipelineConfig struct {
	Type    string `yaml:"type"`
	Name    string `yaml:"name"`
	Feature string `yaml:"feature"`
}

var loadedRegistry *Registry

func init() {
	var r Registry
	if err := yaml.Unmarshal(registryData, &r); err != nil {
		panic(fmt.Sprintf("failed to parse embedded archetype registry: %v", err))
	}
	loadedRegistry = &r
}

func GetRegistry() *Registry {
	return loadedRegistry
}

func LookupArchetype(name string) (Archetype, bool) {
	for _, a := range loadedRegistry.Archetypes {
		if a.Name == name {
			return a, true
		}
	}
	return Archetype{}, false
}
