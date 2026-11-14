package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/model"
)

type ProfileInput struct {
	Body    ProfileUpdate
	RawBody []byte
}
type NetworkProfileInput struct {
	ID      string `path:"id"`
	Body    ProfileUpdate
	RawBody []byte
}
type ProfileUpdate struct {
	DisplayName          *string   `json:"displayName,omitempty" nullable:"true" maxLength:"80" doc:"Explicit profile name, at most 80 UTF-8 bytes. Omit to retain it; send an empty string or null to clear it."`
	Bio                  *string   `json:"bio,omitempty" maxLength:"300" doc:"At most 300 characters in local storage. Omit to retain it; an empty string clears it."`
	NameColor            *string   `json:"nameColor,omitempty" nullable:"true" maxLength:"7" doc:"A #RRGGBB color. Null or empty clears; omission retains."`
	AvatarDataURL        *string   `json:"avatarDataUrl,omitempty" nullable:"true" maxLength:"180000" doc:"PNG, JPEG or WebP data URL, at most 180000 bytes. This takes precedence over avatarUrl."`
	AvatarURL            *string   `json:"avatarUrl,omitempty" nullable:"true" maxLength:"300" doc:"HTTPS URL, at most 300 UTF-8 bytes. An owned HTTP upload URL remains local and is not shared over IRC. Null or empty clears; omission retains."`
	Pronouns             *string   `json:"pronouns,omitempty" nullable:"true" maxLength:"80" doc:"Single-line text without controls, at most 80 UTF-8 bytes. Null or empty clears; omission retains."`
	HomepageURL          *string   `json:"homepageUrl,omitempty" nullable:"true" maxLength:"300" doc:"Absolute HTTPS URL without credentials, at most 300 UTF-8 bytes. Null or empty clears; omission retains."`
	StatusText           *string   `json:"statusText,omitempty" nullable:"true" maxLength:"200" doc:"Single-line text without controls, at most 200 UTF-8 bytes. Null or empty clears; omission retains."`
	StatusEmoji          *string   `json:"statusEmoji,omitempty" nullable:"true" maxLength:"64" doc:"Single-line text without controls, at most 64 UTF-8 bytes. Null or empty clears; omission retains."`
	StatusExpiresAt      *string   `json:"statusExpiresAt,omitempty" nullable:"true" maxLength:"64" doc:"RFC3339 status deadline, at most 64 bytes. Null or empty clears the deadline. Editing status without a deadline clears the old deadline."`
	Headline             *string   `json:"headline,omitempty" nullable:"true" maxLength:"120" doc:"Single-line text without controls, at most 120 UTF-8 bytes. Null or empty clears; omission retains."`
	BannerURL            *string   `json:"bannerUrl,omitempty" nullable:"true" maxLength:"300" doc:"HTTPS URL, at most 300 UTF-8 bytes. An owned HTTP upload URL remains local and is not shared over IRC. Null or empty clears; omission retains."`
	ProfileBackgroundURL *string   `json:"profileBackgroundUrl,omitempty" nullable:"true" maxLength:"300" doc:"HTTPS URL, at most 300 UTF-8 bytes. An owned HTTP upload URL remains local and is not shared over IRC. Null or empty clears; omission retains."`
	ProfileAccentColor   *string   `json:"profileAccentColor,omitempty" nullable:"true" maxLength:"7" doc:"A #RRGGBB color. Null or empty clears; omission retains."`
	AvatarFrame          *string   `json:"avatarFrame,omitempty" nullable:"true" maxLength:"32" doc:"Up to 32 ASCII lowercase letters, digits or hyphens. Null or empty clears; omission retains."`
	Location             *string   `json:"location,omitempty" nullable:"true" maxLength:"120" doc:"Single-line text without controls, at most 120 UTF-8 bytes. Null or empty clears; omission retains."`
	Timezone             *string   `json:"timezone,omitempty" nullable:"true" maxLength:"64" doc:"IANA timezone or UTC, at most 64 UTF-8 bytes. Null or empty clears; omission retains."`
	Languages            *[]string `json:"languages,omitempty" nullable:"true" maxItems:"10" doc:"Up to 10 BCP 47 tags of at most 32 UTF-8 bytes each. Compact JSON must fit 300 bytes. Null or an empty array clears; omission retains."`
	Interests            *[]string `json:"interests,omitempty" nullable:"true" maxItems:"10" doc:"Up to 10 entries of at most 32 UTF-8 bytes each. Compact JSON must fit 300 bytes. Null or an empty array clears; omission retains."`
}

