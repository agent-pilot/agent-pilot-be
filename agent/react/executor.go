package react

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	context2 "github.com/agent-pilot/agent-pilot-be/agent/context"
	"github.com/agent-pilot/agent-pilot-be/agent/memory"
	atype "github.com/agent-pilot/agent-pilot-be/agent/type"
	einomodel "github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

const defaultMaxTurns = 8

type Executor struct {
	model          einomodel.ToolCallingChatModel
	tools          []einotool.BaseTool
	maxTurns       int
	now            func() time.Time
	contextBuilder *context2.Builder
}

func NewExecutor(
	model einomodel.ToolCallingChatModel,
	tools []einotool.BaseTool,
	contextBuilder *context2.Builder,
) *Executor {
	return &Executor{
		model:          model,
		tools:          tools,
		maxTurns:       defaultMaxTurns,
		now:            time.Now,
		contextBuilder: contextBuilder,
	}
}

func (e *Executor) ExecuteStep(ctx context.Context, exeCtx *memory.ExecutionContext) (*atype.StepResult, error) {
	//用来保存过程中的msg存到memory中
	var executionMessages []*atype.Message
	toolInfos, invokables, err := e.prepareTools(ctx)
	if err != nil {
		return nil, err
	}

	modelWithTools, err := e.model.WithTools(toolInfos)
	if err != nil {
		return nil, err
	}

	//构建上下文信息
	messages, err := e.contextBuilder.BuildExecutionContext(exeCtx)
	if err != nil {
		return nil, err
	}

	messages = append([]*schema.Message{
		schema.SystemMessage(e.systemPrompt()),
	}, messages...)

	for turn := 0; turn < e.maxTurns; turn++ {
		msg, err := modelWithTools.Generate(ctx, messages)
		if err != nil {
			return nil, err
		}
		if msg == nil {
			return nil, fmt.Errorf("model returned nil message")
		}

		executionMessages = append(
			executionMessages,
			&atype.Message{
				SessionID: exeCtx.Plan.SessionID,
				PlanID:    exeCtx.Plan.ID,
				StepID:    exeCtx.Step.ID,
				Role:      atype.RoleAssistant,
				Content:   msg.Content,
				CreatedAt: time.Now(),
			},
		)
		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 {
			return nil, fmt.Errorf("model returned no tool call; expected complete_step or request_human_input")
		}

		for _, call := range msg.ToolCalls {
			toolName := call.Function.Name
			//保存工具调用信息
			executionMessages = append(
				executionMessages,
				&atype.Message{
					SessionID: exeCtx.Plan.SessionID,
					PlanID:    exeCtx.Plan.ID,
					StepID:    exeCtx.Step.ID,
					Role:      atype.RoleToolCall,
					Content:   fmt.Sprintf("tool_name:%s,arguments:%s", call.Function.Name, call.Function.Arguments),
					Metadata: map[string]any{
						"tool_name": call.Function.Name,
						"call_id":   call.ID,
						"arguments": call.Function.Arguments,
					},
					CreatedAt: time.Now(),
				},
			)

			switch call.Function.Name {

			//调用中断工具：中断等待用户输入
			case "request_human_input":
				question, err := e.extractQuestion(call.Function.Arguments)
				if err != nil {
					return nil, fmt.Errorf("invalid request_human_input args: %w", err)
				}
				question = strings.TrimSpace(question)

				executionMessages = append(executionMessages, &atype.Message{
					SessionID: exeCtx.Plan.SessionID,
					PlanID:    exeCtx.Plan.ID,
					StepID:    exeCtx.Step.ID,
					Role:      atype.RoleToolResult,
					Content:   question,
					Metadata: map[string]any{
						"tool_name": call.Function.Name,
						"call_id":   call.ID,
					},
					CreatedAt: time.Now(),
				})

				return &atype.StepResult{
					StepID:    exeCtx.Step.ID,
					Paused:    true,
					Completed: false,
					Output:    question,
					Messages:  executionMessages,
				}, nil

			//调用完成工具：
			case "complete_step":
				summary, err := e.extractCompleteResult(
					call.Function.Arguments,
				)
				if err != nil {
					return nil, err
				}

				executionMessages = append(executionMessages, &atype.Message{
					SessionID: exeCtx.Plan.SessionID,
					PlanID:    exeCtx.Plan.ID,
					StepID:    exeCtx.Step.ID,
					Role:      atype.RoleToolResult,
					Content:   summary,
					Metadata: map[string]any{
						"tool_name": toolName,
						"call_id":   call.ID,
					},
					CreatedAt: time.Now(),
				})

				return &atype.StepResult{
					StepID:    exeCtx.Step.ID,
					Output:    summary,
					Paused:    false,
					Completed: true,
					Messages:  executionMessages,
				}, nil

			//其他工具调用
			default:
				t, ok := invokables[toolName]
				if !ok {
					messages = append(messages, schema.ToolMessage("unknown tool: "+toolName, call.ID, schema.WithToolName(toolName)))
					continue
				}

				result, err := t.InvokableRun(ctx, call.Function.Arguments)
				if err != nil {
					result = "tool execution error: " + err.Error()
				}
				messages = append(messages, schema.ToolMessage(result, call.ID, schema.WithToolName(toolName)))
				executionMessages = append(
					executionMessages,
					&atype.Message{
						SessionID: exeCtx.Plan.SessionID,
						PlanID:    exeCtx.Plan.ID,
						StepID:    exeCtx.Step.ID,
						Role:      atype.RoleToolResult,
						Content:   result,
						Metadata: map[string]any{
							"tool_name": call.Function.Name,
							"call_id":   call.ID,
						},
					},
				)
			}
		}
	}
	return nil, fmt.Errorf("react executor exceeded max turns for step %s", exeCtx.Step.ID)
}

