package irc

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/language"
)

// profileText accepts one IRC metadata line, bounded in UTF-8 bytes.
func profileText(s string, limit int) bool {
	if len(s) > limit || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func profileColor(s string) bool {
	if s == "" {
		return true
	}
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, r := range s[1:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return false
		}
	}
	return true
}

func profileURL(s string, allowOwned bool, m *Manager, owner string) bool {
	if s == "" {
		return true
	}
	if !profileText(s, 300) {
		return false
	}
	u, err := url.Parse(s)
	return err == nil && u.Hostname() != "" && u.User == nil && (u.Scheme == "https" || allowOwned && u.Scheme == "http" && m != nil && m.options.Files != nil && m.options.Files.OwnsURL(owner, s))
}

func normalizedProfileList(values []string, languages bool) ([]string, error) {
	if len(values) > 10 {
		return nil, fmt.Errorf("profile lists accept at most 10 entries")
	}
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || !profileText(value, 32) {
			return nil, fmt.Errorf("invalid profile list entry")
		}
		if languages {
			for _, r := range value {
				if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
					return nil, fmt.Errorf("invalid language tag")
				}
			}
			tag, err := language.Parse(value)
			if err != nil {
				return nil, fmt.Errorf("invalid language tag")
			}
			value = tag.String()
			if len(value) > 32 {
				return nil, fmt.Errorf("invalid language tag")
			}
		}
		key := strings.ToLower(value)
		if !seen[key] {
			seen[key] = true
			out = append(out, value)
		}
	}
	encoded, _ := json.Marshal(out)
	if len(encoded) > 300 {
		return nil, fmt.Errorf("profile list exceeds 300 bytes")
	}
	return out, nil
}

func validateRichProfilePatch(m *Manager, owner string, p ProfilePatch) error {
	for _, v := range []struct {
		p   *string
		max int
	}{{p.Pronouns, 80}, {p.Headline, 120}, {p.Location, 120}, {p.StatusText, 200}, {p.StatusEmoji, 64}} {
		if v.p != nil && !profileText(*v.p, v.max) {
			return fmt.Errorf("invalid profile text")
		}
	}
	if p.HomepageURL != nil && !profileURL(*p.HomepageURL, false, m, owner) {
		return fmt.Errorf("profile URL must be an HTTPS URL")
	}
	for _, v := range []*string{p.BannerURL, p.ProfileBackgroundURL} {
		if v != nil && !profileURL(*v, true, m, owner) {
			return fmt.Errorf("profile URL must be an HTTPS URL")
		}
	}
	if p.ProfileAccentColor != nil && !profileColor(*p.ProfileAccentColor) {
		return fmt.Errorf("invalid profile accent color")
	}
	if p.AvatarFrame != nil {
		if len(*p.AvatarFrame) > 32 {
			return fmt.Errorf("invalid avatar frame")
		}
		for _, r := range *p.AvatarFrame {
			if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyz0123456789-", r) {
				return fmt.Errorf("invalid avatar frame")
			}
		}
	}
	if p.Timezone != nil && *p.Timezone != "" {
		if *p.Timezone == "Local" || !profileText(*p.Timezone, 64) {
			return fmt.Errorf("invalid timezone")
		}
		if _, err := time.LoadLocation(*p.Timezone); err != nil {
			return fmt.Errorf("invalid timezone")
		}
	}
	if p.StatusExpiresAt != nil && *p.StatusExpiresAt != "" {
		if !profileText(*p.StatusExpiresAt, 64) {
			return fmt.Errorf("invalid status expiry")
		}
		if _, err := time.Parse(time.RFC3339, *p.StatusExpiresAt); err != nil {
			return fmt.Errorf("invalid status expiry")
		}
	}
	if p.Languages != nil {
		if _, err := normalizedProfileList(*p.Languages, true); err != nil {
			return err
		}
	}
	if p.Interests != nil {
		if _, err := normalizedProfileList(*p.Interests, false); err != nil {
			return err
		}
	}
	return nil
}

