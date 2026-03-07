package api

import "testing"

func TestNormalizeMIMEType(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain", in: "application/pdf", want: "application/pdf"},
		{name: "with params", in: "application/pdf; charset=binary", want: "application/pdf"},
		{name: "mixed case", in: " Application/PDF ", want: "application/pdf"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeMIMEType(tt.in)
			if got != tt.want {
				t.Fatalf("normalizeMIMEType(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCanExtractAttachmentText(t *testing.T) {
	tests := []struct {
		name string
		mime string
		want bool
	}{
		{name: "text plain", mime: "text/plain", want: true},
		{name: "json", mime: "application/json", want: true},
		{name: "pdf", mime: "application/pdf", want: true},
		{name: "pdf with params", mime: "application/pdf; charset=binary", want: true},
		{name: "image", mime: "image/png", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := canExtractAttachmentText(tt.mime)
			if got != tt.want {
				t.Fatalf("canExtractAttachmentText(%q) = %v, want %v", tt.mime, got, tt.want)
			}
		})
	}
}