type PreferencesInput struct {
	Body struct {
		Theme       *string `json:"theme,omitempty" enum:"dark,light"`
		Compact     *bool   `json:"compact,omitempty"`
		ShowMembers *bool   `json:"showMembers,omitempty"`
	}
}

func (s *Server) registerProfile() {
	huma.Patch(s.API, "/api/v1/profile", func(ctx context.Context, in *ProfileInput) (*Output[irc.Member], error) {
		return result(s.profile(ctx, "", in))
	})
	huma.Get(s.API, "/api/v1/networks/{id}/profile", func(ctx context.Context, in *struct {
		ID string `path:"id"`
	}) (*Output[irc.Member], error) {
		return result(s.Manager.NetworkProfile(owner(ctx), in.ID))
	})
	huma.Patch(s.API, "/api/v1/networks/{id}/profile", func(ctx context.Context, in *NetworkProfileInput) (*Output[irc.Member], error) {
		return result(s.profile(ctx, in.ID, &ProfileInput{Body: in.Body, RawBody: in.RawBody}))
	})
	if s.options.Auth == nil {
		huma.Patch(s.API, "/api/v1/preferences", func(ctx context.Context, in *PreferencesInput) (*Output[irc.Preferences], error) {
			p := s.Manager.Snapshot(owner(ctx)).Preferences
			if in.Body.Theme != nil {
				p.Theme = *in.Body.Theme
			}
			if in.Body.Compact != nil {
				p.Compact = *in.Body.Compact
			}
			if in.Body.ShowMembers != nil {
				p.ShowMembers = *in.Body.ShowMembers
			}
			return result(s.Manager.Preferences(owner(ctx), p), nil)
		})
	}
}