func applyRichProfilePatch(v *Member, p ProfilePatch) {
	for _, pair := range []struct {
		src *string
		dst *string
	}{
		{p.Pronouns, &v.Pronouns}, {p.HomepageURL, &v.HomepageURL}, {p.StatusText, &v.StatusText}, {p.StatusEmoji, &v.StatusEmoji},
		{p.Headline, &v.Headline}, {p.BannerURL, &v.BannerURL}, {p.ProfileBackgroundURL, &v.ProfileBackgroundURL},
		{p.ProfileAccentColor, &v.ProfileAccentColor}, {p.AvatarFrame, &v.AvatarFrame}, {p.Location, &v.Location}, {p.Timezone, &v.Timezone},
	} {
		if pair.src != nil {
			*pair.dst = *pair.src
		}
	}
	// Editing a status without a deadline makes the new status permanent. A
	// deadline-only patch changes only its deadline.
	if p.StatusExpiresAt != nil {
		v.StatusExpiresAt = nil
		if *p.StatusExpiresAt != "" {
			t, _ := time.Parse(time.RFC3339, *p.StatusExpiresAt)
			v.StatusExpiresAt = &t
		}
	} else if p.StatusText != nil || p.StatusEmoji != nil {
		v.StatusExpiresAt = nil
	}
	if p.Languages != nil {
		v.Languages, _ = normalizedProfileList(*p.Languages, true)
	}
	if p.Interests != nil {
		v.Interests, _ = normalizedProfileList(*p.Interests, false)
	}
}

func richProfileValues(v Member) map[string]string {
	urlValue := func(s string) string {
		if profileURL(s, false, nil, "") {
			return s
		}
		return ""
	}
	list := func(values []string) string {
		if len(values) == 0 {
			return ""
		}
		b, _ := json.Marshal(values)
		return string(b)
	}
	return map[string]string{
		"pronouns": v.Pronouns, "homepage": urlValue(v.HomepageURL), "status": v.StatusText,
		"ircord/status-emoji": v.StatusEmoji, "ircord/headline": v.Headline,
		"ircord/banner": urlValue(v.BannerURL), "ircord/profile-background": urlValue(v.ProfileBackgroundURL),
		"ircord/profile-accent": v.ProfileAccentColor, "ircord/avatar-frame": v.AvatarFrame,
		"ircord/location": v.Location, "ircord/timezone": v.Timezone,
		"ircord/languages": list(v.Languages), "ircord/interests": list(v.Interests),
	}
}

func applyRichProfileMetadata(v *Member, key, value string) bool {
	p := ProfilePatch{}
	switch key {
	case "pronouns":
		p.Pronouns = &value
	case "homepage":
		if !profileURL(value, false, nil, "") {
			return false
		}
		p.HomepageURL = &value
	case "status":
		p.StatusText = &value
	case "ircord/status-emoji":
		p.StatusEmoji = &value
	case "ircord/headline":
		p.Headline = &value
	case "ircord/banner":
		if !profileURL(value, false, nil, "") {
			return false
		}
		p.BannerURL = &value
	case "ircord/profile-background":
		if !profileURL(value, false, nil, "") {
			return false
		}
		p.ProfileBackgroundURL = &value
	case "ircord/profile-accent":
		p.ProfileAccentColor = &value
	case "ircord/avatar-frame":
		p.AvatarFrame = &value
	case "ircord/location":
		p.Location = &value
	case "ircord/timezone":
		p.Timezone = &value
	case "ircord/languages", "ircord/interests":
		values := []string{}
		if value != "" && (len(value) > 300 || json.Unmarshal([]byte(value), &values) != nil || values == nil) {
			return false
		}
		if key == "ircord/languages" {
			p.Languages = &values
		} else {
			p.Interests = &values
		}
	default:
		return false
	}
	if validateRichProfilePatch(nil, "", p) != nil {
		return false
	}
	applyRichProfilePatch(v, p)
	return true
}
