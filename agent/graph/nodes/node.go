package nodes

import (
	"context"

	"github.com/agent-pilot/agent-pilot-be/agent/scheduler"
	atype "github.com/agent-pilot/agent-pilot-be/agent/type"
)

type Node interface {
	Invoke(ctx context.Context, state *State) (*State, error)
}

type State struct {
	Request  atype.Request
	Decision *scheduler.Decision
	Plan     *atype.Plan
	Result   *atype.Result
}
