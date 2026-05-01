package graph

import (
	"context"
	"fmt"

	"github.com/agent-pilot/agent-pilot-be/agent/graph/nodes"
	"github.com/agent-pilot/agent-pilot-be/agent/scheduler"
	atype "github.com/agent-pilot/agent-pilot-be/agent/type"
	"github.com/cloudwego/eino/compose"
)

type AgentGraph struct {
	schedulerNode nodes.Node
	plannerNode   nodes.Node
	executorNode  nodes.Node
}

func NewAgentGraph(
	schedulerNode nodes.Node,
	plannerNode nodes.Node,
	executorNode nodes.Node,
) *AgentGraph {
	return &AgentGraph{
		schedulerNode: schedulerNode,
		plannerNode:   plannerNode,
		executorNode:  executorNode,
	}

}

func (ag *AgentGraph) BuildGraph(opts ...compose.GraphCompileOption) (compose.Runnable[*nodes.State, *atype.Result], error) {
	g := compose.NewGraph[*nodes.State, *atype.Result]()
	//scheduler
	err := g.AddLambdaNode(
		"scheduler",
		compose.InvokableLambda(func(ctx context.Context, state *nodes.State) (*nodes.State, error) {
			return ag.schedulerNode.Invoke(ctx, state)
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
			func(ctx context.Context, state *nodes.State) (*nodes.State, error) {
				return ag.plannerNode.Invoke(ctx, state)
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
			func(ctx context.Context, state *nodes.State) (*nodes.State, error) {
				return ag.executorNode.Invoke(ctx, state)
			},
		),
		compose.WithNodeName("Executor"),
	)
	if err != nil {
		return nil, err
	}

	err = g.AddLambdaNode(
		"finisher",
		compose.InvokableLambda(
			func(ctx context.Context, state *nodes.State) (*atype.Result, error) {
				if state == nil || state.Result == nil {
					return &atype.Result{}, nil
				}
				return state.Result, nil
			},
		),
		compose.WithNodeName("Finisher"),
	)
	if err != nil {
		return nil, err
	}

	if err := g.AddEdge(compose.START, "scheduler"); err != nil {
		return nil, err
	}

	if err := g.AddBranch(
		"scheduler",
		compose.NewGraphBranch(
			func(ctx context.Context, state *nodes.State) (string, error) {
				if state == nil || state.Decision == nil {
					return "finisher", nil
				}
				switch state.Decision.Action {
				case scheduler.ActionPlan:
					return "planner", nil
				case scheduler.ActionExecute:
					return "executor", nil
				case scheduler.ActionResume:
					return "executor", nil
				default:
					return "finisher", nil
				}
			},
			map[string]bool{
				"planner":  true,
				"executor": true,
				"finisher": true,
			},
		),
	); err != nil {
		return nil, err
	}

	if err := g.AddBranch(
		"executor",
		compose.NewGraphBranch(
			func(ctx context.Context, state *nodes.State) (string, error) {
				if state == nil || state.Decision == nil {
					return "finisher", nil
				}
				switch state.Decision.Action {
				case scheduler.ActionExecute:
					return "executor", nil

				case scheduler.ActionPause:
					return "finisher", nil
				case scheduler.ActionComplete:
					return "finisher", nil
				default:
					return "finisher", nil
				}
			},
			map[string]bool{
				"executor": true,
				"finisher": true,
			},
		),
	); err != nil {
		return nil, err
	}

	// planner -> executor
	if err := g.AddEdge("planner", "executor"); err != nil {
		return nil, err
	}

	if err := g.AddEdge("finisher", compose.END); err != nil {
		return nil, err
	}

	runnable, err := g.Compile(context.Background(), opts...)

	if err != nil {
		return nil, fmt.Errorf(
			"compile graph failed: %w",
			err,
		)
	}
	return runnable, nil

}
