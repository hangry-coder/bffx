package wire

import (
	"strings"
)

// Config represents the wire battery configuration options mapped from ProjectSpec.
type Config struct {
	Enabled      bool   `yaml:"enabled"`
	GRPCPort     int    `yaml:"grpc_port"`
	Package      string `yaml:"package"`
	ProdJSON     bool   `yaml:"prod_json"`
	ForceDevJSON bool   `yaml:"force_dev_json"`
}

// HasTransport checks if a given transport (e.g., "rest" or "grpc") is allowed.
// If the transports slice is empty, it defaults to true (all transports are allowed).
func HasTransport(transports []string, transport string) bool {
	if len(transports) == 0 {
		return true
	}
	tLower := strings.ToLower(transport)
	for _, t := range transports {
		if strings.ToLower(t) == tLower {
			return true
		}
	}
	return false
}
