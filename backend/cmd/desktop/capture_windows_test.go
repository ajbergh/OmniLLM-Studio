//go:build windows

package main

import (
	"reflect"
	"testing"
)

func TestParseDShowDevices(t *testing.T) {
	fixture := []byte(`
[dshow @ 000] DirectShow video devices (some may be both video and audio devices)
[dshow @ 000]  "Integrated Camera"
[dshow @ 000]     Alternative name "@device_pnp_camera"
[dshow @ 000]  "Integrated Camera"
[dshow @ 000] DirectShow audio devices
[dshow @ 000]  "Microphone Array"
[dshow @ 000]     Alternative name "@device_cm_audio"
[dshow @ 000]  "Stereo Mix"
`)
	audio, video := parseDShowDevices(fixture)
	if !reflect.DeepEqual(video, []string{"Integrated Camera"}) {
		t.Fatalf("unexpected video devices: %v", video)
	}
	if !reflect.DeepEqual(audio, []string{"Microphone Array", "Stereo Mix"}) {
		t.Fatalf("unexpected audio devices: %v", audio)
	}
}

func TestUniqueStringsTrimsSortsAndDeduplicates(t *testing.T) {
	got := uniqueStrings([]string{" B ", "A", "B", "", "A"})
	want := []string{"A", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
