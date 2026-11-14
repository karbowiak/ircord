package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

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
	DisplayName   *string `json:"displayName,omitempty" nullable:"true" maxLength:"100" doc:"Explicit profile name. Omit to keep the current one; send an empty string or null to clear it and fall back to the network real name or nickname."`
	Bio           *string `json:"bio,omitempty" maxLength:"300" doc:"Omit to keep the current biography; send an empty string to clear it."`
	NameColor     *string `json:"nameColor,omitempty" nullable:"true"`
	AvatarDataURL *string `json:"avatarDataUrl,omitempty" nullable:"true" maxLength:"180000"`
	AvatarURL     *string `json:"avatarUrl,omitempty" nullable:"true"`
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
	p := irc.ProfilePatch{DisplayName: in.Body.DisplayName, Bio: in.Body.Bio, NameColor: in.Body.NameColor, AvatarURL: in.Body.AvatarURL}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(in.RawBody, &fields)
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
