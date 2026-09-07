package leaderboard

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/redis/go-redis/v9"
)

type Ranking struct {
	UserID string  `json:"user_id"`
	Score  float64 `json:"score"`
	Rank   int     `json:"rank"`
}

type LeaderboardService struct {
	store  storage.Store
	reg    *manifest.Registry
	redis  *redis.Client
	db     *sql.DB
	driver string

	mu sync.RWMutex
}

const (
	scoreTableName    = "bffx_leaderboard_score"
	snapshotTableName = "bffx_leaderboard_snapshot"
	nonceTableName    = "bffx_leaderboard_nonce"
)

func NewService(store storage.Store, reg *manifest.Registry, rdb *redis.Client) *LeaderboardService {
	s := &LeaderboardService{
		store: store,
		reg:   reg,
		redis: rdb,
	}
	s.detectSQL(store)
	return s
}

func (s *LeaderboardService) detectSQL(store storage.Store) {
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

func (s *LeaderboardService) Init() error {
	if s.db != nil {
		// 1. Create bffx_leaderboard_score table
		var stmt string
		switch s.driver {
		case "postgres":
			stmt = `CREATE TABLE IF NOT EXISTS ` + scoreTableName + ` (
				id TEXT PRIMARY KEY,
				leaderboard_name TEXT NOT NULL,
				user_id TEXT NOT NULL,
				score DOUBLE PRECISION NOT NULL,
				updated_at TEXT NOT NULL,
				UNIQUE (leaderboard_name, user_id)
			)`
		default:
			stmt = `CREATE TABLE IF NOT EXISTS ` + scoreTableName + ` (
				id TEXT PRIMARY KEY,
				leaderboard_name TEXT NOT NULL,
				user_id TEXT NOT NULL,
				score REAL NOT NULL,
				updated_at TEXT NOT NULL,
				UNIQUE (leaderboard_name, user_id)
			)`
		}
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("create %s: %w", scoreTableName, err)
		}

		// 2. Create bffx_leaderboard_snapshot table
		switch s.driver {
		case "postgres":
			stmt = `CREATE TABLE IF NOT EXISTS ` + snapshotTableName + ` (
				id TEXT PRIMARY KEY,
				leaderboard_name TEXT NOT NULL,
				season TEXT NOT NULL,
				user_id TEXT NOT NULL,
				score DOUBLE PRECISION NOT NULL,
				rank INTEGER NOT NULL,
				archived_at TEXT NOT NULL
			)`
		default:
			stmt = `CREATE TABLE IF NOT EXISTS ` + snapshotTableName + ` (
				id TEXT PRIMARY KEY,
				leaderboard_name TEXT NOT NULL,
				season TEXT NOT NULL,
				user_id TEXT NOT NULL,
				score REAL NOT NULL,
				rank INTEGER NOT NULL,
				archived_at TEXT NOT NULL
			)`
		}
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("create %s: %w", snapshotTableName, err)
		}

		// 3. Create bffx_leaderboard_nonce table
		stmt = `CREATE TABLE IF NOT EXISTS ` + nonceTableName + ` (
			nonce TEXT PRIMARY KEY,
			created_at TEXT NOT NULL
		)`
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("create %s: %w", nonceTableName, err)
		}
	}
	return nil
}

func leaderboardScoresKey(leaderboardName string) string {
	return fmt.Sprintf("bffx:leaderboard:%s:scores", leaderboardName)
}

func leaderboardSegmentScoresKey(leaderboardName, segment string) string {
	return fmt.Sprintf("bffx:leaderboard:%s:segment:%s:scores", leaderboardName, segment)
}

func normalizeRankingsLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func scoreOrderDirection(sortOrder string) string {
	if sortOrder == "asc" {
		return "ASC"
	}
	return "DESC"
}

func rankComparisonOperator(sortOrder string) string {
	if sortOrder == "asc" {
		return "<"
	}
	return ">"
}

