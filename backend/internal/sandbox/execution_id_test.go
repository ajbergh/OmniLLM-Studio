package sandbox

import (
	"strings"
	"testing"
)

func TestExecutionIDCanonicalGenerationAndPreservation(t *testing.T) {
	generated := NewExecutionID()
	if err := validateExecutionID(generated); err != nil {
		t.Fatalf("generated execution id %q: %v", generated, err)
	}
	resolved, err := executionIDOrNew(generated)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != generated {
		t.Fatalf("resolved execution id = %q, want %q", resolved, generated)
	}
	allocated, err := executionIDOrNew("")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateExecutionID(allocated); err != nil {
		t.Fatalf("allocated execution id %q: %v", allocated, err)
	}
}

func TestExecutionIDRejectsNonCanonicalValues(t *testing.T) {
	canonical := NewExecutionID()
	for _, value := range []string{
		"",
		"exec-1",
		"exec_not-a-uuid",
		" " + canonical,
		canonical + " ",
		strings.ToUpper(canonical),
	} {
		if err := validateExecutionID(value); err == nil {
			t.Fatalf("validateExecutionID(%q) unexpectedly succeeded", value)
		}
	}
}
