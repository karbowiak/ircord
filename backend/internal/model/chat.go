package model

import (
	"encoding/json"
	"slices"
	"time"
)

type Member struct {
	Realname    string `json:"realname,omitempty"`
	NameSource  string `json:"nameSource,omitempty" enum:"nick,realname,profile"`
	Ignored     bool   `json:"-"`
	Username    string `json:"username,omitempty"`
	Host        string `json:"host,omitempty"`
	ID          string `json:"id"`
	Nick        string `json:"nick"`
	DisplayName string `json:"displayName"`
	Color       string `json:"color"`
	NameColor   string `json:"nameColor,omitempty"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
	Status      string `json:"status" enum:"online,away,offline"`
	AwayMessage string `json:"awayMessage,omitempty" doc:"The AWAY text while away, when known."`
	Role        string `json:"role" enum:"member,operator,bot"`
	Bio         string `json:"bio"`
	Account     string `json:"account"`
	// Device is set while this IRCord user is on a phone or tablet (#136).
	Device string `json:"device,omitempty" enum:"phone,tablet" doc:"Set while this IRCord user is on a phone or tablet, from the ircord/device metadata key; absent on a desktop, when unknown, or for other clients."`
}
type NetworkAuth struct {
	Method      string `json:"method" enum:"none,sasl-plain,sasl-external,server-password,nickserv"`
	Account     string `json:"account,omitempty"`
	PasswordSet bool   `json:"passwordSet"`
}
type AuthInput struct {
	Method   string  `json:"method" enum:"none,sasl-plain,sasl-external,server-password,nickserv"`
	Account  string  `json:"account,omitempty"`
	Password *string `json:"password,omitempty" writeOnly:"true"`
}
type Features struct {
	// These are effective send features for the current connection, not the
	// server's offered CAP list. Stored history remains readable without them.
	Replies   bool `json:"replies"`
	Reactions bool `json:"reactions"`
	Typing    bool `json:"typing"`
	// Attract reports whether /attract can reach other IRCord users: it needs
	// message-tags and a CLIENTTAGDENY policy that lets +ircord/attract through.
	Attract     bool `json:"attract"`
	ReadMarkers bool `json:"readMarkers"`
	History     bool `json:"history"`
	// MessageLineBytes excludes tags, source prefix and CRLF, matching the IRC
	// library's safe event budget. PRIVMSG syntax and target consume this budget.
	MessageLineBytes int              `json:"messageLineBytes" minimum:"0" maximum:"510"`
	Multiline        *MultilineLimits `json:"multiline,omitempty"`
	Setname          bool             `json:"setname"`
	Metadata         []string         `json:"metadata" nullable:"false"`
	Away             bool             `json:"away"`
	// Redact reports whether the network negotiated draft/message-redaction,
	// which IRCord needs to delete messages for everyone in a conversation.
	Redact bool `json:"redact"`
	// Edit reports whether IRCord can edit messages on this network: message-tags,
	// echo-message and account-tag are negotiated and CLIENTTAGDENY allows
	// +ircord/edit.
	Edit bool `json:"edit"`
}
type MultilineLimits struct {
	// Limits include IRCord's own safety bounds and the negotiated server bounds.
	MaxBytes int `json:"maxBytes" minimum:"1" maximum:"4096"`
	MaxLines int `json:"maxLines" minimum:"1" maximum:"100"`
}
type Capabilities struct {
	Upload *UploadCapability `json:"upload,omitempty"`
}
type UploadCapability struct {
	MaxBytes int64    `json:"maxBytes"`
	Types    []string `json:"types" nullable:"false"`
}
type Removal struct {
	ID string `json:"id"`
}

// ConversationCounts is the complete unread state of one conversation. It is sent
// instead of the whole conversation when only the counts changed, so a busy
// channel's member list is not resent on every message.
type ConversationCounts struct {
	ID           string `json:"id"`
	UnreadCount  int    `json:"unreadCount"`
	MentionCount int    `json:"mentionCount"`
	UnreadFrom   string `json:"unreadFrom,omitempty" doc:"Message a manual mark-as-unread starts at; absent when the conversation has no manual boundary"`
}
type ChannelServicesChange struct {
	ConversationID string `json:"conversationId"`
}

type ChannelListing struct {
	Name  string `json:"name"`
	Users int    `json:"users"`
	Topic string `json:"topic"`
}
type Network struct {
	Account      string            `json:"account,omitempty" doc:"Account reported by the connected IRC server, independent of saved login settings."`
	StatusDetail string            `json:"statusDetail,omitempty"`
	Auth         NetworkAuth       `json:"auth"`
	AltNick      string            `json:"altNick,omitempty"`
	Username     string            `json:"username,omitempty"`
	Realname     string            `json:"realname,omitempty"`
	AutoJoin     []string          `json:"autoJoin" nullable:"false"`
	Perform      []string          `json:"perform" nullable:"false"`
	ISupport     map[string]string `json:"isupport"`
	Features     Features          `json:"features"`

	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Host         string   `json:"host"`
	Port         int      `json:"port"`
	TLS          bool     `json:"tls"`
	Initials     string   `json:"initials"`
	Color        string   `json:"color"`
	Status       string   `json:"status" enum:"connected,disconnected,connecting,error"`
	Capabilities []string `json:"capabilities" nullable:"false"`
	Nick         string   `json:"nick"`
}

// ChannelSettings are channel-wide: one value per upstream network and channel,
// shared by every IRCord member of that channel and changed only by its operators.
type ChannelSettings struct {
	Notifications string     `json:"notifications" enum:"default,all,mentions,none" doc:"Channel notification level for every member; default uses each member's own channel default."`
	GroupMentions string     `json:"groupMentions" enum:"all,ops,off" doc:"Who may reach the channel with @here, @channel and @everyone: anyone, channel operators (default), or nobody. Role mentions such as @operator stay open to everyone."`
	UpdatedBy     string     `json:"updatedBy,omitempty" doc:"IRC nick of the operator who last changed these settings."`
	UpdatedAt     *time.Time `json:"updatedAt,omitempty"`
}

type Conversation struct {
	// ChannelSettings is present on joined channels once the shared values are known.
	ChannelSettings *ChannelSettings  `json:"channelSettings,omitempty"`
	ModeParams      map[string]string `json:"modeParams,omitempty"`
	TopicSetBy      string            `json:"topicSetBy,omitempty"`
	TopicSetAt      *time.Time        `json:"topicSetAt,omitempty"`
	CreatedAt       *time.Time        `json:"createdAt,omitempty"`
	URL             string            `json:"url,omitempty"`
	MemberModes     map[string]string `json:"memberModes" nullable:"false"`
	Modes           string            `json:"modes"`
	ID              string            `json:"id"`
	NetworkID       string            `json:"networkId"`
	Kind            string            `json:"kind" enum:"channel,direct"`
	Name            string            `json:"name"`
	Target          string            `json:"target,omitempty"`
	Topic           string            `json:"topic"`
	UnreadCount     int               `json:"unreadCount"`
	MentionCount    int               `json:"mentionCount"`
	UnreadFrom      string            `json:"unreadFrom,omitempty" doc:"Message a manual mark-as-unread starts at; cleared when the conversation is read again"`
	Favorite        bool              `json:"favorite"`
	MemberIDs       []string          `json:"memberIds" nullable:"false"`
	Joined          bool              `json:"joined"`
	// Kicked is set while you are out of a channel because someone kicked you.
	Kicked *Kick `json:"kicked,omitempty" doc:"Who kicked you from this channel and why; cleared when you join it again. IRCord doesn't rejoin a channel you were kicked from."`
}

// Kick says who removed you from a channel.
type Kick struct {
	ByID   string `json:"byId" doc:"Member who kicked you"`
	Reason string `json:"reason,omitempty"`
}
type Reaction struct {
	Emoji     string   `json:"emoji"`
	MemberIDs []string `json:"memberIds" nullable:"false"`
}
type Reply struct {
	ID   string `json:"id"`
	Nick string `json:"nick"`
	Text string `json:"text"`
}
type GIF struct {
	ID         string `json:"id"`
	Slug       string `json:"slug"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	PreviewURL string `json:"previewUrl"`
	StillURL   string `json:"stillUrl"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Provider   string `json:"provider" enum:"klipy"`
}
type MembershipEvent struct {
	Type     string `json:"type" enum:"join,part,quit,nick,kick,mode,topic,invite,registered,registration-failed"`
	ActorID  string `json:"actorId"`
	TargetID string `json:"targetId,omitempty"`
	Text     string `json:"text,omitempty"`
}
type Attachment struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Alt         string `json:"alt"`
	ContentType string `json:"contentType,omitempty"`
	Size        int64  `json:"size,omitempty"`
}

// MaxAttachments is the upload batch limit shared with the composer.
const MaxAttachments = 10

const MaxEmbedGeometry = 8

// EmbedGeometry is cached sizing advice for an exact posted URL. It grants no
// media access and carries no delivery URL, token, markup, or expiry renewal.
type EmbedGeometry struct {
	URL    string `json:"url" maxLength:"8192"`
	Width  int    `json:"width" minimum:"1" maximum:"8192"`
	Height int    `json:"height" minimum:"1" maximum:"8192"`
}

type Message struct {
	EmbedGeometry []EmbedGeometry  `json:"embedGeometry,omitempty" maxItems:"8" nullable:"false" doc:"Cached validated single-image dimensions bound to exact URLs in the current message. Sizing advice only; media still requires authorization."`
	Deleted       bool             `json:"deleted,omitempty"`
	Mention       bool             `json:"-"`
	Position      int64            `json:"position,omitempty"`
	Event         *MembershipEvent `json:"event,omitempty"`
	Attachment    *Attachment      `json:"attachment,omitempty" doc:"A single uploaded file. Absent when attachments is present."`
	Attachments   []Attachment     `json:"attachments,omitempty" minItems:"2" maxItems:"10" nullable:"false" doc:"An ordered upload batch. Absent for single files; never combined with attachment."`

	ID             string     `json:"id"`
	ConversationID string     `json:"conversationId"`
	SenderID       string     `json:"senderId"`
	Text           string     `json:"text"`
	CreatedAt      time.Time  `json:"createdAt"`
	ClientID       string     `json:"clientId,omitempty"`
	Kind           string     `json:"kind" enum:"message,system,action,notice,event"`
	Reactions      []Reaction `json:"reactions" nullable:"false"`
	ReplyTo        *Reply     `json:"replyTo,omitempty"`
	GIF            *GIF       `json:"gif,omitempty"`
	Historical     bool       `json:"historical,omitempty"`
	UpstreamID     string     `json:"upstreamId,omitempty"`
	// Account is the IRC account the network attributed one of your own messages
	// to (account-tag), empty when it was sent without a login. Ergo authorizes
	// deleting your own message against this account, not the current session.
	Account string `json:"account,omitempty"`
	// EditedAt is the time of the newest applied edit; it is empty for a message
	// that was never edited.
	EditedAt *time.Time `json:"editedAt,omitempty"`
	// OriginalText is the text before the first edit.
	OriginalText string `json:"originalText,omitempty" doc:"Text before the first applied edit."`
	// Edits are the applied edits, oldest first. The newest is the current text.
	Edits []MessageEdit `json:"edits,omitempty" maxItems:"20"`
	// SenderAccount is the account the network attributed this message to
	// (account-tag), for anyone's message. Edits must come from the same account.
	SenderAccount string `json:"senderAccount,omitempty"`
	// EditOf is set on an edit stored as an ordinary line because its original
	// had not arrived yet; it names the original's msgid so the edit can be
	// folded in when the original arrives.
	EditOf string `json:"editOf,omitempty"`
}

func (m Message) HasAttachments() bool {
	return m.Attachment != nil || len(m.Attachments) > 0
}

// AllAttachments returns a copy, so callers cannot mutate a stored gallery.
func (m Message) AllAttachments() []Attachment {
	if len(m.Attachments) > 0 {
		return slices.Clone(m.Attachments)
	}
	if m.Attachment != nil {
		return []Attachment{*m.Attachment}
	}
	return nil
}

// SetAttachments keeps the single-file and batch representations exclusive.
func (m *Message) SetAttachments(attachments []Attachment) {
	m.Attachment, m.Attachments = nil, nil
	switch len(attachments) {
	case 0:
	case 1:
		attachment := attachments[0]
		m.Attachment = &attachment
	default:
		m.Attachments = slices.Clone(attachments)
	}
}

// MessageEdit is one edit of a message: an IRC PRIVMSG tagged +ircord/edit that
// names the original message's msgid.
type MessageEdit struct {
	UpstreamID string    `json:"upstreamId" doc:"The network msgid of the edit itself."`
	Text       string    `json:"text"`
	At         time.Time `json:"at"`
}
type Preferences struct {
	Theme       string `json:"theme"`
	Compact     bool   `json:"compact"`
	ShowMembers bool   `json:"showMembers"`
}
type Snapshot struct {
	Invitations   []Invitation   `json:"invitations" nullable:"false"`
	Capabilities  Capabilities   `json:"capabilities" nullable:"false"`
	Version       int            `json:"version" enum:"1"`
	Demo          bool           `json:"demo"`
	User          Member         `json:"user"`
	Members       []Member       `json:"members" nullable:"false"`
	Networks      []Network      `json:"networks" nullable:"false"`
	Conversations []Conversation `json:"conversations" nullable:"false"`
	Preferences   Preferences    `json:"-"`
	Cursor        uint64         `json:"cursor"`
}
type Event struct {
	V        int             `json:"v"`
	Seq      uint64          `json:"seq,omitempty"`
	Type     string          `json:"type"`
	Data     json.RawMessage `json:"data,omitempty"`
	ClientID string          `json:"clientId,omitempty"`
	Message  string          `json:"message,omitempty"`
}
type SendInput struct {
	Kind           string   `json:"kind,omitempty" enum:"message,action"`
	Attachments    []string `json:"attachments,omitempty" maxItems:"10" uniqueItems:"true" doc:"Ordered upload IDs. Text must contain only their URLs in the same order, separated by spaces or newlines."`
	ClientID       string   `json:"clientId"`
	ConversationID string   `json:"conversationId"`
	Text           string   `json:"text"`
	ReplyToID      string   `json:"replyToId,omitempty"`
	GIF            *GIF     `json:"gif,omitempty"`
}
type NetworkConfig struct {
	AltNick  string     `json:"altNick,omitempty"`
	Username string     `json:"username,omitempty"`
	Realname string     `json:"realname,omitempty"`
	Auth     *AuthInput `json:"auth,omitempty"`
	AutoJoin []string   `json:"autoJoin,omitempty"`
	Perform  []string   `json:"perform,omitempty"`

	ID             string   `json:"id,omitempty"`
	Name           string   `json:"name"`
	Host           string   `json:"host"`
	Port           int      `json:"port"`
	TLS            bool     `json:"tls"`
	Nick           string   `json:"nick"`
	SASLUser       string   `json:"saslUser,omitempty"`
	SASLPassword   string   `json:"saslPassword,omitempty" writeOnly:"true"`
	ServerPassword string   `json:"serverPassword,omitempty" writeOnly:"true"`
	Channels       []string `json:"channels,omitempty"`
}
type Profile struct {
	AvatarDataURL json.RawMessage `json:"avatarDataUrl,omitempty"`
	// DisplayName is the explicit IRCord profile name: nil keeps the current one, "" clears it
	// so the network real name or nickname shows instead.
	DisplayName *string `json:"displayName,omitempty"`
	Bio         string  `json:"bio"`
	NameColor   *string `json:"nameColor,omitempty"`
	AvatarURL   *string `json:"avatarUrl,omitempty"`
}
type MessagePage struct {
	Cursor     uint64    `json:"cursor,omitempty" doc:"Account event position captured with this page. Later live message revisions take precedence."`
	SyncError  string    `json:"syncError,omitempty"`
	Items      []Message `json:"items" nullable:"false"`
	NextCursor *string   `json:"nextCursor"`
	// HasNewer is set on a page read around a message when newer messages exist
	// after it: the page is a window into history, not the live end.
	HasNewer bool `json:"hasNewer,omitempty"`
}

// SavedMessage is one of the account's private bookmarks. It names the stored
// message by its canonical, account-scoped ID; the conversation and network IDs
// are stable per upstream network and target, so same-named channels elsewhere
// never resolve to it.
type SavedMessage struct {
	MessageID      string    `json:"messageId"`
	ConversationID string    `json:"conversationId"`
	NetworkID      string    `json:"networkId"`
	SavedAt        time.Time `json:"savedAt"`
}

// SavedRemoval reports a bookmark the account removed. Only the reference goes;
// the message itself is untouched.
type SavedRemoval struct {
	MessageID      string `json:"messageId"`
	ConversationID string `json:"conversationId"`
}

// InboxEntry is one item of the account's attention inbox: a message that
// mentioned the account, replied to one of its messages, or arrived in a
// direct conversation. There is at most one entry per canonical message, so a
// message matching several reasons, or seen again through replay or another
// device, never becomes a second item.
type InboxEntry struct {
	MessageID      string    `json:"messageId"`
	ConversationID string    `json:"conversationId"`
	NetworkID      string    `json:"networkId"`
	SenderID       string    `json:"senderId"`
	Reasons        []string  `json:"reasons" nullable:"false" enum:"mention,reply,direct" doc:"Why the message is in the inbox"`
	CreatedAt      time.Time `json:"createdAt"`
}

// InboxChange reports that the account dismissed or restored an inbox item.
// Dismissing is inbox-only: the conversation's read position is unchanged.
type InboxChange struct {
	MessageID string `json:"messageId"`
	Dismissed bool   `json:"dismissed"`
}
