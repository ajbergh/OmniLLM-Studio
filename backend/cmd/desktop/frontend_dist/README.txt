This placeholder keeps the go:embed target valid for go build, go vet, CodeQL,
and other source-only checks.

Wails release and development scripts replace this directory with the compiled
frontend/dist contents before producing a desktop binary.
