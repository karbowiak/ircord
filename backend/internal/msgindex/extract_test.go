package msgindex

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"ircord/backend/internal/model"
)

func urls(entries []Entry) []string {
	out := []string{}
	for _, e := range entries {
		out = append(out, e.Kind+" "+e.Source+" "+e.URL)
	}
	return out
}

func TestImagesFollowsTimelineRules(t *testing.T) {
	msg := model.Message{ID: "m", Kind: "message", Text: "look https://i.imgur.com/a.png, and (https://media.tenor.com/x.webp) and `https://example.com/code.png`\n```\nhttps://example.com/fenced.jpg\n```\nhttp://example.com/plain.png https://example.com/page https://user:pw@example.com/x.png https://i.imgur.com/a.png https://example.com/b.GIF"}
	got := strings.Join(urls(Images(msg)), "\n")
	want := strings.Join([]string{
		"image link https://i.imgur.com/a.png",
		"gif link https://media.tenor.com/x.webp",
		"gif link https://example.com/b.GIF",
	}, "\n")
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestImagesKeepsUploadsPickedGIFsAndPreviews(t *testing.T) {
	upload := "https://chat.example/files/abc123/tok/cat%20pic.jpg"
	msg := model.Message{ID: "m", Kind: "message", Text: upload + " https://i.redd.it/b.jpg", Attachment: &model.Attachment{Name: "cat pic.jpg", URL: upload, ContentType: "image/jpeg", Width: 640, Height: 480},
		PreviewSource: "server", Preview: &model.LinkPreview{URL: "https://imgur.com/a/x", Images: []model.PreviewImage{{URL: "https://i.imgur.com/1.jpg", Width: 10, Height: 20}, {URL: "http://insecure.example/2.jpg"}}}}
	got := Images(msg)
	if len(got) != 3 || got[0].Source != SourceUpload || got[0].FileID != "abc123" || got[0].Width != 640 || got[1].URL != "https://i.redd.it/b.jpg" || got[2].Source != SourcePreview || got[2].Height != 20 {
		t.Fatalf("%+v", got)
	}
	gif := model.Message{ID: "g", Kind: "message", Text: "https://example.com/ignored.png", GIF: &model.GIF{URL: "https://static.klipy.com/x.gif", Width: 3, Height: 4}}
	if got := urls(Images(gif)); len(got) != 1 || got[0] != "gif gif https://static.klipy.com/x.gif" {
		t.Fatalf("picked GIF: %v", got)
	}
	share := model.Message{ID: "s", Kind: "message", Text: "https://tenor.com/view/x", PreviewSource: "server", Preview: &model.LinkPreview{URL: "https://tenor.com/view/x", Media: &model.PreviewMedia{URL: "https://media.tenor.com/y.gif", Kind: "gif"}}}
	if got := urls(Images(share)); len(got) != 1 || got[0] != "gif preview https://media.tenor.com/y.gif" {
		t.Fatalf("share page: %v", got)
	}
	// A client-side preview is the browser's own guess and is not catalogued.
	share.PreviewSource = "client"
	if got := Images(share); len(got) != 0 {
		t.Fatalf("client preview indexed: %v", got)
	}
	file := model.Message{ID: "f", Kind: "message", Attachment: &model.Attachment{Name: "notes.pdf", URL: "https://chat.example/files/z/t/notes.pdf", ContentType: "application/pdf"}}
	if got := Images(file); len(got) != 0 {
		t.Fatalf("non-image upload indexed: %v", got)
	}
}

func TestImagesSkipsDeletedAndNonChatLinesAndIsBounded(t *testing.T) {
	text := ""
	for i := range 15 {
		text += fmt.Sprintf(" https://i.imgur.com/%d.png", i)
	}
	msg := model.Message{ID: "m", Kind: "message", Text: text}
	if got := Images(msg); len(got) != MaxPerMessage || got[9].URL != "https://i.imgur.com/9.png" {
		t.Fatalf("bound: %d", len(got))
	}
	msg.Deleted = true
	if Images(msg) != nil {
		t.Fatal("deleted message indexed")
	}
	msg.Deleted, msg.Kind = false, "system"
	if Images(msg) != nil {
		t.Fatal("system line indexed")
	}
}

// The demo backend's TypeScript copy and this one must catalogue the same
// images; both test suites load this table.
func TestImagesMatchesTheFrontendCopy(t *testing.T) {
	raw, err := os.ReadFile("../../../frontend/tests/fixtures/gallery-parity.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name    string
		Message model.Message
		Images  []struct {
			URL, Kind, Source, FileID string
			Width, Height             int
		}
	}
	if err = json.Unmarshal(raw, &cases); err != nil || len(cases) == 0 {
		t.Fatalf("parity fixture: %v", err)
	}
	for _, tc := range cases {
		got := Images(tc.Message)
		want := []Entry{}
		for _, w := range tc.Images {
			want = append(want, Entry{URL: w.URL, Kind: w.Kind, Source: w.Source, FileID: w.FileID, Width: w.Width, Height: w.Height})
		}
		if len(got) == 0 && len(want) == 0 {
			continue
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s:\n got %+v\nwant %+v", tc.Name, got, want)
		}
	}
}
