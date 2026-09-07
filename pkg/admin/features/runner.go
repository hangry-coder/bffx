package features

import (
	"github.com/hangry-coder/bffx/pkg/runtimecontracts"
)

// Re-export shared runtime contracts for backwards compatibility.
type RunOpts = runtimecontracts.RunOpts
type RunResult = runtimecontracts.RunResult
type ScreenRunner = runtimecontracts.ScreenRunner
type CacheInvalidator = runtimecontracts.CacheInvalidator
