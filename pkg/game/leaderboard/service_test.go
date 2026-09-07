package leaderboard

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func newTestService(t *testing.T, manifestsYaml string) (*LeaderboardService, *sql.DB) {
	// Wrap in a memory/sqlite wrapper that matches storage.Store
	store, err := storage.NewSQLiteStore(":memory:")
	require.NoError(t, err)

	// Parse registry manifests
	var registry []*manifest.Manifest
	if manifestsYaml != "" {
		var m manifest.Manifest
		err := yaml.Unmarshal([]byte(manifestsYaml), &m)
		require.NoError(t, err)
		registry = append(registry, &m)
	}

	reg := &manifest.Registry{
		Leaderboards: registry,
	}

	// Create service
	s := NewService(store, reg, nil)
	err = s.Init()
	require.NoError(t, err)

	// Retrieve SQL DB from sqlite store to return
	sqlDB := store.GetDB()

	return s, sqlDB
}

func TestLeaderboardService_SubmitScore_Validations(t *testing.T) {
	manifests := `
apiVersion: bffx.io/v1alpha1
kind: Leaderboard
metadata:
  name: weekly_high_scores
spec:
  title: "Weekly High Scores"
  sort_order: desc
  score_type: int
  aggregate_strategy: max
  reset_schedule: "0 0 * * 0"
  retention_seasons: 4
  rules:
    min_score: 0
    max_score: 1000
    anti_spam_window_sec: 2
`
	s, _ := newTestService(t, manifests)
	ctx := context.Background()

	// 1. Valid submission
	score, rank, err := s.SubmitScore(ctx, "weekly_high_scores", "user_alice", 500, "nonce_1", time.Now(), nil)
	require.NoError(t, err)
	assert.Equal(t, float64(500), score)
	assert.Equal(t, 1, rank)

	// 2. Replay attack blocked (duplicate nonce)
	_, _, err = s.SubmitScore(ctx, "weekly_high_scores", "user_bob", 600, "nonce_1", time.Now(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonce already processed")

	// 3. Spam protection block (too fast)
	_, _, err = s.SubmitScore(ctx, "weekly_high_scores", "user_alice", 550, "nonce_2", time.Now(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "spam blocked")

	// 4. Out of boundary score
	_, _, err = s.SubmitScore(ctx, "weekly_high_scores", "user_charlie", 1500, "nonce_3", time.Now(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum limit")
}

func TestLeaderboardService_SubmitScore_AggregationAndSorting(t *testing.T) {
	manifestsDesc := `
apiVersion: bffx.io/v1alpha1
kind: Leaderboard
metadata:
  name: test_desc
spec:
  title: "Desc Board"
  sort_order: desc
  score_type: int
  aggregate_strategy: max
  rules:
    min_score: 0
    max_score: 1000
`
	s, _ := newTestService(t, manifestsDesc)
	ctx := context.Background()

	// Submit alice score
	_, _, err := s.SubmitScore(ctx, "test_desc", "user_alice", 500, "n1", time.Now(), nil)
	require.NoError(t, err)

	// Alice submits lower score with aggregate strategy max -> score remains 500
	score, _, err := s.SubmitScore(ctx, "test_desc", "user_alice", 300, "n2", time.Now(), nil)
	require.NoError(t, err)
	assert.Equal(t, float64(500), score)

	// Alice submits higher score -> score becomes 700
	score, _, err = s.SubmitScore(ctx, "test_desc", "user_alice", 700, "n3", time.Now(), nil)
	require.NoError(t, err)
	assert.Equal(t, float64(700), score)
}

func TestLeaderboardService_GetRankings_and_UserRanking(t *testing.T) {
	manifests := `
apiVersion: bffx.io/v1alpha1
kind: Leaderboard
metadata:
  name: board_r
spec:
  title: "Board R"
  sort_order: desc
  score_type: int
  aggregate_strategy: max
  rules:
    min_score: 0
    max_score: 10000
`
	s, _ := newTestService(t, manifests)
	ctx := context.Background()

	// Seed multiple users
	_, _, _ = s.SubmitScore(ctx, "board_r", "user_bob", 200, "n1", time.Now(), nil)
	_, _, _ = s.SubmitScore(ctx, "board_r", "user_alice", 500, "n2", time.Now(), nil)
	_, _, _ = s.SubmitScore(ctx, "board_r", "user_charlie", 300, "n3", time.Now(), nil)

	// Retrieve rankings
	rankings, err := s.GetRankings(ctx, "board_r", 5, 0, "")
	require.NoError(t, err)
	require.Len(t, rankings, 3)

	// Assert order: Alice (500), Charlie (300), Bob (200)
	assert.Equal(t, "user_alice", rankings[0].UserID)
	assert.Equal(t, float64(500), rankings[0].Score)
	assert.Equal(t, 1, rankings[0].Rank)

	assert.Equal(t, "user_charlie", rankings[1].UserID)
	assert.Equal(t, float64(300), rankings[1].Score)
	assert.Equal(t, 2, rankings[1].Rank)

	assert.Equal(t, "user_bob", rankings[2].UserID)
	assert.Equal(t, float64(200), rankings[2].Score)
	assert.Equal(t, 3, rankings[2].Rank)

	// Get user Alice ranking
	ar, err := s.GetUserRanking(ctx, "board_r", "user_alice", "")
	require.NoError(t, err)
	assert.Equal(t, 1, ar.Rank)
	assert.Equal(t, float64(500), ar.Score)
}

func TestLeaderboardService_ResetSeason(t *testing.T) {
	manifests := `
apiVersion: bffx.io/v1alpha1
kind: Leaderboard
metadata:
  name: board_reset
spec:
  title: "Board Reset"
  sort_order: desc
  score_type: int
  aggregate_strategy: max
  rules:
    min_score: 0
    max_score: 1000
`
	s, db := newTestService(t, manifests)
	ctx := context.Background()

	_, _, _ = s.SubmitScore(ctx, "board_reset", "user_alice", 800, "n1", time.Now(), nil)
	_, _, _ = s.SubmitScore(ctx, "board_reset", "user_bob", 400, "n2", time.Now(), nil)

	// Reset season
	err := s.ResetSeason(ctx, "board_reset", "season_1")
	require.NoError(t, err)

	// Verify current rankings are empty
	rankings, err := s.GetRankings(ctx, "board_reset", 10, 0, "")
	require.NoError(t, err)
	assert.Len(t, rankings, 0)

	// Verify snapshots are stored in DB
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM bffx_leaderboard_snapshot WHERE leaderboard_name = ? AND season = ?`, "board_reset", "season_1").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}
