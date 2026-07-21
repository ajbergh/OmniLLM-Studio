package video

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type schedulerTestRenderer struct {
	mu      sync.Mutex
	active  int
	max     int
	started chan RenderRequest
	release chan struct{}
	order   []int
}

func (r *schedulerTestRenderer) Render(
	ctx context.Context,
	req RenderRequest,
	progress func(RenderProgress),
) (*RenderResult, error) {
	r.mu.Lock()
	r.active++
	if r.active > r.max {
		r.max = r.active
	}
	r.order = append(r.order, req.Settings.Priority)
	r.mu.Unlock()
	if r.started != nil {
		select {
		case r.started <- req:
		case <-ctx.Done():
			r.finish()
			return nil, ctx.Err()
		}
	}
	if progress != nil {
		progress(RenderProgress{Stage: "rendering", Progress: 0.5})
	}
	if r.release != nil {
		select {
		case <-r.release:
		case <-ctx.Done():
			r.finish()
			return nil, ctx.Err()
		}
	} else {
		select {
		case <-time.After(30 * time.Millisecond):
		case <-ctx.Done():
			r.finish()
			return nil, ctx.Err()
		}
	}
	r.finish()
	return &RenderResult{}, nil
}

func (r *schedulerTestRenderer) finish() {
	r.mu.Lock()
	r.active--
	r.mu.Unlock()
}

func (r *schedulerTestRenderer) maximum() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.max
}

func (r *schedulerTestRenderer) priorities() []int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]int(nil), r.order...)
}

func renderRequestFor(user, workspace string, priority int) RenderRequest {
	req := RenderRequest{}
	if user != "" {
		req.Project.UserID = &user
	}
	req.Settings.WorkspaceID = workspace
	req.Settings.Priority = priority
	return req
}

func TestScheduledRendererBoundsConcurrency(t *testing.T) {
	delegate := &schedulerTestRenderer{}
	scheduler := NewScheduledRenderer(delegate, RenderSchedulerConfig{
		MaxConcurrent: 2, MaxPerUser: 1, MaxPerWorkspace: 2, StallTimeout: time.Second,
	})
	defer scheduler.Shutdown(context.Background())

	var wait sync.WaitGroup
	for index := 0; index < 3; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, _ = scheduler.Render(context.Background(), renderRequestFor("user", "workspace", 0), nil)
		}()
	}
	wait.Wait()
	if maximum := delegate.maximum(); maximum != 1 {
		t.Fatalf("expected per-user maximum 1, got %d", maximum)
	}
}

func TestScheduledRendererBoundsWorkspaceAcrossUsers(t *testing.T) {
	delegate := &schedulerTestRenderer{}
	scheduler := NewScheduledRenderer(delegate, RenderSchedulerConfig{
		MaxConcurrent: 3, MaxPerUser: 2, MaxPerWorkspace: 1, StallTimeout: time.Second,
	})
	defer scheduler.Shutdown(context.Background())

	var wait sync.WaitGroup
	for _, user := range []string{"one", "two", "three"} {
		wait.Add(1)
		go func(userID string) {
			defer wait.Done()
			_, _ = scheduler.Render(context.Background(), renderRequestFor(userID, "shared", 0), nil)
		}(user)
	}
	wait.Wait()
	if maximum := delegate.maximum(); maximum != 1 {
		t.Fatalf("expected per-workspace maximum 1, got %d", maximum)
	}
}

