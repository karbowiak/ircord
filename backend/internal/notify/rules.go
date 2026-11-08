package notify

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

type QuietHours struct {
	Enabled  bool   `json:"enabled"`
	Start    string `json:"start"`
	End      string `json:"end"`
	TimeZone string `json:"timeZone"`
}
type Settings struct {
	ChannelDefault string            `json:"channelDefault"`
	DirectDefault  string            `json:"directDefault"`
	Levels         map[string]string `json:"levels"`
	HighlightWords []string          `json:"highlightWords"`
	Ignore         []string          `json:"ignore"`
	QuietHours     QuietHours        `json:"quietHours"`
	// Routing across devices (#84) and automatic away (#136), in seconds.
	LastDeviceDelay int `json:"lastDeviceDelay" minimum:"0" maximum:"3600" doc:"Seconds without an active IRCord client before the most recently active device alone is alerted. 0 alerts it as soon as no client is active."`
	AllDevicesDelay int `json:"allDevicesDelay" minimum:"0" maximum:"86400" doc:"Seconds without an active client before every device is alerted, with one reminder of what is still unread. At least lastDeviceDelay; equal values skip the last-device-only window."`
	AutoAwayDelay   int `json:"autoAwayDelay" minimum:"0" maximum:"86400" doc:"Seconds with no device connected before you are marked away on IRC. 0 never marks you away automatically."`
}

// Default routing and away delays, in seconds.
const (
	DefaultLastDeviceDelay = 30
	DefaultAllDevicesDelay = 300
	DefaultAutoAwayDelay   = 600
	MaxLastDeviceDelay     = 3600
	MaxDelay               = 86400
)

type Policy struct {
	Settings   Settings
	ignored    []*regexp.Regexp
	highlights []*regexp.Regexp
	zone       *time.Location
}

func Defaults() Settings {
	return Settings{ChannelDefault: "mentions", DirectDefault: "all", Levels: map[string]string{}, HighlightWords: []string{}, Ignore: []string{}, QuietHours: QuietHours{Start: "22:00", End: "07:00", TimeZone: "UTC"}, LastDeviceDelay: DefaultLastDeviceDelay, AllDevicesDelay: DefaultAllDevicesDelay, AutoAwayDelay: DefaultAutoAwayDelay}
}
func Compile(raw []byte) *Policy {
	settings := Defaults()
	_ = json.Unmarshal(raw, &settings)
	return New(settings)
}

var clock = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

func level(v string, fallback string) string {
	if v == "all" || v == "mentions" || v == "none" {
		return v
	}
	return fallback
}
func words(values []string, count, max int) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || len(v) > max || strings.ContainsAny(v, "\r\n\x00") || seen[v] {
			continue
		}
		seen[v] = true
		result = append(result, v)
		if len(result) >= count {
			break
		}
	}
	return result
}
func New(settings Settings) *Policy {
	settings.ChannelDefault = level(settings.ChannelDefault, "mentions")
	settings.DirectDefault = level(settings.DirectDefault, "all")
	levels := map[string]string{}
	for id, v := range settings.Levels {
		if len(levels) >= 500 {
			break
		}
		if id != "" && len(id) <= 200 && level(v, "") != "" {
			levels[id] = v
		}
	}
	settings.Levels = levels
	settings.LastDeviceDelay = bound(settings.LastDeviceDelay, MaxLastDeviceDelay)
	settings.AllDevicesDelay = bound(settings.AllDevicesDelay, MaxDelay)
	if settings.AllDevicesDelay < settings.LastDeviceDelay {
		settings.AllDevicesDelay = settings.LastDeviceDelay
	}
	settings.AutoAwayDelay = bound(settings.AutoAwayDelay, MaxDelay)
	settings.HighlightWords = words(settings.HighlightWords, 50, 64)
	settings.Ignore = words(settings.Ignore, 200, 120)
	if !clock.MatchString(settings.QuietHours.Start) {
		settings.QuietHours.Start = "22:00"
	}
	if !clock.MatchString(settings.QuietHours.End) {
		settings.QuietHours.End = "07:00"
	}
	if settings.QuietHours.TimeZone == "" || settings.QuietHours.TimeZone == "Local" {
		settings.QuietHours.TimeZone = "UTC"
	}
	zone, err := time.LoadLocation(settings.QuietHours.TimeZone)
	if err != nil {
		settings.QuietHours.TimeZone = "UTC"
		zone = time.UTC
	}
	p := &Policy{Settings: settings, zone: zone}
	for _, mask := range settings.Ignore {
		if !strings.ContainsAny(mask, "!@") {
			mask += "!*@*"
		}
		pattern := regexp.QuoteMeta(mask)
		pattern = strings.ReplaceAll(strings.ReplaceAll(pattern, `\*`, `.*`), `\?`, `.`)
		p.ignored = append(p.ignored, regexp.MustCompile(`(?i)^`+pattern+`$`))
	}
	for _, word := range settings.HighlightWords {
		p.highlights = append(p.highlights, boundary(word))
	}
	return p
}
func bound(v, max int) int {
	if v < 0 {
		return 0
	}
	if v > max {
		return max
	}
	return v
}

