package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
	"github.com/google/uuid"
)

type SQLiteJobStore struct {
	db *sql.DB
}

func NewSQLiteJobStore(dbPath string) (*SQLiteJobStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// Create table if not exists
	schema := `
	CREATE TABLE IF NOT EXISTS bffx_jobs (
		id TEXT PRIMARY KEY,
		user_id TEXT,
		kind TEXT,
		name TEXT,
		input TEXT,
		status TEXT,
		result TEXT,
		error TEXT,
		created_at DATETIME,
		updated_at DATETIME
	);`
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}

	return &SQLiteJobStore{db: db}, nil
}

func (s *SQLiteJobStore) Enqueue(ctx context.Context, userID, kind, name string, input map[string]any) (Job, error) {
	inputJSON, _ := json.Marshal(input)
	now := time.Now().UTC()
	job := Job{
		ID:        uuid.New().String(),
		UserID:    userID,
		Kind:      kind,
		Name:      name,
		Input:     input,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	query := `INSERT INTO bffx_jobs (id, user_id, kind, name, input, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, query, job.ID, job.UserID, job.Kind, job.Name, string(inputJSON), job.Status, job.CreatedAt, job.UpdatedAt)
	if err != nil {
		return Job{}, err
	}

	return job, nil
}

func (s *SQLiteJobStore) Get(ctx context.Context, id string) (Job, error) {
	query := `SELECT id, user_id, kind, name, input, status, result, error, created_at, updated_at FROM bffx_jobs WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)

	var j Job
	var inputStr, resultStr, errorStr sql.NullString
	err := row.Scan(&j.ID, &j.UserID, &j.Kind, &j.Name, &inputStr, &j.Status, &resultStr, &errorStr, &j.CreatedAt, &j.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return Job{}, fmt.Errorf("job not found: %s", id)
		}
		return Job{}, err
	}

	if inputStr.Valid {
		json.Unmarshal([]byte(inputStr.String), &j.Input)
	}
	if resultStr.Valid {
		json.Unmarshal([]byte(resultStr.String), &j.Result)
	}
	if errorStr.Valid {
		j.Error = errorStr.String
	}

	return j, nil
}

func (s *SQLiteJobStore) UpdateResult(ctx context.Context, id, status string, result map[string]any, errMsg string) error {
	resultJSON, _ := json.Marshal(result)
	now := time.Now().UTC()
	query := `UPDATE bffx_jobs SET status = ?, result = ?, error = ?, updated_at = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, status, string(resultJSON), errMsg, now, id)
	return err
}

func (s *SQLiteJobStore) List(ctx context.Context, userID string, limit, offset int) ([]Job, error) {
	query := `SELECT id, user_id, kind, name, input, status, result, error, created_at, updated_at FROM bffx_jobs WHERE user_id = ? OR ? = '' ORDER BY created_at DESC LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, query, userID, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var j Job
		var inputStr, resultStr, errorStr sql.NullString
		if err := rows.Scan(&j.ID, &j.UserID, &j.Kind, &j.Name, &inputStr, &j.Status, &resultStr, &errorStr, &j.CreatedAt, &j.UpdatedAt); err != nil {
			continue
		}
		if inputStr.Valid {
			json.Unmarshal([]byte(inputStr.String), &j.Input)
		}
		if resultStr.Valid {
			json.Unmarshal([]byte(resultStr.String), &j.Result)
		}
		if errorStr.Valid {
			j.Error = errorStr.String
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (s *SQLiteJobStore) Prune(ctx context.Context, days int) (int64, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	query := `DELETE FROM bffx_jobs WHERE updated_at < ?`
	res, err := s.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *SQLiteJobStore) RetryJob(ctx context.Context, id string) error {
	now := time.Now().UTC()
	query := `UPDATE bffx_jobs SET status = ?, error = ?, updated_at = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, StatusPending, "", now, id)
	return err
}

func (s *SQLiteJobStore) CancelJob(ctx context.Context, id string) error {
	now := time.Now().UTC()
	query := `UPDATE bffx_jobs SET status = ?, error = ?, updated_at = ? WHERE id = ? AND status IN (?, ?)`
	res, err := s.db.ExecContext(ctx, query, StatusFailed, "cancelled", now, id, StatusPending, StatusRunning)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("job not found or already completed/failed")
	}
	return nil
}