func TestScheduledRendererPrioritizesQueuedJobs(t *testing.T) {
	delegate := &schedulerTestRenderer{
		started: make(chan RenderRequest, 3),
		release: make(chan struct{}, 3),
	}
	scheduler := NewScheduledRenderer(delegate, RenderSchedulerConfig{
		MaxConcurrent: 1, MaxPerUser: 1, MaxPerWorkspace: 1, StallTimeout: time.Second,
	})
	defer scheduler.Shutdown(context.Background())

	results := make(chan error, 3)
	go func() {
		_, err := scheduler.Render(context.Background(), renderRequestFor("first", "first", 0), nil)
		results <- err
	}()
	<-delegate.started
	go func() {
		_, err := scheduler.Render(context.Background(), renderRequestFor("low", "low", -10), nil)
		results <- err
	}()
	go func() {
		_, err := scheduler.Render(context.Background(), renderRequestFor("high", "high", 10), nil)
		results <- err
	}()
	time.Sleep(20 * time.Millisecond)
	delegate.release <- struct{}{}
	second := <-delegate.started
	if second.Settings.Priority != 10 {
		t.Fatalf("expected high priority job second, got %d", second.Settings.Priority)
	}
	delegate.release <- struct{}{}
	<-delegate.started
	delegate.release <- struct{}{}
	for index := 0; index < 3; index++ {
		if err := <-results; err != nil {
			t.Fatalf("unexpected render error: %v", err)
		}
	}
	if got := delegate.priorities(); len(got) != 3 || got[1] != 10 || got[2] != -10 {
		t.Fatalf("unexpected execution order: %v", got)
	}
}

func TestScheduledRendererRemovesCancelledQueuedJob(t *testing.T) {
	delegate := &schedulerTestRenderer{started: make(chan RenderRequest, 2), release: make(chan struct{}, 1)}
	scheduler := NewScheduledRenderer(delegate, RenderSchedulerConfig{
		MaxConcurrent: 1, MaxPerUser: 1, MaxPerWorkspace: 1, StallTimeout: time.Second,
	})
	defer scheduler.Shutdown(context.Background())

	firstDone := make(chan error, 1)
	go func() {
		_, err := scheduler.Render(context.Background(), renderRequestFor("first", "first", 0), nil)
		firstDone <- err
	}()
	<-delegate.started

	ctx, cancel := context.WithCancel(context.Background())
	secondDone := make(chan error, 1)
	go func() {
		_, err := scheduler.Render(ctx, renderRequestFor("second", "second", 0), nil)
		secondDone <- err
	}()
	cancel()
	if err := <-secondDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected queued cancellation, got %v", err)
	}
	delegate.release <- struct{}{}
	if err := <-firstDone; err != nil {
		t.Fatalf("unexpected first render error: %v", err)
	}
	if got := delegate.priorities(); len(got) != 1 {
		t.Fatalf("cancelled queued job reached delegate: %v", got)
	}
}

func TestScheduledRendererShutdownCancelsActiveJob(t *testing.T) {
	delegate := &schedulerTestRenderer{started: make(chan RenderRequest, 1), release: make(chan struct{})}
	scheduler := NewScheduledRenderer(delegate, RenderSchedulerConfig{
		MaxConcurrent: 1, MaxPerUser: 1, MaxPerWorkspace: 1, StallTimeout: time.Second,
	})
	renderDone := make(chan error, 1)
	go func() {
		_, err := scheduler.Render(context.Background(), renderRequestFor("user", "workspace", 0), nil)
		renderDone <- err
	}()
	<-delegate.started

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := scheduler.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
	if err := <-renderDone; !errors.Is(err, errSchedulerClosed) {
		t.Fatalf("expected scheduler-closed render result, got %v", err)
	}
}

type stalledRenderer struct{}

func (stalledRenderer) Render(ctx context.Context, _ RenderRequest, _ func(RenderProgress)) (*RenderResult, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestScheduledRendererDetectsStall(t *testing.T) {
	scheduler := NewScheduledRenderer(stalledRenderer{}, RenderSchedulerConfig{
		MaxConcurrent: 1, MaxPerUser: 1, MaxPerWorkspace: 1, StallTimeout: 25 * time.Millisecond,
	})
	defer scheduler.Shutdown(context.Background())
	_, err := scheduler.Render(context.Background(), renderRequestFor("user", "workspace", 0), nil)
	if err == nil || !strings.Contains(err.Error(), "stalled") {
		t.Fatalf("expected stall error, got %v", err)
	}
}
