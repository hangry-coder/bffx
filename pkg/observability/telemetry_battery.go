package observability

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/hangry-coder/bffx/pkg/storage"
	"github.com/prometheus/client_golang/prometheus"
)

type LocalBatteryTelemetryProvider struct {
	store       storage.Store
	stopCh      chan struct{}
	lastCPUTime int64
	lastSample  time.Time
}

func NewLocalBatteryTelemetryProvider(store storage.Store) *LocalBatteryTelemetryProvider {
	p := &LocalBatteryTelemetryProvider{
		store:  store,
		stopCh: make(chan struct{}),
	}
	go p.startCollection()
	return p
}

func (p *LocalBatteryTelemetryProvider) Close() {
	close(p.stopCh)
}

func (p *LocalBatteryTelemetryProvider) Info(ctx context.Context) ProviderInfo {
	return ProviderInfo{
		Name:        "battery",
		DisplayName: "Built-in (Local Metrics)",
		Capabilities: []Capability{
			CapWrite,
			CapReadSummary,
			CapReadStream,
		},
		Healthy: true,
	}
}

func (p *LocalBatteryTelemetryProvider) GetSummary(ctx context.Context) ([]MetricSample, error) {
	if p.store == nil {
		return []MetricSample{}, nil
	}

	// Fetch last 50 metric samples from the database
	records, err := p.store.Query(ctx, "MetricSample").OrderBy("timestamp", true).Limit(50).Execute(ctx)
	if err != nil {
		return nil, fmt.Errorf("query metric samples: %w", err)
	}

	samples := make([]MetricSample, 0, len(records))
	for _, r := range records {
		t, _ := r["timestamp"].(string)
		
		// Parse timestamp string if we want to format it as "15:04" for the front-end chart
		parsedTime := t
		if pt, err := time.Parse(time.RFC3339, t); err == nil {
			parsedTime = pt.Format("15:04")
		}

		cpuVal, _ := r["cpu_percent"].(float64)
		memVal, _ := r["mem_sys"].(int64)
		goroutines, _ := r["goroutines"].(int64)
		
		traffic, _ := r["http_total"].(int64)
		errorsCount, _ := r["http_errors"].(int64)
		latency, _ := r["http_p95_ms"].(int64)

		samples = append(samples, MetricSample{
			Time:    parsedTime,
			Traffic: float64(traffic),
			Load:    float64(goroutines),
			Latency: float64(latency),
			Errors:  float64(errorsCount),
			Users:   0,
			CPU:     cpuVal,
			Mem:     float64(memVal) / (1024 * 1024), // MB
		})
	}

	return samples, nil
}

func (p *LocalBatteryTelemetryProvider) startCollection() {
	interval := 30 * time.Second
	if env := os.Getenv("BFFX_TELEMETRY_INTERVAL"); env != "" {
		if d, err := time.ParseDuration(env); err == nil {
			interval = d
		} else if sec, err := strconv.Atoi(env); err == nil {
			interval = time.Duration(sec) * time.Second
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial collection to populate first data point
	p.collectSample(context.Background())

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.collectSample(context.Background())
		}
	}
}

func (p *LocalBatteryTelemetryProvider) collectSample(ctx context.Context) {
	if p.store == nil {
		return
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	cpuPct := p.getCPUPercent()
	totalReqs, totalErrors, p95Ms := p.getPrometheusMetrics()

	_, err := p.store.Create(ctx, "MetricSample", map[string]any{
		"timestamp":   time.Now().Format(time.RFC3339),
		"goroutines":  runtime.NumGoroutine(),
		"mem_alloc":   int(m.Alloc),
		"mem_sys":     int(m.Sys),
		"cpu_percent": cpuPct,
		"http_total":  int(totalReqs),
		"http_errors": int(totalErrors),
		"http_p95_ms": int(p95Ms),
	})
	if err != nil {
		return
	}

	// Retention cleanup: delete samples older than 7 days
	retentionDays := 7
	if env := os.Getenv("BFFX_METRIC_RETENTION_DAYS"); env != "" {
		if val, err := strconv.Atoi(env); err == nil && val > 0 {
			retentionDays = val
		}
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays).Format(time.RFC3339)
	records, err := p.store.Query(ctx, "MetricSample").Where("timestamp", "<", cutoff).Execute(ctx)
	if err == nil {
		for _, r := range records {
			if id, ok := r["id"].(string); ok {
				_ = p.store.Delete(ctx, "MetricSample", id)
			}
		}
	}
}

func (p *LocalBatteryTelemetryProvider) getCPUPercent() float64 {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0
	}
	now := time.Now()
	cpuTime := int64(usage.Utime.Sec)*1e6 + int64(usage.Utime.Usec) + int64(usage.Stime.Sec)*1e6 + int64(usage.Stime.Usec)
	if p.lastSample.IsZero() {
		p.lastCPUTime = cpuTime
		p.lastSample = now
		return 0
	}
	deltaCPUTime := cpuTime - p.lastCPUTime
	deltaTime := now.Sub(p.lastSample).Microseconds()
	p.lastCPUTime = cpuTime
	p.lastSample = now
	if deltaTime == 0 {
		return 0
	}
	pct := float64(deltaCPUTime) / float64(deltaTime) * 100.0
	if pct < 0 {
		pct = 0
	}
	return pct
}

func (p *LocalBatteryTelemetryProvider) getPrometheusMetrics() (totalReqs int64, totalErrors int64, p95Ms int64) {
	metricFamilies, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return 0, 0, 0
	}

	for _, mf := range metricFamilies {
		if mf.GetName() == "bffx_http_requests_total" {
			for _, m := range mf.GetMetric() {
				var count int64
				if m.GetCounter() != nil {
					count = int64(m.GetCounter().GetValue())
				}
				totalReqs += count
				for _, lp := range m.GetLabel() {
					if lp.GetName() == "status" {
						val := lp.GetValue()
						if len(val) > 0 && (val[0] == '4' || val[0] == '5') {
							totalErrors += count
						}
					}
				}
			}
		} else if mf.GetName() == "bffx_http_request_duration_seconds" {
			for _, m := range mf.GetMetric() {
				h := m.GetHistogram()
				if h == nil {
					continue
				}
				count := h.GetSampleCount()
				if count == 0 {
					continue
				}
				totalCount := float64(count)
				target := totalCount * 0.95
				var prevUpper float64
				var p95 float64
				for _, b := range h.GetBucket() {
					if float64(b.GetCumulativeCount()) >= target {
						p95 = prevUpper + (b.GetUpperBound()-prevUpper)*(target-prevUpper)/float64(b.GetCumulativeCount())
						break
					}
					prevUpper = b.GetUpperBound()
				}
				if p95 == 0 && len(h.GetBucket()) > 0 {
					p95 = h.GetBucket()[len(h.GetBucket())-1].GetUpperBound()
				}
				p95Ms = int64(p95 * 1000)
			}
		}
	}
	return totalReqs, totalErrors, p95Ms
}
