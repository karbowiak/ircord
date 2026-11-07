package model

import "time"

type ServerLine struct {
	Tags      map[string]string `json:"tags,omitempty"`
	ID        string            `json:"id"`
	NetworkID string            `json:"networkId"`
	Time      time.Time         `json:"time"`
	Kind      string            `json:"kind" enum:"numeric,notice,error,motd,raw,outgoing"`
	Command   string            `json:"command"`
	Source    string            `json:"source,omitempty"`
	Params    []string          `json:"params" nullable:"false"`
	Text      string            `json:"text"`
}
type ServerLogPage struct {
	Items      []ServerLine `json:"items" nullable:"false"`
	NextCursor *string      `json:"nextCursor"`
}
type TLSInfo struct {
	Version           string `json:"version"`
	Cipher            string `json:"cipher"`
	Verified          bool   `json:"verified"`
	CertificateSHA256 string `json:"certificateSha256,omitempty"`
	ServerName        string `json:"serverName"`
}
type ConnectionInfo struct {
	NetworkID              string            `json:"networkId"`
	State                  string            `json:"state"`
	ServerName             string            `json:"serverName"`
	ConnectedAt            *time.Time        `json:"connectedAt"`
	LastError              string            `json:"lastError,omitempty"`
	ISupport               map[string]string `json:"isupport" nullable:"false"`
	OfferedCapabilities    map[string]string `json:"offeredCapabilities" nullable:"false"`
	NegotiatedCapabilities []string          `json:"negotiatedCapabilities" nullable:"false"`
	MOTD                   []string          `json:"motd" nullable:"false"`
	TLS                    *TLSInfo          `json:"tls,omitempty"`
	FilehostURL            string            `json:"upstreamFilehostUrl,omitempty"`
}

type ServerLogBatch struct {
	NetworkID string       `json:"networkId"`
	Lines     []ServerLine `json:"lines,omitempty"`
}
