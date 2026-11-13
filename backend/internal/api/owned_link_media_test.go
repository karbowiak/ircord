package api

import (
	"context"
	"strings"
	"testing"
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
	} {
		if value, handled := s.ownedLinkMedia(context.Background(), raw); handled {
			t.Fatalf("%s was handled as an IRCord file: %+v", raw, value)
		}
	}
}

func TestOwnedLinkMediaKeepsFileCapabilitiesRestricted(t *testing.T) {
	s := &Server{}
	s.options.PublicURL = "https://chat.example.test"
	id, token := strings.Repeat("a", 32), strings.Repeat("b", 48)
	for _, raw := range []string{
		"https://chat.example.test/files/" + id + "/" + token + "/photo.png", // this instance, no file host
		"https://other.example.test/files/" + id + "/" + token + "/photo.png",
		"https://other.example.test/sub/files/" + id + "/" + token + "/photo.png",
	} {
		value, handled := s.ownedLinkMedia(context.Background(), raw)
		if !handled || value.Kind != "file" || value.File == nil || value.File.Reason != "restricted" {
			t.Fatalf("%s: handled=%t %+v", raw, handled, value)
		}
	}
}