func (s *Server) profile(ctx context.Context, network string, in *ProfileInput) (model.Member, error) {
	p := irc.ProfilePatch{DisplayName: in.Body.DisplayName, Bio: in.Body.Bio, NameColor: in.Body.NameColor, AvatarURL: in.Body.AvatarURL,
		Pronouns: in.Body.Pronouns, HomepageURL: in.Body.HomepageURL, StatusText: in.Body.StatusText, StatusEmoji: in.Body.StatusEmoji, StatusExpiresAt: in.Body.StatusExpiresAt,
		Headline: in.Body.Headline, BannerURL: in.Body.BannerURL, ProfileBackgroundURL: in.Body.ProfileBackgroundURL, ProfileAccentColor: in.Body.ProfileAccentColor,
		AvatarFrame: in.Body.AvatarFrame, Location: in.Body.Location, Timezone: in.Body.Timezone, Languages: in.Body.Languages, Interests: in.Body.Interests}
	if !utf8.Valid(in.RawBody) || !validProfileJSONUnicode(in.RawBody) {
		return model.Member{}, errors.New("invalid UTF-8 in profile")
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(in.RawBody, &fields)
	for key, dst := range map[string]**string{"avatarUrl": &p.AvatarURL, "pronouns": &p.Pronouns, "homepageUrl": &p.HomepageURL, "statusText": &p.StatusText, "statusEmoji": &p.StatusEmoji, "statusExpiresAt": &p.StatusExpiresAt, "headline": &p.Headline, "bannerUrl": &p.BannerURL, "profileBackgroundUrl": &p.ProfileBackgroundURL, "profileAccentColor": &p.ProfileAccentColor, "avatarFrame": &p.AvatarFrame, "location": &p.Location, "timezone": &p.Timezone} {
		if raw, ok := fields[key]; ok && string(raw) == "null" {
			empty := ""
			*dst = &empty
		}
	}
	for key, dst := range map[string]**[]string{"languages": &p.Languages, "interests": &p.Interests} {
		if raw, ok := fields[key]; ok && string(raw) == "null" {
			empty := []string{}
			*dst = &empty
		}
	}
	if raw, ok := fields["bio"]; ok && string(raw) == "null" {
		return model.Member{}, errors.New("bio cannot be null")
	}
	if raw, ok := fields["displayName"]; ok && string(raw) == "null" {
		cleared := ""
		p.DisplayName = &cleared
	}
	if raw, ok := fields["nameColor"]; ok && string(raw) == "null" {
		empty := ""
		p.NameColor = &empty
	}
	var avatar []byte
	if raw, ok := fields["avatarDataUrl"]; ok {
		if string(raw) == "null" {
			empty := ""
			p.AvatarURL = &empty
		} else {
			// The image takes precedence over avatarUrl, as on existing clients.
			p.AvatarURL = nil
			var value string
			if json.Unmarshal(raw, &value) != nil || len(value) > 180000 {
				return model.Member{}, errors.New("invalid profile image")
			}
			prefix, encoded, ok := strings.Cut(value, ",")
			if !ok || !strings.Contains(prefix, ";base64") || (!strings.HasPrefix(prefix, "data:image/png;") && !strings.HasPrefix(prefix, "data:image/jpeg;") && !strings.HasPrefix(prefix, "data:image/webp;")) {
				return model.Member{}, errors.New("choose a PNG, JPEG or WebP profile image")
			}
			data, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil || len(data) > 128<<10 || len(data) == 0 {
				return model.Member{}, errors.New("profile image exceeds 128 KiB")
			}
			avatar = data
		}
	}
	// Capture authorization and identity before the upload, without keeping a
	// stale profile snapshot or holding the workspace lock during I/O.
	scope, err := s.Manager.PrepareProfilePatch(owner(ctx), network, p)
	if err != nil {
		return model.Member{}, err
	}
	if avatar != nil {
		if s.options.Files == nil {
			return model.Member{}, irc.ErrUnsupported
		}
		upload, err := s.options.Files.Upload(ctx, owner(ctx), "", "avatar", avatar, s.options.Files.AvatarProvider())
		if err != nil {
			return model.Member{}, err
		}
		p.AvatarURL = &upload.URL
	}
	return s.Manager.ApplyProfilePatch(owner(ctx), network, p, scope)
}

// encoding/json replaces unpaired UTF-16 surrogates with U+FFFD. Reject them
// before that replacement makes malformed profile input indistinguishable.
func validProfileJSONUnicode(raw []byte) bool {
	inside := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			inside = !inside
			continue
		}
		if !inside || raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) {
			return false
		}
		if raw[i] != 'u' {
			continue
		}
		read := func(at int) (uint16, bool) {
			if at+4 > len(raw) {
				return 0, false
			}
			var n uint16
			for _, c := range raw[at : at+4] {
				n <<= 4
				switch {
				case c >= '0' && c <= '9':
					n += uint16(c - '0')
				case c >= 'a' && c <= 'f':
					n += uint16(c - 'a' + 10)
				case c >= 'A' && c <= 'F':
					n += uint16(c - 'A' + 10)
				default:
					return 0, false
				}
			}
			return n, true
		}
		unit, ok := read(i + 1)
		if !ok {
			return false
		}
		i += 4
		if unit >= 0xdc00 && unit <= 0xdfff {
			return false
		}
		if unit >= 0xd800 && unit <= 0xdbff {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return false
			}
			low, ok := read(i + 3)
			if !ok || low < 0xdc00 || low > 0xdfff {
				return false
			}
			i += 6
		}
	}
	return true
}
