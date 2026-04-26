package session

import "agent-pilot-be/internal/llm"

type State string

const (
	StateWaitingInput   State = "waiting_input"
	StateWaitingConfirm State = "waiting_confirm"
	StateComplete       State = "complete"
)

type Session struct {
	State          State
	Messages       []llm.Message
	PendingCommand string
}
