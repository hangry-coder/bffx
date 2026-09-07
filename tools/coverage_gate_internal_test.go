package main

import (
	"strings"
	"testing"
)

func TestParseCoverageStats_basic(t *testing.T) {
	const profile = `mode: count
github.com/hangry-coder/bffx/pkg/auth/jwt.go:10.1,12.1 3 2
github.com/hangry-coder/bffx/pkg/auth/jwt.go:20.1,22.1 1 0
github.com/hangry-coder/bffx/pkg/api/handlers/health.go:1.1,5.1 10 10
`
	stats := parseCoverageStats(strings.NewReader(profile))
	if got := stats["github.com/hangry-coder/bffx/pkg/auth"]; got[0] != 4 || got[1] != 3 {
		t.Fatalf("auth package: want statements 4 covered-weight 3, got %+v", got)
	}
	if got := stats["github.com/hangry-coder/bffx/pkg/api/handlers"]; got[0] != 10 || got[1] != 10 {
		t.Fatalf("handlers: want 10/10, got %+v", got)
	}
}

func TestEvaluateFloors_passAndFail(t *testing.T) {
	stats := map[string][2]int{
		"github.com/hangry-coder/bffx/pkg/auth": {100, 60},
	}
	lines, failed := evaluateFloors(stats, []PackageConfig{
		{Name: "github.com/hangry-coder/bffx/pkg/auth", Floor: 55.0},
	})
	if failed || len(lines) != 1 || !lines[0].ok {
		t.Fatalf("expected pass, got failed=%v lines=%v", failed, lines)
	}

	stats["github.com/hangry-coder/bffx/pkg/auth"] = [2]int{100, 40} // 40%
	lines, failed = evaluateFloors(stats, []PackageConfig{
		{Name: "github.com/hangry-coder/bffx/pkg/auth", Floor: 55.0},
	})
	if !failed || len(lines) != 1 || lines[0].ok {
		t.Fatalf("expected fail, got failed=%v lines=%v", failed, lines)
	}

	_, failed = evaluateFloors(stats, []PackageConfig{
		{Name: "github.com/hangry-coder/bffx/pkg/missing", Floor: 1.0},
	})
	if !failed {
		t.Fatal("expected missing package to fail gate")
	}
}
