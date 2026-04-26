package prompt

import (
	"agent-pilot-be/internal/llm"
)

func BuildSkillCatalogPrompt(catalog string, messages []llm.Message) []llm.Message {
	systemPrompt := `
	Select the minimal set of skills needed to fulfill the user's request.
	Output JSON:
	{
	  "skills": ["..."],
	  "why": "..."
	}
	`
	out := []llm.Message{
		{Role: "system", Content: systemPrompt + "\n\n" + catalog},
	}
	out = append(out, messages...)
	return out
}

func BuildReferenceCatalogPrompt(refCatalog string, skillCtx string, messages []llm.Message) []llm.Message {
	prompt := `
	Select the minimal reference files needed to answer the user request correctly.
	Output ONLY JSON: {
		"references":[
			{
				"skill":"<skill name>",
				"path":"references/<...>.md"
			},
			...
		],
		"why":"..."
	}.
	If none are needed, output {"references":[]}. 
	Do not include non-listed paths. Keep references <=
	`
	out := []llm.Message{
		{Role: "system", Content: prompt + "\n\n" + refCatalog + "\n\n" + skillCtx},
	}
	out = append(out, messages...)
	return out
}

func BuildCommandPromptWithMessages(skillCtx string, refCtx string, messages []llm.Message) []llm.Message {
	prompt := `
	You are a Feishu/Lark operator agent. Using ONLY the provided skills instructions, output ONE JSON object:
	{"type":"command","args":"<args to append after lark-cli>","requires_confirmation":true|false,"why":"..."}
	or {"type":"message","content":"..."}.
	If the action may create/update/delete/send, set requires_confirmation=true.
	`
	out := []llm.Message{
		{Role: "system", Content: prompt + "\n\n" + skillCtx + "\n\n" + refCtx},
	}
	out = append(out, messages...)
	return out
}
