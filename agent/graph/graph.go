package graph

import (
	"context"
	"fmt"
	"time"

	actx "github.com/agent-pilot/agent-pilot-be/agent/context"
	"github.com/agent-pilot/agent-pilot-be/agent/memory"
	"github.com/agent-pilot/agent-pilot-be/agent/plan"
	"github.com/agent-pilot/agent-pilot-be/agent/react"
	"github.com/agent-pilot/agent-pilot-be/agent/scheduler"
	atype "github.com/agent-pilot/agent-pilot-be/agent/type"
	"github.com/cloudwego/eino/compose"
	"github.com/google/uuid"
)

type State struct {
	Request  atype.Request
	Decision *scheduler.Decision
	Plan     *atype.Plan
	Result   *atype.Result
}

type AgentGraph struct {
	memory    memory.MemoryService
	scheduler *scheduler.Scheduler
	planner   plan.Planner
	executor  *react.Executor
	ctxb      *actx.Builder
}

func NewAgentGraph(
	mem memory.MemoryService,
	sch *scheduler.Scheduler,
	planner plan.Planner,
	executor *react.Executor,
) *AgentGraph {
	return &AgentGraph{
		memory:    mem,
		scheduler: sch,
		planner:   planner,
		executor:  executor,
		ctxb:      actx.NewBuilder(actx.Options{MaxPlans: 5, DropEmptyContent: true}),
	}

}

func (ag *AgentGraph) BuildGraph(opts ...compose.GraphCompileOption) (compose.Runnable[atype.Request, *atype.Result], error) {
	g := compose.NewGraph[atype.Request, *atype.Result]()
	//scheduler
	err := g.AddLambdaNode(
		"scheduler",
		compose.InvokableLambda(func(ctx context.Context, req any) (*State, error) {
			switch v := req.(type) {
			case atype.Request:
				return ag.SchedulerInvoke(ctx, v)
			case *State:
				return ag.SchedulerReEnter(ctx, v)
			default:
				return nil, fmt.Errorf("invalid scheduler input")
			}
		}),
		compose.WithNodeName("Scheduler"),
	)
	if err != nil {
		return nil, err
	}

	//planner
	err = g.AddLambdaNode(
		"planner",
		compose.InvokableLambda(
			func(ctx context.Context, state *State) (*State, error) {
				return ag.PlannerInvoke(ctx, state)
			},
		),
		compose.WithNodeName("Planner"),
	)
	if err != nil {
		return nil, err
	}

	//executor
	err = g.AddLambdaNode(
		"executor",
		compose.InvokableLambda(
			func(ctx context.Context, state *State) (*State, error) {
				return ag.ExecutorInvoke(ctx, state)
			},
		),
		compose.WithNodeName("Executor"),
	)
	if err != nil {
		return nil, err
	}

	//add edges
	if err := g.AddEdge(compose.START, "scheduler"); err != nil {
		return nil, err
	}

	if err := g.AddBranch(
		"scheduler",
		compose.NewGraphBranch(
			func(ctx context.Context, state *State) (string, error) {
				if state == nil || state.Decision == nil {
					return "end", nil
				}
				switch state.Decision.Action {
				case scheduler.ActionPlan:
					return "planner", nil
				case scheduler.ActionExecute:
					return "executor", nil
				case scheduler.ActionResume:
					return "executor", nil
				default:
					return compose.END, nil
				}
			},
			map[string]bool{
				"planner":  true,
				"executor": true,
				"end":      true,
			},
		),
	); err != nil {
		return nil, err
	}

	// planner -> executor
	if err := g.AddEdge("planner", "executor"); err != nil {
		return nil, err
	}
	// executor -> scheduler
	err = g.AddEdge("executor", "scheduler")

	runnable, err := g.Compile(context.Background(), opts...)

	if err != nil {
		return nil, fmt.Errorf(
			"compile graph failed: %w",
			err,
		)
	}
	return runnable, nil

}

func (ag *AgentGraph) SchedulerInvoke(ctx context.Context, req atype.Request) (*State, error) {
	decision, err := ag.scheduler.Decide(ctx, req.SessionID)
	if err != nil {
		return nil, err
	}
	return &State{
		Request:  req,
		Decision: decision,
	}, nil
}

