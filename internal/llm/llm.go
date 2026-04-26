package llm

import (
	"agent-pilot-be/configs"
	"context"
	"encoding/json"
	"fmt"
	goopenai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"strings"
)

type LLM struct {
	model  string
	client *goopenai.Client
}

func NewClient(cfg *configs.Conf) (*LLM, error) {
	if strings.TrimSpace(cfg.OpenAI.APIKey) == "" {
		return nil, fmt.Errorf("missing openai apiKey")
	}
	if strings.TrimSpace(cfg.OpenAI.BaseURL) == "" {
		return nil, fmt.Errorf("missing openai baseUrl")
	}
	if strings.TrimSpace(cfg.OpenAI.EndpointID) == "" {
		return nil, fmt.Errorf("missing openai endpointID/model")
	}
	c := goopenai.NewClient(
		option.WithAPIKey(cfg.OpenAI.APIKey),
		option.WithBaseURL(strings.TrimRight(cfg.OpenAI.BaseURL, "/")),
	)
	return &LLM{model: cfg.OpenAI.EndpointID, client: &c}, nil
}

type Message struct {
	Role    string
	Content string
}

func (c *LLM) Chat(ctx context.Context, messages []Message, out any) (string, error) {
	if c == nil || c.client == nil {
		return "", fmt.Errorf("nil llm client")
	}

	var msgs []goopenai.ChatCompletionMessageParamUnion

	msgs = append(msgs,
		goopenai.SystemMessage(
			"You MUST output ONLY valid JSON. No markdown. No explanation.",
		),
	)

	for _, m := range messages {
		switch m.Role {
		case "system":
			msgs = append(msgs, goopenai.SystemMessage(m.Content))
		case "user":
			msgs = append(msgs, goopenai.UserMessage(m.Content))
		case "assistant":
			msgs = append(msgs, goopenai.AssistantMessage(m.Content))
		}
	}

	resp, err := c.client.Chat.Completions.New(ctx, goopenai.ChatCompletionNewParams{
		Model:       c.model,
		Messages:    msgs,
		Temperature: goopenai.Float(0.2),
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response")
	}
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	if err := json.Unmarshal([]byte(content), out); err != nil {
		return content, err
	}
	return content, nil
}
