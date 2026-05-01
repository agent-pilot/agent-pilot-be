package nodes

import (
	"context"

	actx "github.com/agent-pilot/agent-pilot-be/agent/context"
	"github.com/agent-pilot/agent-pilot-be/agent/memory"
	"github.com/agent-pilot/agent-pilot-be/agent/plan"
	"github.com/agent-pilot/agent-pilot-be/agent/scheduler"
	atype "github.com/agent-pilot/agent-pilot-be/agent/type"
)

type PlannerNode struct {
	memory  memory.MemoryService
	planner plan.Planner
	ctxb    *actx.Builder
}

func NewPlannerNode(memory memory.MemoryService, planner plan.Planner, ctxb *actx.Builder) Node {
	return &PlannerNode{
		memory:  memory,
		planner: planner,
		ctxb:    ctxb,
	}
}

func (n *PlannerNode) Invoke(ctx context.Context, state *State) (*State, error) {
	if state.Decision.Action != scheduler.ActionPlan {
		return state, nil
	}

	var plans []atype.Plan
	plans, err := n.memory.ListSessionPlans(ctx, state.Request.SessionID, 5)
	if err != nil {
		return nil, err
	}

	ctxMsgs, err := n.ctxb.BuildPlanContext(state.Request, plans)
	if err != nil {
		return nil, err
	}

	req := state.Request
	req.History = ctxMsgs

	p, err := n.memory.CreatePlan(ctx, req.SessionID, req.UserInput)
	if err != nil {
		return nil, err
	}

	newPlan, err := n.planner.Plan(ctx, req)
	if err != nil {
		return nil, err
	}
	p.Goal = newPlan.Goal
	p.Steps = newPlan.Steps
	p.Status = atype.StatusReady

	if err := n.memory.SavePlan(ctx, p); err != nil {
		return nil, err
	}

	state.Plan = p
	state.Decision = &scheduler.Decision{
		Action:  scheduler.ActionExecute,
		Session: state.Decision.Session,
		Plan:    p,
		Step:    &p.Steps[0],
	}
	return state, nil
}