func primarySegment(segments []string) string {
	if len(segments) == 0 {
		return ""
	}
	return segments[0]
}

func applyAggregateStrategy(strategy, sortOrder string, existingScore, incomingScore float64) float64 {
	switch strategy {
	case "max":
		if sortOrder == "desc" {
			if incomingScore > existingScore {
				return incomingScore
			}
			return existingScore
		}
		if incomingScore < existingScore {
			return incomingScore
		}
		return existingScore
	case "sum":
		return existingScore + incomingScore
	case "latest":
		return incomingScore
	default:
		return incomingScore
	}
}

func (s *LeaderboardService) archiveAndClearSeasonScores(ctx context.Context, leaderboardName, seasonName, sortOrder string) error {
	orderBy := scoreOrderDirection(sortOrder)
	query := fmt.Sprintf(`
			SELECT user_id, score 
			FROM %s 
			WHERE leaderboard_name = ? 
			ORDER BY score %s`, scoreTableName, orderBy)
	stmt := s.placeholders(query)
	rows, err := s.db.QueryContext(ctx, stmt, leaderboardName)
	if err != nil {
		return fmt.Errorf("read scores before reset: %w", err)
	}
	defer rows.Close()

	type scoreEntry struct {
		userID string
		score  float64
	}
	var entries []scoreEntry
	for rows.Next() {
		var se scoreEntry
		if err := rows.Scan(&se.userID, &se.score); err == nil {
			entries = append(entries, se)
		}
	}

	nowStr := time.Now().Format(time.RFC3339)
	for idx, entry := range entries {
		rank := idx + 1
		insStmt := s.placeholders(`INSERT INTO ` + snapshotTableName + ` (id, leaderboard_name, season, user_id, score, rank, archived_at) VALUES (?, ?, ?, ?, ?, ?, ?)`)
		_, _ = s.db.ExecContext(ctx, insStmt, uuid.NewString(), leaderboardName, seasonName, entry.userID, entry.score, rank, nowStr)
	}

	delStmt := s.placeholders(`DELETE FROM ` + scoreTableName + ` WHERE leaderboard_name = ?`)
	_, err = s.db.ExecContext(ctx, delStmt, leaderboardName)
	if err != nil {
		return fmt.Errorf("clear scores table: %w", err)
	}
	return nil
}

func (s *LeaderboardService) clearRedisLeaderboardKeys(ctx context.Context, leaderboardName string) {
	redisKey := leaderboardScoresKey(leaderboardName)
	_ = s.redis.Del(ctx, redisKey).Err()

	// Retrieve keys matching pattern bffx:leaderboard:{name}:segment:*:scores and clear them.
	pattern := fmt.Sprintf("bffx:leaderboard:%s:segment:*:scores", leaderboardName)
	keys, err := s.redis.Keys(ctx, pattern).Result()
	if err == nil && len(keys) > 0 {
		_ = s.redis.Del(ctx, keys...).Err()
	}
}