// Delays are the escalation thresholds: the most recently active device, then every device.
func (p *Policy) Delays() (last, all time.Duration) {
	return time.Duration(p.Settings.LastDeviceDelay) * time.Second, time.Duration(p.Settings.AllDevicesDelay) * time.Second
}

func boundary(word string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)(^|[^a-z0-9_-])@?` + regexp.QuoteMeta(word) + `($|[^a-z0-9_-])`)
}

var formatting = regexp.MustCompile("\\x03[0-9]{0,2}(,[0-9]{1,2})?|\\x04[0-9a-fA-F]{0,6}(,[0-9a-fA-F]{1,6})?|[\\x02\\x0f\\x11\\x16\\x1d\\x1e\\x1f]")

func Plain(text string) string { return formatting.ReplaceAllString(text, "") }
func (p *Policy) Ignored(nick, user, host string) bool {
	if user == "" {
		user = "*"
	}
	if host == "" {
		host = "*"
	}
	value := nick + "!" + user + "@" + host
	for _, pattern := range p.ignored {
		if pattern.MatchString(value) {
			return true
		}
	}
	return false
}
func (p *Policy) Highlight(text, nick string) bool {
	text = Plain(text)
	if nick != "" && boundary(nick).MatchString(text) {
		return true
	}
	for _, pattern := range p.highlights {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}
func (p *Policy) Quiet(now time.Time) bool {
	q := p.Settings.QuietHours
	if !q.Enabled || q.Start == q.End {
		return false
	}
	at := now.In(p.zone).Format("15:04")
	if q.Start < q.End {
		return at >= q.Start && at < q.End
	}
	return at >= q.Start || at < q.End
}

// Rule is the notification level for a conversation. Channels follow the
// channel-wide level (shared); "default" or "" falls back to this account's
// own channel default. Per-account levels apply to direct messages only, so a
// personal value saved for a channel never overrides the channel's level.
func (p *Policy) Rule(conversation, kind, shared string) string {
	if kind == "direct" {
		if v, ok := p.Settings.Levels[conversation]; ok {
			return v
		}
		return p.Settings.DirectDefault
	}
	if v := level(shared, ""); v != "" {
		return v
	}
	return p.Settings.ChannelDefault
}

// Notification decides whether a message notifies: "mention", "message" or "".
// Quiet hours, ignores and highlight words stay personal.
func (p *Policy) Notification(conversation, kind, shared, messageKind, text string, me Recipient, sender, user, host string, self bool, now time.Time) string {
	if self || messageKind == "event" || messageKind == "system" || p.Ignored(sender, user, host) || p.Quiet(now) {
		return ""
	}
	rule := p.Rule(conversation, kind, shared)
	if rule == "none" {
		return ""
	}
	if kind == "direct" || p.Mentioned(text, me) {
		return "mention"
	}
	if rule == "all" {
		return "message"
	}
	return ""
}

// Attention lists why a message belongs in the account's silent inbox:
// "mention" (its nick, a highlight word or a group mention that includes it), "reply" (a reply to one of its own
// messages) and "direct" (any message of a direct conversation). It applies the
// same eligibility as Notification — ignores, own messages, events and a level
// of "none" — but not quiet hours: those only hold back interruptions, so the
// inbox still records what arrived during them.
func (p *Policy) Attention(conversation, kind, shared, messageKind, text string, me Recipient, sender, user, host string, self, replyToSelf bool) []string {
	if self || messageKind == "event" || messageKind == "system" || p.Ignored(sender, user, host) || p.Rule(conversation, kind, shared) == "none" {
		return nil
	}
	reasons := []string{}
	if p.Mentioned(text, me) {
		reasons = append(reasons, "mention")
	}
	if replyToSelf {
		reasons = append(reasons, "reply")
	}
	if kind == "direct" {
		reasons = append(reasons, "direct")
	}
	if len(reasons) == 0 {
		return nil
	}
	return reasons
}
