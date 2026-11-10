package activity

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// DeviceID is the same one-way session digest used by device management.
func DeviceID(session string) string {
	if session == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("ircord-device\x00" + session))
	return hex.EncodeToString(sum[:16])
}

type RoutingSnapshot struct {
	Revision      string     `json:"revision" doc:"Monotonically increasing decimal ordering token; compare as an integer."`
	DeviceID      string     `json:"deviceId"`
	FocalDeviceID string     `json:"focalDeviceId,omitempty"`
	Phase         string     `json:"phase" enum:"active,sound,target,all"`
	InactiveSince *time.Time `json:"inactiveSince,omitempty"`
	ConnectionID  string     `json:"connectionId,omitempty" doc:"Opaque current-socket capability, private to this connection; fence activity reports with it."`
}

type Report struct {
	Tab                  string `json:"tab" minLength:"1" maxLength:"64"`
	Incarnation          string `json:"incarnation" minLength:"1" maxLength:"64"`
	ConnectionID         string `json:"connectionId" minLength:"1" maxLength:"64"`
	Sequence             uint64 `json:"sequence" minimum:"1" maximum:"9007199254740991"`
	Signal               string `json:"signal" enum:"engagement,renewal,inactive"`
	Active               bool   `json:"active" doc:"Visible and focused. Reading remains active while renewed; no input-idle cutoff."`
	ViewedConversationID string `json:"viewedConversationId,omitempty" maxLength:"200"`
	FormFactor           string `json:"formFactor,omitempty" enum:"desktop,phone,tablet"`
	CanAlert             bool   `json:"canAlert,omitempty" doc:"This open client can show an OS notification; permission alone does not enable desired delivery."`
	Permission           string `json:"permission" enum:"unknown,default,granted,denied,unsupported"`
}
