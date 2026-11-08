package api

import (
	"encoding/json"
	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/storage"
	"reflect"
)

type Items[T any] struct {
	Items []T `json:"items" nullable:"false"`
}

func (p Items[T]) MarshalJSON() ([]byte, error) {
	type wire Items[T]
	if p.Items == nil {
		p.Items = []T{}
	}
	return json.Marshal(wire(p))
}

type AccountSession struct {
	Account *storage.Account `json:"account"`
}

func (AccountSession) Schema(reg huma.Registry) *huma.Schema {
	return &huma.Schema{Type: "object", AdditionalProperties: false, Required: []string{"account"}, Properties: map[string]*huma.Schema{"account": {AnyOf: []*huma.Schema{reg.Schema(reflect.TypeOf(storage.Account{}), true, "Account"), {Type: "null"}}}}}
}

type PushKey struct {
	PublicKey string `json:"publicKey"`
	Enabled   bool   `json:"enabled"`
}
