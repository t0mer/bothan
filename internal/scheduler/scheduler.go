package scheduler

import (
	"context"
	"log/slog"
	"sync"

	"github.com/robfig/cron/v3"

	"github.com/t0mer/bothan/internal/model"
	"github.com/t0mer/bothan/internal/store"
)

// Enqueuer triggers a scan for a host (satisfied by *scanner.Service).
type Enqueuer interface {
	Trigger(ctx context.Context, hostID int64, trigger string) (*model.Scan, error)
}

// Service owns the cron registry and rebuilds it from the database whenever
// schedules or host links change.
type Service struct {
	schedules *store.ScheduleRepo
	enqueuer  Enqueuer
	logger    *slog.Logger

	mu   sync.Mutex
	cron *cron.Cron
}

// New builds a scheduler Service.
func New(st *store.Store, enqueuer Enqueuer, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{schedules: st.Schedules(), enqueuer: enqueuer, logger: logger}
}

// Rebuild reloads enabled schedules and re-registers cron jobs. It is called on
// startup and after any schedule or host-schedule change.
func (s *Service) Rebuild(ctx context.Context) error {
	scheds, err := s.schedules.ListEnabled(ctx)
	if err != nil {
		return err
	}

	c := cron.New()
	for _, sched := range scheds {
		sched := sched // capture
		if _, err := c.AddFunc(sched.Spec, func() { s.fire(sched) }); err != nil {
			s.logger.Error("registering schedule",
				slog.String("name", sched.Name), slog.String("spec", sched.Spec), slog.String("error", err.Error()))
			continue
		}
	}

	s.mu.Lock()
	old := s.cron
	s.cron = c
	s.mu.Unlock()

	if old != nil {
		old.Stop()
	}
	c.Start()
	s.logger.Info("scheduler rebuilt", slog.Int("schedules", len(scheds)))
	return nil
}

// fire enqueues a scan for every enabled host linked to the schedule.
func (s *Service) fire(sched model.Schedule) {
	ctx := context.Background()
	hosts, err := s.schedules.EnabledHostsForSchedule(ctx, sched.ID)
	if err != nil {
		s.logger.Error("loading hosts for schedule", slog.String("name", sched.Name), slog.String("error", err.Error()))
		return
	}
	trigger := "schedule:" + sched.Name
	for _, h := range hosts {
		if _, err := s.enqueuer.Trigger(ctx, h.ID, trigger); err != nil {
			// A scan already in progress is expected and harmless (dedup).
			s.logger.Debug("schedule enqueue skipped",
				slog.String("host", h.Hostname), slog.String("schedule", sched.Name), slog.String("error", err.Error()))
			continue
		}
		s.logger.Info("scheduled scan enqueued", slog.String("host", h.Hostname), slog.String("schedule", sched.Name))
	}
}

// Stop halts the cron registry, returning a context that completes when running
// jobs finish.
func (s *Service) Stop() context.Context {
	s.mu.Lock()
	c := s.cron
	s.mu.Unlock()
	if c == nil {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}
	return c.Stop()
}
