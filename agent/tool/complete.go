package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type FinishArgs struct {
	Summary string `json:"summary"`
}

type FinishTool struct{}

func (t *FinishTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "complete_step",
		Desc: `
Mark the current step as completed.
Call this tool ONLY when:
- the current step objective is fully completed
- all required actions are finished
Provide a concise summary of what was completed.
`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"summary": {
				Type:     schema.String,
				Required: true,
			},
		}),
	}, nil
}

func (t *FinishTool) InvokableRun(ctx context.Context, args string, opts ...tool.Option) (string, error) {
	var req FinishArgs
	if err := json.Unmarshal([]byte(args), &req); err != nil {
		return "", fmt.Errorf(
			"invalid complete_step args: %w",
			err,
		)
	}
	req.Summary = strings.TrimSpace(req.Summary)
	if req.Summary == "" {
		return "", fmt.Errorf("summary is required")
	}
	return req.Summary, nil
}
