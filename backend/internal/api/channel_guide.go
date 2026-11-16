package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/channelguide"
	"ircord/backend/internal/ergo"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/storage"
)

// The channel guide (#258): server owners curate a short, grouped list of home network
// channels. New members pick from it in the welcome dialog and everyone can browse it in Join
// existing. Channel operators can ask for their channel to be listed. Nothing here joins or
// parts anyone except the member's own explicit join request.

// ChannelGuideChannel is one guide entry as a member sees it.
type ChannelGuideChannel struct {
	Channel     string `json:"channel"`
	Group       string `json:"group,omitempty"`
	Description string `json:"description,omitempty"`
	State       string `json:"state" enum:"everyone,suggested,listed"`
	Topic       string `json:"topic,omitempty"`
	Members     *int   `json:"members,omitempty" minimum:"0" doc:"Member count, when the server could be asked"`
	NSFW        bool   `json:"nsfw" doc:"The channel's operators marked it as sensitive content"`
}

// ChannelGuideView is the guide a member picks channels from.
type ChannelGuideView struct {
	Available bool                  `json:"available" doc:"False on networks other than this server's own, which have no guide"`
	Revision  int64                 `json:"revision" minimum:"0" doc:"0 while no owner has saved a guide and the default channels are shown"`
	Groups    []channelguide.Group  `json:"groups" nullable:"false"`
	Entries   []ChannelGuideChannel `json:"entries" nullable:"false"`
}

type ChannelGuideJoinInput struct {
	Channels []string `json:"channels" nullable:"false" maxItems:"50" doc:"Channels from the guide to join"`
}

type ChannelGuideJoinFailure struct {
	Channel string `json:"channel"`
	Reason  string `json:"reason"`
}

type ChannelGuideJoinResult struct {
	Joined []irc.Conversation        `json:"joined" nullable:"false"`
	Queued []string                  `json:"queued" nullable:"false" doc:"Channels saved while the connection is down; they are joined when it is back"`
	Failed []ChannelGuideJoinFailure `json:"failed" nullable:"false"`
}

// ChannelGuideWarning flags a guide entry that can't work as intended.
type ChannelGuideWarning struct {
	Channel string `json:"channel"`
	Kind    string `json:"kind" enum:"missing,hidden,unregistered" doc:"missing: the channel doesn't exist right now; hidden: it is secret, invite-only or has a key; unregistered: nobody registered it, so it could vanish or be taken over"`
}

