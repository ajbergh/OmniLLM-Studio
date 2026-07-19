package agent

import "testing"

func TestEmitPublishesWithoutLiveCallback(t *testing.T) {
	var recorded []Event
	SetGlobalEventSink(func(event Event) { recorded = append(recorded, event) })
	defer SetGlobalEventSink(nil)

	emit(nil, Event{Type: EventCheckpoint, RunID: "run-1"})
	if len(recorded) != 1 || recorded[0].RunID != "run-1" {
		t.Fatalf("expected one globally published event, got %+v", recorded)
	}
}
