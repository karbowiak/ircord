package api

import (
	"context"
	"errors"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/storage"
)

type MessageInput struct {
	ID   string `path:"id"`
	Body struct {
		ClientID    string   `json:"clientId" minLength:"1" maxLength:"100"`
		Text        string   `json:"text" maxLength:"4000"`
		ReplyToID   string   `json:"replyToId,omitempty"`
		GIF         *irc.GIF `json:"gif,omitempty"`
		Kind        string   `json:"kind,omitempty" enum:"message,action"`
		Attachments []string `json:"attachments,omitempty" nullable:"false" maxItems:"10" uniqueItems:"true" doc:"Ordered upload IDs. Text must contain only their URLs in the same order, separated by spaces or newlines; one upload requires its exact URL."`
	}
}
type CommandResponse struct {
	storage.Command
	Message *irc.Message `json:"message,omitempty"`
}

func (s *Server) commandResponse(ctx context.Context, owner, client string) (CommandResponse, error) {
	command, err := s.Manager.Command(owner, client)
	out := CommandResponse{Command: command}
	if err == nil && command.MessageID != "" && s.options.Store != nil {
		msg, e := s.Manager.Message(owner, command.ConversationID, command.MessageID)
		if e == nil {
			msg = s.messageWithGeometry(ctx, msg)
			out.Message = &msg
		}
	}
	return out, err
}

type RedactInput struct {
	ID          string `path:"id"`
	MessageID   string `path:"messageId"`
	Reason      string `query:"reason" maxLength:"300" doc:"Sent with the deletion when it is someone else's message. Other IRC clients may show it."`
	DeleteFiles bool   `query:"deleteFiles" doc:"Server owners deleting someone else's message only: also delete the files this server hosts for it. Ignored otherwise."`
}
type RedactResponse struct {
	Message            irc.Message `json:"message"`
	Authority          string      `json:"authority" enum:"author,operator,owner" doc:"How the deletion was carried out: as the author, as a channel operator, or as a server owner through the server's moderation login."`
	AttachmentRetained bool        `json:"attachmentRetained" doc:"True when the deleted message carried one or more attachments that were not deleted with it."`
	FilesDeleted       int         `json:"filesDeleted,omitempty" doc:"Files hosted by this server that were deleted with the message at a server owner's request."`
	UploadID           string      `json:"uploadId,omitempty" doc:"The caller's own deletable upload behind a single attachment. File deletion requires a separate DELETE /api/v1/uploads/{id} request."`
	UploadIDs          []string    `json:"uploadIds,omitempty" nullable:"false" maxItems:"10" uniqueItems:"true" doc:"The caller's own deletable uploads from a gallery, in attachment order. File deletion requires separate DELETE /api/v1/uploads/{id} requests."`
}

type EditInput struct {
	ID        string `path:"id"`
	MessageID string `path:"messageId"`
	Body      struct {
		Text string `json:"text" minLength:"1" maxLength:"4096"`
	}
}
type EditResponse struct {
	Message irc.Message `json:"message"`
}

func (s *Server) registerCommands() {
	huma.Register(s.API, huma.Operation{OperationID: "edit-message", Method: "PATCH", Path: "/api/v1/conversations/{id}/messages/{messageId}", Summary: "Edit one of your messages", Description: "Sends the corrected text as a PRIVMSG tagged +ircord/edit=<original msgid>. IRCord clients fold it into the original message; other IRC clients show it as a new line. Only the author may edit, and only on networks whose features report edit. The canonical change is delivered as message.updated with editedAt set."}, func(ctx context.Context, in *EditInput) (*Output[EditResponse], error) {
		msg, err := s.Manager.Edit(owner(ctx), in.ID, in.MessageID, in.Body.Text)
		if err == nil {
			msg = s.messageWithGeometry(ctx, msg)
		}
		return result(EditResponse{Message: msg}, err)
	})
	huma.Register(s.API, huma.Operation{OperationID: "redact-message", Method: "DELETE", Path: "/api/v1/conversations/{id}/messages/{messageId}", Summary: "Delete a message for everyone", Description: "Sends an IRC REDACT through draft/message-redaction. The author may delete their own message; a channel operator (@ and above) any message in that channel; and a server owner any channel message on the associated server, through the server's moderation login when one is configured. Server-owner deletions are recorded in the owner audit log without the message text. The canonical change is delivered as message.updated with deleted set; uploaded files are kept unless a server owner asks for deleteFiles."}, func(ctx context.Context, in *RedactInput) (*Output[RedactResponse], error) {
		r, err := s.Manager.RedactAs(ctx, owner(ctx), in.ID, in.MessageID, in.Reason)
		if err != nil {
			if r.Authority == irc.AuthorityOwner {
				s.auditDeletion(ctx, r, in.Reason, err, 0)
			}
			return result(RedactResponse{}, err)
		}
		msg, attachments := r.Message, r.Attachments
		msg.EmbedGeometry = nil
		out := RedactResponse{Message: msg, Authority: r.Authority, AttachmentRetained: len(attachments) > 0}
		if in.DeleteFiles && r.Authority == irc.AuthorityOwner && s.options.Files != nil {
			kept := 0
			for _, attachment := range attachments {
				removed, e := s.options.Files.DeleteHostedURL(ctx, attachment.URL)
				if e != nil {
					slog.Error("Deleting a moderated message's file failed", "error", e)
				}
				if removed {
					out.FilesDeleted++
				} else {
					kept++
				}
			}
			out.AttachmentRetained = kept > 0
		}
		if r.Authority == irc.AuthorityOwner {
			s.auditDeletion(ctx, r, in.Reason, nil, out.FilesDeleted)
			return result(out, nil)
		}
		if s.options.Files != nil && r.Authority == irc.AuthorityAuthor {
			for _, attachment := range attachments {
				if id := s.options.Files.DeletableUpload(owner(ctx), attachment.URL); id != "" {
					if len(attachments) == 1 {
						out.UploadID = id
					} else {
						out.UploadIDs = append(out.UploadIDs, id)
					}
				}
			}
		}
		return result(out, nil)
	})
	huma.Register(s.API, huma.Operation{OperationID: "send-message", Method: "POST", Path: "/api/v1/conversations/{id}/messages", DefaultStatus: 202}, func(ctx context.Context, in *MessageInput) (*Output[CommandResponse], error) {
		input := irc.SendInput{ClientID: in.Body.ClientID, ConversationID: in.ID, Text: in.Body.Text, ReplyToID: in.Body.ReplyToID, GIF: in.Body.GIF, Kind: in.Body.Kind, Attachments: in.Body.Attachments}
		if err := s.Manager.Send(owner(ctx), input); err != nil {
			return result(CommandResponse{}, err)
		}
		v, err := s.commandResponse(ctx, owner(ctx), input.ClientID)
		if errors.Is(err, irc.ErrUnsupported) {
			v = CommandResponse{Command: storage.Command{ClientID: input.ClientID, ConversationID: input.ConversationID, State: "pending"}}
			err = nil
		}
		return result(v, err)
	})
	huma.Get(s.API, "/api/v1/commands/{clientId}", func(ctx context.Context, in *struct {
		ClientID string `path:"clientId"`
	}) (*Output[CommandResponse], error) {
		v, err := s.commandResponse(ctx, owner(ctx), in.ClientID)
		if errors.Is(err, storage.ErrNotFound) {
			err = irc.ErrNotFound
		}
		return result(v, err)
	})
}
