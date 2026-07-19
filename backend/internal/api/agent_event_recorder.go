package api

import (
	"context"
	"encoding/json"
	"log"

	"github.com/ajbergh/omnillm-studio/internal/agent"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// AgentEventRecorder buffers append-only run events so live SSE writes never
// block on SQLite contention.
type AgentEventRecorder struct {
	repo   *repository.AgentEventRepo
	events chan agent.Event
	cancel context.CancelFunc
	done   chan struct{}
}

func NewAgentEventRecorder(repo *repository.AgentEventRepo) *AgentEventRecorder {
	ctx, cancel := context.WithCancel(context.Background())
	recorder := &AgentEventRecorder{
		repo: repo, events: make(chan agent.Event, 1024), cancel: cancel, done: make(chan struct{}),
	}
	go recorder.loop(ctx)
	return recorder
}

func (r *AgentEventRecorder) Record(event agent.Event) {
	select {
	case r.events <- event:
	default:
		log.Printf("WARN: agent event audit buffer full; dropping %s for run %s", event.Type, event.RunID)
	}
}

func (r *AgentEventRecorder) Shutdown(ctx context.Context) error {
	r.cancel()
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *AgentEventRecorder) loop(ctx context.Context) {
	defer close(r.done)
	for {
		select {
		case event := <-r.events:
			r.persist(event)
		case <-ctx.Done():
			for {
				select {
				case event := <-r.events:
					r.persist(event)
				default:
					return
				}
			}
		}
	}
}

func (r *AgentEventRecorder) persist(event agent.Event) {
	data, err := json.Marshal(event.Data)
	if err != nil {
		data = []byte(`{"error":"event data could not be encoded"}`)
	}
	if _, err := r.repo.Append(event.RunID, event.StepID, string(event.Type), data); err != nil {
		log.Printf("WARN: persist agent event: %v", err)
	}
}
