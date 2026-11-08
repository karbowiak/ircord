package api

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/admin"
	"ircord/backend/internal/model"
)

type historyUpdate struct {
	ConversationID string `json:"conversationId"`
	Reason         string `json:"reason,omitempty"`
	Message        string `json:"message,omitempty"`
}
type revisionUpdate struct {
	Revision string `json:"revision"`
}
type networkInvalidation struct {
	NetworkID string `json:"networkId"`
}
type sessionReady struct {
	V      int    `json:"v" enum:"1"`
	Type   string `json:"type" enum:"session.ready"`
	Cursor uint64 `json:"cursor"`
	// EmoteVersion lets a reconnecting client refetch an emote catalog that changed while it was away.
	EmoteVersion int64 `json:"emoteVersion,omitempty"`
}
type sessionResync struct {
	V      int    `json:"v" enum:"1"`
	Type   string `json:"type" enum:"session.resync"`
	Reason string `json:"reason"`
}
type commandError struct {
	V        int    `json:"v" enum:"1"`
	Type     string `json:"type" enum:"command.error"`
	ClientID string `json:"clientId,omitempty"`
	Message  string `json:"message"`
}
type sessionResume struct {
	V     int    `json:"v" enum:"1"`
	Type  string `json:"type" enum:"session.resume"`
	After uint64 `json:"after"`
}
type typingCommand struct {
	V              int               `json:"v" enum:"1"`
	Type           string            `json:"type" enum:"typing.set"`
	ConversationID string            `json:"conversationId"`
	State          model.TypingState `json:"state" enum:"active,paused,done"`
}
type traceLine struct {
	V    int              `json:"v" enum:"1"`
	Type string           `json:"type" enum:"connection.line"`
	Data model.ServerLine `json:"data"`
}
type traceResync struct {
	V      int    `json:"v" enum:"1"`
	Type   string `json:"type" enum:"connection.resync"`
	Reason string `json:"reason"`
}

func writeProblem(w http.ResponseWriter, detail string, status int) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(huma.ErrorModel{Status: status, Title: http.StatusText(status), Detail: detail})
}

