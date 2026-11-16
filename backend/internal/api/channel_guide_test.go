package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"ircord/backend/internal/channelguide"
	"ircord/backend/internal/ergo"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/storage"
)

// fakeErgoChannels serves the associated server's channel list. Tests change it in place.
type fakeErgoChannels struct {
	mu       sync.Mutex
	channels []map[string]any
}

func (f *fakeErgoChannels) set(channels ...map[string]any) {
	f.mu.Lock()
	f.channels = channels
	f.mu.Unlock()
}

func ergoChannel(name string, users int, owner string, flags ...string) map[string]any {
	c := map[string]any{"name": name, "usercount": users, "topic": "about " + name, "owner": owner, "registered": owner != ""}
	for _, flag := range flags {
		c[flag] = true
	}
	return c
}

func fakeErgo(t *testing.T) (*ergo.Client, *fakeErgoChannels) {
	t.Helper()
	token := strings.Repeat("ef", 32)
	list := &fakeErgoChannels{}
	vendor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token || r.URL.Path != "/v1/list" {
			w.WriteHeader(404)
			return
		}
		list.mu.Lock()
		defer list.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "channels": list.channels})
	}))
	t.Cleanup(vendor.Close)
	client, err := ergo.New(ergo.Options{Server: "irc.example.com:6697", URL: vendor.URL, Token: token})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return client, list
}

// silentIRC accepts IRC connections and never answers, so a home network connection exists
// and queues JOINs without a real server.
func silentIRC(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var conns []net.Conn
	var mu sync.Mutex
	go func() {
		for {
			c, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, c)
			mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		listener.Close()
		mu.Lock()
		for _, c := range conns {
			c.Close()
		}
		mu.Unlock()
	})
	return listener.Addr().(*net.TCPAddr).Port
}

type guideRig struct {
	app     *Server
	url     string
	store   *storage.Store
	list    *fakeErgoChannels
	home    irc.NetworkConfig
	owner   testAccount
	member  testAccount
	oClient *http.Client
	mClient *http.Client
}

func newGuideRig(t *testing.T, home irc.NetworkConfig, withErgo bool) guideRig {
	t.Helper()
	base, _, store, sessions := authenticatedApp(t, func(o *irc.Options) { o.Home = home })
	var client *ergo.Client
	var list *fakeErgoChannels
	if withErgo {
		client, list = fakeErgo(t)
	}
	app := New(base.Manager, Options{Store: store, Auth: sessions, Ergo: client})
	server := httptest.NewServer(app.Handler)
	t.Cleanup(server.Close)
	r := guideRig{app: app, url: server.URL, store: store, list: list, home: home, oClient: cookieClient(), mClient: cookieClient()}
	r.owner = register(t, r.oClient, server.URL)
	r.member = register(t, r.mClient, server.URL)
	if err := store.GrantOwner(r.owner.Account.ID, storage.OwnerFromOperator, "test"); err != nil {
		t.Fatal(err)
	}
	return r
}

func (r guideRig) provision(t *testing.T, account testAccount) {
	t.Helper()
	cfg := r.home
	cfg.Nick = account.Username
	cfg.AutoJoin = nil
	if err := r.app.Manager.ProvisionHome(account.Account.ID, cfg); err != nil {
		t.Fatal(err)
	}
}

func decode[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("%v: %s", err, raw)
	}
	return v
}

