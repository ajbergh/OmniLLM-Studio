package video

import (
	"container/heap"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// RenderSchedulerConfig controls global and scoped FFmpeg admission.
type RenderSchedulerConfig struct {
	MaxConcurrent   int
	MaxPerUser      int
	MaxPerWorkspace int
	StallTimeout    time.Duration
	MinFreeBytes    uint64
	TempMaxAge      time.Duration
}

// RenderSchedulerConfigFromEnv returns safe local-first defaults. Desktop runs
// one FFmpeg job at a time unless explicitly configured otherwise.
func RenderSchedulerConfigFromEnv() RenderSchedulerConfig {
	return RenderSchedulerConfig{
		MaxConcurrent:   envInt("OMNILLM_VIDEO_RENDER_CONCURRENCY", 1, 1, 16),
		MaxPerUser:      envInt("OMNILLM_VIDEO_RENDER_PER_USER", 1, 1, 8),
		MaxPerWorkspace: envInt("OMNILLM_VIDEO_RENDER_PER_WORKSPACE", 1, 1, 8),
		StallTimeout:    time.Duration(envInt("OMNILLM_VIDEO_RENDER_STALL_SECONDS", 300, 30, 3600)) * time.Second,
		MinFreeBytes:    uint64(envInt64("OMNILLM_VIDEO_RENDER_MIN_FREE_BYTES", 512*1024*1024, 0, 1<<50)),
		TempMaxAge:      time.Duration(envInt("OMNILLM_VIDEO_RENDER_TEMP_MAX_HOURS", 24, 1, 720)) * time.Hour,
	}
}

func envInt(name string, fallback, min, max int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil {
		value = fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func envInt64(name string, fallback, min, max int64) int64 {
	value, err := strconv.ParseInt(strings.TrimSpace(os.Getenv(name)), 10, 64)
	if err != nil {
		value = fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

type scheduledRenderResult struct {
	result *RenderResult
	err    error
}
type scheduledRender struct {
	id          uint64
	ctx         context.Context
	req         RenderRequest
	progress    func(RenderProgress)
	result      chan scheduledRenderResult
	priority    int
	userID      string
	workspaceID string
	enqueuedAt  time.Time
	index       int
}

type renderPriorityQueue []*scheduledRender

func (q renderPriorityQueue) Len() int { return len(q) }
func (q renderPriorityQueue) Less(i, j int) bool {
	if q[i].priority == q[j].priority {
		return q[i].id < q[j].id
	}
	return q[i].priority > q[j].priority
}
func (q renderPriorityQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i]; q[i].index = i; q[j].index = j }
func (q *renderPriorityQueue) Push(x any) {
	item := x.(*scheduledRender)
	item.index = len(*q)
	*q = append(*q, item)
}
func (q *renderPriorityQueue) Pop() any {
	old := *q
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*q = old[:n-1]
	return item
}

// ScheduledRenderer applies bounded, priority-aware admission around another renderer.
type ScheduledRenderer struct {
	delegate          Renderer
	config            RenderSchedulerConfig
	mu                sync.Mutex
	cond              *sync.Cond
	queue             renderPriorityQueue
	active            int
	activeByUser      map[string]int
	activeByWorkspace map[string]int
	sequence          atomic.Uint64
	closed            bool
	workers           sync.WaitGroup
}

// NewScheduledRenderer starts a bounded render scheduler.
func NewScheduledRenderer(delegate Renderer, config RenderSchedulerConfig) *ScheduledRenderer {
	if config.MaxConcurrent <= 0 {
		config = RenderSchedulerConfigFromEnv()
	}
	scheduler := &ScheduledRenderer{
		delegate: delegate, config: config,
		activeByUser: map[string]int{}, activeByWorkspace: map[string]int{},
	}
	scheduler.cond = sync.NewCond(&scheduler.mu)
	heap.Init(&scheduler.queue)
	cleanupStaleRenderTemps(config.TempMaxAge)
	for i := 0; i < config.MaxConcurrent; i++ {
		scheduler.workers.Add(1)
		go scheduler.worker()
	}
	return scheduler
}

// NewProductionRenderer composes fidelity expansion, scheduling, preflight,
// cancellation, and stall detection around the FFmpeg renderer.
func NewProductionRenderer(binary string) Renderer {
	base := NewFFmpegRenderer(binary)
	return NewScheduledRenderer(NewFidelityRenderer(base), RenderSchedulerConfigFromEnv())
}

// Render enqueues one render and waits for completion or cancellation.
func (s *ScheduledRenderer) Render(ctx context.Context, req RenderRequest, progress func(RenderProgress)) (*RenderResult, error) {
	if s == nil || s.delegate == nil {
		return nil, fmt.Errorf("render scheduler is not configured")
	}
	if err := renderDiskPreflight(req, s.config.MinFreeBytes); err != nil {
		return nil, err
	}
	item := &scheduledRender{
		id: s.sequence.Add(1), ctx: ctx, req: req, progress: progress,
		result: make(chan scheduledRenderResult, 1), priority: req.Settings.Priority,
		userID: renderUserID(req), workspaceID: renderWorkspaceID(req), enqueuedAt: time.Now(),
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, fmt.Errorf("render scheduler is shutting down")
	}
	heap.Push(&s.queue, item)
	position := s.queue.Len()
	s.cond.Broadcast()
	s.mu.Unlock()
	if progress != nil {
		progress(RenderProgress{Stage: "queued", Message: fmt.Sprintf("Queued for renderer (position %d)", position), Progress: 0.01})
	}
	select {
	case outcome := <-item.result:
		return outcome.result, outcome.err
	case <-ctx.Done():
		s.mu.Lock()
		s.cond.Broadcast()
		s.mu.Unlock()
		return nil, ctx.Err()
	}
}

func (s *ScheduledRenderer) worker() {
	defer s.workers.Done()
	for {
		s.mu.Lock()
		item := s.nextEligibleLocked()
		for item == nil && !s.closed {
			s.cond.Wait()
			item = s.nextEligibleLocked()
		}
		if item == nil && s.closed {
			s.mu.Unlock()
			return
		}
		s.active++
		if item.userID != "" {
			s.activeByUser[item.userID]++
		}
		if item.workspaceID != "" {
			s.activeByWorkspace[item.workspaceID]++
		}
		s.mu.Unlock()

		result, err := s.execute(item)
		select {
		case item.result <- scheduledRenderResult{result: result, err: err}:
		default:
		}

		s.mu.Lock()
		s.active--
		if item.userID != "" {
			s.activeByUser[item.userID]--
			if s.activeByUser[item.userID] <= 0 {
				delete(s.activeByUser, item.userID)
			}
		}
		if item.workspaceID != "" {
			s.activeByWorkspace[item.workspaceID]--
			if s.activeByWorkspace[item.workspaceID] <= 0 {
				delete(s.activeByWorkspace, item.workspaceID)
			}
		}
		s.cond.Broadcast()
		s.mu.Unlock()
	}
}

func (s *ScheduledRenderer) nextEligibleLocked() *scheduledRender {
	if len(s.queue) == 0 {
		return nil
	}
	deferred := make([]*scheduledRender, 0)
	var selected *scheduledRender
	for s.queue.Len() > 0 {
		candidate := heap.Pop(&s.queue).(*scheduledRender)
		if candidate.ctx.Err() != nil {
			select {
			case candidate.result <- scheduledRenderResult{err: candidate.ctx.Err()}:
			default:
			}
			continue
		}
		userAllowed := candidate.userID == "" || s.activeByUser[candidate.userID] < s.config.MaxPerUser
		workspaceAllowed := candidate.workspaceID == "" || s.activeByWorkspace[candidate.workspaceID] < s.config.MaxPerWorkspace
		if userAllowed && workspaceAllowed {
			selected = candidate
			break
		}
		deferred = append(deferred, candidate)
	}
	for _, candidate := range deferred {
		heap.Push(&s.queue, candidate)
	}
	return selected
}

func (s *ScheduledRenderer) execute(item *scheduledRender) (*RenderResult, error) {
	ctx, cancel := context.WithCancel(item.ctx)
	defer cancel()
	var lastProgress atomic.Int64
	lastProgress.Store(time.Now().UnixNano())
	progress := func(update RenderProgress) {
		lastProgress.Store(time.Now().UnixNano())
		if item.progress != nil {
			item.progress(update)
		}
	}
	if progress != nil {
		progress(RenderProgress{Stage: "preflight", Message: "Renderer slot acquired", Progress: 0.03})
	}
	resultCh := make(chan scheduledRenderResult, 1)
	go func() {
		result, err := s.delegate.Render(ctx, item.req, progress)
		resultCh <- scheduledRenderResult{result: result, err: err}
	}()
	ticker := time.NewTicker(minDuration(10*time.Second, s.config.StallTimeout/4))
	defer ticker.Stop()
	for {
		select {
		case result := <-resultCh:
			return result.result, result.err
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if s.config.StallTimeout > 0 && time.Since(time.Unix(0, lastProgress.Load())) > s.config.StallTimeout {
				cancel()
				return nil, fmt.Errorf("render stalled for more than %s", s.config.StallTimeout)
			}
		}
	}
}

func minDuration(a, b time.Duration) time.Duration {
	if b <= 0 || a < b {
		return a
	}
	return b
}

// Shutdown stops admission, cancels queued waits, and waits for active workers.
func (s *ScheduledRenderer) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		for s.queue.Len() > 0 {
			item := heap.Pop(&s.queue).(*scheduledRender)
			select {
			case item.result <- scheduledRenderResult{err: context.Canceled}:
			default:
			}
		}
		s.cond.Broadcast()
	}
	s.mu.Unlock()
	done := make(chan struct{})
	go func() { s.workers.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Shutdown drains the active video renderer during application shutdown.
func (s *Service) Shutdown(ctx context.Context) error {
	if shutdowner, ok := s.renderer.(interface{ Shutdown(context.Context) error }); ok {
		return shutdowner.Shutdown(ctx)
	}
	return nil
}

func renderUserID(req RenderRequest) string {
	if req.Project.UserID != nil {
		return strings.TrimSpace(*req.Project.UserID)
	}
	return ""
}
func renderWorkspaceID(req RenderRequest) string {
	if strings.TrimSpace(req.Settings.WorkspaceID) != "" {
		return strings.TrimSpace(req.Settings.WorkspaceID)
	}
	var metadata map[string]any
	if json.Unmarshal([]byte(req.Project.MetadataJSON), &metadata) == nil {
		if value, ok := metadata["workspace_id"].(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func renderDiskPreflight(req RenderRequest, minFree uint64) error {
	if minFree == 0 {
		return nil
	}
	free, err := diskFreeBytes(os.TempDir())
	if err != nil {
		return nil
	}
	estimate := uint64(maxInt(1, req.Timeline.Canvas.Width)*maxInt(1, req.Timeline.Canvas.Height)) * 4
	seconds := uint64(req.Timeline.DurationMS / 1000)
	if seconds < 1 {
		seconds = 1
	}
	estimate += seconds * 2 * 1024 * 1024
	required := minFree + estimate
	if free < required {
		return fmt.Errorf("insufficient temporary disk space: %d MiB free, %d MiB required", free/(1024*1024), required/(1024*1024))
	}
	return nil
}

func cleanupStaleRenderTemps(maxAge time.Duration) {
	if maxAge <= 0 {
		return
	}
	entries, _ := os.ReadDir(os.TempDir())
	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "omnillm-video-render-") {
			continue
		}
		path := filepath.Join(os.TempDir(), entry.Name())
		info, err := entry.Info()
		if err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(path)
		}
	}
}

var errSchedulerClosed = errors.New("render scheduler closed")
