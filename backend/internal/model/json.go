package model

import "encoding/json"

// Public collections are always arrays/objects, including while connecting or
// before any history exists. HTTP, saved events and WebSockets share this shape.
func array[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}
func object[V any](v map[string]V) map[string]V {
	if v == nil {
		return map[string]V{}
	}
	return v
}

func (v Features) MarshalJSON() ([]byte, error) {
	type wire Features
	c := wire(v)
	c.Metadata = array(c.Metadata)
	return json.Marshal(c)
}
func (v Network) MarshalJSON() ([]byte, error) {
	type wire Network
	c := wire(v)
	c.AutoJoin = array(c.AutoJoin)
	c.Perform = array(c.Perform)
	c.Capabilities = array(c.Capabilities)
	c.ISupport = object(c.ISupport)
	return json.Marshal(c)
}
func (v Conversation) MarshalJSON() ([]byte, error) {
	type wire Conversation
	c := wire(v)
	c.MemberIDs = array(c.MemberIDs)
	c.MemberModes = object(c.MemberModes)
	return json.Marshal(c)
}
func (v Reaction) MarshalJSON() ([]byte, error) {
	type wire Reaction
	c := wire(v)
	c.MemberIDs = array(c.MemberIDs)
	return json.Marshal(c)
}
func (v Message) MarshalJSON() ([]byte, error) {
	type wire Message
	c := wire(v)
	c.Reactions = array(c.Reactions)
	return json.Marshal(c)
}
func (v Snapshot) MarshalJSON() ([]byte, error) {
	type wire Snapshot
	c := wire(v)
	c.Members = array(c.Members)
	c.Networks = array(c.Networks)
	c.Conversations = array(c.Conversations)
	c.Invitations = array(c.Invitations)
	return json.Marshal(c)
}
func (v MessagePage) MarshalJSON() ([]byte, error) {
	type wire MessagePage
	c := wire(v)
	c.Items = array(c.Items)
	return json.Marshal(c)
}
func (v ServerLine) MarshalJSON() ([]byte, error) {
	type wire ServerLine
	c := wire(v)
	c.Params = array(c.Params)
	return json.Marshal(c)
}
func (v ServerLogPage) MarshalJSON() ([]byte, error) {
	type wire ServerLogPage
	c := wire(v)
	c.Items = array(c.Items)
	return json.Marshal(c)
}
func (v ConnectionInfo) MarshalJSON() ([]byte, error) {
	type wire ConnectionInfo
	c := wire(v)
	c.ISupport = object(c.ISupport)
	c.OfferedCapabilities = object(c.OfferedCapabilities)
	c.NegotiatedCapabilities = array(c.NegotiatedCapabilities)
	c.MOTD = array(c.MOTD)
	return json.Marshal(c)
}