func TestChannelGuideOwnersCurateAndMembersChoose(t *testing.T) {
	home := irc.NetworkConfig{ID: "ircord", Name: "IRCord", Host: "127.0.0.1", Port: silentIRC(t), AutoJoin: []string{"#lobby"}}
	r := newGuideRig(t, home, true)
	r.list.set(
		ergoChannel("#lobby", 40, "founder"),
		ergoChannel("#games", 12, "alice"),
		ergoChannel("#food", 7, ""),
		ergoChannel("#hideout", 3, "bob", "secret"),
		ergoChannel("#vault", 2, "bob", "haskey"),
	)
	admin := r.url + administrationPrefix + "/channel-guide"

	// Only owners reach the editor; signed-out requests are refused before that.
	request(t, r.mClient, "GET", admin, nil, 403)
	request(t, http.DefaultClient, "GET", admin, nil, 401)

	// Before any save, the configured default channels are the guide, all for everyone.
	initial := decode[AdminChannelGuide](t, request(t, r.oClient, "GET", admin, nil, 200))
	if initial.Saved || initial.Revision != 0 || len(initial.Guide.Entries) != 1 || initial.Guide.Entries[0].Channel != "#lobby" || initial.Guide.Entries[0].State != channelguide.Everyone || !initial.Directory {
		t.Fatalf("initial guide = %+v", initial)
	}

	guide := channelguide.Guide{
		Groups: []channelguide.Group{{Name: "Games"}, {Name: "Off topic"}},
		Entries: []channelguide.Entry{
			{Channel: "#lobby", State: channelguide.Everyone, Description: "Say hi"},
			{Channel: "games", Group: "games", State: channelguide.Suggested, Description: "Fleets and fits"},
			{Channel: "#food", Group: "Off topic", State: channelguide.Listed},
			{Channel: "#gone", Group: "Off topic", State: channelguide.Listed},
		},
	}
	// Hidden channels can't be listed, and members can't save.
	hidden := guide
	hidden.Entries = append(append([]channelguide.Entry{}, guide.Entries...), channelguide.Entry{Channel: "#hideout", State: channelguide.Listed})
	request(t, r.oClient, "PUT", admin, AdminChannelGuideInput{Revision: 0, Guide: hidden}, 422)
	request(t, r.mClient, "PUT", admin, AdminChannelGuideInput{Revision: 0, Guide: guide}, 403)
	bad := guide
	bad.Entries = []channelguide.Entry{{Channel: "#food", Group: "Nope", State: channelguide.Listed}}
	request(t, r.oClient, "PUT", admin, AdminChannelGuideInput{Revision: 0, Guide: bad}, 422)

	saved := decode[AdminChannelGuide](t, request(t, r.oClient, "PUT", admin, AdminChannelGuideInput{Revision: 0, Guide: guide}, 200))
	if !saved.Saved || saved.Revision != 1 || saved.UpdatedBy != r.owner.Username || saved.Guide.Entries[1].Channel != "#games" || saved.Guide.Entries[1].Group != "Games" {
		t.Fatalf("saved guide = %+v", saved)
	}
	warnings := map[string]string{}
	for _, w := range saved.Warnings {
		warnings[w.Channel] = w.Kind
	}
	if warnings["#gone"] != "missing" || warnings["#food"] != "unregistered" || len(warnings) != 2 {
		t.Fatalf("warnings = %+v", saved.Warnings)
	}
	// A second owner working from the old revision can't overwrite it unnoticed.
	request(t, r.oClient, "PUT", admin, AdminChannelGuideInput{Revision: 0, Guide: guide}, 409)
	events, _ := r.store.OwnerEvents(context.Background(), 10)
	if len(events) == 0 || events[0].Action != "channel-guide-save" {
		t.Fatalf("audit = %+v", events)
	}

	// Members see the guide on the home network only, without missing channels.
	r.provision(t, r.member)
	view := decode[ChannelGuideView](t, request(t, r.mClient, "GET", r.url+"/api/v1/networks/ircord/channel-guide", nil, 200))
	if !view.Available || view.Revision != 1 || len(view.Groups) != 2 || len(view.Entries) != 3 {
		t.Fatalf("member view = %+v", view)
	}
	if e := view.Entries[1]; e.Channel != "#games" || e.Members == nil || *e.Members != 12 || e.Topic != "about #games" || e.State != channelguide.Suggested {
		t.Fatalf("games entry = %+v", e)
	}
	other := decode[ChannelGuideView](t, request(t, r.oClient, "GET", r.url+"/api/v1/networks/ircord/channel-guide", nil, 200))
	if other.Available {
		t.Fatal("an account without the home network sees the guide")
	}

	// Joining takes only guide channels, and joins exactly the ones asked for.
	request(t, r.mClient, "POST", r.url+"/api/v1/networks/ircord/channel-guide/join", ChannelGuideJoinInput{Channels: []string{"#hideout"}}, 422)
	joined := decode[ChannelGuideJoinResult](t, request(t, r.mClient, "POST", r.url+"/api/v1/networks/ircord/channel-guide/join", ChannelGuideJoinInput{Channels: []string{"#GAMES", "#games"}}, 200))
	// The silent server never finishes registration, so the channel is saved for the
	// connection to join once it is up.
	if len(joined.Queued) != 1 || joined.Queued[0] != "#games" || len(joined.Joined) != 0 || len(joined.Failed) != 0 {
		t.Fatalf("join = %+v", joined)
	}
	channels := map[string]bool{}
	for _, c := range r.app.Manager.Snapshot(r.member.Account.ID).Conversations {
		if c.Kind == "channel" {
			channels[c.Name] = true
		}
	}
	if !channels["games"] || channels["food"] || channels["lobby"] || len(channels) != 1 {
		t.Fatalf("member channels = %v", channels)
	}

	// Saving the guide never joins anyone: the owner's guide edits left the member alone.
	if n := len(r.app.Manager.Snapshot(r.owner.Account.ID).Conversations); n != 0 {
		t.Fatalf("owner has %d conversations", n)
	}
}

