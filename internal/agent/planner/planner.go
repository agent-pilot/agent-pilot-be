package planner

import (
	"agent-pilot-be/internal/llm"
	"context"
	"fmt"
	"strings"
)

type Plan struct {
	Title string `json:"title"`
	Steps []Step `json:"steps"`
}

type Step struct {
	Name string `json:"name"`
	Goal string `json:"goal"`
}

type Planner struct {
	llm *llm.LLM
}

func NewPlanner(l *llm.LLM) *Planner {
	return &Planner{llm: l}
}

func (p *Planner) CreatePlan(ctx context.Context, messages []llm.Message) (Plan, string, error) {
	if p == nil || p.llm == nil {
		return Plan{}, "", fmt.Errorf("nil planner llm")
	}
	sys := `
	You are a task planner. Produce a small multi-stage plan for the user goal.
	Output ONLY valid JSON:
	{
	  "title": "...",
	  "steps": [
		{ "name": "...", "goal": "..." }
	  ]
	}
	Rules:
	- steps length: 1-4
	- each goal should be executable as a single agent session (one main action; may require a few follow-up clarifications)
	- keep goals concise and specific
	`
	prompt := []llm.Message{{Role: "system", Content: strings.TrimSpace(sys)}}
	prompt = append(prompt, messages...)

	var plan Plan
	raw, err := p.llm.Chat(ctx, prompt, &plan)
	if err != nil {
		return Plan{}, raw, err
	}
	if len(plan.Steps) == 0 {
		return Plan{}, raw, fmt.Errorf("empty plan")
	}
	return plan, raw, nil
}