func (ag *AgentGraph) SchedulerReEnter(ctx context.Context, state *State) (*State, error) {
	if state == nil {
		return nil, fmt.Errorf("state nil")
	}
	decision, err := ag.scheduler.Decide(
		ctx,
		state.Request.SessionID,
	)
	if err != nil {
		return nil, err
	}
	state.Decision = decision
	return state, nil
}

func (ag *AgentGraph) PlannerInvoke(ctx context.Context, state *State) (*State, error) {
	if state.Decision.Action != scheduler.ActionPlan {
		return state, nil
	}

	var plans []atype.Plan
	plans, err := ag.memory.ListSessionPlans(ctx, state.Request.SessionID, 5)
	if err != nil {
		return nil, err
	}

	ctxMsgs, err := ag.ctxb.BuildPlanContext(state.Request, plans)
	if err != nil {
		return nil, err
	}

	req := state.Request
	req.History = ctxMsgs

	p, err := ag.memory.CreatePlan(ctx, req.SessionID, req.UserInput)
	if err != nil {
		return nil, err
	}

	newPlan, err := ag.planner.Plan(ctx, req)
	if err != nil {
		return nil, err
	}
	p.Goal = newPlan.Goal
	p.Steps = newPlan.Steps
	p.Status = atype.StatusReady

	if err := ag.memory.SavePlan(ctx, p); err != nil {
		return nil, err
	}

	state.Plan = p
	return state, nil
}

func (ag *AgentGraph) ExecutorInvoke(ctx context.Context, state *State) (*State, error) {
	decision := state.Decision

	if decision == nil {
		return nil, fmt.Errorf("decision nil")
	}
	if decision.Action != scheduler.ActionExecute &&
		decision.Action != scheduler.ActionResume {

		if state.Plan != nil {
			state.Result = &atype.Result{Plan: state.Plan, Steps: nil, Summary: ""}
			return state, nil
		}
		return nil, nil
	}

	p := decision.Plan
	if p == nil {
		return nil, fmt.Errorf("plan nil")
	}

	switch decision.Action {
	case scheduler.ActionResume:
		// 恢复 plan
		if err := ag.memory.ResumePlan(ctx, p.ID); err != nil {
			return nil, err
		}
	case scheduler.ActionExecute:
		// 开始下一步
		step := decision.Step
		if step == nil {
			return nil, fmt.Errorf("no executable step")
		}
		if err := ag.memory.StartStep(ctx, p.ID, step.ID); err != nil {
			return nil, err
		}
	}

	//构建执行上下文
	exeCtx, err := ag.memory.BuildExecutionContext(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	step := exeCtx.Step
	if step == nil {
		return nil, fmt.Errorf("execution step is nil")
	}

	//执行step
	stepResult, err := ag.executor.ExecuteStep(ctx, exeCtx)
	//更新失败状态
	if err != nil {
		_ = ag.memory.UpdatePlanStatus(ctx, p.ID, atype.StatusFailed)
		_ = ag.memory.FailStep(ctx, p.ID, step.ID)
		return nil, err
	}

	//持久化消息
	if len(stepResult.Messages) > 0 {
		err = ag.memory.AppendMessage(ctx, stepResult.Messages)
		if err != nil {
			return nil, err
		}
	}

	if stepResult.Paused {
		// 保存 checkpoint
		checkpoint := &atype.Checkpoint{
			ID:        uuid.NewString(),
			StepID:    step.ID,
			Question:  stepResult.Output,
			CreatedAt: time.Now(),
		}
		p.Checkpoint = checkpoint
		err := ag.memory.SavePlan(ctx, p)
		if err != nil {
			return nil, err
		}

		// 更新状态
		err = ag.memory.UpdatePlanStatus(
			ctx,
			p.ID,
			atype.StatusPaused,
		)
		if err != nil {
			return nil, err
		}
	}

	if stepResult.Completed {
		err = ag.memory.CompleteStep(ctx, p.ID, step.ID, stepResult.Output)
		if err != nil {
			return nil, err
		}
	}
	state.Result = &atype.Result{
		Plan: p,
		Steps: []atype.StepResult{
			*stepResult,
		},
		Summary: stepResult.Output,
	}
	return state, nil

}