func (s *LeaderboardService) SubmitScore(ctx context.Context, leaderboardName, userID string, score float64, nonce string, submittedAt time.Time, userSegments []string) (float64, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Load leaderboard spec
	spec, err := s.getSpec(leaderboardName)
	if err != nil {
		return 0, 0, err
	}

	// 2. Idempotency Check: Verify Nonce
	if nonce != "" {
		isUsed, err := s.checkAndMarkNonce(ctx, nonce)
		if err != nil {
			return 0, 0, err
		}
		if isUsed {
			return 0, 0, errors.New("nonce already processed")
		}
	}

	// 3. Timestamp Sanity Check (Max 5 minutes in future)
	if submittedAt.After(time.Now().Add(5 * time.Minute)) {
		return 0, 0, errors.New("score submission timestamp in the future")
	}

	// 4. Boundary rules validation
	if spec.Rules.MaxScore > 0 && score > spec.Rules.MaxScore {
		return 0, 0, fmt.Errorf("score %.2f exceeds maximum limit %.2f", score, spec.Rules.MaxScore)
	}
	if score < spec.Rules.MinScore {
		return 0, 0, fmt.Errorf("score %.2f is below minimum limit %.2f", score, spec.Rules.MinScore)
	}

	// 5. Monotonic / Anti-Spam Check
	existingScore, lastUpdated, err := s.getUserScoreAndLastUpdate(ctx, leaderboardName, userID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, 0, err
	}

	if lastUpdated != nil && spec.Rules.AntiSpamWindowSec > 0 {
		elapsed := time.Since(*lastUpdated)
		if elapsed < time.Duration(spec.Rules.AntiSpamWindowSec)*time.Second {
			return 0, 0, fmt.Errorf("score submission spam blocked: must wait %d seconds", spec.Rules.AntiSpamWindowSec)
		}
	}

	// 6. Aggregate strategy application
	newScore := score
	if lastUpdated != nil {
		newScore = applyAggregateStrategy(spec.AggregateStrategy, spec.SortOrder, existingScore, score)
	}

	// 7. Save to SQL Database (Source of Truth)
	if s.db != nil {
		var err error
		nowStr := time.Now().Format(time.RFC3339)
		if lastUpdated == nil {
			stmt := s.placeholders(`INSERT INTO ` + scoreTableName + ` (id, leaderboard_name, user_id, score, updated_at) VALUES (?, ?, ?, ?, ?)`)
			_, err = s.db.ExecContext(ctx, stmt, uuid.NewString(), leaderboardName, userID, newScore, nowStr)
		} else {
			stmt := s.placeholders(`UPDATE ` + scoreTableName + ` SET score = ?, updated_at = ? WHERE leaderboard_name = ? AND user_id = ?`)
			_, err = s.db.ExecContext(ctx, stmt, newScore, nowStr, leaderboardName, userID)
		}
		if err != nil {
			return 0, 0, fmt.Errorf("failed to save score in database: %w", err)
		}
	}

	// 8. Update Redis ZSET Cache (if available)
	if s.redis != nil {
		redisKey := leaderboardScoresKey(leaderboardName)
		err := s.redis.ZAdd(ctx, redisKey, redis.Z{Score: newScore, Member: userID}).Err()
		if err != nil {
			// Redis write error is non-fatal; we proceed and rely on SQL fallback
		}

		// Update segment ZSETs if specified
		for _, seg := range userSegments {
			segKey := leaderboardSegmentScoresKey(leaderboardName, seg)
			_ = s.redis.ZAdd(ctx, segKey, redis.Z{Score: newScore, Member: userID}).Err()
		}
	}

	// 9. Get user's rank
	rank, err := s.getUserRank(ctx, leaderboardName, userID, newScore, spec.SortOrder, userSegments)
	if err != nil {
		return newScore, 0, nil
	}

	return newScore, rank, nil
}

