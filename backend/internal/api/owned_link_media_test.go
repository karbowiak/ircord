package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/linkmedia"
	"ircord/backend/internal/model"
)

// Only IRCord's own file route is answered without fetching. Other links with
// /files/ in the path, such as shop and CMS image URLs, go to the resolver.
func TestOwnedLinkMediaIgnoresOtherFilesPaths(t *testing.T) {
	s := &Server{}
	s.options.PublicURL = "https://chat.example.test"
	for _, raw := range []string{
		"https://cdn.shopify.com/s/files/1/0001/0002/products/photo.png",
		"https://www.drupal.org/sites/default/files/styles/photo.jpg",
		"https://example.com/files/report",
		"https://example.com/photo.png",
		"https://chat.example.test/files/short/token/photo.png",
		// Right lengths, not hexadecimal.
		"https://example.com/files/" + strings.Repeat("z", 32) + "/" + strings.Repeat("z", 48) + "/a.png",
		// No segment after the token.
		"https://example.com/files/" + strings.Repeat("a", 32) + "/" + strings.Repeat("b", 48),
	} {
		if value, handled := s.ownedLinkMedia(context.Background(), raw); handled {
			t.Fatalf("%s was handled as an IRCord file: %+v", raw, value)
		}
	}
}

// The file route ignores everything after the token, so every spelling of it
// is the same capability and none may reach the public resolver or the CDN.
func TestOwnedLinkMediaKeepsFileCapabilitiesRestricted(t *testing.T) {
	s := &Server{}
	s.options.PublicURL = "https://chat.example.test"
	id, token := strings.Repeat("a", 32), strings.Repeat("b", 48)
	for _, raw := range []string{
		"https://chat.example.test/files/" + id + "/" + token + "/photo.png", // this instance, no file host
		"https://other.example.test/files/" + id + "/" + token + "/photo.png",
		"https://other.example.test/sub/files/" + id + "/" + token + "/photo.png",
		"https://other.example.test/files/" + id + "/" + token + "/x/photo.png",
		"https://other.example.test/files/" + id + "/" + token + "/photo.png/",
		"https://other.example.test/files/" + id + "/" + token + "//photo.png",
		"https://chat.example.test/files/" + id + "/" + token + "/x/photo.png",
	} {
		value, handled := s.ownedLinkMedia(context.Background(), raw)
		if !handled || value.Kind != "file" || value.File == nil || value.File.Reason != "restricted" {
			t.Fatalf("%s: handled=%t %+v", raw, handled, value)
		}
	}
}

// A page whose address has a dot in its last segment is not a direct file, so
// an oEmbed photo for it belongs in the metadata card, which is the only layout
// an unknown site has.
func TestDottedPageWithPhotoReturnsCard(t *testing.T) {
	server, store, client := linkMediaTestApp(t, linkmedia.ResolverOptions{})
	alice := cookieClient()
	a := register(t, alice, server.URL)
	const page = "https://photos.example.test/people/john.doe"
	key := "link-media/" + strings.Repeat("c", 64)
	seedLinkResult(t, store, client, "native-file-v2", page, linkmedia.Result{URL: page, Provider: "oembed", Title: "John", Assets: []linkmedia.Asset{{
		URL: client.URL(key), ObjectKey: key, SourceURL: page, ContentType: "image/png", Size: seededObjectSize, Kind: "image", Width: 640, Height: 480, ExpiresAt: time.Now().Add(time.Hour),
	}}})
	conv := model.Conversation{ID: "net_dev", NetworkID: "net", Kind: "channel", Name: "dev", Target: "#dev", MemberIDs: []string{}, MemberModes: map[string]string{}}
	_, _ = store.Commit(a.Account.ID, "conversation.updated", conv)
	msg := model.Message{ID: "net:abc", ConversationID: conv.ID, SenderID: "peer", Kind: "message", Text: page, CreatedAt: time.Now(), Reactions: []model.Reaction{}}
	_, _ = store.Commit(a.Account.ID, "message.created", msg)
	raw := request(t, alice, http.MethodPost, server.URL+"/api/v1/conversations/net_dev/messages/net:abc/link-media", map[string]any{"url": page}, 200)
	var out LinkMediaResponse
	if json.Unmarshal(raw, &out) != nil || out.Kind != "card" || out.Card == nil || out.Card.Image != client.URL(key) || len(out.Assets) != 0 {
		t.Fatalf("dotted page response: %s", raw)
	}
}
