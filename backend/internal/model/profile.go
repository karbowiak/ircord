package model

import "time"

// WhoisDetails is what one WHOIS on a network says about a nickname. Fields the server
// doesn't send are left out.
type WhoisDetails struct {
	Found       bool       `json:"found" doc:"False when the server says no one uses this nickname."`
	Nickname    string     `json:"nickname"`
	Username    string     `json:"username,omitempty"`
	Host        string     `json:"host,omitempty"`
	RealName    string     `json:"realName,omitempty"`
	Server      string     `json:"server,omitempty"`
	ServerInfo  string     `json:"serverInfo,omitempty"`
	Account     string     `json:"account,omitempty" doc:"Services account they are logged in to."`
	Channels    []string   `json:"channels" nullable:"false" doc:"Channels the server shows them in, with status prefixes."`
	IdleSeconds *int64     `json:"idleSeconds,omitempty"`
	SignedOnAt  *time.Time `json:"signedOnAt,omitempty"`
	Secure      bool       `json:"secure" doc:"Connected with TLS (numeric 671)."`
	Operator    bool       `json:"operator" doc:"An IRC operator (numeric 313)."`
	Away        string     `json:"away,omitempty" doc:"Away message, when they are away."`
}

// CTCPReply is a client's answer to one CTCP query.
type CTCPReply struct {
	Command  string `json:"command" enum:"VERSION,TIME,PING"`
	Answered bool   `json:"answered" doc:"False when no reply arrived in time; many clients ignore CTCP."`
	Reply    string `json:"reply,omitempty"`
	// Round trip for PING, measured here.
	LatencyMs *int64 `json:"latencyMs,omitempty"`
}
