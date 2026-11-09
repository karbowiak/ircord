package api

import (
	"context"
	"errors"
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

func (s *Server) commandResponse(owner, client string) (CommandResponse, error) {
	command, err := s.Manager.Command(owner, client)
	out := CommandResponse{Command: command}
	if err == nil && command.MessageID != "" && s.options.Store != nil {
		msg, e := s.options.Store.Message(owner, command.ConversationID, command.MessageID)
		if e == nil {
			out.Message = &msg
		}
	}
	return out, err
}

type RedactInput struct {
	ID        string `path:"id"`
	MessageID string `path:"messageId"`
}
type RedactResponse struct {
	Message            irc.Message `json:"message"`
	AttachmentRetained bool        `json:"attachmentRetained" doc:"True when the deleted message carried one or more attachments. Deleting a message never deletes uploaded files."`
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
		return result(EditResponse{Message: msg}, err)
	})
	huma.Register(s.API, huma.Operation{OperationID: "redact-message", Method: "DELETE", Path: "/api/v1/conversations/{id}/messages/{messageId}", Summary: "Delete one of your messages for everyone", Description: "Sends an IRC REDACT through draft/message-redaction. Only the author may delete a message. The canonical change is delivered as message.updated with deleted set; uploaded files are kept."}, func(ctx context.Context, in *RedactInput) (*Output[RedactResponse], error) {
		msg, attachments, err := s.Manager.RedactAttachments(owner(ctx), in.ID, in.MessageID)
		if err != nil {
			return result(RedactResponse{}, err)
		}
		out := RedactResponse{Message: msg, AttachmentRetained: len(attachments) > 0}
		if s.options.Files != nil {
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
		v, err := s.commandResponse(owner(ctx), input.ClientID)
		if errors.Is(err, irc.ErrUnsupported) {
			v = CommandResponse{Command: storage.Command{ClientID: input.ClientID, ConversationID: input.ConversationID, State: "pending"}}
			err = nil
		}
		return result(v, err)
	})
	huma.Get(s.API, "/api/v1/commands/{clientId}", func(ctx context.Context, in *struct {
		ClientID string `path:"clientId"`
	}) (*Output[CommandResponse], error) {
		v, err := s.commandResponse(owner(ctx), in.ClientID)
		if errors.Is(err, storage.ErrNotFound) {
			err = irc.ErrNotFound
		}
		return result(v, err)
	})
}
