package liveops

import (
	"testing"
	"time"

	"github.com/hangry-coder/bffx/pkg/featureflags"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestScheduler_Evaluate(t *testing.T) {
	s := NewScheduler(nil, nil)

	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)

	// Set up memory events
	s.mu.Lock()
	s.mem = map[string]LiveOpsEvent{
		"evt_double_xp": {
			Name:            "evt_double_xp",
			Title:           "Double XP",
			Enabled:         true,
			Priority:        10,
			AudienceSegment: "paying_users",
			Schedule: EventSchedule{
				StartTime: "2026-06-05T00:00:00Z",
				EndTime:   "2026-06-07T00:00:00Z",
			},
			ConfigurationPayload: map[string]any{"multiplier": 2.0},
		},
		"evt_gold_rush": {
			Name:            "evt_gold_rush",
			Title:           "Gold Rush",
			Enabled:         true,
			Priority:        20,
			AudienceSegment: "all",
			Schedule: EventSchedule{
				StartTime: "2026-06-05T00:00:00Z",
				EndTime:   "2026-06-07T00:00:00Z",
			},
			ConfigurationPayload: map[string]any{"gold_multiplier": 1.5},
		},
		"evt_ended": {
			Name:            "evt_ended",
			Title:           "Ended Event",
			Enabled:         true,
			Priority:        100,
			AudienceSegment: "all",
			Schedule: EventSchedule{
				StartTime: "2026-06-01T00:00:00Z",
				EndTime:   "2026-06-02T00:00:00Z",
			},
		},
		"evt_disabled": {
			Name:            "evt_disabled",
			Title:           "Disabled Event",
			Enabled:         false,
			Priority:        1000,
			AudienceSegment: "all",
			Schedule: EventSchedule{
				StartTime: "2026-06-05T00:00:00Z",
				EndTime:   "2026-06-07T00:00:00Z",
			},
		},
	}
	s.mu.Unlock()

	// Scenario A: Evaluates matching segments + time
	t.Run("Paying User Evaluation", func(t *testing.T) {
		ctx := featureflags.EvalContext{
			UserID:   "user_1",
			Segments: []string{"paying_users"},
		}
		active := s.Evaluate(ctx, now)
		require.Len(t, active, 2)
		// Priority sorting check: evt_gold_rush (Priority 20) first, then evt_double_xp (Priority 10)
		assert.Equal(t, "evt_gold_rush", active[0].Name)
		assert.Equal(t, "evt_double_xp", active[1].Name)
	})

	// Scenario B: Evaluates mismatching segment (not paying)
	t.Run("Non-Paying User Evaluation", func(t *testing.T) {
		ctx := featureflags.EvalContext{
			UserID:   "user_2",
			Segments: []string{"free_tier"},
		}
		active := s.Evaluate(ctx, now)
		require.Len(t, active, 1)
		assert.Equal(t, "evt_gold_rush", active[0].Name)
	})
}

func TestScheduler_CRUD_InMemory(t *testing.T) {
	s := NewScheduler(nil, nil)

	ev := LiveOpsEvent{
		Name:            "test_evt",
		Title:           "Test Event",
		Enabled:         true,
		Priority:        5,
		AudienceSegment: "all",
		Schedule: EventSchedule{
			StartTime: "2026-06-05T00:00:00Z",
			EndTime:   "2026-06-07T00:00:00Z",
		},
		ConfigurationPayload: map[string]any{"foo": "bar"},
	}

	// Create
	created, err := s.CreateEvent(ev)
	require.NoError(t, err)
	assert.Equal(t, ev.Name, created.Name)
	assert.Equal(t, 1, created.Version)
	assert.NotEmpty(t, created.ID)

	// Get
	fetched, err := s.GetEvent("test_evt")
	require.NoError(t, err)
	assert.Equal(t, "Test Event", fetched.Title)

	// Update
	fetched.Title = "Updated Title"
	updated, err := s.UpdateEvent("test_evt", *fetched)
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", updated.Title)
	assert.Equal(t, 2, updated.Version)

	// List
	list, err := s.ListEvents()
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// Delete
	err = s.DeleteEvent("test_evt")
	require.NoError(t, err)

	_, err = s.GetEvent("test_evt")
	assert.Error(t, err)
}

func TestScheduler_RegistryLoad(t *testing.T) {
	// Parse a mock manifest registry with a LiveOpsEvent using yaml unmarshaling
	var m manifest.Manifest
	err := yaml.Unmarshal([]byte(`
apiVersion: bffx.io/v1alpha1
kind: LiveOpsEvent
metadata:
  name: weekly_double_xp
spec:
  title: "Double XP Weekend"
  description: "Earn double rewards."
  priority: 10
  audience_segment: "all"
  schedule:
    start_time: "2026-06-05T18:00:00Z"
    end_time: "2026-06-07T23:59:59Z"
  configuration_payload:
    xp_multiplier: 2.0
`), &m)
	require.NoError(t, err)

	reg := &manifest.Registry{
		LiveOpsEvents: []*manifest.Manifest{&m},
	}

	s := NewScheduler(nil, reg)
	err = s.Reload()
	require.NoError(t, err)

	fetched, err := s.GetEvent("weekly_double_xp")
	require.NoError(t, err)
	assert.Equal(t, "Double XP Weekend", fetched.Title)
	assert.Equal(t, 10, fetched.Priority)
	assert.Equal(t, 2.0, fetched.ConfigurationPayload["xp_multiplier"])
}
