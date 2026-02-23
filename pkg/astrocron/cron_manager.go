// ================ Version : V1.1.1 ===========
package astrocron

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Task is the function your job will execute
type Task func() error

// Job holds everything about a scheduled task
type Job struct {
	ID       string
	Name     string
	Schedule string // "every 5m", "every 1h", "daily at 15:04", "* * * * *"
	Task     Task
	Enabled  bool

	// Stats
	RunCount  int
	LastRun   time.Time
	NextRun   time.Time
	LastError string
}

// Manager controls all your scheduled jobs
type Manager struct {
	jobs   map[string]*jobRunner
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Internal job runner
type jobRunner struct {
	job    *Job
	cancel context.CancelFunc
	mu     sync.Mutex
}

// ============================================================================
// CREATE MANAGER - One simple function
// ============================================================================

// New creates a new cron manager with default logger
func NewCronManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		jobs:   make(map[string]*jobRunner),
		ctx:    ctx,
		cancel: cancel,
	}
}

// NewWithLogger creates a new cron manager with custom logger
func NewWithLogger(logger zerolog.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	// Add component field to logger
	logger = logger.With().Str("component", "cron").Logger()

	return &Manager{
		jobs:   make(map[string]*jobRunner),
		ctx:    ctx,
		cancel: cancel,
	}
}

// SetLogger updates the logger
func (m *Manager) SetLogger(logger zerolog.Logger) {
	m.mu.Lock()
	defer m.mu.Unlock()
}

// ============================================================================
// ADD JOBS - Simple and intuitive
// ============================================================================

