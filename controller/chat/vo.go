package chat

type request struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id"`
}
