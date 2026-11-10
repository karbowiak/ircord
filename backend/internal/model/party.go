package model

import "time"

type PartyCapability struct {
	NetworkID            string `json:"networkId"`
	MaxMembers           int    `json:"maxMembers"`
	MaxNameLength        int    `json:"maxNameLength"`
	MaxDescriptionLength int    `json:"maxDescriptionLength"`
}

type PartyCandidate struct {
	ID                   string    `json:"id"`
	ServicesAccount      string    `json:"servicesAccount"`
	ServicesRegisteredAt time.Time `json:"servicesRegisteredAt"`
	BackendAccountID     string    `json:"backendAccountId,omitempty"`
	// AccountID is a deprecated local display hint, never an IRC services name.
	AccountID   string `json:"accountId,omitempty"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
}

func (p PartyCandidate) PrincipalID() string {
	if p.ID != "" {
		return p.ID
	}
	return p.AccountID // Retained development records are migrated before admission.
}
func (p PartyCandidate) LocalAccountID() string {
	if p.BackendAccountID != "" {
		return p.BackendAccountID
	}
	return p.AccountID
}

type PartyMember struct {
	PartyCandidate
	JoinedAt time.Time `json:"joinedAt"`
}

// Party is account membership, independent of the channel's online roster.
type Party struct {
	ID             string        `json:"id"`
	ConversationID string        `json:"conversationId"`
	NetworkID      string        `json:"networkId"`
	Name           string        `json:"name"`
	Description    string        `json:"description"`
	OwnerID        string        `json:"ownerId"`
	State          string        `json:"state" enum:"provisioning,active,recovering,archived,converted"`
	Revision       int64         `json:"revision" minimum:"1" maximum:"9007199254740991"`
	Members        []PartyMember `json:"members" nullable:"false"`
	CreatedAt      time.Time     `json:"createdAt"`
	UpdatedAt      time.Time     `json:"updatedAt"`
}

type PartyInvitation struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	InviterAccountID string `json:"inviterAccountId"`
	MemberCount      int    `json:"memberCount" minimum:"1"`
}

type PartyRemoval struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversationId"`
	Revision       int64  `json:"revision" minimum:"1" maximum:"9007199254740991"`
}
