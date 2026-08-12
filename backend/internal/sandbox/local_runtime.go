package sandbox

// LocalRuntimeConfig configures the first-party local sandbox runtime. Platform
// implementations may require only a subset of fields.
type LocalRuntimeConfig struct {
	RootFS      string
	ScratchRoot string
	BwrapPath   string
}
