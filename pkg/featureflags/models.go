package featureflags

import (
	"time"
)

// FlagType defines the data type of the flag value
type FlagType string

const (
	FlagTypeBoolean FlagType = "boolean"
	FlagTypeString  FlagType = "string"
	FlagTypeNumber  FlagType = "number"
	FlagTypeJSON    FlagType = "json"
)

// FlagDefinition represents a complete feature flag with its targeting rules
type FlagDefinition struct {
	Key           string              `json:"key" yaml:"key"`
	Name          string              `json:"name" yaml:"name"`
	Description   string              `json:"description" yaml:"description"`
	Enabled       bool                `json:"enabled" yaml:"enabled"`
	Type          FlagType            `json:"type" yaml:"type"`
	Tags          []string            `json:"tags,omitempty" yaml:"tags,omitempty"`
	Variations    []Variation         `json:"variations" yaml:"variations"`
	Rules         []TargetingRule     `json:"rules,omitempty" yaml:"rules,omitempty"`
	Fallthrough   VariationSelection  `json:"fallthrough" yaml:"fallthrough"`
	OffVariation  int                 `json:"off_variation" yaml:"off_variation"`
	
	// Bindings link flags to screen sections
	Bindings      []FlagBinding       `json:"bindings,omitempty" yaml:"bindings,omitempty"`
	
	// Targeting is the simplified targeting rules metadata for rules-based evaluation
	Targeting     *TargetingRules     `json:"targeting,omitempty" yaml:"targeting,omitempty"`
	
	// Prerequisites for flag dependencies
	Prerequisites []Prerequisite      `json:"prerequisites,omitempty" yaml:"prerequisites,omitempty"`
	
	// Audit metadata
	CreatedAt     time.Time           `json:"created_at" yaml:"created_at"`
	UpdatedAt     time.Time           `json:"updated_at" yaml:"updated_at"`
	Version       int                 `json:"version" yaml:"version"`
}

// Variation defines a specific value option for a flag
type Variation struct {
	Key         string      `json:"key" yaml:"key"`                 // e.g., "on", "off", "red-button"
	Value       interface{} `json:"value" yaml:"value"`             // The actual value returned to the client
	Name        string      `json:"name,omitempty" yaml:"name,omitempty"`
	Description string      `json:"description,omitempty" yaml:"description,omitempty"`
}

// TargetingRule defines criteria for serving a specific variation
type TargetingRule struct {
	Clauses   []Clause           `json:"clauses" yaml:"clauses"`
	Variation *int               `json:"variation,omitempty" yaml:"variation,omitempty"`
	Rollout   *Rollout           `json:"rollout,omitempty" yaml:"rollout,omitempty"`
}

// Clause represents a single targeting condition
type Clause struct {
	Attribute string        `json:"attribute" yaml:"attribute"`
	Operator  string        `json:"operator" yaml:"operator"` // in, notIn, matches, startsWith, endsWith, contains, greaterThan, lessThan, semVerEqual, etc.
	Values    []interface{} `json:"values" yaml:"values"`
	Negate    bool          `json:"negate" yaml:"negate"`
}

// Rollout defines a percentage-based distribution of variations
type Rollout struct {
	Variations []WeightedVariation `json:"variations" yaml:"variations"`
	BucketBy   string              `json:"bucketBy,omitempty" yaml:"bucketBy,omitempty"` // defaults to "id"
}

// WeightedVariation defines a variation with its rollout weight
type WeightedVariation struct {
	Variation int `json:"variation" yaml:"variation"`
	Weight    int `json:"weight" yaml:"weight"` // 0-100000 (representing 0.00% to 100.00%)
}

// VariationSelection specifies which variation or rollout to use
type VariationSelection struct {
	Variation *int     `json:"variation,omitempty" yaml:"variation,omitempty"`
	Rollout   *Rollout `json:"rollout,omitempty" yaml:"rollout,omitempty"`
}

// FlagBinding links a flag to specific UI elements in a mobile app
type FlagBinding struct {
	ScreenName  string   `json:"screen_name" yaml:"screen_name"`   // e.g., "Home"
	SectionKeys []string `json:"section_keys" yaml:"section_keys"` // e.g., ["premium_banner", "search_bar"]
	Scope       string   `json:"scope" yaml:"scope"`               // "visibility", "data", "behavior"
}

// Prerequisite defines a dependency on another flag's evaluation
type Prerequisite struct {
	FlagKey   string `json:"flag_key" yaml:"flag_key"`
	Variation int    `json:"variation" yaml:"variation"` // The required variation index of the prerequisite flag
}

// EvalContext represents the identity and attributes used for flag evaluation
type EvalContext struct {
	UserID      string                 `json:"user_id"`
	DeviceID    string                 `json:"device_id"`
	Anonymous   bool                   `json:"anonymous"`
	UserRole    string                 `json:"user_role"`
	Email       string                 `json:"email"`
	Country     string                 `json:"country"`
	AppVersion  string                 `json:"app_version"`
	Platform    string                 `json:"platform"` // "ios", "android"
	OSVersion   string                 `json:"os_version"`
	BuildNumber int                    `json:"build_number"`
	Segments    []string               `json:"segments,omitempty"`
	Claims      map[string]interface{} `json:"claims,omitempty"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
}

type TargetingRules struct {
	Users    []string `json:"users" yaml:"users"`
	Segments []string `json:"segments" yaml:"segments"`
	Rollout  int      `json:"rollout" yaml:"rollout"` // 0-100
}
