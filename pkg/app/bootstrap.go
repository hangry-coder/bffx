package app

// BootstrapEnv loads .env and config/<BFFX_ENV>.yaml overlays for CLI commands
// (sync, dev) that run before the orchestrator starts.
func BootstrapEnv(root string) error {
	if err := LoadEnv(root); err != nil {
		return err
	}
	if _, err := LoadConfigOverlay(root); err != nil {
		return err
	}
	return nil
}