func TestChannelGuideRequestsQueue(t *testing.T) {
	home := irc.NetworkConfig{ID: "ircord", Name: "IRCord", Host: "127.0.0.1", Port: silentIRC(t)}
	r := newGuideRig(t, home, true)
	channels := func(art, quiet string) {
		r.list.set(ergoChannel("#games", 12, "alice"), ergoChannel("#art", 5, art), ergoChannel("#movies", 4, "dave"),
			ergoChannel("#x", 2, "xena"), ergoChannel("#quiet", 2, quiet), ergoChannel("#LATE{", 3, "lou"))
	}
	channels("carol", "zoe")
	ctx := context.Background()
	for _, req := range []storage.ChannelGuideRequest{
		{Channel: "#games", Name: "#games", Founder: "alice", Group: "Games", Description: "Fleets"},
		{Channel: "#art", Name: "#art", Founder: "carol", Group: "Creative"},
		{Channel: "#movies", Name: "#movies", Founder: "dave"},
		{Channel: "#gone", Name: "#gone", Founder: "erin"},
		{Channel: "#x", Name: "#x", Founder: "xena"},
		// Asked while the server's channel list was unavailable, so no founder was recorded.
		{Channel: "#quiet", Name: "#quiet"},
		// Keyed with IRC casemapping: the server lists the same channel as #LATE{.
		{Channel: channelguide.Fold("#Late["), Name: "#Late["},
	} {
		req.AccountID, req.Requester = r.member.Account.ID, r.member.Username
		if err := r.store.SaveChannelGuideRequest(ctx, req, channelguide.MaxRequests); err != nil {
			t.Fatal(err)
		}
	}
	admin := r.url + administrationPrefix + "/channel-guide"
	pending := func() []string {
		r.app.forgetChannelFacts()
		v := decode[AdminChannelGuide](t, request(t, r.oClient, "GET", admin, nil, 200))
		out := []string{}
		for _, q := range v.Requests {
			out = append(out, q.Channel)
		}
		return out
	}
	// A request for a channel that disappeared is dropped; one without a founder gets one.
	if got := strings.Join(pending(), ","); got != "#games,#art,#movies,#x,#quiet,#Late[" {
		t.Fatalf("pending = %s", got)
	}
	if q, _ := r.store.ChannelGuideRequest(ctx, "#quiet"); q.Founder != "zoe" {
		t.Fatalf("recorded founder = %q", q.Founder)
	}
	// A channel that changed hands loses its request, including one whose founder was
	// recorded later.
	channels("mallory", "yan")
	if got := strings.Join(pending(), ","); got != "#games,#movies,#x,#Late[" {
		t.Fatalf("pending after handover = %s", got)
	}
	// Declining removes the request; members can't decline. One-character names work too.
	request(t, r.mClient, "DELETE", admin+"/requests/movies", nil, 403)
	request(t, r.oClient, "DELETE", admin+"/requests/movies", nil, 204)
	request(t, r.oClient, "DELETE", admin+"/requests/movies", nil, 404)
	request(t, r.oClient, "DELETE", admin+"/requests/x", nil, 204)
	// Approving is saving the guide with the channel in it, which settles its request, also
	// when the guide spells it with other IRC case.
	request(t, r.oClient, "PUT", admin, AdminChannelGuideInput{Guide: channelguide.Guide{Groups: []channelguide.Group{{Name: "Games"}}, Entries: []channelguide.Entry{
		{Channel: "#games", Group: "Games", State: channelguide.Listed, Description: "Fleets"},
		{Channel: "#late{", State: channelguide.Listed},
	}}}, 200)
	if got := pending(); len(got) != 0 {
		t.Fatalf("pending after approval = %v", got)
	}
	// Directory search lists public channels only.
	r.list.set(ergoChannel("#games", 12, "alice"), ergoChannel("#gamers", 30, ""), ergoChannel("#gamesecret", 3, "bob", "secret"))
	r.app.forgetChannelFacts()
	found := decode[Items[GuideDirectoryChannel]](t, request(t, r.oClient, "GET", admin+"/channels?query=%23game", nil, 200))
	if len(found.Items) != 2 || found.Items[0].Name != "#gamers" || found.Items[1].Name != "#games" {
		t.Fatalf("directory = %+v", found.Items)
	}
}

