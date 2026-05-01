package scheduler

import (
	"context"
	"errors"

	"github.com/agent-pilot/agent-pilot-be/agent/memory"
	atype "github.com/agent-pilot/agent-pilot-be/agent/type"
)

type Action string

const (
	ActionPlan     Action = "plan"
	ActionExecute  Action = "execute"
	ActionResume   Action = "resume"
	ActionComplete Action = "complete"
	ActionPause    Action = "pause"
)

type Decision struct {
	Action     Action
	Session    *atype.Session
	Plan       *atype.Plan
	Step       *atype.Step
	Checkpoint *atype.Checkpoint
}

type Scheduler struct {
	memory memory.MemoryService
}

func NewScheduler(mem memory.MemoryService) *Scheduler {
	return &Scheduler{
		memory: mem,
	}
}

func (s *Scheduler) Decide(ctx context.Context, sessionID string) (*Decision, error) {
	//1.获取session
	session, err := s.memory.GetChatSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	//2.获取当前plan
	plan, ok, err := s.memory.GetActivePlan(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	//如果当前无plan || 上一个plan已完成->走planner
	if !ok || plan == nil || plan.Status == atype.StatusCompleted {
		return &Decision{
			Action:  ActionPlan,
			Session: &session,
		}, nil
	}

	//plan是paused->resume
	if plan.Status == atype.StatusPaused {
		if plan.Checkpoint == nil {
			return nil, errors.New("plan paused but checkpoint missing")
		}

		step := s.findStep(plan.Steps, plan.Checkpoint.StepID)
		if step == nil {
			return nil, errors.New("checkpoint step not found")
		}

		return &Decision{
			Action:     ActionResume,
			Session:    &session,
			Plan:       plan,
			Step:       step,
			Checkpoint: plan.Checkpoint,
		}, nil
	}

	//plan是executing/ready->找下一个step
	step := s.findNextStep(plan)
	if step == nil {
		return nil, errors.New("no executable step exists")
	}

	return &Decision{
		Action:  ActionExecute,
		Session: &session,
		Plan:    plan,
		Step:    step,
	}, nil
}

func (s *Scheduler) findStep(steps []atype.Step, stepID string) *atype.Step {
	for i := range steps {
		if steps[i].ID == stepID {
			return &steps[i]
		}
	}
	return nil
}

// step按顺序执行，找第一个pending step
func (s *Scheduler) findNextStep(plan *atype.Plan) *atype.Step {
	for i := range plan.Steps {
		st := &plan.Steps[i]
		if st.Status == atype.StepStatusPending {
			return st
		}
	}
	return nil
}
