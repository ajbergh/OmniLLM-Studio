package video

import (
	"context"
	"sync"
	"testing"
	"time"
)

type schedulerTestRenderer struct {
	mu          sync.Mutex
	active, max int
}

func (r *schedulerTestRenderer) Render(ctx context.Context, req RenderRequest, progress func(RenderProgress)) (*RenderResult, error) {
	r.mu.Lock()
	r.active++
	if r.active > r.max {
		r.max = r.active
	}
	r.mu.Unlock()
	select {
	case <-time.After(30 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	r.mu.Lock()
	r.active--
	r.mu.Unlock()
	return &RenderResult{}, nil
}
func TestScheduledRendererBoundsConcurrency(t *testing.T) {
	delegate := &schedulerTestRenderer{}
	scheduler := NewScheduledRenderer(delegate, RenderSchedulerConfig{MaxConcurrent: 2, MaxPerUser: 1, MaxPerWorkspace: 2, StallTimeout: time.Second})
	defer scheduler.Shutdown(context.Background())
	user := "u"
	req := RenderRequest{}
	req.Project.UserID = &user
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = scheduler.Render(context.Background(), req, nil) }()
	}
	wg.Wait()
	if delegate.max != 1 {
		t.Fatalf("expected per-user max 1, got %d", delegate.max)
	}
}