func (s *LeaderboardService) GetRankings(ctx context.Context, leaderboardName string, limit, offset int, segment string) ([]Ranking, error) {
	spec, err := s.getSpec(leaderboardName)
	if err != nil {
		return nil, err
	}

	limit = normalizeRankingsLimit(limit)

	// Primary Path: Redis
	if s.redis != nil {
		var redisKey string
		if segment != "" {
			redisKey = leaderboardSegmentScoresKey(leaderboardName, segment)
		} else {
			redisKey = leaderboardScoresKey(leaderboardName)
		}

		var zRange []redis.Z
		var err error
		start := int64(offset)
		stop := int64(offset + limit - 1)

		if spec.SortOrder == "desc" {
			zRange, err = s.redis.ZRevRangeWithScores(ctx, redisKey, start, stop).Result()
		} else {
			zRange, err = s.redis.ZRangeWithScores(ctx, redisKey, start, stop).Result()
		}

		if err == nil && len(zRange) > 0 {
			rankings := make([]Ranking, 0, len(zRange))
			for i, r := range zRange {
				rankings = append(rankings, Ranking{
					UserID: r.Member.(string),
					Score:  r.Score,
					Rank:   offset + i + 1,
				})
			}
			return rankings, nil
		}
	}

	// Fallback Path: SQL
	if s.db == nil {
		return []Ranking{}, nil
	}

	var query string
	var rows *sql.Rows
	if segment != "" {
		// Note: SQL segment filtering requires joining with User table to match segment or we filter in query
		// Let's check user roles/segments or resolve user IDs by segment first.
		// For robustness, let's look up user segments. Since the user table stores segments, we join user on user_id.
		orderBy := scoreOrderDirection(spec.SortOrder)
		query = fmt.Sprintf(`
			SELECT s.user_id, s.score 
			FROM %s s
			JOIN bffx_user u ON s.user_id = u.id
			WHERE s.leaderboard_name = ? AND u.segments LIKE ?
			ORDER BY s.score %s LIMIT ? OFFSET ?`, scoreTableName, orderBy)
		stmt := s.placeholders(query)
		rows, err = s.db.QueryContext(ctx, stmt, leaderboardName, "%"+segment+"%", limit, offset)
	} else {
		orderBy := scoreOrderDirection(spec.SortOrder)
		query = fmt.Sprintf(`
			SELECT user_id, score 
			FROM %s 
			WHERE leaderboard_name = ? 
			ORDER BY score %s LIMIT ? OFFSET ?`, scoreTableName, orderBy)
		stmt := s.placeholders(query)
		rows, err = s.db.QueryContext(ctx, stmt, leaderboardName, limit, offset)
	}

	if err != nil {
		return nil, fmt.Errorf("query rankings from database: %w", err)
	}
	defer rows.Close()

	rankings := []Ranking{}
	idx := 1
	for rows.Next() {
		var r Ranking
		if err := rows.Scan(&r.UserID, &r.Score); err != nil {
			return nil, err
		}
		r.Rank = offset + idx
		rankings = append(rankings, r)
		idx++
	}
	return rankings, nil
}

func (s *LeaderboardService) GetUserRanking(ctx context.Context, leaderboardName string, userID string, segment string) (*Ranking, error) {
	spec, err := s.getSpec(leaderboardName)
	if err != nil {
		return nil, err
	}

	// Fetch score from DB
	score, _, err := s.getUserScoreAndLastUpdate(ctx, leaderboardName, userID)
	if err != nil {
		return nil, err
	}

	var segments []string
	if segment != "" {
		segments = []string{segment}
	}
	rank, err := s.getUserRank(ctx, leaderboardName, userID, score, spec.SortOrder, segments)
	if err != nil {
		return nil, err
	}

	return &Ranking{
		UserID: userID,
		Score:  score,
		Rank:   rank,
	}, nil
}

func (s *LeaderboardService) ResetSeason(ctx context.Context, leaderboardName string, seasonName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	spec, err := s.getSpec(leaderboardName)
	if err != nil {
		return err
	}

	if s.db != nil {
		if err := s.archiveAndClearSeasonScores(ctx, leaderboardName, seasonName, spec.SortOrder); err != nil {
			return err
		}
	}

	// 4. Clear Redis keys
	if s.redis != nil {
		s.clearRedisLeaderboardKeys(ctx, leaderboardName)
	}

	return nil
}

func (s *LeaderboardService) getSpec(name string) (*manifest.LeaderboardSpec, error) {
	if s.reg == nil {
		return nil, errors.New("manifest registry not loaded")
	}
	for _, m := range s.reg.Leaderboards {
		if m.Metadata.Name == name {
			var spec manifest.LeaderboardSpec
			if err := m.UnmarshalSpec(&spec); err == nil {
				return &spec, nil
			}
		}
	}
	return nil, fmt.Errorf("leaderboard %q not found in manifests", name)
}

