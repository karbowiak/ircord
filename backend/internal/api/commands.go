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
		Attachments []string `json:"attachments,omitempty" nullable:"false"`
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
func (s *Server) registerCommands() {
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
