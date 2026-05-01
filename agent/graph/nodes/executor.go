package nodes

import (
	"context"
	"fmt"
	"time"

	"github.com/agent-pilot/agent-pilot-be/agent/memory"
	"github.com/agent-pilot/agent-pilot-be/agent/react"
	"github.com/agent-pilot/agent-pilot-be/agent/scheduler"
	atype "github.com/agent-pilot/agent-pilot-be/agent/type"
	"github.com/google/uuid"
)

type ExecutorNode struct {
	memory   memory.MemoryService
	executor *react.Executor
}

func NewExecutorNode(memory memory.MemoryService, executor *react.Executor) Node {
	return &ExecutorNode{
		memory:   memory,
		executor: executor,
	}
}

func (n *ExecutorNode) Invoke(ctx context.Context, state *State) (*State, error) {
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
		if err := n.memory.ResumePlan(ctx, p.ID); err != nil {
			return nil, err
		}
	case scheduler.ActionExecute:
		// 开始下一步
		step := decision.Step
		if step == nil {
			return nil, fmt.Errorf("no executable step")
		}
		if err := n.memory.StartStep(ctx, p.ID, step.ID); err != nil {
			return nil, err
		}
	}

	//构建执行上下文
	exeCtx, err := n.memory.BuildExecutionContext(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	step := exeCtx.Step
	if step == nil {
		return nil, fmt.Errorf("execution step is nil")
	}

	//执行step
	stepResult, err := n.executor.ExecuteStep(ctx, exeCtx)
	//更新失败状态
	if err != nil {
		_ = n.memory.UpdatePlanStatus(ctx, p.ID, atype.StatusFailed)
		_ = n.memory.FailStep(ctx, p.ID, step.ID)
		return nil, err
	}

	//持久化消息
	if len(stepResult.Messages) > 0 {
		err = n.memory.AppendMessage(ctx, stepResult.Messages)
		if err != nil {
			return nil, err
		}
	}

	//如果暂停：更新decision状态返回结果
	if stepResult.Paused {
		// 保存 checkpoint
		checkpoint := &atype.Checkpoint{
			ID:        uuid.NewString(),
			StepID:    step.ID,
			Question:  stepResult.Output,
			CreatedAt: time.Now(),
		}
		p.Checkpoint = checkpoint
		if err := n.memory.SavePlan(ctx, p); err != nil {
			return nil, err
		}

		// 更新状态
		if err := n.memory.UpdatePlanStatus(ctx, p.ID, atype.StatusPaused); err != nil {
			return nil, err
		}

		state.Decision = &scheduler.Decision{
			Action: scheduler.ActionPause,
			Plan:   p,
			Step:   step,
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

	//如果完成：
	if stepResult.Completed {
		planCompleted, err := n.memory.CompleteStep(ctx, p.ID, step.ID, stepResult.Output)
		if err != nil {
			return nil, err
		}
		//若计划完成：直接返回结果
		if planCompleted {
			state.Decision = &scheduler.Decision{
				Action: scheduler.ActionComplete,
				Plan:   p,
			}
		} else {
			//否则寻找下一个step执行
			nextStep := n.findNextStep(p, step)
			if nextStep == nil {
				return nil, fmt.Errorf("next step not found")
			}
			state.Decision = &scheduler.Decision{
				Action: scheduler.ActionExecute,
				Plan:   p,
				Step:   nextStep,
			}
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

func (n *ExecutorNode) findNextStep(plan *atype.Plan, currentStep *atype.Step) *atype.Step {
	for i := range plan.Steps {
		if plan.Steps[i].ID == currentStep.ID {
			if i >= len(plan.Steps)-1 {
				return nil
			}
			return &plan.Steps[i+1]
		}
	}
	return nil
}
