// Package tasks provides distributed cron and background task orchestration.
package tasks

import (
	"context"
	"fmt"

	"layr.sh/core"
)

func init() {
	factory := func(kernel *core.Kernel) (core.ServiceRunner, error) {
		return NewService(kernel), nil
	}

	core.RegisterServiceFactory("tasks", factory)
}

// Service encapsulates the distributed cron and background task orchestrator service.
type Service struct {
	kernel              *core.Kernel
	configManager       *ConfigManager
	jobManager          *JobManager
	dispatcher          *Dispatcher
	jobPoller           *JobPoller
	workerQueue         *WorkerQueue
	controlPlaneHandler *ControlPlaneHandler
}

// NewService initializes the Tasks service coordinator.
func NewService(kernel *core.Kernel) *Service {
	configManager := NewConfigManager(kernel)
	jobManager := NewJobManager(kernel, configManager)
	dispatcher := NewDispatcher(kernel)
	jobPoller := NewJobPoller(kernel, configManager)

	workerQueue := NewWorkerQueue(kernel, configManager, dispatcher)

	service := &Service{
		kernel:        kernel,
		configManager: configManager,
		jobManager:    jobManager,
		dispatcher:    dispatcher,
		jobPoller:     jobPoller,
		workerQueue:   workerQueue,
	}
	service.controlPlaneHandler = NewControlPlaneHandler(service)
	return service
}

// Kernel returns the parent kernel instance.
func (service *Service) Kernel() *core.Kernel {
	return service.kernel
}

// Start loads runtime configuration and starts poller and worker loops.
func (service *Service) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled before start: %w", err)
	}

	if err := service.configManager.Load(ctx); err != nil {
		return fmt.Errorf("failed to load tasks config: %w", err)
	}

	service.jobPoller.Start(ctx)
	service.workerQueue.Start(ctx)
	return nil
}

// Stop gracefully shuts down poller and worker processing loops.
func (service *Service) Stop() {
	service.jobPoller.Stop()
	service.workerQueue.Stop()
}

// ConfigManager returns the dynamic runtime configuration manager.
func (service *Service) ConfigManager() *ConfigManager {
	return service.configManager
}

// JobManager returns the job management coordinator.
func (service *Service) JobManager() *JobManager {
	return service.jobManager
}

// Dispatcher returns the execution dispatcher.
func (service *Service) Dispatcher() *Dispatcher {
	return service.dispatcher
}

// JobPoller returns the autonomous cron poller.
func (service *Service) JobPoller() *JobPoller {
	return service.jobPoller
}

// WorkerQueue returns the execution worker queue runner.
func (service *Service) WorkerQueue() *WorkerQueue {
	return service.workerQueue
}

// ControlPlaneHandler returns the administrative control plane handler.
func (service *Service) ControlPlaneHandler() *ControlPlaneHandler {
	return service.controlPlaneHandler
}