func (e *Executor) prepareTools(ctx context.Context) ([]*schema.ToolInfo, map[string]einotool.InvokableTool, error) {
	infos := make([]*schema.ToolInfo, 0, len(e.tools))
	invokables := make(map[string]einotool.InvokableTool, len(e.tools))

	for _, baseTool := range e.tools {
		info, err := baseTool.Info(ctx)
		if err != nil {
			return nil, nil, err
		}
		infos = append(infos, info)

		if invokable, ok := baseTool.(einotool.InvokableTool); ok {
			invokables[info.Name] = invokable
		}
	}

	return infos, invokables, nil
}

func (e *Executor) systemPrompt() string {
	return `You are the execution layer of a plan-execute agent.

Core Rules:
- Execute ONLY the current step. Never execute future steps.
- Use ReAct: think silently about the next action before acting.
- Use load_skill before following a skill.
- Use load_skill_references only when the loaded skill requires reference files.
- Use shell for lark-cli commands.
- The Lark user access token is provided by the runtime environment. DO NOT ask for it or print it.
- Prefer --as user for user-owned Lark resources unless bot identity is explicitly required.
- Stop immediately after completing the current step.

Required Function Calls:
- When the current step is finished: you MUST explicitly call complete_step.
- When required information is missing, ambiguous, or needs confirmation:
  you MUST explicitly call request_human_input, then stop execution immediately.
  Examples: missing document ID, unknown target user, ambiguous instruction, confirmation before destructive action.

When requesting human input: ask a concise question and stop execution.
When the step is completed: return a concise result and do not proceed to other steps.
`
}

func (e *Executor) extractQuestion(args string) (string, error) {
	var payload struct {
		Question string `json:"question"`
	}
	err := json.Unmarshal(
		[]byte(args),
		&payload,
	)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(payload.Question), nil
}

func (e *Executor) extractCompleteResult(args string) (string, error) {
	type finishArgs struct {
		Summary string `json:"summary"`
	}
	var req finishArgs
	if err := json.Unmarshal([]byte(args), &req); err != nil {
		return "", fmt.Errorf(
			"invalid complete_step args: %w",
			err,
		)
	}
	req.Summary = strings.TrimSpace(req.Summary)
	if req.Summary == "" {
		return "", fmt.Errorf("complete_step summary is empty")
	}
	return req.Summary, nil
}
