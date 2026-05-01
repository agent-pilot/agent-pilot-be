package nodes

import (
	"context"

	"github.com/agent-pilot/agent-pilot-be/agent/scheduler"
)

type SchedulerNode struct {
	scheduler *scheduler.Scheduler
}

func NewSchedulerNode(s *scheduler.Scheduler) Node {
	return &SchedulerNode{
		scheduler: s,
	}
}

func (n *SchedulerNode) Invoke(ctx context.Context, state *State) (*State, error) {
	decision := state.Decision
	if decision == nil {
		d, err := n.scheduler.Decide(
			ctx,
			state.Request.SessionID,
		)
		if err != nil {
			return nil, err
		}

		decision = d
	}
	state.Decision = decision
	return state, nil
}
