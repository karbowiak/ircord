// Package irc owns upstream connections. Browser devices subscribe to an owner's
// workspace; they never create a second upstream connection for that identity.
package irc

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"ircord/backend/internal/model"
	"strings"
)

var (
	ErrNotFound     = errors.New("not found in this workspace")
	ErrDisconnected = errors.New("IRC network is disconnected")
	ErrUnsupported  = errors.New("this network does not support this operation")
	ErrBusy         = errors.New("IRC send queue is full; try again shortly")
)

type ChannelListing = model.ChannelListing
type Member = model.Member
type Network = model.Network
type Conversation = model.Conversation
type Reaction = model.Reaction
type Reply = model.Reply
type GIF = model.GIF
type Message = model.Message
type Preferences = model.Preferences
type Snapshot = model.Snapshot
type Event = model.Event
type SendInput = model.SendInput
type NetworkConfig = model.NetworkConfig
type Profile = model.Profile
type MessagePage = model.MessagePage

func id() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func clean(value string, limit int) bool {
	return len(value) <= limit && !strings.ContainsAny(value, "\r\n\x00")
}
func clone[T any](v T) T { b, _ := json.Marshal(v); var out T; _ = json.Unmarshal(b, &out); return out }

func token(value string, limit int) bool {
	if value == "" || len(value) > limit {
		return false
	}
	for _, r := range value {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-", r) {
			return false
		}
	}
	return true
}
