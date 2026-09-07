package featureflags

import (
	"context"
	"errors"
)

var (
	ErrNotSupported = errors.New("feature flag operation not supported by this provider")
	ErrNotFound      = errors.New("feature flag not found")
)

// ProviderConfig holds configuration for a flag provider
type ProviderConfig struct {
	Name         string
	SDKKey       string
	URL          string
	ProjectKey   string
	Environment  string
	Root         string
	CustomConfig map[string]interface{}
}

// ResolvedFlag represents an evaluated flag with its value and metadata
type ResolvedFlag struct {
	Key       string      `json:"key"`
	Value     interface{} `json:"value"`
	Variation string      `json:"variation,omitempty"`
}

// FlagProvider defines the interface for feature flag management and evaluation
type FlagProvider interface {
	// Lifecycle
	Init(config ProviderConfig) error
	Close() error

	// CRUD (Admin Operations)
	// Note: These may return ErrNotSupported for 3rd party providers that don't allow management via SDK
	CreateFlag(flag FlagDefinition) (*FlagDefinition, error)
	UpdateFlag(key string, flag FlagDefinition) (*FlagDefinition, error)
	DeleteFlag(key string) error
	GetFlag(key string) (*FlagDefinition, error)
	ListFlags() ([]FlagDefinition, error)

	// Evaluation (Client Operations)
	Evaluate(key string, ctx EvalContext) (interface{}, error)
	EvaluateAll(ctx EvalContext) (map[string]ResolvedFlag, error)

	// Real-time updates
	Subscribe(ctx context.Context, onChange func(key string, value interface{})) error

	// Metadata
	Name() string // e.g., "bffx", "launchdarkly"
}
