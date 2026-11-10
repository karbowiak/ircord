package linkmedia

// Public URL conventions adapted from Iframely plugins/domains/google.com at
// 962bf9287f2ab5880ff96d4ac796ebb62217be57 (MIT, Itteco 2013-2022).
// See iframely-providers.LICENSE.txt. Private enabling flags and API keys are
// deliberately excluded. Calendar eligibility uses only its public iCal feed:
// https://support.google.com/calendar/answer/37083

import (
	"context"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/artyom/oembed"
	parse "golang.org/x/net/html"
	"ircord/backend/internal/mediafetch"
)

type googleDocument struct {
	kind, id, action string
	published        bool
}

var googleWorkspace = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}$`)
var googleAccount = regexp.MustCompile(`^[0-9]{1,2}$`)
var googleCalendarID = regexp.MustCompile(`^[A-Za-z0-9_.+%#@-]{1,300}$`)
var googleMapID = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,200}$`)

func googleQueryKeys(u *url.URL, allowed string) bool {
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(u.RawQuery) > 4096 || len(q) > 20 || !publicProviderQuery(u, 0) {
		return false
	}
	for key, values := range q {
		if !strings.Contains("|"+allowed+"|", "|"+key+"|") || len(values) != 1 && key != "src" {
			return false
		}
	}
	return true
}

func googleDocumentURL(u *url.URL) (d googleDocument, ok bool) {
	if u == nil || u.Scheme != "https" || u.Hostname() != "docs.google.com" && u.Hostname() != "drive.google.com" ||
		!googleQueryKeys(u, "id|usp|pli|sle|embedded|gid|single|widget|headers|start|loop|delayms|rm|minimal|chrome|hl|output") {
		return
	}
	if u.Hostname() == "drive.google.com" && u.Path == "/open" && providerID.MatchString(u.Query().Get("id")) {
		return googleDocument{kind: "file", id: u.Query().Get("id"), action: "open"}, true
	}
	p := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(p) > 2 && p[0] == "a" && googleWorkspace.MatchString(p[1]) {
		p = p[2:]
	}
	if len(p) < 3 || !strings.Contains("|document|spreadsheets|presentation|forms|drawings|file|", "|"+p[0]+"|") {
		return
	}
	d.kind, p = p[0], p[1:]
	if len(p) > 2 && p[0] == "u" && googleAccount.MatchString(p[1]) {
		p = p[2:]
	}
	if len(p) < 2 || p[0] != "d" {
		return d, false
	}
	p = p[1:]
	if p[0] == "e" {
		d.published, p = true, p[1:]
	}
	if len(p) < 1 || len(p) > 2 || !providerID.MatchString(p[0]) {
		return d, false
	}
	d.id = p[0]
	if len(p) == 2 {
		d.action = p[1]
	}
	actions := map[string]string{
		"document":     "|edit|preview|pub|mobilebasic|htmlview",
		"spreadsheets": "|edit|preview|pubhtml|htmlview",
		"presentation": "|edit|preview|pub|embed",
		"forms":        "|edit|viewform",
		"drawings":     "|edit|preview|pub",
		"file":         "|view|preview",
	}
	if !strings.Contains("|"+actions[d.kind]+"|", "|"+d.action+"|") ||
		u.Hostname() == "drive.google.com" && d.kind != "file" || d.kind == "file" && d.published {
		return d, false
	}
	return d, true
}

func googleCalendarQuery(u *url.URL) (url.Values, bool) {
	if u == nil || u.Scheme != "https" || u.Hostname() != "calendar.google.com" && u.Hostname() != "www.google.com" ||
		!googleQueryKeys(u, "src|ctz|mode|showTitle|showNav|showDate|showPrint|showTabs|showCalendars|showTz|wkst|hl|color|bgcolor|title|height|width") {
		return nil, false
	}
	p := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(p) == 4 && p[0] == "calendar" && p[1] == "u" && googleAccount.MatchString(p[2]) {
		p = []string{p[0], p[3]}
	}
	if len(p) != 2 || p[0] != "calendar" || p[1] != "embed" && p[1] != "newembed" {
		return nil, false
	}
	input, q := u.Query(), url.Values{}
	if len(input["src"]) < 1 || len(input["src"]) > 4 {
		return nil, false
	}
	for _, id := range input["src"] {
		if !googleCalendarID.MatchString(id) || strings.Contains(id, "%") {
			return nil, false
		}
		q.Add("src", id)
	}
	if zone := input.Get("ctz"); zone != "" {
		if _, err := time.LoadLocation(zone); err != nil {
			return nil, false
		}
		q.Set("ctz", zone)
	}
	if mode := input.Get("mode"); mode != "" {
		if mode != "MONTH" && mode != "WEEK" && mode != "AGENDA" {
			return nil, false
		}
		q.Set("mode", mode)
	}
	for _, key := range []string{"showTitle", "showNav", "showDate", "showPrint", "showTabs", "showCalendars", "showTz"} {
		if value := input.Get(key); value == "0" || value == "1" {
			q.Set(key, value)
		}
	}
	return q, true
}

func googleMapQuery(u *url.URL) (string, url.Values, bool) {
	if u == nil || u.Scheme != "https" || u.Hostname() != "google.com" && u.Hostname() != "www.google.com" ||
		!googleQueryKeys(u, "mid|pb|ll|z|hl|usp") {
		return "", nil, false
	}
	p := strings.TrimSuffix(u.Path, "/")
	if p == "/maps/embed" && len(u.Query()) == 1 {
		pb := u.Query().Get("pb")
		if pb != "" && len(pb) <= 3500 && !strings.ContainsAny(pb, "\x00\r\n\\") {
			return "/maps/embed", url.Values{"pb": {pb}}, true
		}
	}
	p = strings.Replace(p, "/maps/d/u/0/", "/maps/d/", 1)
	if (p == "/maps/d/viewer" || p == "/maps/d/edit" || p == "/maps/d/embed") && googleMapID.MatchString(u.Query().Get("mid")) && !u.Query().Has("pb") {
		return "/maps/d/embed", url.Values{"mid": {u.Query().Get("mid")}}, true
	}
	return "", nil, false
}

// Every frame is reconstructed from the posted content identity. Neither an
// oEmbed result nor a schema embedURL can switch documents, calendars or maps.
func googlePlayer(name string, source, frame *url.URL) (string, int, float64, bool) {
	height, ratio := 360, 4.0/3
	q := url.Values{}
	switch name {
	case "google-docs":
		s, ok := googleDocumentURL(source)
		f, valid := googleDocumentURL(frame)
		if !ok || !valid || s.id != f.id || s.published != f.published || s.kind != f.kind && s.kind != "file" ||
			f.kind == "file" && frame.Hostname() != "drive.google.com" || f.action == "edit" || f.action == "view" || f.action == "open" || f.action == "" {
			return "", 0, 0, false
		}
		switch s.kind {
		case "file", "document":
			height, ratio = 640, 3.0/4
		case "forms":
			if f.action != "viewform" {
				return "", 0, 0, false
			}
			height, ratio = 640, 0
			q.Set("embedded", "true")
		case "presentation":
			height, ratio = 270, 16.0/9
			q.Set("start", "false")
			q.Set("loop", "false")
		case "spreadsheets":
			for _, key := range []string{"gid", "single", "widget", "headers"} {
				value := source.Query().Get(key)
				if value == "" {
					value = frame.Query().Get(key)
				}
				if value == "" {
					continue
				}
				if key == "gid" && !decimalID.MatchString(value) || key != "gid" && value != "true" && value != "false" {
					return "", 0, 0, false
				}
				q.Set(key, value)
			}
		}
		// Remove account/workspace prefixes from the validated frame path.
		frame.Path = "/" + f.kind + "/d/"
		if f.published {
			frame.Path += "e/"
		}
		action := f.action
		if s.kind == "presentation" && (action == "preview" || action == "pub") {
			action = "embed"
		}
		frame.Path += f.id + "/" + action
	case "google-calendar":
		var ok, valid bool
		q, ok = googleCalendarQuery(source)
		fq, valid := googleCalendarQuery(frame)
		if !ok || !valid {
			return "", 0, 0, false
		}
		a, b := append([]string{}, q["src"]...), append([]string{}, fq["src"]...)
		sort.Strings(a)
		sort.Strings(b)
		if strings.Join(a, "\n") != strings.Join(b, "\n") {
			return "", 0, 0, false
		}
		frame.Host, frame.Path = "calendar.google.com", "/calendar/embed"
	case "google-maps":
		path, sq, ok := googleMapQuery(source)
		_, fq, valid := googleMapQuery(frame)
		if !ok || !valid || sq.Encode() != fq.Encode() {
			return "", 0, 0, false
		}
		frame.Host, frame.Path, q = "www.google.com", path, sq
	default:
		return "", 0, 0, false
	}
	frame.RawQuery, frame.Fragment = q.Encode(), ""
	return frame.String(), height, ratio, true
}

func googlePageSource(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	switch u.Hostname() {
	case "docs.google.com", "drive.google.com":
		return true
	case "calendar.google.com":
		return strings.HasPrefix(u.Path, "/calendar/")
	case "google.com", "www.google.com":
		return strings.HasPrefix(u.Path, "/maps/") || strings.HasPrefix(u.Path, "/calendar/")
	}
	return false
}

func googleSchemaEmbed(data []byte) string {
	z := parse.NewTokenizer(strings.NewReader(string(data)))
	z.SetMaxBuf(maxPageBytes)
	for {
		switch z.Next() {
		case parse.ErrorToken:
			return ""
		case parse.StartTagToken, parse.SelfClosingTagToken:
			t := z.Token()
			item, target := "", ""
			for _, a := range t.Attr {
				switch a.Key {
				case "itemprop":
					item = a.Val
				case "content", "href":
					target = a.Val
				}
			}
			if (item == "embedURL" || item == "embedUrl") && len(target) <= 4096 {
				return target
			}
		}
	}
}

// Google embeds never use generic discovery, third-party document viewers or
// browser credentials. An anonymous login redirect cannot establish authority.
func (r *Resolver) resolveGooglePage(ctx context.Context, raw string) (Result, error) {
	fallback := func(title, reason string) (Result, error) {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		return infoCard(raw, title, "", reason, 5*time.Minute), nil
	}
	u, err := mediafetch.URL(raw)
	if err != nil {
		return Result{}, err
	}
	p := publicProviderFor(u)
	if p == nil {
		return fallback("", "unsupported_type")
	}
	fetchURL := raw
	if d, valid := googleDocumentURL(u); valid && d.kind == "forms" {
		prefix := "https://docs.google.com/forms/d/"
		if d.published {
			prefix += "e/"
		}
		// Read the public response page, never the editor or submission endpoint.
		fetchURL = prefix + d.id + "/viewform?embedded=true"
	}
	page, err := r.fetch(ctx, fetchURL, "text/html, application/xhtml+xml;q=0.9")
	if err != nil {
		return fallback("", failureReason(err))
	}
	defer page.Body.Close()
	if page.StatusCode == http.StatusUnauthorized || page.StatusCode == http.StatusForbidden {
		return fallback("", "restricted")
	}
	if page.StatusCode != http.StatusOK || page.Request == nil || page.Request.URL == nil {
		return fallback("", "fetch_failed")
	}
	final := page.Request.URL
	if final.User != nil || final.RawPath != "" || final.Port() != "" && final.Port() != "443" || !publicProviderQuery(final, 0) {
		return fallback("", "restricted")
	}
	data, err := io.ReadAll(io.LimitReader(page.Body, maxPageBytes+1))
	if err != nil || len(data) > maxPageBytes || !isHTML(data) {
		return fallback("", "fetch_failed")
	}
	m := readPageMetadata(data)
	title, _ := pageText(data)
	if m.title != "" {
		title = m.title
	}
	for _, marker := range []string{"accounts.google.com/ServiceLogin", "accounts.google.com/v3/signin", "id=\"gaia_loginform\""} {
		if strings.Contains(string(data), marker) && googleSchemaEmbed(data) == "" {
			return fallback("", "restricted")
		}
	}
	candidate := ""
	if p.name == "google-docs" {
		s, _ := googleDocumentURL(u)
		f, valid := googleDocumentURL(final)
		if !valid || s.id != f.id || s.published != f.published || s.kind != f.kind && s.kind != "file" {
			return fallback(title, "restricted")
		}
		candidate = googleSchemaEmbed(data)
		if candidate == "" && title != "" && (f.action == "pub" || f.action == "pubhtml" || f.action == "mobilebasic" || f.action == "htmlview" || f.kind == "forms" && f.action == "viewform" && m.title != "") {
			candidate = final.String()
		}
		if s.kind == "file" && candidate != "" {
			// A Drive player can advertise a different player origin. The only
			// accepted replacement is its own verified file preview.
			c, e := mediafetch.URL(candidate)
			if e != nil {
				return fallback(title, "restricted")
			}
			if d, valid := googleDocumentURL(c); !valid || d.id != s.id || d.published != s.published {
				return fallback(title, "restricted")
			}
			candidate = "https://drive.google.com/file/d/" + s.id + "/preview"
		}
	} else {
		// Binding the final route detects redirects to login, another map or
		// another calendar even when that destination replies with HTTP 200.
		if _, _, _, valid := googlePlayer(p.name, u, final); !valid {
			return fallback(title, "restricted")
		}
		candidate = final.String()
		if p.name == "google-maps" {
			path, q, _ := googleMapQuery(u)
			if q.Has("mid") && m.title == "" {
				return fallback(title, "restricted")
			}
			candidate = "https://www.google.com" + path + "?" + q.Encode()
		} else if q, valid := googleCalendarQuery(u); valid {
			for _, id := range q["src"] {
				if public, reason := r.googleCalendarPublic(ctx, id); !public {
					return fallback(title, reason)
				}
			}
			candidate = "https://calendar.google.com/calendar/embed?" + q.Encode()
		}
	}
	frame, _, _, ok := ControlledPlayer(raw, candidate, "")
	if !ok {
		return fallback(title, "unsupported_type")
	}
	return r.oembedResult(ctx, raw, &oembed.Metadata{Type: oembed.TypeRich, Provider: "Google", Title: title, HTML: `<iframe src="` + html.EscapeString(frame) + `"></iframe>`}, 24*time.Hour)
}

func (r *Resolver) googleCalendarPublic(ctx context.Context, id string) (bool, string) {
	target := "https://calendar.google.com/calendar/ical/" + url.PathEscape(id) + "/public/basic.ics"
	res, err := r.fetch(ctx, target, "text/calendar")
	if err != nil {
		return false, failureReason(err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return false, "restricted"
	}
	if res.StatusCode != http.StatusOK || res.Request == nil || res.Request.URL == nil || res.Request.URL.String() != target {
		return false, "fetch_failed"
	}
	// Checking only the public feed signature avoids retaining calendar events.
	prefix, err := io.ReadAll(io.LimitReader(res.Body, 512))
	if err != nil || !strings.HasPrefix(strings.TrimSpace(string(prefix)), "BEGIN:VCALENDAR") {
		return false, "fetch_failed"
	}
	return true, ""
}
