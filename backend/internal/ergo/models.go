// Package ergo adapts the associated server's privileged HTTP API to a small,
// read-only contract. Credentials and private API addresses never enter DTOs.
package ergo

import "time"

type ErgoUsers struct {
	Total     int `json:"total" minimum:"0"`
	Invisible int `json:"invisible" minimum:"0"`
	Operators int `json:"operators" minimum:"0"`
	Unknown   int `json:"unknown" minimum:"0"`
	Peak      int `json:"peak" minimum:"0"`
}

type ErgoStatistics struct {
	Version       string    `json:"version"`
	GoVersion     string    `json:"goVersion"`
	StartedAt     time.Time `json:"startedAt"`
	UptimeSeconds int64     `json:"uptimeSeconds" minimum:"0"`
	Users         ErgoUsers `json:"users"`
	Channels      int       `json:"channels" minimum:"0"`
	Servers       int       `json:"servers" minimum:"0"`
}

type ErgoStatus struct {
	Server     string          `json:"server"`
	State      string          `json:"state" enum:"ok,unavailable"`
	CheckedAt  time.Time       `json:"checkedAt"`
	Error      string          `json:"error,omitempty"`
	Statistics *ErgoStatistics `json:"statistics,omitempty"`
}

type ErgoChannel struct {
	Name       string `json:"name"`
	Users      int    `json:"users" minimum:"0"`
	Topic      string `json:"topic"`
	Secret     bool   `json:"secret"`
	InviteOnly bool   `json:"inviteOnly"`
	HasKey     bool   `json:"hasKey"`
	Registered bool   `json:"registered"`
	Owner      string `json:"owner,omitempty"`
}

type ErgoUserChannel struct {
	Name string `json:"name"`
	Mode string `json:"mode,omitempty"`
}

type ErgoUser struct {
	Nickname string            `json:"nickname"`
	Account  string            `json:"account,omitempty"`
	Realname string            `json:"realname,omitempty"`
	Modes    string            `json:"modes,omitempty"`
	Away     string            `json:"away,omitempty"`
	Sessions int               `json:"sessions" minimum:"0"`
	Channels []ErgoUserChannel `json:"channels" nullable:"false"`
}

type ErgoAccount struct {
	Name         string     `json:"name"`
	RegisteredAt *time.Time `json:"registeredAt,omitempty"`
	Channels     []string   `json:"channels" nullable:"false"`
}
