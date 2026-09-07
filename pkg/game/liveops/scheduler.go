package liveops

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hangry-coder/bffx/pkg/featureflags"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
)

// LiveOpsEvent represents a scheduled or overridden event.
type LiveOpsEvent struct {
	ID                   string         `json:"id"`
	Name                 string         `json:"name"`
	Title                string         `json:"title"`
	Description          string         `json:"description"`
	Priority             int            `json:"priority"`
	AudienceSegment      string         `json:"audience_segment"`
	Schedule             EventSchedule  `json:"schedule"`
	ConfigurationPayload map[string]any `json:"configuration_payload"`
	Enabled              bool           `json:"enabled"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	Version              int            `json:"version"`
}

// EventSchedule defines the start/end window for LiveOps events.
type EventSchedule struct {
	StartTime       string `json:"start_time" yaml:"start_time"`
	EndTime         string `json:"end_time" yaml:"end_time"`
	CooldownSeconds int    `json:"cooldown_seconds,omitempty" yaml:"cooldown_seconds,omitempty"`
}

type Scheduler struct {
	store  storage.Store
	reg    *manifest.Registry
	db     *sql.DB
	driver string

	mu  sync.RWMutex
	mem map[string]LiveOpsEvent
}

const tableName = "bffx_liveops_event"

func NewScheduler(store storage.Store, reg *manifest.Registry) *Scheduler {
	s := &Scheduler{
		store: store,
		reg:   reg,
		mem:   make(map[string]LiveOpsEvent),
	}
	s.detectSQL(store)
	return s
}

func (s *Scheduler) detectSQL(store storage.Store) {
	if store == nil {
		return
	}
	unwrapped := storage.UnwrapStore(store)
	type sqlBacked interface{ GetDB() *sql.DB }
	if router, ok := unwrapped.(*storage.RouterStore); ok {
		if rb, ok := router.Primary.(sqlBacked); ok {
			s.db = rb.GetDB()
			switch router.Primary.(type) {
			case *storage.SQLiteStore:
				s.driver = "sqlite"
			case *storage.PostgresStore:
				s.driver = "postgres"
			}
		}
	} else if rb, ok := unwrapped.(sqlBacked); ok {
		s.db = rb.GetDB()
		switch unwrapped.(type) {
		case *storage.SQLiteStore:
			s.driver = "sqlite"
		case *storage.PostgresStore:
			s.driver = "postgres"
		}
	}
}

func (s *Scheduler) Init() error {
	if s.db != nil {
		var stmt string
		switch s.driver {
		case "postgres":
			stmt = `CREATE TABLE IF NOT EXISTS ` + tableName + ` (
				id TEXT PRIMARY KEY,
				name TEXT UNIQUE NOT NULL,
				definition JSONB NOT NULL,
				version BIGINT NOT NULL DEFAULT 1,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			)`
		default:
			stmt = `CREATE TABLE IF NOT EXISTS ` + tableName + ` (
				id TEXT PRIMARY KEY,
				name TEXT UNIQUE NOT NULL,
				definition TEXT NOT NULL,
				version INTEGER NOT NULL DEFAULT 1,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			)`
		}
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("create %s: %w", tableName, err)
		}
	}
	return s.Reload()
}

func (s *Scheduler) loadManifestEvents(now time.Time, dst map[string]LiveOpsEvent) {
	if s.reg == nil {
		return
	}
	for _, m := range s.reg.LiveOpsEvents {
		var spec manifest.LiveOpsEventSpec
		if err := m.UnmarshalSpec(&spec); err == nil {
			name := m.Metadata.Name
			if name == "" {
				continue
			}
			dst[name] = LiveOpsEvent{
				ID:              uuid.NewString(),
				Name:            name,
				Title:           spec.Title,
				Description:     spec.Description,
				Priority:        spec.Priority,
				AudienceSegment: spec.AudienceSegment,
				Schedule: EventSchedule{
					StartTime:       spec.Schedule.StartTime,
					EndTime:         spec.Schedule.EndTime,
					CooldownSeconds: spec.Schedule.CooldownSeconds,
				},
				ConfigurationPayload: spec.ConfigurationPayload,
				Enabled:              true,
				CreatedAt:            now,
				UpdatedAt:            now,
				Version:              1,
			}
		}
	}
}

func (s *Scheduler) loadDBEvents(dst map[string]LiveOpsEvent) error {
	if s.db == nil {
		return nil
	}
	rows, err := s.db.Query(`SELECT definition FROM ` + tableName)
	if err != nil {
		// If table doesn't exist, it is fine (Init handles it)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var def string
		if err := rows.Scan(&def); err != nil {
			continue
		}
		var ev LiveOpsEvent
		if err := json.Unmarshal([]byte(def), &ev); err == nil {
			dst[ev.Name] = ev
		}
	}
	return nil
}

func (s *Scheduler) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	newMem := make(map[string]LiveOpsEvent)

	s.loadManifestEvents(time.Now().UTC(), newMem)
	if err := s.loadDBEvents(newMem); err != nil {
		return err
	}

	s.mem = newMem
	return nil
}

func (s *Scheduler) CreateEvent(ev LiveOpsEvent) (*LiveOpsEvent, error) {
	now := time.Now().UTC()
	ev.CreatedAt = now
	ev.UpdatedAt = now
	ev.Version = 1
	if ev.Name == "" {
		return nil, errors.New("event name is required")
	}
	if ev.ID == "" {
		ev.ID = uuid.NewString()
	}

	def, err := json.Marshal(ev)
	if err != nil {
		return nil, fmt.Errorf("marshal event: %w", err)
	}

	if s.db != nil {
		stmt := s.placeholders(`INSERT INTO ` + tableName + ` (id, name, definition, version, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`)
		if _, err := s.db.Exec(stmt, ev.ID, ev.Name, string(def), ev.Version, now.Format(time.RFC3339), now.Format(time.RFC3339)); err != nil {
			if isUniqueViolation(err) {
				return nil, fmt.Errorf("event with name %q already exists", ev.Name)
			}
			return nil, fmt.Errorf("insert event: %w", err)
		}
	}

	s.mu.Lock()
	s.mem[ev.Name] = ev
	s.mu.Unlock()
	return &ev, nil
}

func (s *Scheduler) UpdateEvent(name string, ev LiveOpsEvent) (*LiveOpsEvent, error) {
	existing, err := s.GetEvent(name)
	if err != nil {
		return nil, err
	}
	ev.Name = existing.Name
	ev.ID = existing.ID
	ev.CreatedAt = existing.CreatedAt
	ev.UpdatedAt = time.Now().UTC()
	ev.Version = existing.Version + 1

	def, err := json.Marshal(ev)
	if err != nil {
		return nil, fmt.Errorf("marshal event: %w", err)
	}

	if s.db != nil {
		stmt := s.placeholders(`UPDATE ` + tableName + ` SET definition = ?, version = ?, updated_at = ? WHERE name = ?`)
		if _, err := s.db.Exec(stmt, string(def), ev.Version, ev.UpdatedAt.Format(time.RFC3339), name); err != nil {
			return nil, fmt.Errorf("update event: %w", err)
		}
	}

	s.mu.Lock()
	s.mem[name] = ev
	s.mu.Unlock()
	return &ev, nil
}

func (s *Scheduler) DeleteEvent(name string) error {
	if s.db != nil {
		stmt := s.placeholders(`DELETE FROM ` + tableName + ` WHERE name = ?`)
		_, err := s.db.Exec(stmt, name)
		if err != nil {
			return fmt.Errorf("delete event: %w", err)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.mem[name]; !ok {
		return errors.New("event not found")
	}
	delete(s.mem, name)
	return nil
}

func (s *Scheduler) GetEvent(name string) (*LiveOpsEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ev, ok := s.mem[name]
	if !ok {
		return nil, errors.New("event not found")
	}
	return &ev, nil
}

func (s *Scheduler) ListEvents() ([]LiveOpsEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]LiveOpsEvent, 0, len(s.mem))
	for _, ev := range s.mem {
		out = append(out, ev)
	}
	// Sort by name for stability
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func eventActiveInWindow(ev LiveOpsEvent, now time.Time) bool {
	start, err1 := time.Parse(time.RFC3339, ev.Schedule.StartTime)
	end, err2 := time.Parse(time.RFC3339, ev.Schedule.EndTime)
	if err1 != nil || err2 != nil {
		return false
	}
	return !now.Before(start) && !now.After(end)
}

func audienceMatches(segment string, userSegments []string) bool {
	if segment == "" || strings.EqualFold(segment, "all") {
		return true
	}
	for _, seg := range userSegments {
		if strings.EqualFold(seg, segment) {
			return true
		}
	}
	return false
}

func sortEventsByPriority(active []LiveOpsEvent) {
	sort.Slice(active, func(i, j int) bool {
		if active[i].Priority != active[j].Priority {
			return active[i].Priority > active[j].Priority
		}
		return active[i].Name < active[j].Name
	})
}

func (s *Scheduler) Evaluate(ctx featureflags.EvalContext, now time.Time) []LiveOpsEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var active []LiveOpsEvent

	for _, ev := range s.mem {
		if !ev.Enabled {
			continue
		}

		// 1. Check if current time is within the event window.
		if !eventActiveInWindow(ev, now) {
			continue
		}

		// 2. Check audience segment targeting.
		if !audienceMatches(ev.AudienceSegment, ctx.Segments) {
			continue
		}

		active = append(active, ev)
	}

	// 3. Overlap conflict rules: sort by Priority descending, then Name ascending.
	sortEventsByPriority(active)

	return active
}

func (s *Scheduler) placeholders(stmt string) string {
	if s.driver != "postgres" {
		return stmt
	}
	var b strings.Builder
	b.Grow(len(stmt))
	idx := 0
	for _, r := range stmt {
		if r == '?' {
			idx++
			fmt.Fprintf(&b, "$%d", idx)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate")
}