func (s *LeaderboardService) checkAndMarkNonce(ctx context.Context, nonce string) (bool, error) {
	if s.redis != nil {
		key := fmt.Sprintf("bffx:leaderboard:nonce:%s", nonce)
		isNew, err := s.redis.SetNX(ctx, key, "1", 24*time.Hour).Result()
		if err == nil {
			return !isNew, nil
		}
	}

	if s.db != nil {
		var exists bool
		stmt := s.placeholders(`SELECT EXISTS(SELECT 1 FROM ` + nonceTableName + ` WHERE nonce = ?)`)
		err := s.db.QueryRowContext(ctx, stmt, nonce).Scan(&exists)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return false, err
		}
		if exists {
			return true, nil
		}

		insStmt := s.placeholders(`INSERT INTO ` + nonceTableName + ` (nonce, created_at) VALUES (?, ?)`)
		_, err = s.db.ExecContext(ctx, insStmt, nonce, time.Now().Format(time.RFC3339))
		if err != nil {
			return false, err
		}
	}
	return false, nil
}

func (s *LeaderboardService) getUserScoreAndLastUpdate(ctx context.Context, leaderboardName, userID string) (float64, *time.Time, error) {
	if s.db == nil {
		return 0, nil, sql.ErrNoRows
	}
	stmt := s.placeholders(`SELECT score, updated_at FROM ` + scoreTableName + ` WHERE leaderboard_name = ? AND user_id = ?`)
	var score float64
	var updatedAtStr string
	err := s.db.QueryRowContext(ctx, stmt, leaderboardName, userID).Scan(&score, &updatedAtStr)
	if err != nil {
		return 0, nil, err
	}
	t, err := time.Parse(time.RFC3339, updatedAtStr)
	if err != nil {
		return score, nil, nil
	}
	return score, &t, nil
}

func (s *LeaderboardService) getUserRank(ctx context.Context, leaderboardName string, userID string, score float64, sortOrder string, segments []string) (int, error) {
	segment := primarySegment(segments)

	// Redis Path
	if s.redis != nil {
		var redisKey string
		if segment != "" {
			redisKey = leaderboardSegmentScoresKey(leaderboardName, segment)
		} else {
			redisKey = leaderboardScoresKey(leaderboardName)
		}

		var rank int64
		var err error
		if sortOrder == "desc" {
			rank, err = s.redis.ZRevRank(ctx, redisKey, userID).Result()
		} else {
			rank, err = s.redis.ZRank(ctx, redisKey, userID).Result()
		}
		if err == nil {
			return int(rank) + 1, nil
		}
	}

	// SQL Path
	if s.db == nil {
		return 0, errors.New("no database connection for fallback rank computation")
	}

	var query string
	var err error
	var count int

	comp := rankComparisonOperator(sortOrder)
	if segment != "" {
		query = fmt.Sprintf(`
			SELECT COUNT(*) 
			FROM %s s
			JOIN bffx_user u ON s.user_id = u.id
			WHERE s.leaderboard_name = ? AND u.segments LIKE ? AND s.score %s ?`, scoreTableName, comp)
		stmt := s.placeholders(query)
		err = s.db.QueryRowContext(ctx, stmt, leaderboardName, "%"+segment+"%", score).Scan(&count)
	} else {
		query = fmt.Sprintf(`
			SELECT COUNT(*) 
			FROM %s 
			WHERE leaderboard_name = ? AND score %s ?`, scoreTableName, comp)
		stmt := s.placeholders(query)
		err = s.db.QueryRowContext(ctx, stmt, leaderboardName, score).Scan(&count)
	}

	if err != nil {
		return 0, err
	}
	return count + 1, nil
}

func (s *LeaderboardService) placeholders(stmt string) string {
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
