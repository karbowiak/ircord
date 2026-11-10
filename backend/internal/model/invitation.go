package model

import "time"

// Invitations are private workspace state, never channel membership.
type Invitation struct {
	Party       *PartyInvitation `json:"party,omitempty"`
	ID          string           `json:"id"`
	NetworkID   string           `json:"networkId"`
	Channel     string           `json:"channel"`
	InviterID   string           `json:"inviterId"`
	InviterNick string           `json:"inviterNick"`
	CreatedAt   time.Time        `json:"createdAt"`
	ChannelKey  string           `json:"-"`
	Rejected    bool             `json:"-"`
}

type InvitationRemoval struct {
	ID       string `json:"id"`
	Rejected bool   `json:"-"`
}
