package worker

import (
	"context"
	"fmt"
	"sync"

	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/robfig/cron/v3"
)

// CronJob describes a scheduled manifest job.
type CronJob struct {
	Name     string
	Schedule string
	Action   string
}

// CronScheduler registers cron expressions and invokes callbacks (worker integration point).
type CronScheduler struct {
	mu    sync.Mutex
	jobs  []CronJob
	start bool
	cron  *cron.Cron
}

func NewCronScheduler() *CronScheduler {
	return &CronScheduler{}
}

// LoadFromRegistry collects CronJob manifests from the registry.
func (s *CronScheduler) LoadFromRegistry(reg *manifest.Registry) error {
	if reg == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs = nil
	for _, m := range reg.CronJobs {
		var spec manifest.CronJobSpec
		if err := m.UnmarshalSpec(&spec); err != nil {
			return err
		}
		if spec.Schedule == "" || spec.Action == "" {
			return fmt.Errorf("cronjob %s: schedule and action required", m.Metadata.Name)
		}
		s.jobs = append(s.jobs, CronJob{
			Name:     m.Metadata.Name,
			Schedule: spec.Schedule,
			Action:   spec.Action,
		})
	}
	for _, m := range reg.Leaderboards {
		var spec manifest.LeaderboardSpec
		if err := m.UnmarshalSpec(&spec); err == nil && spec.ResetSchedule != "" {
			s.jobs = append(s.jobs, CronJob{
				Name:     "leaderboard_reset_" + m.Metadata.Name,
				Schedule: spec.ResetSchedule,
				Action:   "leaderboard_reset:" + m.Metadata.Name,
			})
		}
	}
	return nil
}

func (s *CronScheduler) Jobs() []CronJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]CronJob, len(s.jobs))
	copy(out, s.jobs)
	return out
}

// Start registers and starts all cron jobs using robfig/cron/v3.
func (s *CronScheduler) Start(ctx context.Context, run func(job CronJob) error) error {
	s.mu.Lock()
	if s.start {
		s.mu.Unlock()
		return fmt.Errorf("cron scheduler already started")
	}
	s.start = true
	jobs := append([]CronJob(nil), s.jobs...)
	s.mu.Unlock()

	if run == nil {
		return nil
	}

	c := cron.New(cron.WithParser(cron.NewParser(
		cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)))
	s.cron = c

	for _, j := range jobs {
		jobCopy := j
		_, err := c.AddFunc(j.Schedule, func() {
			_ = run(jobCopy)
		})
		if err != nil {
			return fmt.Errorf("failed to schedule job %s (%s): %w", j.Name, j.Schedule, err)
		}
	}

	c.Start()

	go func() {
		<-ctx.Done()
		c.Stop()
	}()

	return nil
}
