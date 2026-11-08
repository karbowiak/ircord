package model

import (
	"encoding/json"
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
	Role        string `json:"role" enum:"member,operator,bot"`
	Bio         string `json:"bio"`
	Account     string `json:"account"`
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
	Replies     bool `json:"replies"`
	Reactions   bool `json:"reactions"`
	Typing      bool `json:"typing"`
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
}
type MultilineLimits struct {
	// Limits include IRCord's own safety bounds and the negotiated server bounds.
	MaxBytes int `json:"maxBytes" minimum:"1" maximum:"4096"`
	MaxLines int `json:"maxLines" minimum:"1" maximum:"100"`
}
type Capabilities struct {
	ServerPreviews bool              `json:"serverPreviews"`
	Upload         *UploadCapability `json:"upload,omitempty"`
}
type UploadCapability struct {
	MaxBytes int64    `json:"maxBytes"`
	Types    []string `json:"types" nullable:"false"`
}
type Removal struct {
	ID string `json:"id"`
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
	PreviewMode   string     `json:"previewMode" enum:"client,server" doc:"Whether IRCord's server fetches link previews for this channel."`
	UpdatedBy     string     `json:"updatedBy,omitempty" doc:"IRC nick of the operator who last changed these settings."`
	UpdatedAt     *time.Time `json:"updatedAt,omitempty"`
}

type Conversation struct {
	PreviewMode  string            `json:"previewMode,omitempty" enum:"client,server" doc:"Effective preview mode. For channels it follows channelSettings; for direct messages it is personal."`
	// ChannelSettings is present on joined channels once the shared values are known.
	ChannelSettings *ChannelSettings `json:"channelSettings,omitempty"`
	ModeParams   map[string]string `json:"modeParams,omitempty"`
	TopicSetBy   string            `json:"topicSetBy,omitempty"`
	TopicSetAt   *time.Time        `json:"topicSetAt,omitempty"`
	CreatedAt    *time.Time        `json:"createdAt,omitempty"`
	URL          string            `json:"url,omitempty"`
	MemberModes  map[string]string `json:"memberModes" nullable:"false"`
	Modes        string            `json:"modes"`
	ID           string            `json:"id"`
	NetworkID    string            `json:"networkId"`
	Kind         string            `json:"kind" enum:"channel,direct"`
	Name         string            `json:"name"`
	Target       string            `json:"target,omitempty"`
	Topic        string            `json:"topic"`
	UnreadCount  int               `json:"unreadCount"`
	MentionCount int               `json:"mentionCount"`
	UnreadFrom   string            `json:"unreadFrom,omitempty" doc:"Message a manual mark-as-unread starts at; cleared when the conversation is read again"`
	Favorite     bool              `json:"favorite"`
	MemberIDs    []string          `json:"memberIds" nullable:"false"`
	Joined       bool              `json:"joined"`
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
	Type     string `json:"type" enum:"join,part,quit,nick,kick,mode,topic,invite"`
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
type LinkPreview struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Image       string `json:"image,omitempty"`
	SiteName    string `json:"siteName,omitempty"`
	// Kind picks the card a client renders: an ordinary website, a news/article page (headline
	// with a lead image), or a video page. Absent means website.
	Kind string `json:"kind,omitempty" enum:"website,article,video"`
	// Author is the channel or byline the provider names, when it names one.
	Author string `json:"author,omitempty"`
	// ImageWidth and ImageHeight are the lead image's declared size, so clients reserve its frame.
	ImageWidth  int `json:"imageWidth,omitempty"`
	ImageHeight int `json:"imageHeight,omitempty"`
	// Images are the validated images of a gallery or album page, in page order. Present only
	// when the page declares at least two.
	Images []PreviewImage `json:"images,omitempty" maxItems:"10"`
	// Media is inline media resolved from a supported share page (for example a Tenor GIF).
	Media *PreviewMedia `json:"media,omitempty"`
	// Embed names a supported provider player. Clients build the player URL from these fields
	// themselves; no markup or player URL from the page is ever passed on.
	Embed *PreviewEmbed `json:"embed,omitempty"`
}

// PreviewImage is one image of a gallery page.
type PreviewImage struct {
	URL    string `json:"url"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// PreviewEmbed identifies content a known provider can play inside the client.
type PreviewEmbed struct {
	Provider string `json:"provider" enum:"youtube"`
	ID       string `json:"id" pattern:"^[A-Za-z0-9_-]{11}$"`
	// Start is the offset in seconds the link asked playback to start at.
	Start int `json:"start,omitempty" minimum:"0"`
	// Short is a vertical YouTube Short.
	Short bool `json:"short,omitempty"`
	// Embeddable is false when the provider does not allow playback outside its own site.
	Embeddable bool `json:"embeddable"`
	// Unavailable reports that the provider says the content does not exist or is private.
	Unavailable bool `json:"unavailable,omitempty"`
}

// PreviewMedia is a directly loadable media file a share page points at.
type PreviewMedia struct {
	URL    string `json:"url"`
	Kind   string `json:"kind" enum:"gif"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}
type PreviewResult struct {
	Mode    string       `json:"mode" enum:"client,server"`
	Preview *LinkPreview `json:"preview"`
	Reason  string       `json:"reason,omitempty"`
}
type Message struct {
	Preview       *LinkPreview     `json:"preview,omitempty"`
	PreviewSource string           `json:"previewSource,omitempty" enum:"client,server"`
	Deleted       bool             `json:"deleted,omitempty"`
	Mention       bool             `json:"-"`
	Position      int64            `json:"position,omitempty"`
	Event         *MembershipEvent `json:"event,omitempty"`
	Attachment    *Attachment      `json:"attachment,omitempty"`

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
	Attachments    []string `json:"attachments,omitempty"`
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
	SyncError  string    `json:"syncError,omitempty"`
	Items      []Message `json:"items" nullable:"false"`
	NextCursor *string   `json:"nextCursor"`
}