// Add registers a new job with simple schedule format
//
// Schedule examples:
//
//	"every 5s"           - every 5 seconds
//	"every 10m"          - every 10 minutes
//	"every 2h"           - every 2 hours
//	"daily 14:30"        - every day at 14:30
//	"weekly 14:30"       - every Monday at 14:30
//	"biweekly 14:30"     - every 2 weeks (Monday) at 14:30
//	"monthly 14:30"      - 1st of each month at 14:30
//	"every 3months 14:30" - every 3 months (1st) at 14:30
//	"yearly 01-15 14:30" - every year on January 15th at 14:30
func (m *Manager) Add(id, name, schedule string, task Task) error {
	if id == "" {
		log.Error().Msg("Job ID cannot be empty")
		return fmt.Errorf("job ID cannot be empty")
	}
	if task == nil {
		log.Error().Str("job_id", id).Msg("Task cannot be nil")
		return fmt.Errorf("task cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if job already exists
	if _, exists := m.jobs[id]; exists {
		log.Warn().Str("job_id", id).Msg("Job already exists")
		return fmt.Errorf("job '%s' already exists", id)
	}

	// Parse schedule
	interval, nextRun, err := parseSchedule(schedule)
	if err != nil {
		log.Error().
			Str("job_id", id).
			Str("schedule", schedule).
			Err(err).
			Msg("Invalid schedule format")
		return fmt.Errorf("invalid schedule '%s': %w", schedule, err)
	}

	// Create job
	job := &Job{
		ID:       id,
		Name:     name,
		Schedule: schedule,
		Task:     task,
		Enabled:  true,
		NextRun:  nextRun,
	}

	// Create runner
	ctx, cancel := context.WithCancel(m.ctx)
	runner := &jobRunner{
		job:    job,
		cancel: cancel,
	}

	m.jobs[id] = runner

	// Start the job
	m.wg.Add(1)
	go m.runJob(ctx, runner, interval)

	log.Info().
		Str("job_id", id).
		Str("job_name", name).
		Str("schedule", schedule).
		Time("next_run", nextRun).
		Msg("Job added successfully")

	return nil
}

// Quick adds a simple interval-based job
func (m *Manager) Quick(id, name string, interval time.Duration, task Task) error {
	schedule := fmt.Sprintf("every %v", interval)

	log.Debug().
		Str("job_id", id).
		Dur("interval", interval).
		Msg("Adding quick interval job")

	return m.Add(id, name, schedule, task)
}

// Daily adds a job that runs once per day at specific time
func (m *Manager) Daily(id, name, timeOfDay string, task Task) error {
	schedule := fmt.Sprintf("daily %s", timeOfDay)

	log.Debug().
		Str("job_id", id).
		Str("time", timeOfDay).
		Msg("Adding daily job")

	return m.Add(id, name, schedule, task)
}

// Weekly adds a job that runs every Monday at specific time
func (m *Manager) Weekly(id, name, timeOfDay string, task Task) error {
	schedule := fmt.Sprintf("weekly %s", timeOfDay)

	log.Debug().
		Str("job_id", id).
		Str("time", timeOfDay).
		Msg("Adding weekly job")

	return m.Add(id, name, schedule, task)
}

// BiWeekly adds a job that runs every 2 weeks (Monday) at specific time
func (m *Manager) BiWeekly(id, name, timeOfDay string, task Task) error {
	schedule := fmt.Sprintf("biweekly %s", timeOfDay)

	log.Debug().
		Str("job_id", id).
		Str("time", timeOfDay).
		Msg("Adding bi-weekly job")

	return m.Add(id, name, schedule, task)
}

// Monthly adds a job that runs on 1st of each month at specific time
func (m *Manager) Monthly(id, name, timeOfDay string, task Task) error {
	schedule := fmt.Sprintf("monthly %s", timeOfDay)

	log.Debug().
		Str("job_id", id).
		Str("time", timeOfDay).
		Msg("Adding monthly job")

	return m.Add(id, name, schedule, task)
}

// EveryMonths adds a job that runs every X months on 1st at specific time
func (m *Manager) EveryMonths(id, name string, months int, timeOfDay string, task Task) error {
	schedule := fmt.Sprintf("every %dmonths %s", months, timeOfDay)

	log.Debug().
		Str("job_id", id).
		Int("months", months).
		Str("time", timeOfDay).
		Msg("Adding every X months job")

	return m.Add(id, name, schedule, task)
}

// Yearly adds a job that runs every year on specific date at specific time
func (m *Manager) Yearly(id, name, date, timeOfDay string, task Task) error {
	schedule := fmt.Sprintf("yearly %s %s", date, timeOfDay)

	log.Debug().
		Str("job_id", id).
		Str("date", date).
		Str("time", timeOfDay).
		Msg("Adding yearly job")

	return m.Add(id, name, schedule, task)
}

// ============================================================================
// MANAGE JOBS - Simple operations
// ============================================================================

// Remove stops and removes a job
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	runner, exists := m.jobs[id]
	if !exists {
		log.Warn().Str("job_id", id).Msg("Job not found for removal")
		return fmt.Errorf("job '%s' not found", id)
	}

	runner.cancel()
	delete(m.jobs, id)

	log.Info().Str("job_id", id).Msg("Job removed successfully")
	return nil
}

// Enable activates a job
func (m *Manager) Enable(id string) error {
	m.mu.RLock()
	runner, exists := m.jobs[id]
	m.mu.RUnlock()

	if !exists {
		log.Warn().Str("job_id", id).Msg("Job not found for enabling")
		return fmt.Errorf("job '%s' not found", id)
	}

	runner.mu.Lock()
	runner.job.Enabled = true
	runner.mu.Unlock()

	log.Info().Str("job_id", id).Msg("Job enabled")
	return nil
}

// Disable pauses a job without removing it
func (m *Manager) Disable(id string) error {
	m.mu.RLock()
	runner, exists := m.jobs[id]
	m.mu.RUnlock()

	if !exists {
		log.Warn().Str("job_id", id).Msg("Job not found for disabling")
		return fmt.Errorf("job '%s' not found", id)
	}

	runner.mu.Lock()
	runner.job.Enabled = false
	runner.mu.Unlock()

	log.Info().Str("job_id", id).Msg("Job disabled")
	return nil
}

// Get returns job information
func (m *Manager) Get(id string) (*Job, error) {
	m.mu.RLock()
	runner, exists := m.jobs[id]
	m.mu.RUnlock()

	if !exists {
		log.Debug().Str("job_id", id).Msg("Job not found")
		return nil, fmt.Errorf("job '%s' not found", id)
	}

	runner.mu.Lock()
	defer runner.mu.Unlock()

	// Return a copy
	jobCopy := *runner.job
	return &jobCopy, nil
}

// List returns all jobs
func (m *Manager) List() []*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]*Job, 0, len(m.jobs))
	for _, runner := range m.jobs {
		runner.mu.Lock()
		jobCopy := *runner.job
		runner.mu.Unlock()
		jobs = append(jobs, &jobCopy)
	}

	log.Debug().Int("count", len(jobs)).Msg("Listed all jobs")
	return jobs
}

