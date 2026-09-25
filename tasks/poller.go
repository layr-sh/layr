// Package tasks provides distributed cron and background task orchestration.
package tasks

import (
	"context"
	"fmt"
	"sync"
	"time"

	"layr.sh/core"
)

// JobPoller autonomously discovers due cron jobs and enqueues execution tasks using PostgreSQL row locks.
type JobPoller struct {
	kernel         *core.Kernel
	configManager  *ConfigManager
	stopChannel    chan struct{}
	stoppedChannel chan struct{}
	syncMutex      sync.Mutex
	isRunning      bool
}

// NewJobPoller initializes a new JobPoller.
func NewJobPoller(kernel *core.Kernel, configManager *ConfigManager) *JobPoller {
	return &JobPoller{
		kernel:         kernel,
		configManager:  configManager,
		stopChannel:    make(chan struct{}),
		stoppedChannel: make(chan struct{}),
	}
}

// Start launches the background polling loop.
func (jobPoller *JobPoller) Start(ctx context.Context) {
	jobPoller.syncMutex.Lock()
	if jobPoller.isRunning {
		jobPoller.syncMutex.Unlock()
		return
	}
	jobPoller.isRunning = true
	jobPoller.stopChannel = make(chan struct{})
	jobPoller.stoppedChannel = make(chan struct{})
	jobPoller.syncMutex.Unlock()

	go jobPoller.pollLoop(ctx)
}

// Stop terminates the polling loop gracefully.
func (jobPoller *JobPoller) Stop() {
	jobPoller.syncMutex.Lock()
	if !jobPoller.isRunning {
		jobPoller.syncMutex.Unlock()
		return
	}
	jobPoller.isRunning = false
	close(jobPoller.stopChannel)
	jobPoller.syncMutex.Unlock()

	<-jobPoller.stoppedChannel
}

func (jobPoller *JobPoller) pollLoop(ctx context.Context) {
	defer close(jobPoller.stoppedChannel)
	log.Debug("tasks job poller loop started")

	for {
		tasksConfig := jobPoller.configManager.Get()
		interval := time.Duration(tasksConfig.PollIntervalMs) * time.Millisecond
		if interval < 100*time.Millisecond {
			interval = 100 * time.Millisecond
		}

		select {
		case <-jobPoller.stopChannel:
			log.Debug("tasks job poller loop stopped")
			return
		case <-ctx.Done():
			log.Debug("tasks job poller context canceled")
			return
		case <-time.After(interval):
			if _, err := jobPoller.PollOnce(ctx); err != nil {
				log.Errorf("error in job poller poll cycle: %v", err)
			}
		}
	}
}

// PollOnce inspects tasks.jobs for due jobs using FOR UPDATE SKIP LOCKED and enqueues executions.
func (jobPoller *JobPoller) PollOnce(ctx context.Context) (int, error) {
	tx, err := jobPoller.kernel.DB().Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	rows, _ := tx.Query(ctx, `
		SELECT id, name, cron_expression, timezone, target_type, target_payload, is_enabled, last_run_at, next_run_at, created_at, last_updated_at
		FROM tasks.jobs
		WHERE is_enabled = true AND next_run_at <= clock_timestamp()
		FOR UPDATE SKIP LOCKED
	`)
	defer rows.Close()

	var dueJobs []Job
	for rows.Next() {
		var targetDueJob Job
		_ = rows.Scan(
			&targetDueJob.ID, &targetDueJob.Name, &targetDueJob.CronExpression, &targetDueJob.Timezone,
			&targetDueJob.TargetType, &targetDueJob.TargetPayload, &targetDueJob.IsEnabled,
			&targetDueJob.LastRunAt, &targetDueJob.NextRunAt, &targetDueJob.CreatedAt, &targetDueJob.LastUpdatedAt,
		)
		dueJobs = append(dueJobs, targetDueJob)
	}
	rows.Close()

	if rows.Err() != nil {
		return 0, fmt.Errorf("failed to query due jobs: %w", rows.Err())
	}

	if len(dueJobs) == 0 {
		_ = tx.Commit(ctx)
		return 0, nil
	}

	tasksConfig := jobPoller.configManager.Get()
	scheduledCount := 0

	for _, targetDueJob := range dueJobs {
		cronSchedule, err := ParseCron(targetDueJob.CronExpression)
		if err != nil {
			log.Errorf("skipping job %s (%s) due to invalid cron expression: %v", targetDueJob.ID, targetDueJob.Name, err)
			continue
		}

		timezoneLocation, err := time.LoadLocation(targetDueJob.Timezone)
		if err != nil {
			timezoneLocation = time.UTC
		}

		now := time.Now()
		newNextRunAt := cronSchedule.Next(targetDueJob.NextRunAt, timezoneLocation)
		// Advance next run if the calculated run is still in the past
		for newNextRunAt.Before(now) {
			newNextRunAt = cronSchedule.Next(newNextRunAt, timezoneLocation)
		}

		var execution Execution
		err = tx.QueryRow(ctx, `
			INSERT INTO tasks.executions (
				job_id, status, run_at, payload, attempts, max_attempts, created_at
			) VALUES (
				$1, 'pending', clock_timestamp(), $2, 0, $3, clock_timestamp()
			)
			RETURNING id, job_id, status, run_at, payload, attempts, max_attempts, locked_at, locked_by, last_error, created_at
		`, targetDueJob.ID, targetDueJob.TargetPayload, tasksConfig.RetryMaxAttempts).Scan(
			&execution.ID,
			&execution.JobID,
			&execution.Status,
			&execution.RunAt,
			&execution.Payload,
			&execution.Attempts,
			&execution.MaxAttempts,
			&execution.LockedAt,
			&execution.LockedBy,
			&execution.LastError,
			&execution.CreatedAt,
		)
		if err != nil {
			log.Errorf("failed to enqueue execution for job %s: %v", targetDueJob.ID, err)
			continue
		}

		_, err = tx.Exec(ctx, `
			UPDATE tasks.jobs SET
				last_run_at = clock_timestamp(),
				next_run_at = $1,
				last_updated_at = clock_timestamp()
			WHERE id = $2
		`, newNextRunAt, targetDueJob.ID)
		if err != nil {
			log.Errorf("failed to advance next_run_at for job %s: %v", targetDueJob.ID, err)
			continue
		}

		scheduledCount++

		previousRunAt := targetDueJob.NextRunAt
		jobPoller.kernel.EventBus().Publish(ctx, NewJobScheduledEvent(targetDueJob.ID.String(), JobScheduledEventData{
			Job:           targetDueJob,
			PreviousRunAt: &previousRunAt,
			NextRunAt:     newNextRunAt,
		}))

		jobPoller.kernel.EventBus().Publish(ctx, NewExecutionEnqueuedEvent(execution.ID.String(), ExecutionEnqueuedEventData{
			Execution:   execution,
			JobName:     targetDueJob.Name,
			IsImmediate: false,
		}))
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("failed to commit scheduling transaction: %w", err)
	}

	return scheduledCount, nil
}
