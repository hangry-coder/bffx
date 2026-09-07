package runtimecontracts

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// RunOpts captures the impersonation knobs used to inspect a Screen.
type RunOpts struct {
	UserID      string
	Locale      string
	Query       url.Values
	Headers     http.Header
	ExtraClaims map[string]any
}

// RunResult is the structured payload returned by in-process screen runs.
type RunResult struct {
	Output map[string]any `json:"output"`
	Errors []string       `json:"errors,omitempty"`
}

// ScreenRunner executes a screen in-process with impersonation options.
type ScreenRunner interface {
	RunScreen(ctx context.Context, screen string, opts RunOpts) (RunResult, error)
}

// CacheInvalidator flushes cached screen/section responses by tag.
type CacheInvalidator interface {
	InvalidateScreenCache(ctx context.Context, screen string) (int, error)
	InvalidateSectionCache(ctx context.Context, screen, section string) (int, error)
}

// KillSwitch is the persistent record for a screen- or section-level kill switch.
type KillSwitch struct {
	Screen    string     `json:"screen"`
	Section   string     `json:"section,omitempty"`
	Enabled   bool       `json:"enabled"`
	Reason    string     `json:"reason,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	UpdatedBy string     `json:"updated_by,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// IsActive returns true when the switch is enabled and not expired.
func (ks *KillSwitch) IsActive(now time.Time) bool {
	if ks == nil || !ks.Enabled {
		return false
	}
	if ks.ExpiresAt != nil && now.After(*ks.ExpiresAt) {
		return false
	}
	return true
}

// IsActiveNow is shorthand for IsActive(time.Now()).
func (ks *KillSwitch) IsActiveNow() bool {
	return ks.IsActive(time.Now())
}

// KillSwitchUpdate is the input payload for SetKillSwitch.
type KillSwitchUpdate struct {
	Enabled   bool       `json:"enabled"`
	Reason    string     `json:"reason,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	UpdatedBy string     `json:"updated_by,omitempty"`
}

// KillSwitchStore is the storage contract for kill switches.
type KillSwitchStore interface {
	Set(ctx context.Context, screen, section string, upd KillSwitchUpdate) (KillSwitch, error)
	Get(ctx context.Context, screen, section string) (*KillSwitch, error)
	Delete(ctx context.Context, screen, section string) error
	List(ctx context.Context) ([]KillSwitch, error)
}