// RunNow executes a job immediately (doesn't affect schedule)
func (m *Manager) RunNow(id string) error {
	m.mu.RLock()
	runner, exists := m.jobs[id]
	m.mu.RUnlock()

	if !exists {
		log.Warn().Str("job_id", id).Msg("Job not found for manual run")
		return fmt.Errorf("job '%s' not found", id)
	}

	log.Info().Str("job_id", id).Msg("Triggering job manually")
	go m.executeTask(runner)
	return nil
}

// ============================================================================
// LIFECYCLE - Start and Stop
// ============================================================================

// Stop gracefully stops all jobs
func (m *Manager) Stop() {
	log.Info().Msg("Stopping cron manager...")
	m.cancel()
	m.wg.Wait()
	log.Info().Msg("Cron manager stopped successfully")
}

// StopWithTimeout stops with a maximum wait time
func (m *Manager) StopWithTimeout(timeout time.Duration) error {
	log.Info().Dur("timeout", timeout).Msg("Stopping cron manager with timeout...")

	done := make(chan struct{})
	go func() {
		m.Stop()
		close(done)
	}()

	select {
	case <-done:
		log.Info().Msg("Cron manager stopped within timeout")
		return nil
	case <-time.After(timeout):
		log.Error().Dur("timeout", timeout).Msg("Shutdown timeout exceeded")
		return fmt.Errorf("shutdown timeout after %v", timeout)
	}
}

// ============================================================================
// INTERNAL - Job execution logic
// ============================================================================

func (m *Manager) runJob(ctx context.Context, runner *jobRunner, interval time.Duration) {
	defer m.wg.Done()

	jobID := runner.job.ID
	log.Debug().
		Str("job_id", jobID).
		Dur("interval", interval).
		Msg("Job scheduler started")

	timer := time.NewTimer(time.Until(runner.job.NextRun))
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Debug().Str("job_id", jobID).Msg("Job scheduler stopped")
			return
		case <-timer.C:
			runner.mu.Lock()
			enabled := runner.job.Enabled
			schedule := runner.job.Schedule
			runner.mu.Unlock()

			if enabled {
				m.executeTask(runner)
			} else {
				log.Debug().Str("job_id", jobID).Msg("Job is disabled, skipping execution")
			}

			// Schedule next run
			runner.mu.Lock()
			// For calendar-based schedules (interval=0), recalculate next run based on schedule
			if interval == 0 {
				// Recalculate for calendar-based schedules (monthly, every X months, yearly)
				_, nextRunRecalc, err := parseSchedule(schedule)
				if err != nil {
					log.Error().Str("job_id", jobID).Err(err).Msg("Failed to recalculate next run")
					// Fallback: add 1 month as default
					runner.job.NextRun = time.Now().AddDate(0, 1, 0)
				} else {
					runner.job.NextRun = nextRunRecalc
				}
				nextRun := runner.job.NextRun
				runner.mu.Unlock()

				log.Debug().
					Str("job_id", jobID).
					Time("next_run", nextRun).
					Msg("Next execution scheduled (calendar-based)")

				timer.Reset(time.Until(nextRun))
			} else {
				// For interval-based schedules, just add the interval
				runner.job.NextRun = time.Now().Add(interval)
				nextRun := runner.job.NextRun
				runner.mu.Unlock()

				log.Debug().
					Str("job_id", jobID).
					Time("next_run", nextRun).
					Msg("Next execution scheduled")

				timer.Reset(interval)
			}
		}
	}
}

func (m *Manager) executeTask(runner *jobRunner) {
	runner.mu.Lock()
	job := runner.job
	runner.mu.Unlock()

	log.Info().
		Str("job_id", job.ID).
		Str("job_name", job.Name).
		Msg("Executing job")

	startTime := time.Now()
	err := job.Task()
	duration := time.Since(startTime)

	// Update stats
	runner.mu.Lock()
	runner.job.LastRun = startTime
	runner.job.RunCount++
	if err != nil {
		runner.job.LastError = err.Error()
		log.Error().
			Str("job_id", job.ID).
			Str("job_name", job.Name).
			Dur("duration", duration).
			Err(err).
			Msg("Job execution failed")
	} else {
		runner.job.LastError = ""
		log.Info().
			Str("job_id", job.ID).
			Str("job_name", job.Name).
			//Dur("duration", duration).
			Int("total_runs", runner.job.RunCount).
			Msg("Job executed successfully")
	}
	runner.mu.Unlock()
}

