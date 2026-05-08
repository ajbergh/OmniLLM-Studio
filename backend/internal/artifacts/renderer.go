package artifacts

import (
	"context"
	"fmt"
)

// Registry maps ArtifactFormat values to their ArtifactRenderer implementations.
type Registry struct {
	renderers map[ArtifactFormat]ArtifactRenderer
}

// NewRegistry returns a Registry pre-populated with all built-in renderers.
func NewRegistry() *Registry {
	reg := &Registry{renderers: make(map[ArtifactFormat]ArtifactRenderer)}
	reg.Register(&MarkdownRenderer{})
	reg.Register(&HTMLRenderer{})
	reg.Register(&JSONRenderer{})
	reg.Register(&YAMLRenderer{})
	reg.Register(&CSVRenderer{})
	reg.Register(&XLSXRenderer{})
	reg.Register(&PDFRenderer{})
	return reg
}

// Register adds (or replaces) a renderer for its self-reported format.
func (reg *Registry) Register(r ArtifactRenderer) {
	reg.renderers[r.Format()] = r
}

// Render finds the renderer for format and calls it.
func (reg *Registry) Render(ctx context.Context, format ArtifactFormat, artifact Artifact) (*RenderedArtifact, error) {
	r, ok := reg.renderers[format]
	if !ok {
		return nil, fmt.Errorf("no renderer registered for format %q", format)
	}
	return r.Render(ctx, artifact)
}
