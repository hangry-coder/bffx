package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/game/leaderboard"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestLeaderboardHandler_REST_Endpoints(t *testing.T) {
	// Initialize sqlite storage
	store, err := storage.NewSQLiteStore(":memory:")
	require.NoError(t, err)

	// Create manifest
	var m manifest.Manifest
	err = yaml.Unmarshal([]byte(`
apiVersion: bffx.io/v1alpha1
kind: Leaderboard
metadata:
  name: weekly_high_scores
spec:
  title: "Weekly High Scores"
  sort_order: desc
  score_type: int
  aggregate_strategy: max
  rules:
    min_score: 0
    max_score: 1000
    anti_spam_window_sec: 0
`), &m)
	require.NoError(t, err)

	reg := &manifest.Registry{
		Leaderboards: []*manifest.Manifest{&m},
	}

	// Create service & handler
	svc := leaderboard.NewService(store, reg, nil)
	err = svc.Init()
	require.NoError(t, err)

	h := NewLeaderboardHandler(svc)

	// 1. Submit Score (Success)
	t.Run("Submit Score - Success", func(t *testing.T) {
		body := map[string]any{
			"score": 500,
			"nonce": "n1",
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/v1/leaderboards/weekly_high_scores/submit", bytes.NewReader(b))
		req.SetPathValue("name", "weekly_high_scores")
		req = req.WithContext(middleware.WithClaims(req.Context(), map[string]any{"sub": "user_alice"}))
		rr := httptest.NewRecorder()

		h.Submit(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var res map[string]any
		json.Unmarshal(rr.Body.Bytes(), &res)
		assert.Equal(t, "success", res["status"])
		assert.Equal(t, float64(500), res["score"])
		assert.Equal(t, float64(1), res["rank"])
	})

	// 2. Submit Score - Unauthorized
	t.Run("Submit Score - Unauthorized", func(t *testing.T) {
		body := map[string]any{
			"score": 600,
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/v1/leaderboards/weekly_high_scores/submit", bytes.NewReader(b))
		req.SetPathValue("name", "weekly_high_scores")
		rr := httptest.NewRecorder()

		h.Submit(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	// 3. Get Rankings
	t.Run("Get Rankings - Success", func(t *testing.T) {
		// Submit score for user_bob
		_, _, _ = svc.SubmitScore(context.Background(), "weekly_high_scores", "user_bob", 800, "n2", time.Now(), nil)

		req := httptest.NewRequest("GET", "/api/v1/leaderboards/weekly_high_scores/rankings", nil)
		req.SetPathValue("name", "weekly_high_scores")
		req = req.WithContext(middleware.WithClaims(req.Context(), map[string]any{"sub": "user_alice"}))
		rr := httptest.NewRecorder()

		h.Rankings(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var res map[string]any
		json.Unmarshal(rr.Body.Bytes(), &res)

		rankings := res["rankings"].([]any)
		require.Len(t, rankings, 2)

		// Top is Bob (800)
		top := rankings[0].(map[string]any)
		assert.Equal(t, "user_bob", top["user_id"])
		assert.Equal(t, float64(800), top["score"])

		// User ranking envelope for alice
		ur := res["user_ranking"].(map[string]any)
		assert.Equal(t, "user_alice", ur["user_id"])
		assert.Equal(t, float64(500), ur["score"])
		assert.Equal(t, float64(2), ur["rank"])
	})
}
