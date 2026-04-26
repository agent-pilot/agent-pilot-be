package selector

import (
	"agent-pilot-be/internal/llm"
	prompt2 "agent-pilot-be/internal/prompt"
	"agent-pilot-be/internal/runtime"
	"context"
	"fmt"
	"strings"
)

type SkillSelector struct {
	llm *llm.LLM
}

func NewSkillSelector(llm *llm.LLM) *SkillSelector {
	return &SkillSelector{
		llm: llm,
	}
}

type sel struct {
	Skills []string `json:"skills"`
	Why    string   `json:"why,omitempty"`
}

// Select 根据用户输入选择需要的skills
func (s *SkillSelector) Select(
	ctx context.Context,
	messages []llm.Message,
	rt *runtime.Runtime,
) ([]string, string, error) {
	catalog := s.catalogBuilder(rt)
	prompt := prompt2.BuildSkillCatalogPrompt(catalog, messages)
	var chosen sel
	resp, err := s.llm.Chat(ctx, prompt, &chosen)
	if err != nil {
		return nil, resp, fmt.Errorf("llm skill selection failed: %w", err)
	}
	if len(chosen.Skills) == 0 {
		return nil, resp, fmt.Errorf("llm selected no skills")
	}
	return chosen.Skills, resp, nil
}

// 构建所有的skills的目录
func (s *SkillSelector) catalogBuilder(rt *runtime.Runtime) string {
	var catalog strings.Builder
	catalog.WriteString("Available skills (choose only from these names):\n")
	for _, s := range rt.Skills.Skills {
		name := s.Meta.Name
		if strings.TrimSpace(name) == "" {
			name = s.Name
		}
		desc := strings.TrimSpace(s.Meta.Description)
		cliHelp := strings.TrimSpace(s.Meta.Metadata.CLIHelp)
		requires := strings.Join(s.Meta.Metadata.Requires.Bins, ",")
		catalog.WriteString("- " + name)
		if desc != "" {
			catalog.WriteString(" — " + desc)
		}
		if cliHelp != "" {
			catalog.WriteString(" | cliHelp: " + cliHelp)
		}
		if requires != "" {
			catalog.WriteString(" | requires: " + requires)
		}
		catalog.WriteString("\n")
	}

	return catalog.String()
}
