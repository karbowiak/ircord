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
	Version            string     `json:"version"`
	Cipher             string     `json:"cipher"`
	Verified           bool       `json:"verified"`
	CertificateSHA256  string     `json:"certificateSha256,omitempty"`
	ServerName         string     `json:"serverName"`
	Subject            string     `json:"subject,omitempty" doc:"The server certificate's common name"`
	Issuer             string     `json:"issuer,omitempty" doc:"Who issued the server certificate"`
	NotBefore          *time.Time `json:"notBefore,omitempty"`
	NotAfter           *time.Time `json:"notAfter,omitempty"`
	KeyAlgorithm       string     `json:"keyAlgorithm,omitempty" enum:"RSA,ECDSA,Ed25519,other"`
	KeyBits            int        `json:"keyBits,omitempty"`
	SignatureAlgorithm string     `json:"signatureAlgorithm,omitempty"`
	ForwardSecrecy     bool       `json:"forwardSecrecy"`
	Grade              string     `json:"grade" enum:"A+,A,A-,B+,B,B-,C,D,F" doc:"A summary of the connection's security, graded when read"`
	GradeNotes         []string   `json:"gradeNotes" nullable:"false" doc:"Why the grade is not higher, in plain language"`
}

// LatencySample is one keep-alive round trip to the IRC server.
type LatencySample struct {
	At           time.Time `json:"at"`
	Milliseconds float64   `json:"ms"`
}

// ConnectionLatency is a network's recent keep-alive round trips, oldest first.
type ConnectionLatency struct {
	NetworkID       string          `json:"networkId"`
	IntervalSeconds int             `json:"intervalSeconds" doc:"How often IRCord pings the server"`
	WindowSeconds   int             `json:"windowSeconds" doc:"How far back samples are kept"`
	Samples         []LatencySample `json:"samples" nullable:"false"`
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
