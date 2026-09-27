package usecase

import (
	"context"
	"fmt"
	"time"
)

const schedulerInterval = 15 * time.Second
const summaryInterval = 15 * time.Second
const retentionInterval = 24 * time.Hour

type LifecycleUseCase struct {
	scheduler  lifecycleScheduler
	dispatcher lifecycleDispatcher
	retention  lifecycleRetention
}

func NewLifecycleUseCase(scheduler lifecycleScheduler, dispatcher lifecycleDispatcher, retention lifecycleRetention) (*LifecycleUseCase, error) {
	if scheduler == nil || dispatcher == nil || retention == nil {
		return nil, fmt.Errorf("publication lifecycle dependencies are required")
	}
	return &LifecycleUseCase{scheduler: scheduler, dispatcher: dispatcher, retention: retention}, nil
}

func (u *LifecycleUseCase) Run(ctx context.Context) error {
	if err := u.scheduler.Recover(ctx); err != nil {
		return fmt.Errorf("recover publication scheduler: %w", err)
	}
	if err := u.dispatcher.Recover(ctx); err != nil {
		return fmt.Errorf("recover summary dispatcher: %w", err)
	}
	schedulerTicker := time.NewTicker(schedulerInterval)
	summaryTicker := time.NewTicker(summaryInterval)
	retentionTicker := time.NewTicker(retentionInterval)
	defer schedulerTicker.Stop()
	defer summaryTicker.Stop()
	defer retentionTicker.Stop()
	return u.run(ctx, time.Now, schedulerTicker.C, summaryTicker.C, retentionTicker.C)
}

func (u *LifecycleUseCase) run(ctx context.Context, now func() time.Time, schedulerTicks, summaryTicks, retentionTicks <-chan time.Time) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errors := make(chan error, 3)
	go func() { errors <- u.runScheduler(ctx, now, schedulerTicks) }()
	go func() { errors <- u.runDispatcher(ctx, summaryTicks) }()
	go func() { errors <- u.runRetention(ctx, retentionTicks) }()
	for range 3 {
		select {
		case err := <-errors:
			if err != nil {
				cancel()
				for range 2 {
					<-errors
				}
				return err
			}
		case <-ctx.Done():
			cancel()
			for range 3 {
				<-errors
			}
			return nil
		}
	}
	return nil
}

func (u *LifecycleUseCase) runScheduler(ctx context.Context, now func() time.Time, ticks <-chan time.Time) error {
	for {
		if err := u.scheduler.Tick(ctx, now()); err != nil {
			return fmt.Errorf("publication scheduler tick: %w", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticks:
		}
	}
}

func (u *LifecycleUseCase) runDispatcher(ctx context.Context, ticks <-chan time.Time) error {
	for {
		if err := u.dispatcher.Tick(ctx); err != nil {
			return fmt.Errorf("summary dispatcher tick: %w", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticks:
		}
	}
}

func (u *LifecycleUseCase) runRetention(ctx context.Context, ticks <-chan time.Time) error {
	for {
		if err := u.retention.PurgeExpiredText(ctx); err != nil {
			return fmt.Errorf("purge expired report text: %w", err)
		}
		if err := u.retention.PurgeExpired(ctx); err != nil {
			return fmt.Errorf("purge expired summary text: %w", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticks:
		}
	}
}
