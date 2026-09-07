package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	apierrors "github.com/hangry-coder/bffx/pkg/api/errors"
	"github.com/hangry-coder/bffx/pkg/api/middleware"
	"github.com/hangry-coder/bffx/pkg/game/leaderboard"
)

type LeaderboardHandler struct {
	service *leaderboard.LeaderboardService
}

func NewLeaderboardHandler(s *leaderboard.LeaderboardService) *LeaderboardHandler {
	return &LeaderboardHandler{service: s}
}

func (h *LeaderboardHandler) Submit(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		apierrors.Write(w, apierrors.ErrUnauthorized)
		return
	}

	leaderboardName := r.PathValue("name")
	if leaderboardName == "" {
		apierrors.WriteError(w, http.StatusBadRequest, "leaderboard name is required")
		return
	}

	var payload struct {
		Score     float64 `json:"score"`
		Nonce     string  `json:"nonce"`
		Timestamp string  `json:"timestamp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		apierrors.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	submittedAt := time.Now()
	if payload.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339, payload.Timestamp); err == nil {
			submittedAt = t
		} else {
			apierrors.WriteError(w, http.StatusBadRequest, "invalid timestamp format (must be RFC3339)")
			return
		}
	}

	// Extract user segments
	claims := middleware.GetClaims(r.Context())
	var segments []string
	if claims != nil {
		if segsVal, ok := claims["segments"]; ok {
			if list, ok := segsVal.([]any); ok {
				for _, item := range list {
					if s, ok := item.(string); ok {
						segments = append(segments, s)
					}
				}
			} else if list, ok := segsVal.([]string); ok {
				segments = list
			}
		}
	}

	newScore, rank, err := h.service.SubmitScore(r.Context(), leaderboardName, userID, payload.Score, payload.Nonce, submittedAt, segments)
	if err != nil {
		apierrors.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	apierrors.WriteJSON(w, http.StatusOK, map[string]any{
		"status":    "success",
		"score":     newScore,
		"rank":      rank,
		"user_id":   userID,
		"submitted": true,
	})
}

func (h *LeaderboardHandler) Rankings(w http.ResponseWriter, r *http.Request) {
	leaderboardName := r.PathValue("name")
	if leaderboardName == "" {
		apierrors.WriteError(w, http.StatusBadRequest, "leaderboard name is required")
		return
	}

	limit := 50
	offset := 0
	segment := r.URL.Query().Get("segment")

	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil {
			limit = l
		}
	}
	if oStr := r.URL.Query().Get("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil {
			offset = o
		}
	}

	rankings, err := h.service.GetRankings(r.Context(), leaderboardName, limit, offset, segment)
	if err != nil {
		apierrors.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var userRank *leaderboard.Ranking
	userID := middleware.GetUserID(r.Context())
	if userID != "" {
		ur, err := h.service.GetUserRanking(r.Context(), leaderboardName, userID, segment)
		if err == nil {
			userRank = ur
		}
	}

	apierrors.WriteJSON(w, http.StatusOK, map[string]any{
		"rankings":     rankings,
		"user_ranking": userRank,
	})
}
