package model

type TypingState string

const (
	TypingActive TypingState = "active"
	TypingPaused TypingState = "paused"
	TypingDone   TypingState = "done"
)

func (s TypingState) Valid() bool { return s == TypingActive || s == TypingPaused || s == TypingDone }

// TypingEvent is deliberately separate from sequenced, durable events.
type TypingEvent struct {
	V    int        `json:"v" enum:"1"`
	Type string     `json:"type" enum:"typing"`
	Data TypingData `json:"data"`
}
type TypingData struct {
	ConversationID string      `json:"conversationId"`
	MemberID       string      `json:"memberId"`
	State          TypingState `json:"state" enum:"active,paused,done"`
}
