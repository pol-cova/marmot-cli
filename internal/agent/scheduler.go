package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/pol-cova/marmot-cli/internal/config"

	"github.com/robfig/cron/v3"
)

// Scheduler manages cron-based backup scheduling
type Scheduler struct {
	cron     *cron.Cron
	agent    *Agent
	config   *config.Config
	ctx      context.Context
	cancel   context.CancelFunc
	entryIDs map[string]cron.EntryID
}

// NewScheduler creates a new scheduler
func NewScheduler(agent *Agent, cfg *config.Config) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())

	return &Scheduler{
		cron:     cron.New(cron.WithSeconds()),
		agent:    agent,
		config:   cfg,
		ctx:      ctx,
		cancel:   cancel,
		entryIDs: make(map[string]cron.EntryID),
	}
}

// Start starts the scheduler
func (s *Scheduler) Start() error {
	// Schedule all enabled databases
	for _, db := range s.config.Databases {
		if db.Enabled && db.Schedule != "" {
			if err := s.ScheduleDatabase(db.ID); err != nil {
				return fmt.Errorf("failed to schedule database %s: %w", db.ID, err)
			}
		}
	}

	// Schedule queue processor to retry failed uploads every 5 minutes
	_, err := s.cron.AddFunc("0 */5 * * * *", func() {
		if err := s.agent.RetryFailedUploads(s.ctx); err != nil {
			fmt.Printf("Error processing upload queue: %v\n", err)
		}
	})
	if err != nil {
		return fmt.Errorf("failed to schedule queue processor: %w", err)
	}

	s.cron.Start()

	// Process queue immediately on start
	go func() {
		if err := s.agent.RetryFailedUploads(s.ctx); err != nil {
			fmt.Printf("Error processing upload queue on start: %v\n", err)
		}
	}()

	return nil
}

// ScheduleDatabase schedules a database for backup
func (s *Scheduler) ScheduleDatabase(databaseID string) error {
	dbConfig := s.config.GetDatabaseByID(databaseID)
	if dbConfig == nil {
		return fmt.Errorf("database not found: %s", databaseID)
	}

	if dbConfig.Schedule == "" {
		return fmt.Errorf("no schedule defined for database: %s", databaseID)
	}

	// Remove existing entry if present
	if entryID, exists := s.entryIDs[databaseID]; exists {
		s.cron.Remove(entryID)
	}

	// Create closure to capture database config
	db := *dbConfig
	entryID, err := s.cron.AddFunc(dbConfig.Schedule, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		// Use async upload for scheduled backups (waitForUpload=false)
		if err := s.agent.Backup(ctx, &db, false); err != nil {
			// Log error (would use proper logger in production)
			fmt.Printf("Backup failed for database %s: %v\n", db.ID, err)
		}
	})

	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	s.entryIDs[databaseID] = entryID
	return nil
}

// UnscheduleDatabase removes a database from the schedule
func (s *Scheduler) UnscheduleDatabase(databaseID string) error {
	entryID, exists := s.entryIDs[databaseID]
	if !exists {
		return fmt.Errorf("database not scheduled: %s", databaseID)
	}

	s.cron.Remove(entryID)
	delete(s.entryIDs, databaseID)
	return nil
}

// Stop stops the scheduler gracefully
func (s *Scheduler) Stop() {
	s.cancel()
	s.cron.Stop()
}

// NextRun returns the next scheduled run time for a database
func (s *Scheduler) NextRun(databaseID string) (time.Time, error) {
	entryID, exists := s.entryIDs[databaseID]
	if !exists {
		return time.Time{}, fmt.Errorf("database not scheduled: %s", databaseID)
	}

	entry := s.cron.Entry(entryID)
	return entry.Next, nil
}
