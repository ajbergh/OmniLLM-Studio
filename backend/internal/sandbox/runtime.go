package sandbox

import "context"

// Runtime is the trusted execution-plane contract behind Broker. Implementations
// may be an OS-native local sandbox, an authenticated remote worker, or a
// compatibility runtime used only where policy permits it.
type Runtime interface {
	Capabilities() RuntimeCapabilities
	Create(context.Context, RuntimeCreateRequest) (string, error)
	Exec(context.Context, string, ExecRequest) (*ExecResult, error)
	Cancel(context.Context, string, string) error
	Status(context.Context, string) (*Status, error)
	Destroy(context.Context, string) error
}
