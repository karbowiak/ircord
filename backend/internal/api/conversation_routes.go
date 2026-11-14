package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/irc"
)

type ConversationInput struct {
	Body struct {
		NetworkID string `json:"networkId"`
		Name      string `json:"name,omitempty"`
		Key       string `json:"key,omitempty"`
		MemberID  string `json:"memberId,omitempty"`
		Nickname  string `json:"nickname,omitempty" maxLength:"128" doc:"An IRC nickname on networkId; cannot be combined with name or memberId."`
	}
}
type ConversationPath struct {
	ID     string `path:"id"`
	Reason string `query:"reason"`
}
type HistoryInput struct {
	ID     string `path:"id"`
	Before string `query:"before" doc:"Opaque older-page continuation for this account and conversation. Use nextCursor from this endpoint; it is not an event replay cursor."`
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
		Read                 *bool   `json:"read,omitempty"`
		ReadThroughMessageID *string `json:"readThroughMessageId,omitempty" minLength:"1" maxLength:"200" doc:"Acknowledge through this observed conversation message without clearing a manual unread boundary."`
		UnreadFrom           *string `json:"unreadFrom,omitempty" minLength:"1" maxLength:"200" doc:"Mark the conversation unread from this message on"`
		Favorite             *bool   `json:"favorite,omitempty"`
	}
}
type ChannelSettingsPatch struct {
	ID   string `path:"id"`
	Body struct {
		Notifications *string `json:"notifications,omitempty" enum:"default,all,mentions,none"`
		GroupMentions *string `json:"groupMentions,omitempty" enum:"all,ops,off"`
		NSFW          *bool   `json:"nsfw,omitempty"`
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
		targets := 0
		for _, value := range []string{in.Body.Name, in.Body.MemberID, in.Body.Nickname} {
			if value != "" {
				targets++
			}
		}
		if targets != 1 || in.Body.Key != "" && in.Body.Name == "" {
			return nil, huma.Error422UnprocessableEntity("Choose one channel name, member ID, or nickname.")
		}
		if in.Body.Name != "" {
			return result(manager.JoinKey(owner(ctx), in.Body.NetworkID, in.Body.Name, in.Body.Key))
		}
		return result(manager.DirectPerson(owner(ctx), in.Body.NetworkID, in.Body.MemberID, in.Body.Nickname))
	})
	huma.Patch(s.API, "/api/v1/conversations/{id}", func(ctx context.Context, in *ConversationPatch) (*Output[irc.Conversation], error) {
		if in.Body.ReadThroughMessageID != nil {
			if in.Body.Read != nil || in.Body.UnreadFrom != nil {
				return nil, huma.Error422UnprocessableEntity("readThroughMessageId cannot be combined with read or unreadFrom")
			}
			return result(manager.ReadThrough(owner(ctx), in.ID, *in.Body.ReadThroughMessageID, in.Body.Favorite))
		}
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
	huma.Register(s.API, huma.Operation{OperationID: "update-channel-settings", Method: "PUT", Path: "/api/v1/conversations/{id}/channel-settings", Summary: "Change channel-wide settings", Description: "Changes shared notifications, group mentions and sensitive-content classification. Needs channel operator status or higher; every joined IRCord member receives the new values."}, func(ctx context.Context, in *ChannelSettingsPatch) (*Output[irc.Conversation], error) {
		return result(manager.SetChannelSettings(owner(ctx), in.ID, irc.ChannelSettingsInput{Notifications: in.Body.Notifications, GroupMentions: in.Body.GroupMentions, NSFW: in.Body.NSFW}))
	})
	huma.Delete(s.API, "/api/v1/conversations/{id}", func(ctx context.Context, in *ConversationPath) (*struct{}, error) {
		return empty(manager.PartReason(owner(ctx), in.ID, in.Reason))
	})
	huma.Get(s.API, "/api/v1/conversations/{id}/messages", func(ctx context.Context, in *HistoryInput) (*Output[irc.MessagePage], error) {
		if in.Around != "" {
			if in.Before != "" {
				return nil, huma.Error422UnprocessableEntity("around and before cannot be combined")
			}
			page, err := manager.HistoryAroundContext(ctx, owner(ctx), in.ID, in.Around, in.Limit)
			return s.historyResult(ctx, page, err)
		}
		page, err := manager.HistoryContext(ctx, owner(ctx), in.ID, in.Before, in.Limit)
		return s.historyResult(ctx, page, err)
	})
	huma.Post(s.API, "/api/v1/conversations/{id}/reactions", func(ctx context.Context, in *ReactionInput) (*Output[irc.Message], error) {
		message, err := manager.ToggleReaction(owner(ctx), in.ID, in.Body.MessageID, in.Body.Emoji)
		return s.messageResult(ctx, message, err)
	})
	huma.Post(s.API, "/api/v1/conversations/{id}/attract", func(ctx context.Context, in *AttractInput) (*Output[irc.Message], error) {
		message, err := manager.Attract(owner(ctx), in.ID)
		return s.messageResult(ctx, message, err)
	})
}