// ============================================================================
// SCHEDULE PARSER - Simple and intuitive
// ============================================================================

func parseSchedule(schedule string) (time.Duration, time.Time, error) {
	var interval time.Duration
	var nextRun time.Time

	// Parse "every X" format (for minutes, hours, seconds)
	if len(schedule) > 6 && schedule[:5] == "every" {
		// First check if it's "every Xmonths HH:MM" format
		var monthCount int
		var timeStr string
		if _, err := fmt.Sscanf(schedule, "every %dmonths %s", &monthCount, &timeStr); err == nil && monthCount > 0 {
			t, err := time.Parse("15:04", timeStr)
			if err != nil {
				return 0, time.Time{}, fmt.Errorf("invalid time format (use HH:MM): %w", err)
			}

			now := time.Now()
			// Calculate next first of month at specified time
			nextRun = time.Date(now.Year(), now.Month(), 1, t.Hour(), t.Minute(), 0, 0, now.Location())

			// If we're past the 1st of this month or past the time on the 1st, move to next occurrence
			if now.Day() > 1 || (now.Day() == 1 && now.After(nextRun)) {
				nextRun = nextRun.AddDate(0, monthCount, 0)
			}

			// For every X months, we use interval 0 to indicate calendar-based scheduling
			interval = 0 // Special case - recalculate next run after each execution
			return interval, nextRun, nil
		}

		// Otherwise it's a simple duration like "every 5m"
		durationStr := schedule[6:]
		var err error
		interval, err = time.ParseDuration(durationStr)
		if err != nil {
			return 0, time.Time{}, fmt.Errorf("invalid duration: %w", err)
		}
		if interval <= 0 {
			return 0, time.Time{}, fmt.Errorf("duration must be positive")
		}
		nextRun = time.Now().Add(interval)
		return interval, nextRun, nil
	}

	// Parse "daily HH:MM" format
	if len(schedule) > 6 && schedule[:5] == "daily" {
		timeStr := schedule[6:]
		t, err := time.Parse("15:04", timeStr)
		if err != nil {
			return 0, time.Time{}, fmt.Errorf("invalid time format (use HH:MM): %w", err)
		}

		now := time.Now()
		nextRun = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())

		// If time has passed today, schedule for tomorrow
		if nextRun.Before(now) {
			nextRun = nextRun.Add(24 * time.Hour)
		}

		interval = 24 * time.Hour
		return interval, nextRun, nil
	}

	// Parse "weekly HH:MM" format - runs every Monday at specified time
	if len(schedule) > 7 && schedule[:6] == "weekly" {
		timeStr := schedule[7:]
		t, err := time.Parse("15:04", timeStr)
		if err != nil {
			return 0, time.Time{}, fmt.Errorf("invalid time format (use HH:MM): %w", err)
		}

		now := time.Now()
		// Calculate next Monday at specified time
		daysUntilMonday := (8 - int(now.Weekday())) % 7
		if daysUntilMonday == 0 {
			// Today is Monday, check if time has passed
			nextRun = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
			if nextRun.Before(now) || nextRun.Equal(now) {
				daysUntilMonday = 7
			}
		}

		if daysUntilMonday > 0 {
			nextRun = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location()).AddDate(0, 0, daysUntilMonday)
		}

		interval = 7 * 24 * time.Hour
		return interval, nextRun, nil
	}

	// Parse "biweekly HH:MM" format - runs every 2 weeks on Monday at specified time
	if len(schedule) > 9 && schedule[:8] == "biweekly" {
		timeStr := schedule[9:]
		t, err := time.Parse("15:04", timeStr)
		if err != nil {
			return 0, time.Time{}, fmt.Errorf("invalid time format (use HH:MM): %w", err)
		}

		now := time.Now()
		// Calculate next Monday at specified time
		daysUntilMonday := (8 - int(now.Weekday())) % 7
		if daysUntilMonday == 0 {
			// Today is Monday, check if time has passed
			nextRun = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
			if nextRun.Before(now) || nextRun.Equal(now) {
				daysUntilMonday = 7
			}
		}

		if daysUntilMonday > 0 {
			nextRun = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location()).AddDate(0, 0, daysUntilMonday)
		}

		interval = 14 * 24 * time.Hour // 2 weeks
		return interval, nextRun, nil
	}

	// Parse "monthly HH:MM" format - runs on 1st of each month at specified time
	if len(schedule) > 8 && schedule[:7] == "monthly" {
		timeStr := schedule[8:]
		t, err := time.Parse("15:04", timeStr)
		if err != nil {
			return 0, time.Time{}, fmt.Errorf("invalid time format (use HH:MM): %w", err)
		}

		now := time.Now()
		// Calculate next first of month at specified time
		nextRun = time.Date(now.Year(), now.Month(), 1, t.Hour(), t.Minute(), 0, 0, now.Location())

		// If we're past the 1st of this month or past the time on the 1st, move to next month
		if now.Day() > 1 || (now.Day() == 1 && now.After(nextRun)) {
			// Add one month - this handles varying month lengths automatically
			nextRun = nextRun.AddDate(0, 1, 0)
		}

		// For monthly, we use a special marker interval (0) to indicate calendar-based scheduling
		interval = 0 // Special case - recalculate next run after each execution
		return interval, nextRun, nil
	}

	// Parse "yearly MM-DD HH:MM" format - runs every year on specific date at specified time
	if len(schedule) > 7 && schedule[:6] == "yearly" {
		parts := schedule[7:]
		// Expected format: "MM-DD HH:MM" or "01-15 14:30"
		var month, day int
		var timeStr string

		if _, err := fmt.Sscanf(parts, "%d-%d %s", &month, &day, &timeStr); err != nil {
			return 0, time.Time{}, fmt.Errorf("invalid yearly format (use MM-DD HH:MM): %w", err)
		}

		if month < 1 || month > 12 {
			return 0, time.Time{}, fmt.Errorf("invalid month: %d", month)
		}
		if day < 1 || day > 31 {
			return 0, time.Time{}, fmt.Errorf("invalid day: %d", day)
		}

		t, err := time.Parse("15:04", timeStr)
		if err != nil {
			return 0, time.Time{}, fmt.Errorf("invalid time format (use HH:MM): %w", err)
		}

		now := time.Now()
		nextRun = time.Date(now.Year(), time.Month(month), day, t.Hour(), t.Minute(), 0, 0, now.Location())

		// If date has passed this year, schedule for next year
		if nextRun.Before(now) || nextRun.Equal(now) {
			nextRun = nextRun.AddDate(1, 0, 0)
		}

		// For yearly, we use interval 0 to indicate calendar-based scheduling
		interval = 0 // Special case - recalculate next run after each execution
		return interval, nextRun, nil
	}

	return 0, time.Time{}, fmt.Errorf("unsupported schedule format")
}

