package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
)

func TestLifecycleRun(t *testing.T) {
	tests := []struct {
		name        string
		recoverFrom string
		workerError string
		cancel      bool
	}{
		{name: "scheduler recovery error", recoverFrom: "scheduler"},
		{name: "dispatcher recovery error", recoverFrom: "dispatcher"},
		{name: "scheduler error cancels lifecycle", workerError: "scheduler"},
		{name: "dispatcher error cancels lifecycle", workerError: "dispatcher"},
		{name: "context cancellation stops lifecycle", cancel: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			scheduler := NewMocklifecycleScheduler(controller)
			dispatcher := NewMocklifecycleDispatcher(controller)
			retention := NewMocklifecycleRetention(controller)
			lifecycle, err := NewLifecycleUseCase(scheduler, dispatcher, retention)
			if err != nil {
				t.Fatal(err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.recoverFrom == "scheduler" {
				scheduler.EXPECT().Recover(gomock.Any()).Return(errors.New("recover failed"))
				if err := lifecycle.Run(ctx); err == nil {
					t.Fatal("expected recovery error")
				}
				return
			}
			if tt.recoverFrom == "dispatcher" {
				scheduler.EXPECT().Recover(gomock.Any()).Return(nil)
				dispatcher.EXPECT().Recover(gomock.Any()).Return(errors.New("recover failed"))
				if err := lifecycle.Run(ctx); err == nil {
					t.Fatal("expected recovery error")
				}
				return
			}

			started := make(chan struct{}, 3)
			schedulerTickCount := 1
			scheduler.EXPECT().Tick(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, time.Time) error {
				started <- struct{}{}
				if tt.workerError == "scheduler" && schedulerTickCount == 2 {
					return errors.New("tick failed")
				}
				schedulerTickCount++
				return nil
			}).AnyTimes()
			dispatcherTickCount := 1
			dispatcher.EXPECT().Tick(gomock.Any()).DoAndReturn(func(context.Context) error {
				started <- struct{}{}
				if tt.workerError == "dispatcher" && dispatcherTickCount == 2 {
					return errors.New("tick failed")
				}
				dispatcherTickCount++
				return nil
			}).AnyTimes()
			retention.EXPECT().PurgeExpiredText(gomock.Any()).DoAndReturn(func(context.Context) error { started <- struct{}{}; return nil }).AnyTimes()
			retention.EXPECT().PurgeExpired(gomock.Any()).Return(nil).AnyTimes()

			schedulerTicks := make(chan time.Time, 1)
			dispatcherTicks := make(chan time.Time, 1)
			ctx, cancel = context.WithCancel(ctx)
			defer cancel()
			result := make(chan error, 1)
			if tt.cancel {
				scheduler.EXPECT().Recover(gomock.Any()).Return(nil)
				dispatcher.EXPECT().Recover(gomock.Any()).Return(nil)
				go func() { result <- lifecycle.Run(ctx) }()
			} else {
				go func() {
					result <- lifecycle.run(ctx, time.Now, schedulerTicks, dispatcherTicks, make(chan time.Time))
				}()
			}
			for range 3 {
				<-started
			}
			if tt.cancel {
				cancel()
				if err := <-result; err != nil {
					t.Fatalf("Run() error = %v", err)
				}
				return
			}
			if tt.workerError == "dispatcher" {
				dispatcherTicks <- time.Now()
			} else {
				schedulerTicks <- time.Now()
			}
			if err := <-result; err == nil {
				t.Fatal("expected worker error")
			}
		})
	}
}
