package runtime

import (
	"context"
	"fmt"
)

// Runtime is the minimal execution context for the agent.
// For now it only loads and exposes Lark skills as "tools".
type Runtime struct {
	Skills *Registry
}

func NewRuntime(ctx context.Context) *Runtime {
	reg := NewRegistry(ctx)

	return &Runtime{Skills: reg}
}

func (rt *Runtime) DebugSummary() string {
	if rt == nil || rt.Skills == nil {
		return "no runtime"
	}
	return fmt.Sprintf("skills=%d", len(rt.Skills.Skills))
}
