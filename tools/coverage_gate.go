package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
)

// PackageConfig defines the coverage floor for a specific package.
type PackageConfig struct {
	Name  string
	Floor float64
}

var defaultPackages = []PackageConfig{
	{Name: "github.com/hangry-coder/bffx/pkg/api/handlers", Floor: 50.0},
	{Name: "github.com/hangry-coder/bffx/pkg/api/middleware", Floor: 50.0},
	{Name: "github.com/hangry-coder/bffx/pkg/auth", Floor: 55.0},
	{Name: "github.com/hangry-coder/bffx/pkg/storage", Floor: 43.0},
	{Name: "github.com/hangry-coder/bffx/pkg/api/router", Floor: 50.0},
}

// parseCoverageStats aggregates statement coverage from a go coverprofile (mode: atomic or count).
func parseCoverageStats(r io.Reader) map[string][2]int {
	stats := make(map[string][2]int)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "mode:") {
			continue
		}
		parts := strings.Split(line, " ")
		if len(parts) < 3 {
			continue
		}
		pathParts := strings.Split(parts[0], ":")
		pkgPath := pathParts[0]
		lastSlash := strings.LastIndex(pkgPath, "/")
		if lastSlash == -1 {
			continue
		}
		pkgName := pkgPath[:lastSlash]
		statements, _ := strconv.Atoi(parts[1])
		covered, _ := strconv.Atoi(parts[2])
		s := stats[pkgName]
		s[0] += statements
		if covered > 0 {
			s[1] += statements
		}
		stats[pkgName] = s
	}
	return stats
}

type gateLine struct {
	ok      bool
	message string
}

func evaluateFloors(stats map[string][2]int, configs []PackageConfig) ([]gateLine, bool) {
	var lines []gateLine
	failed := false
	for _, config := range configs {
		s, ok := stats[config.Name]
		if !ok {
			lines = append(lines, gateLine{ok: false, message: fmt.Sprintf("[MISS] %s (no data in profile)", config.Name)})
			failed = true
			continue
		}
		coverage := 0.0
		if s[0] > 0 {
			coverage = (float64(s[1]) / float64(s[0])) * 100
		}
		if coverage < config.Floor {
			lines = append(lines, gateLine{ok: false, message: fmt.Sprintf("[FAIL] %-30s %.1f%% (floor %.1f%%)", config.Name, coverage, config.Floor)})
			failed = true
		} else {
			lines = append(lines, gateLine{ok: true, message: fmt.Sprintf("[PASS] %-30s %.1f%% (floor %.1f%%)", config.Name, coverage, config.Floor)})
		}
	}
	return lines, failed
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: coverage_gate <coverage.out>")
	}
	file, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatalf("failed to open coverage file: %v", err)
	}
	defer file.Close()

	stats := parseCoverageStats(file)
	lines, failed := evaluateFloors(stats, defaultPackages)

	fmt.Println("BFFX coverage gate")
	fmt.Println("------------------")
	for _, l := range lines {
		fmt.Println(l.message)
	}
	if failed {
		fmt.Println("\nCoverage gate failed.")
		os.Exit(1)
	}
	fmt.Println("\nAll configured packages met their floors.")
}
