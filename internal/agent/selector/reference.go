package selector

import (
	"agent-pilot-be/internal/llm"
	prompt2 "agent-pilot-be/internal/prompt"
	"context"
	"fmt"
)

type Reference struct {
	Skill string `json:"skill"`
	Path  string `json:"path"`
}

type refPick struct {
	References []Reference `json:"references"`
	Why        string      `json:"why,omitempty"`
}

type ReferenceSelector struct {
	llm *llm.LLM
}

func NewReferenceSelector(llm *llm.LLM) *ReferenceSelector {
	return &ReferenceSelector{
		llm: llm,
	}
}

// Select 从具体的skillContext和reference目录中选择要查的reference
func (r *ReferenceSelector) Select(ctx context.Context, skillCtx string, refCatalog string, messages []llm.Message) ([]Reference, string, error) {
	var chosen refPick
	prompt := prompt2.BuildReferenceCatalogPrompt(refCatalog, skillCtx, messages)
	resp, err := r.llm.Chat(ctx, prompt, &chosen)
	if err != nil {
		return nil, resp, fmt.Errorf("llm reference selection failed: %w", err)
	}
	return chosen.References, resp, nil
}
