package backup

import (
	"log/slog"
	"time"
)

type Scheduler struct {
	dataDir        string
	interval       time.Duration
	maxGenerations int
	projectIDs     func() []string
	stop           chan struct{}
	done           chan struct{}
}

func NewScheduler(dataDir string, interval time.Duration, maxGenerations int, projectIDs func() []string) *Scheduler {
	return &Scheduler{
		dataDir:        dataDir,
		interval:       interval,
		maxGenerations: maxGenerations,
		projectIDs:     projectIDs,
		stop:           make(chan struct{}),
		done:           make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.runBackups()
			case <-s.stop:
				return
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	close(s.stop)
	<-s.done
}

func (s *Scheduler) runBackups() {
	for _, pid := range s.projectIDs() {
		_, err := CreateBackup(s.dataDir, pid)
		if err != nil {
			slog.Error("auto backup failed", "projectID", pid, "error", err)
			continue
		}
		if _, err := PruneBackups(s.dataDir, pid, s.maxGenerations); err != nil {
			slog.Error("backup prune failed", "projectID", pid, "error", err)
		}
	}
}