// ============================================================================
// UTILITIES - Helper functions
// ============================================================================

// Print displays all jobs in a nice format
/* func (m *Manager) Print() {
	jobs := m.List()

	fmt.Println("\n╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                      SCHEDULED JOBS                            ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")

	if len(jobs) == 0 {
		fmt.Println("No jobs scheduled")
		log.Debug().Msg("Print: No jobs to display")
		return
	}

	for _, job := range jobs {
		status := "✓ ENABLED"
		if !job.Enabled {
			status = "✗ DISABLED"
		}

		fmt.Printf("\n📋 %s (%s)\n", job.Name, job.ID)
		fmt.Printf("   Status:    %s\n", status)
		fmt.Printf("   Schedule:  %s\n", job.Schedule)
		fmt.Printf("   Runs:      %d times\n", job.RunCount)

		if !job.LastRun.IsZero() {
			fmt.Printf("   Last Run:  %s\n", job.LastRun.Format("2006-01-02 15:04:05"))
		}

		if !job.NextRun.IsZero() {
			fmt.Printf("   Next Run:  %s\n", job.NextRun.Format("2006-01-02 15:04:05"))
		}

		if job.LastError != "" {
			fmt.Printf("   Error:    %s\n", job.LastError)
		}
	}
	fmt.Println()

	log.Debug().Int("jobs_displayed", len(jobs)).Msg("Printed job status")
}
*/