// ChannelGuideRequestView is a channel operator's pending request to list their channel.
type ChannelGuideRequestView struct {
	Channel     string    `json:"channel"`
	Requester   string    `json:"requester" doc:"The IRCord account that asked"`
	Group       string    `json:"group,omitempty"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	Members     *int      `json:"members,omitempty" minimum:"0"`
	Topic       string    `json:"topic,omitempty"`
}

// AdminChannelGuide is the guide as owners edit it.
type AdminChannelGuide struct {
	Guide     channelguide.Guide        `json:"guide"`
	Revision  int64                     `json:"revision" minimum:"0" doc:"Send this back when saving. 0 means no guide was saved and the default channels are in effect."`
	Saved     bool                      `json:"saved"`
	Defaults  []string                  `json:"defaults" nullable:"false" doc:"The server's configured default channels"`
	UpdatedAt *time.Time                `json:"updatedAt,omitempty"`
	UpdatedBy string                    `json:"updatedBy,omitempty"`
	Directory bool                      `json:"directory" doc:"Whether channel facts (existence, member counts, warnings) are available from the associated server"`
	Warnings  []ChannelGuideWarning     `json:"warnings" nullable:"false"`
	Requests  []ChannelGuideRequestView `json:"requests" nullable:"false"`
}

type AdminChannelGuideInput struct {
	Revision int64              `json:"revision" minimum:"0"`
	Guide    channelguide.Guide `json:"guide"`
}

// GuideDirectoryChannel is a public channel on the server, for adding to the guide.
type GuideDirectoryChannel struct {
	Name       string `json:"name"`
	Users      int    `json:"users" minimum:"0"`
	Topic      string `json:"topic,omitempty"`
	Registered bool   `json:"registered"`
}

// ChannelGuideListing is the state of one channel in the guide, for its operators.
type ChannelGuideListing struct {
	Available   bool     `json:"available" doc:"False for channels outside this server's own network"`
	Operator    bool     `json:"operator" doc:"Whether you can ask for the channel to be listed"`
	Status      string   `json:"status" enum:"none,pending,listed"`
	Group       string   `json:"group,omitempty"`
	Description string   `json:"description,omitempty"`
	Groups      []string `json:"groups" nullable:"false" doc:"The guide's groups, to suggest one"`
}

type ChannelGuideRequestInput struct {
	Group       string `json:"group,omitempty" maxLength:"40"`
	Description string `json:"description,omitempty" maxLength:"120"`
}

// guideFacts caches the associated server's channel list briefly, so opening a picker doesn't
// wait on a channel listing every time.
type guideFacts struct {
	mu       sync.Mutex
	at       time.Time
	channels map[string]ergo.ErgoChannel
}

const guideFactsTTL = 30 * time.Second

// channelFacts returns the server's channels by folded name, or false when the associated
// server is not configured or can't be asked.
func (s *Server) channelFacts(ctx context.Context) (map[string]ergo.ErgoChannel, bool) {
	if s.options.Ergo == nil {
		return nil, false
	}
	f := &s.guideFacts
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.channels != nil && time.Since(f.at) < guideFactsTTL {
		return f.channels, true
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	list, err := s.options.Ergo.Channels(ctx)
	if err != nil {
		if f.channels != nil && time.Since(f.at) < 10*guideFactsTTL {
			return f.channels, true
		}
		return nil, false
	}
	channels := make(map[string]ergo.ErgoChannel, len(list))
	for _, c := range list {
		channels[channelguide.Fold(c.Name)] = c
	}
	f.channels, f.at = channels, time.Now()
	return channels, true
}

func (s *Server) forgetChannelFacts() {
	s.guideFacts.mu.Lock()
	s.guideFacts.channels = nil
	s.guideFacts.mu.Unlock()
}

func hiddenChannel(c ergo.ErgoChannel) bool { return c.Secret || c.InviteOnly || c.HasKey }

func (s *Server) loadGuide(ctx context.Context) (channelguide.Guide, storage.StoredChannelGuide, bool, error) {
	return channelguide.Load(ctx, s.options.Store, s.Manager.HomeDefaultChannels())
}

// memberGuide is the guide as a member sees it: missing and hidden channels are left out,
// except channels for everyone, which joining creates.
func (s *Server) memberGuide(ctx context.Context, owner string) (ChannelGuideView, error) {
	view := ChannelGuideView{Groups: []channelguide.Group{}, Entries: []ChannelGuideChannel{}}
	guide, stored, _, err := s.loadGuide(ctx)
	if err != nil {
		return view, huma.Error503ServiceUnavailable("The channel guide couldn't be loaded.")
	}
	view.Available, view.Revision, view.Groups = true, stored.Revision, guide.Groups
	facts, known := s.channelFacts(ctx)
	names := make([]string, 0, len(guide.Entries))
	for _, e := range guide.Entries {
		names = append(names, e.Channel)
	}
	nsfw := s.Manager.HomeChannelsNSFW(owner, names)
	for _, e := range guide.Entries {
		item := ChannelGuideChannel{Channel: e.Channel, Group: e.Group, Description: e.Description, State: e.State, NSFW: nsfw[e.Channel]}
		if known {
			fact, exists := facts[channelguide.Fold(e.Channel)]
			if exists && hiddenChannel(fact) || !exists && e.State != channelguide.Everyone {
				continue
			}
			if exists {
				users := fact.Users
				item.Members, item.Topic = &users, fact.Topic
			}
		}
		view.Entries = append(view.Entries, item)
	}
	return view, nil
}

func (s *Server) adminGuide(ctx context.Context) (AdminChannelGuide, error) {
	guide, stored, saved, err := s.loadGuide(ctx)
	if err != nil {
		return AdminChannelGuide{}, huma.Error503ServiceUnavailable("The channel guide couldn't be loaded.")
	}
	out := AdminChannelGuide{Guide: guide, Revision: stored.Revision, Saved: saved, Defaults: s.Manager.HomeDefaultChannels(), Warnings: []ChannelGuideWarning{}, Requests: []ChannelGuideRequestView{}}
	if out.Defaults == nil {
		out.Defaults = []string{}
	}
	if saved {
		at := stored.UpdatedAt
		out.UpdatedAt, out.UpdatedBy = &at, stored.UpdatedBy
	}
	facts, known := s.channelFacts(ctx)
	out.Directory = known
	if known {
		for _, e := range guide.Entries {
			fact, exists := facts[channelguide.Fold(e.Channel)]
			switch {
			case !exists:
				out.Warnings = append(out.Warnings, ChannelGuideWarning{e.Channel, "missing"})
			case hiddenChannel(fact):
				out.Warnings = append(out.Warnings, ChannelGuideWarning{e.Channel, "hidden"})
			case !fact.Registered:
				out.Warnings = append(out.Warnings, ChannelGuideWarning{e.Channel, "unregistered"})
			}
		}
	}
	requests, err := s.pendingRequests(ctx, guide, facts, known)
	if err != nil {
		return out, huma.Error503ServiceUnavailable("Channel requests couldn't be loaded.")
	}
	for _, r := range requests {
		view := ChannelGuideRequestView{Channel: r.Name, Requester: r.Requester, Group: r.Group, Description: r.Description, CreatedAt: r.CreatedAt}
		if fact, ok := facts[r.Channel]; known && ok {
			users := fact.Users
			view.Members, view.Topic = &users, fact.Topic
		}
		out.Requests = append(out.Requests, view)
	}
	return out, nil
}

// pendingRequests returns the listing requests that still apply. A request is dropped when its
// channel is already in the guide, no longer exists, became hidden, or changed hands since it
// was made. A request made while the server's channel list was unavailable has no founder yet;
// the first listing that sees its channel records the founder instead of dropping it.
func (s *Server) pendingRequests(ctx context.Context, guide channelguide.Guide, facts map[string]ergo.ErgoChannel, known bool) ([]storage.ChannelGuideRequest, error) {
	requests, err := s.options.Store.ChannelGuideRequests(ctx)
	if err != nil {
		return nil, err
	}
	out := requests[:0]
	for _, r := range requests {
		_, listed := guide.Find(r.Name)
		stale := listed
		if known && !stale {
			fact, exists := facts[r.Channel]
			stale = !exists || hiddenChannel(fact) || r.Founder != "" && !strings.EqualFold(fact.Owner, r.Founder)
			if !stale && r.Founder == "" && fact.Owner != "" {
				r.Founder = fact.Owner
				if err = s.options.Store.SaveChannelGuideRequest(ctx, r, channelguide.MaxRequests); err != nil {
					return nil, err
				}
			}
		}
		if stale {
			if _, err = s.options.Store.DeleteChannelGuideRequest(ctx, r.Channel); err != nil {
				return nil, err
			}
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *Server) requireGuideStore() error {
	if s.options.Store == nil {
		return huma.Error404NotFound("this backend has no IRCord accounts")
	}
	return nil
}

func (s *Server) registerChannelGuide() {
	huma.Register(s.API, huma.Operation{OperationID: "channel-guide", Method: http.MethodGet, Path: "/api/v1/networks/{id}/channel-guide", Summary: "The server's channel guide",
		Description: "Owner-curated channels to choose from on this server's own network. Other networks have no guide and return available: false."}, func(ctx context.Context, in *struct {
		ID string `path:"id" maxLength:"100"`
	}) (*Output[ChannelGuideView], error) {
		if s.options.Store == nil || !s.Manager.IsHomeNetwork(owner(ctx), in.ID) {
			return result(ChannelGuideView{Groups: []channelguide.Group{}, Entries: []ChannelGuideChannel{}}, nil)
		}
		view, err := s.memberGuide(ctx, owner(ctx))
		if err != nil {
			return nil, err
		}
		return &Output[ChannelGuideView]{Body: view}, nil
	})
	huma.Register(s.API, huma.Operation{OperationID: "channel-guide-join", Method: http.MethodPost, Path: "/api/v1/networks/{id}/channel-guide/join", Summary: "Join channels from the guide",
		Description: "Joins the chosen guide channels in one request. Channels that aren't in the guide are refused; each channel that can't be joined is reported without stopping the rest.", MaxBodyBytes: 8192}, func(ctx context.Context, in *struct {
		ID   string `path:"id" maxLength:"100"`
		Body ChannelGuideJoinInput
	}) (*Output[ChannelGuideJoinResult], error) {
		if s.options.Store == nil || !s.Manager.IsHomeNetwork(owner(ctx), in.ID) {
			return nil, huma.Error404NotFound("This network has no channel guide.")
		}
		view, err := s.memberGuide(ctx, owner(ctx))
		if err != nil {
			return nil, err
		}
		offered := map[string]string{}
		for _, e := range view.Entries {
			offered[channelguide.Fold(e.Channel)] = e.Channel
		}
		out := ChannelGuideJoinResult{Joined: []irc.Conversation{}, Queued: []string{}, Failed: []ChannelGuideJoinFailure{}}
		seen := map[string]bool{}
		for _, raw := range in.Body.Channels {
			key := channelguide.Fold(raw)
			channel, ok := offered[key]
			if !ok {
				return nil, huma.Error422UnprocessableEntity(strings.TrimSpace(raw) + " isn't in the channel guide.")
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			conv, err := s.Manager.Join(owner(ctx), in.ID, channel)
			if errors.Is(err, irc.ErrDisconnected) {
				// The channel is saved before the JOIN is sent, so reconnecting joins it.
				out.Queued = append(out.Queued, channel)
				continue
			}
			if err != nil {
				reason := err.Error()
				if errors.Is(err, irc.ErrStorage) || internalCause(err) {
					reason = "Something went wrong. Please try again."
				}
				out.Failed = append(out.Failed, ChannelGuideJoinFailure{Channel: channel, Reason: reason})
				continue
			}
			out.Joined = append(out.Joined, conv)
		}
		return result(out, nil)
	})

	huma.Register(s.API, huma.Operation{OperationID: "administration-channel-guide", Method: http.MethodGet, Path: administrationPrefix + "/channel-guide", Summary: "The channel guide, with warnings and listing requests"}, func(ctx context.Context, _ *struct{}) (*Output[AdminChannelGuide], error) {
		if _, err := s.requireOwner(ctx); err != nil {
			return nil, err
		}
		out, err := s.adminGuide(ctx)
		if err != nil {
			return nil, err
		}
		return &Output[AdminChannelGuide]{Body: out}, nil
	})
	huma.Register(s.API, huma.Operation{OperationID: "administration-save-channel-guide", Method: http.MethodPut, Path: administrationPrefix + "/channel-guide", Summary: "Replace the channel guide",
		Description: "Saves the whole guide. The revision must match the saved one, so two owners can't overwrite each other unnoticed. Secret, invite-only and keyed channels are refused. Saving never joins or parts anyone; it removes listing requests for the channels it now lists.", MaxBodyBytes: 64 << 10}, func(ctx context.Context, in *struct{ Body AdminChannelGuideInput }) (*Output[AdminChannelGuide], error) {
		actor, err := s.requireOwner(ctx)
		if err != nil {
			return nil, err
		}
		guide, err := channelguide.Normalize(in.Body.Guide)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity(err.Error())
		}
		s.forgetChannelFacts()
		if facts, known := s.channelFacts(ctx); known {
			for _, e := range guide.Entries {
				if fact, ok := facts[channelguide.Fold(e.Channel)]; ok && hiddenChannel(fact) {
					return nil, huma.Error422UnprocessableEntity(e.Channel + " is secret, invite-only or has a key, so it can't be in the guide.")
				}
			}
		}
		raw, _ := json.Marshal(guide)
		listed := make([]string, 0, len(guide.Entries))
		for _, e := range guide.Entries {
			listed = append(listed, channelguide.Fold(e.Channel))
		}
		if _, err = s.options.Store.SaveChannelGuide(ctx, raw, in.Body.Revision, actor.Account.Username, listed); errors.Is(err, storage.ErrStaleRevision) {
			return nil, problem(409, "Another owner changed the channel guide. Reload it to see their changes.", problemResourceConflict)
		} else if err != nil {
			s.audit(actor, "channel-guide-save", "", "", "failed")
			return nil, huma.Error503ServiceUnavailable("The channel guide couldn't be saved.")
		}
		s.audit(actor, "channel-guide-save", "", "", "saved")
		out, err := s.adminGuide(ctx)
		if err != nil {
			return nil, err
		}
		return &Output[AdminChannelGuide]{Body: out}, nil
	})
	huma.Register(s.API, huma.Operation{OperationID: "administration-channel-guide-channels", Method: http.MethodGet, Path: administrationPrefix + "/channel-guide/channels", Summary: "Find public channels to add to the guide"}, func(ctx context.Context, in *struct {
		Query string `query:"query" maxLength:"64"`
	}) (*Output[Items[GuideDirectoryChannel]], error) {
		if _, err := s.requireOwner(ctx); err != nil {
			return nil, err
		}
		facts, known := s.channelFacts(ctx)
		out := []GuideDirectoryChannel{}
		if !known {
			return result(Items[GuideDirectoryChannel]{out}, nil)
		}
		query := channelguide.Fold(strings.TrimPrefix(strings.TrimSpace(in.Query), "#"))
		for key, c := range facts {
			if hiddenChannel(c) || query != "" && !strings.Contains(key, query) {
				continue
			}
			out = append(out, GuideDirectoryChannel{Name: c.Name, Users: c.Users, Topic: c.Topic, Registered: c.Registered})
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].Users != out[j].Users {
				return out[i].Users > out[j].Users
			}
			return out[i].Name < out[j].Name
		})
		if len(out) > 20 {
			out = out[:20]
		}
		return result(Items[GuideDirectoryChannel]{out}, nil)
	})
	huma.Register(s.API, huma.Operation{OperationID: "administration-decline-channel-guide-request", Method: http.MethodDelete, Path: administrationPrefix + "/channel-guide/requests/{channel}", Summary: "Decline a request to list a channel"}, func(ctx context.Context, in *struct {
		Channel string `path:"channel" minLength:"1" maxLength:"64" doc:"The channel name; the # prefix is optional"`
	}) (*struct{}, error) {
		actor, err := s.requireOwner(ctx)
		if err != nil {
			return nil, err
		}
		channel, _ := channelguide.NormalChannel(in.Channel)
		found, err := s.options.Store.DeleteChannelGuideRequest(ctx, channelguide.Fold(channel))
		if err != nil {
			return nil, huma.Error503ServiceUnavailable("The request couldn't be declined.")
		}
		if !found {
			return nil, huma.Error404NotFound("That request was already handled or withdrawn.")
		}
		s.audit(actor, "channel-guide-decline", channel, "", "declined")
		return nil, nil
	})

	huma.Register(s.API, huma.Operation{OperationID: "channel-guide-listing", Method: http.MethodGet, Path: "/api/v1/conversations/{id}/channel-guide", Summary: "This channel's place in the channel guide"}, func(ctx context.Context, in *struct {
		ID string `path:"id" maxLength:"200"`
	}) (*Output[ChannelGuideListing], error) {
		listing, _, err := s.guideListing(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		return &Output[ChannelGuideListing]{Body: listing}, nil
	})
	huma.Register(s.API, huma.Operation{OperationID: "request-channel-guide-listing", Method: http.MethodPut, Path: "/api/v1/conversations/{id}/channel-guide", Summary: "Ask server owners to list this channel in the guide",
		Description: "Needs channel operator status or higher. Creates or updates the channel's pending request; owners approve, edit or decline it.", MaxBodyBytes: 4096}, func(ctx context.Context, in *struct {
		ID   string `path:"id" maxLength:"200"`
		Body ChannelGuideRequestInput
	}) (*Output[ChannelGuideListing], error) {
		listing, channel, err := s.guideListing(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		if !listing.Available {
			return nil, huma.Error404NotFound("Only channels on this server can be listed in its channel guide.")
		}
		if !listing.Operator {
			return nil, huma.Error403Forbidden("Only the channel's operators can ask for it to be listed.")
		}
		if listing.Status == "listed" {
			return nil, problem(409, "This channel is already in the channel guide.", problemResourceConflict)
		}
		group, err := channelguide.CleanText(in.Body.Group, channelguide.MaxGroupName)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("Group names " + err.Error() + ".")
		}
		description, err := channelguide.CleanText(in.Body.Description, channelguide.MaxDescription)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("Descriptions " + err.Error() + ".")
		}
		founder := ""
		s.forgetChannelFacts()
		if facts, known := s.channelFacts(ctx); known {
			fact, ok := facts[channelguide.Fold(channel.Name)]
			if !ok {
				return nil, huma.Error409Conflict("The server doesn't list this channel right now. Try again in a moment.")
			}
			if hiddenChannel(fact) {
				return nil, huma.Error422UnprocessableEntity("Secret, invite-only and keyed channels can't be in the channel guide.")
			}
			founder = fact.Owner
		}
		p := principal(ctx)
		err = s.options.Store.SaveChannelGuideRequest(ctx, storage.ChannelGuideRequest{Channel: channelguide.Fold(channel.Name), Name: channel.Name, AccountID: p.Account.ID, Requester: p.Account.Username, Founder: founder, Group: group, Description: description}, channelguide.MaxRequests)
		if errors.Is(err, storage.ErrRequestQueueFull) {
			return nil, huma.Error429TooManyRequests("Server owners have too many channel requests waiting. Try again later.")
		}
		if err != nil {
			return nil, huma.Error503ServiceUnavailable("The request couldn't be saved.")
		}
		listing.Status, listing.Group, listing.Description = "pending", group, description
		return &Output[ChannelGuideListing]{Body: listing}, nil
	})
	huma.Register(s.API, huma.Operation{OperationID: "withdraw-channel-guide-listing", Method: http.MethodDelete, Path: "/api/v1/conversations/{id}/channel-guide", Summary: "Withdraw this channel's request to be listed",
		Description: "Needs channel operator status or higher. A channel that is already listed stays listed; owners remove it."}, func(ctx context.Context, in *struct {
		ID string `path:"id" maxLength:"200"`
	}) (*Output[ChannelGuideListing], error) {
		listing, channel, err := s.guideListing(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		if !listing.Available {
			return nil, huma.Error404NotFound("Only channels on this server can be listed in its channel guide.")
		}
		if !listing.Operator {
			return nil, huma.Error403Forbidden("Only the channel's operators can withdraw its request.")
		}
		if listing.Status != "pending" {
			return &Output[ChannelGuideListing]{Body: listing}, nil
		}
		if _, err = s.options.Store.DeleteChannelGuideRequest(ctx, channelguide.Fold(channel.Name)); err != nil {
			return nil, huma.Error503ServiceUnavailable("The request couldn't be withdrawn.")
		}
		listing.Status, listing.Group, listing.Description = "none", "", ""
		return &Output[ChannelGuideListing]{Body: listing}, nil
	})
}

// guideListing reports where a channel conversation stands in the guide.
func (s *Server) guideListing(ctx context.Context, conversation string) (ChannelGuideListing, irc.HomeChannel, error) {
	listing := ChannelGuideListing{Status: "none", Groups: []string{}}
	channel, err := s.Manager.HomeChannelOf(owner(ctx), conversation)
	if errors.Is(err, irc.ErrNotFound) || s.options.Store == nil {
		return listing, channel, nil
	}
	if err != nil {
		return listing, channel, commandFailure(err)
	}
	guide, _, _, err := s.loadGuide(ctx)
	if err != nil {
		return listing, channel, huma.Error503ServiceUnavailable("The channel guide couldn't be loaded.")
	}
	listing.Available, listing.Operator = true, channel.Operator
	for _, g := range guide.Groups {
		listing.Groups = append(listing.Groups, g.Name)
	}
	if e, ok := guide.Find(channel.Name); ok {
		listing.Status, listing.Group, listing.Description = "listed", e.Group, e.Description
		return listing, channel, nil
	}
	request, err := s.options.Store.ChannelGuideRequest(ctx, channelguide.Fold(channel.Name))
	if errors.Is(err, storage.ErrNotFound) {
		return listing, channel, nil
	}
	if err != nil {
		return listing, channel, huma.Error503ServiceUnavailable("The channel guide couldn't be loaded.")
	}
	listing.Status, listing.Group, listing.Description = "pending", request.Group, request.Description
	return listing, channel, nil
}
