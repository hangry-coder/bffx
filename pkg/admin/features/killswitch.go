package features

import (
	"context"
	"github.com/hangry-coder/bffx/pkg/runtimecontracts"
	"sync"
	"time"
)

// Re-export shared runtime contracts for backwards compatibility.
type KillSwitch = runtimecontracts.KillSwitch
type KillSwitchUpdate = runtimecontracts.KillSwitchUpdate
type KillSwitchStore = runtimecontracts.KillSwitchStore

// MemoryKillSwitchStore is an in-process KillSwitchStore used in development
// and tests. It is safe for concurrent use but does not persist across
// restarts. Production deployments should plug in a SQL-backed implementation
// (see killswitch_sql.go).
type MemoryKillSwitchStore struct {
	mu      sync.RWMutex
	entries map[string]KillSwitch
	now     func() time.Time
}

// NewMemoryKillSwitchStore constructs a fresh, empty in-memory store.
func NewMemoryKillSwitchStore() *MemoryKillSwitchStore {
	return &MemoryKillSwitchStore{
		entries: make(map[string]KillSwitch),
		now:     time.Now,
	}
}

// SetClock overrides the time source for deterministic tests.
func (s *MemoryKillSwitchStore) SetClock(now func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if now != nil {
		s.now = now
	}
}

func killSwitchKey(screen, section string) string {
	return screen + "::" + section
}

func (s *MemoryKillSwitchStore) Set(_ context.Context, screen, section string, upd KillSwitchUpdate) (KillSwitch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ks := KillSwitch{
		Screen:    screen,
		Section:   section,
		Enabled:   upd.Enabled,
		Reason:    upd.Reason,
		ExpiresAt: upd.ExpiresAt,
		UpdatedBy: upd.UpdatedBy,
		UpdatedAt: s.now(),
	}
	s.entries[killSwitchKey(screen, section)] = ks
	return ks, nil
}

func (s *MemoryKillSwitchStore) Get(_ context.Context, screen, section string) (*KillSwitch, error) {
	s.mu.RLock()
	ks, ok := s.entries[killSwitchKey(screen, section)]
	s.mu.RUnlock()
	if !ok {
		return nil, nil
	}
	// Auto-revert expired switches: return nil so callers treat the
	// feature as live. The row itself is left in place; the admin UI can
	// still show it (via List) so operators see the history.
	if ks.ExpiresAt != nil && s.now().After(*ks.ExpiresAt) {
		return nil, nil
	}
	out := ks
	return &out, nil
}

func (s *MemoryKillSwitchStore) Delete(_ context.Context, screen, section string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, killSwitchKey(screen, section))
	return nil
}

func (s *MemoryKillSwitchStore) List(_ context.Context) ([]KillSwitch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]KillSwitch, 0, len(s.entries))
	for _, v := range s.entries {
		out = append(out, v)
	}
	return out, nil
}

// Evaluate is a small helper for screen/section pipelines. It returns the
// active kill switch (if any) for the supplied screen[/section]. Section-
// level switches take precedence over screen-level — a screen-wide kill
// supersedes section-level for the screen handler, but for a section route
// the section-level switch (if active) takes precedence; on miss, the
// screen-level switch is consulted.
func Evaluate(ctx context.Context, store KillSwitchStore, screen, section string) (*KillSwitch, error) {
	if store == nil {
		return nil, nil
	}
	now := time.Now()
	if section != "" {
		ks, err := store.Get(ctx, screen, section)
		if err != nil {
			return nil, err
		}
		if ks.IsActive(now) {
			return ks, nil
		}
	}
	ks, err := store.Get(ctx, screen, "")
	if err != nil {
		return nil, err
	}
	if ks.IsActive(now) {
		return ks, nil
	}
	return nil, nil
}
