package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/lrstanley/girc"
	"ircord/backend/internal/irc"
)

type ConversationInput struct {
	Body struct {
		NetworkID string `json:"networkId"`
		Name      string `json:"name,omitempty"`
		Key       string `json:"key,omitempty"`
		MemberID  string `json:"memberId,omitempty"`
	}
}
type ConversationPath struct {
	ID     string `path:"id"`
	Reason string `query:"reason"`
}
type HistoryInput struct {
	ID     string `path:"id"`
	Before string `query:"before"`
	Around string `query:"around" maxLength:"200" doc:"Read a window centred on this message ID instead of the newest page; cannot be combined with before"`
	Limit  int    `query:"limit" default:"40" minimum:"1" maximum:"100"`
}
type AttractInput struct {
	ID string `path:"id"`
}
type ReactionInput struct {
	ID   string `path:"id"`
	Body struct {
		MessageID string `json:"messageId"`
		Emoji     string `json:"emoji"`
	}
}
type ConversationPatch struct {
	ID   string `path:"id"`
	Body struct {
		Read       *bool   `json:"read,omitempty"`
		UnreadFrom *string `json:"unreadFrom,omitempty" minLength:"1" maxLength:"200" doc:"Mark the conversation unread from this message on"`
		Favorite   *bool   `json:"favorite,omitempty"`
	}
}
type ChannelSettingsPatch struct {
	ID   string `path:"id"`
	Body struct {
		Notifications *string `json:"notifications,omitempty" enum:"default,all,mentions,none"`
		GroupMentions *string `json:"groupMentions,omitempty" enum:"all,ops,off"`
	}
}

func (s *Server) registerConversations() {
	manager := s.Manager
	huma.Delete(s.API, "/api/v1/invitations/{id}", func(ctx context.Context, in *struct {
		ID string `path:"id"`
	}) (*struct{}, error) {
		return empty(manager.RejectInvitation(owner(ctx), in.ID))
	})
	huma.Post(s.API, "/api/v1/conversations", func(ctx context.Context, in *ConversationInput) (*Output[irc.Conversation], error) {
		if in.Body.Name != "" {
			return result(manager.JoinKey(owner(ctx), in.Body.NetworkID, in.Body.Name, in.Body.Key))
		}
		if member, err := manager.Member(owner(ctx), in.Body.MemberID); err == nil {
			return result(manager.Direct(owner(ctx), in.Body.NetworkID, member.Nick))
		}
		if girc.IsValidNick(in.Body.MemberID) {
			return result(manager.Direct(owner(ctx), in.Body.NetworkID, in.Body.MemberID))
		}
		return result(irc.Conversation{}, irc.ErrNotFound)
	})
	huma.Patch(s.API, "/api/v1/conversations/{id}", func(ctx context.Context, in *ConversationPatch) (*Output[irc.Conversation], error) {
		if in.Body.UnreadFrom != nil {
			if in.Body.Read != nil && *in.Body.Read {
				return nil, huma.Error422UnprocessableEntity("read and unreadFrom cannot be combined")
			}
			if _, err := manager.MarkUnread(owner(ctx), in.ID, *in.Body.UnreadFrom); err != nil {
				return result(irc.Conversation{}, err)
			}
		}
		conv, err := manager.UpdateConversation(owner(ctx), in.ID, in.Body.Read, in.Body.Favorite)
		return result(conv, err)
	})
	huma.Register(s.API, huma.Operation{OperationID: "update-channel-settings", Method: "PUT", Path: "/api/v1/conversations/{id}/channel-settings", Summary: "Change channel-wide settings", Description: "Changes the notification level for everyone in the channel. Needs channel operator status or higher; every joined IRCord member receives the new values."}, func(ctx context.Context, in *ChannelSettingsPatch) (*Output[irc.Conversation], error) {
		return result(manager.SetChannelSettings(owner(ctx), in.ID, irc.ChannelSettingsInput{Notifications: in.Body.Notifications, GroupMentions: in.Body.GroupMentions}))
	})
	huma.Delete(s.API, "/api/v1/conversations/{id}", func(ctx context.Context, in *ConversationPath) (*struct{}, error) {
		return empty(manager.PartReason(owner(ctx), in.ID, in.Reason))
	})
	huma.Get(s.API, "/api/v1/conversations/{id}/messages", func(ctx context.Context, in *HistoryInput) (*Output[irc.MessagePage], error) {
		if in.Around != "" {
			if in.Before != "" {
				return nil, huma.Error422UnprocessableEntity("around and before cannot be combined")
			}
			return result(manager.HistoryAroundContext(ctx, owner(ctx), in.ID, in.Around, in.Limit))
		}
		return result(manager.HistoryContext(ctx, owner(ctx), in.ID, in.Before, in.Limit))
	})
	huma.Post(s.API, "/api/v1/conversations/{id}/reactions", func(ctx context.Context, in *ReactionInput) (*Output[irc.Message], error) {
		return result(manager.ToggleReaction(owner(ctx), in.ID, in.Body.MessageID, in.Body.Emoji))
	})
	huma.Post(s.API, "/api/v1/conversations/{id}/attract", func(ctx context.Context, in *AttractInput) (*Output[irc.Message], error) {
		return result(manager.Attract(owner(ctx), in.ID))
	})
}
