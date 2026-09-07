package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// StepResult records one assertion in the full-stack integration run.
type StepResult struct {
	Name     string `json:"name"`
	Status   string `json:"status"` // pass, fail, skip
	Detail   string `json:"detail,omitempty"`
	Duration string `json:"duration,omitempty"`
}

// FullStackReport is persisted under tests/e2e/logs for post-run review.
type FullStackReport struct {
	TestName   string       `json:"test_name"`
	StartedAt  string       `json:"started_at"`
	FinishedAt string       `json:"finished_at"`
	AppName    string       `json:"app_name"`
	StoreMode  string       `json:"store_mode"`
	HTTPPort   int          `json:"http_port"`
	GRPCPort   int          `json:"grpc_port"`
	LogPath    string       `json:"server_log_path"`
	ReportPath string       `json:"report_path"`
	Overall    string       `json:"overall"` // pass, fail
	Steps      []StepResult `json:"steps"`
}

// TestReporter collects step outcomes and writes a JSON report on Finish.
type TestReporter struct {
	testName  string
	appName   string
	storeMode string
	httpPort  int
	grpcPort  int
	logPath   string
	started   time.Time
	steps     []StepResult
}

func NewTestReporter(testName, appName, storeMode string, httpPort, grpcPort int, logPath string) *TestReporter {
	return &TestReporter{
		testName:  testName,
		appName:   appName,
		storeMode: storeMode,
		httpPort:  httpPort,
		grpcPort:  grpcPort,
		logPath:   logPath,
		started:   time.Now(),
	}
}

func (r *TestReporter) Record(name, status, detail string, dur time.Duration) {
	step := StepResult{
		Name:   name,
		Status: status,
		Detail: detail,
	}
	if dur > 0 {
		step.Duration = dur.Round(time.Millisecond).String()
	}
	r.steps = append(r.steps, step)
}

func (r *TestReporter) Pass(name, detail string, dur time.Duration) {
	r.Record(name, "pass", detail, dur)
}

func (r *TestReporter) Fail(name, detail string, dur time.Duration) {
	r.Record(name, "fail", detail, dur)
}

func (r *TestReporter) Skip(name, detail string) {
	r.Record(name, "skip", detail, 0)
}

func (r *TestReporter) Finish(logsDir string) (string, error) {
	overall := "pass"
	for _, s := range r.steps {
		if s.Status == "fail" {
			overall = "fail"
			break
		}
	}

	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return "", err
	}
	reportPath := filepath.Join(logsDir, fmt.Sprintf("full_stack_%s.json", r.started.Format("20060102_150405")))
	report := FullStackReport{
		TestName:   r.testName,
		StartedAt:  r.started.Format(time.RFC3339),
		FinishedAt: time.Now().Format(time.RFC3339),
		AppName:    r.appName,
		StoreMode:  r.storeMode,
		HTTPPort:   r.httpPort,
		GRPCPort:   r.grpcPort,
		LogPath:    r.logPath,
		ReportPath: reportPath,
		Overall:    overall,
		Steps:      r.steps,
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(reportPath, data, 0o644); err != nil {
		return "", err
	}
	return reportPath, nil
}

func (r *TestReporter) HasFailures() bool {
	for _, s := range r.steps {
		if s.Status == "fail" {
			return true
		}
	}
	return false
}
