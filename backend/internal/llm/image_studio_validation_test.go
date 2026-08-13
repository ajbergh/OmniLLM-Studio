package llm

import "testing"

func TestValidateImageStudioRequestCapabilities(t *testing.T) {
	seed := 4
	guidance := 3.5
	caps := ImageCapabilities{
		SupportsGeneration:       true,
		SupportsContentReference: true,
		SupportsStyleReference:   true,
		SupportsSeed:             true,
		SupportsGuidance:         true,
		MaxReferenceImages:       2,
		MaxVariants:              2,
	}
	req := ImageStudioRequest{
		ImageRequest: ImageRequest{
			N:                    2,
			ReferenceImages:      []ReferenceImage{{Data: "a"}},
			StyleReferenceImages: []ReferenceImage{{Data: "b"}},
		},
		Seed:     &seed,
		Guidance: &guidance,
	}
	if err := validateImageStudioRequestCapabilities(caps, req); err != nil {
		t.Fatalf("expected request to pass: %v", err)
	}
}

func TestValidateImageStudioRequestCapabilitiesRejectsVariantOverflow(t *testing.T) {
	caps := ImageCapabilities{MaxVariants: 1}
	req := ImageStudioRequest{ImageRequest: ImageRequest{N: 2}}
	if err := validateImageStudioRequestCapabilities(caps, req); err == nil {
		t.Fatal("expected variant overflow to fail")
	}
}

func TestValidateImageStudioRequestCapabilitiesRejectsReferenceOverflow(t *testing.T) {
	caps := ImageCapabilities{SupportsContentReference: true, MaxReferenceImages: 1, MaxVariants: 1}
	req := ImageStudioRequest{ImageRequest: ImageRequest{ReferenceImages: []ReferenceImage{{Data: "a"}, {Data: "b"}}}}
	if err := validateImageStudioRequestCapabilities(caps, req); err == nil {
		t.Fatal("expected reference overflow to fail")
	}
}

func TestValidateImageStudioRequestCapabilitiesDoesNotCountEditBaseAsContentReference(t *testing.T) {
	caps := ImageCapabilities{SupportsEditing: true, MaxVariants: 1}
	req := ImageStudioRequest{ImageRequest: ImageRequest{OperationType: "edit", ReferenceImage: &ReferenceImage{Data: "base"}}}
	if err := validateImageStudioRequestCapabilities(caps, req); err != nil {
		t.Fatalf("edit base image must not consume reference capability: %v", err)
	}
}

func TestValidateImageStudioRequestCapabilitiesRejectsUnsupportedControls(t *testing.T) {
	seed := 4
	caps := ImageCapabilities{MaxVariants: 1}
	req := ImageStudioRequest{ImageRequest: ImageRequest{}, Seed: &seed}
	if err := validateImageStudioRequestCapabilities(caps, req); err == nil {
		t.Fatal("expected unsupported seed to fail")
	}
}