// finalizeSpec documents native WebSocket/stream handlers and derives event data
// from the same Go DTOs as HTTP. The exported JSON and served schema are identical.
func (s *Server) finalizeSpec() {
	doc := s.API.OpenAPI()
	doc.Info.Description = "Authenticated IRCord HTTP commands and account-private WebSocket updates. Login/register sets an HttpOnly session cookie. WebSocket mutations are limited to ephemeral typing; other commands use HTTP."
	doc.Components.SecuritySchemes = map[string]*huma.SecurityScheme{"sessionCookie": {Type: "apiKey", In: "cookie", Name: "ircord_session", Description: "HttpOnly cookie established by register/login. Browsers send it automatically; do not put account keys in headers or URLs."}}
	doc.Components.SecuritySchemes["adminBearer"] = &huma.SecurityScheme{Type: "http", Scheme: "bearer", Description: "Operator-configured administration token. Independent of browser sessions and bot tokens; send only in the Authorization header."}
	registry := doc.Components.Schemas
	schema := func(value any, hint string) *huma.Schema { return registry.Schema(reflect.TypeOf(value), true, hint) }
	ref := func(name string) *huma.Schema { return &huma.Schema{Ref: "#/components/schemas/" + name} }
	definitions := []struct {
		Name, Type string
		Value      any
	}{
		{"MessageCreatedEvent", "message.created", model.Message{}},
		{"InvitationReceivedEvent", "invitation.received", model.Invitation{}},
		{"InvitationRemovedEvent", "invitation.removed", model.Removal{}},
		{"MessageUpdatedEvent", "message.updated", model.Message{}},
		{"ConversationUpdatedEvent", "conversation.updated", model.Conversation{}},
		{"ChannelServicesUpdatedEvent", "channel-services.updated", model.ChannelServicesChange{}},
		{"NetworkUpdatedEvent", "network.updated", model.Network{}},
		{"ProfileUpdatedEvent", "profile.updated", model.Member{}},
		{"MemberUpdatedEvent", "member.updated", model.Member{}},
		{"ConversationRemovedEvent", "conversation.removed", model.Removal{}},
		{"NetworkRemovedEvent", "network.removed", model.Removal{}},
		{"HistorySyncedEvent", "history.synced", historyUpdate{}},
		{"HistoryGapEvent", "history.gap", historyUpdate{}},
		{"HistoryFailedEvent", "history.sync.failed", historyUpdate{}},
		{"ConnectionUpdatedEvent", "connection.updated", model.ConnectionInfo{}},
		{"ServerLineEvent", "server.line", model.ServerLine{}},
		{"ServerLogUpdatedEvent", "server-log.updated", networkInvalidation{}},
		{"AppearanceUpdatedEvent", "appearance.updated", revisionUpdate{}},
		{"LibraryUpdatedEvent", "library.updated", revisionUpdate{}},
		{"PreferencesUpdatedEvent", "client-preferences.updated", revisionUpdate{}},
		{"NotificationsUpdatedEvent", "notification-settings.updated", revisionUpdate{}},
		{"SavedAddedEvent", "saved.added", model.SavedMessage{}},
		{"SavedRemovedEvent", "saved.removed", model.SavedRemoval{}},
		{"InboxAddedEvent", "inbox.added", model.InboxEntry{}},
		{"InboxUpdatedEvent", "inbox.updated", model.InboxChange{}},
	}
	durable := []*huma.Schema{}
	discriminators := map[string]string{}
	for _, d := range definitions {
		registry.Map()[d.Name] = &huma.Schema{Type: "object", AdditionalProperties: false, Required: []string{"v", "seq", "type", "data"}, Properties: map[string]*huma.Schema{
			"v": {Type: "integer", Enum: []any{1}}, "seq": {Type: "integer", Minimum: ptrFloat(1)},
			"type": {Type: "string", Enum: []any{d.Type}}, "data": schema(d.Value, d.Name+"Data"),
		}}
		durable = append(durable, ref(d.Name))
		discriminators[d.Type] = "#/components/schemas/" + d.Name
	}
	registry.Map()["DurableEvent"] = &huma.Schema{OneOf: durable}
	registry.Map()["Event"] = &huma.Schema{OneOf: durable}
	all := append([]*huma.Schema{}, durable...)
	all = append(all, schema(sessionReady{}, "SessionReady"), schema(sessionResync{}, "SessionResync"), schema(commandError{}, "CommandError"), schema(model.TypingEvent{}, "TypingEvent"), schema(model.AttractEvent{}, "AttractEvent"), schema(emotesUpdatedEvent{}, "EmotesUpdatedEvent"), schema(devicesUpdatedEvent{}, "DevicesUpdatedEvent"))
	for _, name := range []string{"SessionReady", "SessionResync", "CommandError", "TypingEvent", "AttractEvent", "EmotesUpdatedEvent", "DevicesUpdatedEvent"} {
		tag := registry.Map()[name].Properties["type"].Enum[0].(string)
		discriminators[tag] = "#/components/schemas/" + name
	}
	registry.Map()["ServerEvent"] = &huma.Schema{Type: "object", OneOf: all, Discriminator: &huma.Discriminator{PropertyName: "type", Mapping: discriminators}}
	registry.Map()["ClientEvent"] = &huma.Schema{OneOf: []*huma.Schema{schema(sessionResume{}, "SessionResume"), schema(typingCommand{}, "TypingCommand")}}
	registry.Map()["TraceEvent"] = &huma.Schema{OneOf: []*huma.Schema{schema(traceLine{}, "TraceLine"), schema(traceResync{}, "TraceResync")}}
	// Opaque frontend-owned JSON documents intentionally remain unknown. Notification
	// levels, in contrast, have a finite value set shared by the server and client.
	if settings := registry.Map()["Settings"]; settings != nil {
		if levels := settings.Properties["levels"]; levels != nil {
			levels.AdditionalProperties = &huma.Schema{Type: "string", Enum: []any{"all", "mentions", "none"}}
		}
		for _, key := range []string{"channelDefault", "directDefault"} {
			if v := settings.Properties[key]; v != nil {
				v.Enum = []any{"all", "mentions", "none"}
			}
		}
	}
	errorSchema := schema(huma.ErrorModel{}, "ErrorModel")
	problem := &huma.Response{Description: "Request failed; see the problem detail.", Content: map[string]*huma.MediaType{"application/problem+json": {Schema: errorSchema}}}
	socketExtension := map[string]any{"clientMessages": ref("ClientEvent"), "serverMessages": ref("ServerEvent")}
	doc.AddOperation(&huma.Operation{OperationID: "stream-events", Method: "GET", Path: "/api/v1/events", Summary: "Open the authenticated event WebSocket", Description: "Upgrade to WebSocket. Send session.resume using the last applied durable cursor. Typing, attract, emotes.updated and devices.updated events have no sequence and are never replayed; session.ready carries the current emote catalog version instead. Send typing.set for activity; use HTTP for durable commands.", Extensions: map[string]any{"x-websocket": socketExtension}, Responses: map[string]*huma.Response{"101": {Description: "WebSocket upgraded"}}})
	doc.AddOperation(&huma.Operation{OperationID: "stream-connection-trace", Method: "GET", Path: "/api/v1/networks/{id}/connection/trace/events", Summary: "Open the receive-only diagnostic WebSocket", Extensions: map[string]any{"x-websocket": map[string]any{"serverMessages": ref("TraceEvent")}}, Parameters: []*huma.Param{{Name: "id", In: "path", Required: true, Schema: &huma.Schema{Type: "string"}}, {Name: "after", In: "query", Schema: &huma.Schema{Type: "string"}}}, Responses: map[string]*huma.Response{"101": {Description: "WebSocket upgraded; connection.line and connection.resync frames."}}})
	for path, item := range doc.Paths {
		for _, operation := range []*huma.Operation{item.Get, item.Post, item.Put, item.Patch, item.Delete, item.Head, item.Options} {
			if operation == nil {
				continue
			}
			operation.Tags = []string{apiTag(path)}
			if admin.IsPath(path) {
				operation.Security = []map[string][]string{{"adminBearer": {}}}
			} else if path != "/api/v1/auth/signup" && path != "/api/v1/auth/login" && path != "/api/v1/auth/session" && path != "/api/v1/auth/username" && path != "/api/v1/auth/registration" {
				operation.Security = []map[string][]string{{"sessionCookie": {}}}
			}
			for _, code := range []string{"400", "401", "403", "404", "409", "413", "422", "429", "500", "501", "503"} {
				if operation.Responses[code] == nil {
					operation.Responses[code] = problem
				}
			}
		}
	}
}
func ptrFloat(v float64) *float64 { return &v }
func apiTag(path string) string {
	if admin.IsPath(path) {
		return "Administration"
	}
	if strings.Contains(path, "/auth/") {
		return "Accounts"
	}
	if strings.Contains(path, "/connection") || strings.Contains(path, "/server-log") {
		return "Diagnostics"
	}
	if strings.Contains(path, "/emotes") {
		return "Emotes"
	}
	if strings.Contains(path, "/uploads") {
		return "Files"
	}
	if strings.Contains(path, "/notifications") || strings.Contains(path, "/push") {
		return "Notifications"
	}
	if strings.Contains(path, "/app-data") || strings.Contains(path, "/hash-field") {
		return "App data"
	}
	if strings.Contains(path, "/networks") {
		return "Networks"
	}
	return "Chat"
}