func TestChannelGuideWithoutDirectory(t *testing.T) {
	home := irc.NetworkConfig{ID: "ircord", Name: "IRCord", Host: "127.0.0.1", Port: silentIRC(t), AutoJoin: []string{"#lobby", " help ", "#lobby"}}
	r := newGuideRig(t, home, false)
	r.provision(t, r.member)
	view := decode[ChannelGuideView](t, request(t, r.mClient, "GET", r.url+"/api/v1/networks/ircord/channel-guide", nil, 200))
	if !view.Available || view.Revision != 0 || len(view.Entries) != 2 || view.Entries[1].Channel != "#help" || view.Entries[1].Members != nil {
		t.Fatalf("fallback view = %+v", view)
	}
	v := decode[AdminChannelGuide](t, request(t, r.oClient, "GET", r.url+administrationPrefix+"/channel-guide", nil, 200))
	if v.Directory || len(v.Warnings) != 0 || strings.Join(v.Defaults, ",") != "#lobby, help ,#lobby" {
		t.Fatalf("admin view = %+v", v)
	}
	// Conversations outside the home network have no place in the guide.
	listing := decode[ChannelGuideListing](t, request(t, r.mClient, "GET", r.url+"/api/v1/conversations/nope/channel-guide", nil, 200))
	if listing.Available {
		t.Fatalf("listing = %+v", listing)
	}
	request(t, r.mClient, "PUT", r.url+"/api/v1/conversations/nope/channel-guide", ChannelGuideRequestInput{}, 404)
}

// A channel operator on the home network asks for their channel to be listed and withdraws
// the request; a member without operator status can't.
func TestErgoChannelGuideOperatorRequest(t *testing.T) {
	address := os.Getenv("IRCORD_ERGO_ADDR")
	if address == "" {
		t.Skip("run make test-integration")
	}
	host, portText, _ := net.SplitHostPort(address)
	port, _ := strconv.Atoi(portText)
	r := newGuideRig(t, irc.NetworkConfig{ID: "ircord", Name: "IRCord", Host: host, Port: port}, false)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	suffix = suffix[len(suffix)-7:]
	channel := "#guide" + suffix
	join := func(account testAccount, client *http.Client) irc.Conversation {
		cfg := r.home
		cfg.Nick = "g" + strings.ReplaceAll(account.Username, "tester", "")
		if err := r.app.Manager.ProvisionHome(account.Account.ID, cfg); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := r.app.Manager.WaitConnected(ctx, account.Account.ID, "ircord"); err != nil {
			t.Fatal(err)
		}
		return decode[irc.Conversation](t, request(t, client, "POST", r.url+"/api/v1/conversations", map[string]string{"networkId": "ircord", "name": channel}, 200))
	}
	opConv := join(r.member, r.mClient)
	listingURL := r.url + "/api/v1/conversations/" + opConv.ID + "/channel-guide"
	waitAPI(t, func() bool {
		return decode[ChannelGuideListing](t, request(t, r.mClient, "GET", listingURL, nil, 200)).Operator
	})
	asked := decode[ChannelGuideListing](t, request(t, r.mClient, "PUT", listingURL, ChannelGuideRequestInput{Group: " Games ", Description: "Fleets  and fits"}, 200))
	if asked.Status != "pending" || asked.Group != "Games" || asked.Description != "Fleets and fits" {
		t.Fatalf("request = %+v", asked)
	}
	queued := decode[AdminChannelGuide](t, request(t, r.oClient, "GET", r.url+administrationPrefix+"/channel-guide", nil, 200))
	if len(queued.Requests) != 1 || queued.Requests[0].Channel != channel || queued.Requests[0].Requester != r.member.Username {
		t.Fatalf("queue = %+v", queued.Requests)
	}
	other := join(r.owner, r.oClient)
	waitAPI(t, func() bool {
		for _, c := range r.app.Manager.Snapshot(r.owner.Account.ID).Conversations {
			if c.ID == other.ID {
				return c.Joined
			}
		}
		return false
	})
	otherURL := r.url + "/api/v1/conversations/" + other.ID + "/channel-guide"
	if v := decode[ChannelGuideListing](t, request(t, r.oClient, "GET", otherURL, nil, 200)); v.Operator || v.Status != "pending" {
		t.Fatalf("non-operator listing = %+v", v)
	}
	request(t, r.oClient, "PUT", otherURL, ChannelGuideRequestInput{}, 403)
	request(t, r.oClient, "DELETE", otherURL, nil, 403)
	withdrawn := decode[ChannelGuideListing](t, request(t, r.mClient, "DELETE", listingURL, nil, 200))
	if withdrawn.Status != "none" {
		t.Fatalf("withdrawn = %+v", withdrawn)
	}
	if v, _ := r.store.ChannelGuideRequests(context.Background()); len(v) != 0 {
		t.Fatalf("requests left = %+v", v)
	}
}
