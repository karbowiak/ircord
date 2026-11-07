package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/irc"
)

type AppDocument struct {
	Data     json.RawMessage `json:"data"`
	Revision string          `json:"revision"`
}

func appKind(name string) (string, error) {
	switch name {
	case "appearance":
		return "appearance.updated", nil
	case "library":
		return "library.updated", nil
	case "preferences":
		return "client-preferences.updated", nil
	}
	return "", fmt.Errorf("unknown app data collection")
}
func (s *Server) registerAppData() {
	huma.Get(s.API, "/api/v1/app-data/{kind}", func(ctx context.Context, in *struct {
		Kind string `path:"kind" enum:"appearance,library,preferences"`
	}) (*Output[AppDocument], error) {
		kind, err := appKind(in.Kind)
		if err != nil {
			return result(AppDocument{}, err)
		}
		data, err := s.Manager.AppData(owner(ctx), kind)
		return result(AppDocument{Data: data, Revision: irc.AppRevision(data)}, err)
	})
	huma.Register(s.API, huma.Operation{OperationID: "save-app-data", Method: "PUT", Path: "/api/v1/app-data/{kind}", MaxBodyBytes: 4 << 20}, func(ctx context.Context, in *struct {
		Kind string `path:"kind" enum:"appearance,library,preferences"`
		Body AppDocument
	}) (*Output[AppDocument], error) {
		kind, err := appKind(in.Kind)
		if err != nil {
			return result(AppDocument{}, err)
		}
		data, err := s.Manager.SaveAppDataCAS(owner(ctx), kind, in.Body.Revision, in.Body.Data)
		return result(AppDocument{Data: data, Revision: irc.AppRevision(data)}, err)
	})
}
