package model

// AttractEvent asks open clients to briefly highlight a message. Like typing it
// carries no sequence: it is never stored, replayed or delivered after the fact.
type AttractEvent struct {
	V    int         `json:"v" enum:"1"`
	Type string      `json:"type" enum:"attract"`
	Data AttractData `json:"data"`
}
type AttractData struct {
	ConversationID string `json:"conversationId"`
	// MemberID is the operator who asked for attention; MessageID is their message.
	MemberID  string `json:"memberId"`
	MessageID string `json:"messageId"`
}
