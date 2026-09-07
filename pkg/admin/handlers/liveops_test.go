package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hangry-coder/bffx/pkg/game/liveops"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLiveOpsAdminHandler(t *testing.T) {
	scheduler := liveops.NewScheduler(nil, nil)
	err := scheduler.Init()
	require.NoError(t, err)

	h := NewLiveOpsHandler(scheduler, nil)

	// Create
	t.Run("Create Event", func(t *testing.T) {
		body := map[string]any{
			"name":             "evt_christmas",
			"title":            "Christmas Special",
			"enabled":          true,
			"priority":         50,
			"audience_segment": "all",
			"schedule": map[string]any{
				"start_time": "2026-12-24T00:00:00Z",
				"end_time":   "2026-12-26T00:00:00Z",
			},
			"configuration_payload": map[string]any{
				"gift": "santa_box",
			},
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/admin/liveops", bytes.NewReader(b))
		rr := httptest.NewRecorder()

		h.CreateEvent(rr, req)
		assert.Equal(t, http.StatusCreated, rr.Code)

		var res liveops.LiveOpsEvent
		json.Unmarshal(rr.Body.Bytes(), &res)
		assert.Equal(t, "evt_christmas", res.Name)
		assert.Equal(t, "Christmas Special", res.Title)
		assert.Equal(t, 50, res.Priority)
	})

	// Get
	t.Run("Get Event", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/liveops/evt_christmas", nil)
		req.SetPathValue("name", "evt_christmas")
		rr := httptest.NewRecorder()

		h.GetEvent(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var res liveops.LiveOpsEvent
		json.Unmarshal(rr.Body.Bytes(), &res)
		assert.Equal(t, "evt_christmas", res.Name)
		assert.Equal(t, "santa_box", res.ConfigurationPayload["gift"])
	})

	// List
	t.Run("List Events", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/liveops", nil)
		rr := httptest.NewRecorder()

		h.ListEvents(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var res []liveops.LiveOpsEvent
		json.Unmarshal(rr.Body.Bytes(), &res)
		require.Len(t, res, 1)
		assert.Equal(t, "evt_christmas", res[0].Name)
	})

	// Toggle (Quick Toggle)
	t.Run("Toggle Event", func(t *testing.T) {
		body := map[string]any{
			"enabled": false,
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("PATCH", "/api/admin/liveops/evt_christmas/toggle", bytes.NewReader(b))
		req.SetPathValue("name", "evt_christmas")
		rr := httptest.NewRecorder()

		h.ToggleEvent(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var res liveops.LiveOpsEvent
		json.Unmarshal(rr.Body.Bytes(), &res)
		assert.False(t, res.Enabled)
	})

	// Update
	t.Run("Update Event", func(t *testing.T) {
		body := map[string]any{
			"title":            "New Christmas Special",
			"enabled":          true,
			"priority":         60,
			"audience_segment": "paying_users",
			"schedule": map[string]any{
				"start_time": "2026-12-24T00:00:00Z",
				"end_time":   "2026-12-27T00:00:00Z",
			},
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("PUT", "/api/admin/liveops/evt_christmas", bytes.NewReader(b))
		req.SetPathValue("name", "evt_christmas")
		rr := httptest.NewRecorder()

		h.UpdateEvent(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var res liveops.LiveOpsEvent
		json.Unmarshal(rr.Body.Bytes(), &res)
		assert.Equal(t, "New Christmas Special", res.Title)
		assert.Equal(t, 60, res.Priority)
		assert.Equal(t, "paying_users", res.AudienceSegment)
		assert.True(t, res.Enabled)
	})

	// Delete
	t.Run("Delete Event", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/admin/liveops/evt_christmas", nil)
		req.SetPathValue("name", "evt_christmas")
		rr := httptest.NewRecorder()

		h.DeleteEvent(rr, req)
		assert.Equal(t, http.StatusNoContent, rr.Code)

		_, err := scheduler.GetEvent("evt_christmas")
		assert.Error(t, err)
	})
}

func TestLiveOpsAdminHandler_NilScheduler(t *testing.T) {
	h := NewLiveOpsHandler(nil, nil)

	t.Run("List with Nil", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/admin/liveops", nil)
		rr := httptest.NewRecorder()
		h.ListEvents(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "[]\n", rr.Body.String())
	})

	t.Run("Create with Nil", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/liveops", bytes.NewReader([]byte("{}")))
		rr := httptest.NewRecorder()
		h.CreateEvent(rr, req)
		assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	})
}
