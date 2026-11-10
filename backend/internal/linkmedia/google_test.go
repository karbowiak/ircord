package linkmedia

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// Positive document/map identities come from the pinned upstream fixture
// manifest. Synthetic IDs are used only in rejection/transport unit cases.
const googleDocumentFixture = "17jg1RRL3RI969cLwbKBIcoGDsPwqaEdBxafGNYGwiY4"
const googleSheetFixture = "10JLM1UniyGNuLaYTfs2fnki-U1iYFsQl4XNHPZTYunw"
const googleSlidesFixture = "2PACX-1vQmrymNWFltfprLl9IX-irRcmvjsL1ahOKAt8YTzuWTdcWyIH2EX6wyUmmJ4ftG3dICaTZ9DCpqXiht"
const googlePublishedSheetFixture = "2PACX-1vRKtFs55r6ow0rVvoGJLlDyqxD_455wR6_eZ42z8izYGT_UM6hNW0ruFhn26m_SzsoT4AQxZZA968Lp"

func TestGoogleControlledPlayers(t *testing.T) {
	for _, tc := range []struct {
		name, source, candidate, expected string
		height                            int
		ratio                             float64
	}{
		{"document", "https://docs.google.com/document/d/" + googleDocumentFixture + "/edit", "https://docs.google.com/document/d/" + googleDocumentFixture + "/preview?sle=true", "/preview", 640, 3.0 / 4},
		{"account-sheet", "https://docs.google.com/spreadsheets/u/1/d/" + googleSheetFixture + "/edit?gid=1926296709", "https://docs.google.com/spreadsheets/d/" + googleSheetFixture + "/htmlview?gid=0&headers=false", "gid=1926296709&headers=false", 360, 4.0 / 3},
		{"workspace-document", "https://docs.google.com/a/example.test/document/u/0/d/" + googleDocumentFixture + "/edit", "https://docs.google.com/document/d/" + googleDocumentFixture + "/preview", "/document/d/", 640, 3.0 / 4},
		{"published-slides", "https://docs.google.com/presentation/d/e/" + googleSlidesFixture + "/pub?start=true&loop=true&delayms=3000", "https://docs.google.com/presentation/d/e/" + googleSlidesFixture + "/pub?start=true", "/embed?loop=false&start=false", 270, 16.0 / 9},
		{"drive", "https://drive.google.com/open?id=1rKcaLuY0WzhPqQFCGy_4aIJblnEziy7-", "https://drive.google.com/file/d/1rKcaLuY0WzhPqQFCGy_4aIJblnEziy7-/preview", "/file/d/1rKcaLuY0WzhPqQFCGy_4aIJblnEziy7-/preview", 640, 3.0 / 4},
		{"calendar", "https://calendar.google.com/calendar/u/0/newembed?src=en.dutch%23holiday%40group.v.calendar.google.com&ctz=Europe/Amsterdam&height=9999", "https://calendar.google.com/calendar/embed?src=en.dutch%23holiday%40group.v.calendar.google.com&ctz=Asia/Tokyo", "ctz=Europe%2FAmsterdam", 360, 4.0 / 3},
		{"map", "https://google.com/maps/d/u/0/edit?mid=1sxAC_yIFaWcc8zntMtpWNQclOJm5FBU&ll=40.97,29.03&z=12", "https://www.google.com/maps/d/embed?mid=1sxAC_yIFaWcc8zntMtpWNQclOJm5FBU&z=99", "https://www.google.com/maps/d/embed?mid=1sxAC_yIFaWcc8zntMtpWNQclOJm5FBU", 360, 4.0 / 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frame, height, ratio, ok := ControlledPlayer(tc.source, tc.candidate, "https://chat.example.test")
			if !ok || height != tc.height || ratio != tc.ratio || !strings.Contains(frame, tc.expected) || strings.Contains(frame, "9999") {
				t.Fatalf("unexpected controlled Google frame: %q %d %f %t", frame, height, ratio, ok)
			}
			if resolutionVersion(tc.source) != "native-providers-v3" {
				t.Fatal("Google retained cached metadata-only epoch")
			}
		})
	}
}

func TestGooglePlayerBoundaries(t *testing.T) {
	source := "https://docs.google.com/document/d/" + googleDocumentFixture + "/edit"
	frame := "https://docs.google.com/document/d/" + googleDocumentFixture + "/preview"
	for _, tc := range []struct{ source, frame string }{
		{source, strings.Replace(frame, googleDocumentFixture, "another_document", 1)},
		{source, strings.Replace(frame, "/document/", "/spreadsheets/", 1)},
		{source, strings.Replace(frame, "/d/", "/d/e/", 1)},
		{source, strings.Replace(frame, "/preview", "/edit", 1)},
		{source, strings.Replace(frame, "docs.google.com", "docs.google.com.evil.example", 1)},
		{source, strings.Replace(frame, "https:", "http:", 1)},
		{source + "?resourcekey=private", frame},
		{source, frame + "?resourcekey=private"},
		{source + "?key=private", frame},
		{source + "?redirect=https://evil.example", frame},
		{source, frame + "?id=other&token=private"},
		{"https://drive.google.com/file/d/another_document/view", frame},
		{"https://accounts.google.com/ServiceLogin", frame},
		{"https://www.google.com/maps/place/Stanford/", "https://www.google.com/maps/embed/v1/place?key=private&q=Stanford"},
		{"https://www.google.com/maps/d/viewer?mid=map_one", "https://www.google.com/maps/d/embed?mid=map_two"},
		{"https://www.google.com/maps/embed?pb=public_value", "https://www.google.com/maps/embed?pb=different_value"},
		{"https://calendar.google.com/calendar/embed?src=public%40example.test", "https://calendar.google.com/calendar/embed?src=private%40example.test"},
		{"https://calendar.google.com/calendar/embed?src=public%40example.test&ctz=invalid", "https://calendar.google.com/calendar/embed?src=public%40example.test"},
		{"https://docs.google.com/spreadsheets/d/" + googleSheetFixture + "/pubchart?oid=123", frame},
		{"https://docs.google.com/forms/d/synthetic_form/closedform", "https://docs.google.com/forms/d/synthetic_form/viewform"},
	} {
		if got, _, _, ok := ControlledPlayer(tc.source, tc.frame, "https://chat.example.test"); ok {
			t.Errorf("unsafe Google frame accepted: %q", got)
		}
	}
	for _, tc := range []struct{ kind, source, frame string }{
		{"forms", "https://docs.google.com/forms/d/e/synthetic_form/viewform", "https://docs.google.com/forms/d/e/synthetic_form/viewform"},
		{"drawings", "https://docs.google.com/drawings/d/synthetic_drawing/edit", "https://docs.google.com/drawings/d/synthetic_drawing/preview"},
	} {
		got, height, ratio, ok := ControlledPlayer(tc.source, tc.frame, "")
		if !ok || tc.kind == "forms" && (height != 640 || ratio != 0 || !strings.Contains(got, "embedded=true")) || tc.kind == "drawings" && ratio != 4.0/3 {
			t.Fatalf("synthetic %s shape failed: %q %d %f %t", tc.kind, got, height, ratio, ok)
		}
	}
}

func TestGoogleAnonymousPageEvidence(t *testing.T) {
	source := "https://docs.google.com/document/d/" + googleDocumentFixture + "/edit"
	for _, tc := range []struct {
		name, final, body string
		status            int
		wantPlayer        bool
	}{
		{"public-schema", source, `<html><head><meta property="og:title" content="Public document"></head><body itemscope><meta itemprop="embedURL" content="https://docs.google.com/document/d/` + googleDocumentFixture + `/preview"></body></html>`, 200, true},
		{"no-public-preview", source, `<html><head><title>You need access</title></head></html>`, 200, false},
		{"login-redirect", "https://accounts.google.com/ServiceLogin", `<html><head><title>Sign in</title></head></html>`, 200, false},
		{"different-document-redirect", "https://docs.google.com/document/d/another_document/edit", `<html><head><title>Other document</title></head></html>`, 200, false},
		{"schema-switch", source, `<html><head><title>Document</title></head><meta itemprop="embedURL" content="https://docs.google.com/document/d/another_document/preview"></html>`, 200, false},
		{"third-party-viewer", source, `<html><head><title>Document</title></head><meta itemprop="embedURL" content="https://evil.example/view?url=private"></html>`, 200, false},
		{"non-200", source, `<html><meta itemprop="embedURL" content="https://docs.google.com/document/d/` + googleDocumentFixture + `/preview"></html>`, 403, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			r := NewResolver(ResolverOptions{})
			r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.URL.String() != source || req.Header.Get("Cookie") != "" || req.Header.Get("Authorization") != "" {
					t.Fatal("Google request escaped its anonymous source")
				}
				res := response(tc.status, "text/html", tc.body)
				final, _ := url.Parse(tc.final)
				res.Request = &http.Request{URL: final}
				return res, nil
			})}
			value, err := r.ResolveSource(context.Background(), source)
			if err != nil || calls != 1 || value.Preview == nil || (value.Preview.Kind == "widget") != tc.wantPlayer || len(value.Assets) != 0 {
				t.Fatalf("incorrect public Google evidence: %+v %v calls=%d", value, err, calls)
			}
		})
	}
}

func TestGooglePublicCalendarEligibility(t *testing.T) {
	source := "https://calendar.google.com/calendar/embed?src=en.dutch%23holiday%40group.v.calendar.google.com&ctz=Europe/Amsterdam"
	for _, public := range []bool{true, false} {
		r := NewResolver(ResolverOptions{})
		calls := 0
		r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
			calls++
			var res *http.Response
			if req.URL.String() == source {
				res = response(200, "text/html", `<html><head><title>Calendar</title></head></html>`)
			} else if req.URL.Host == "calendar.google.com" && strings.HasSuffix(req.URL.Path, "/public/basic.ics") {
				if public {
					res = response(200, "text/calendar", "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nEND:VCALENDAR")
				} else {
					res = response(403, "text/html", "private calendar")
				}
			} else {
				t.Fatal("calendar request escaped public source/feed")
			}
			res.Request = req
			return res, nil
		})}
		value, err := r.ResolveSource(context.Background(), source)
		if err != nil || calls != 2 || value.Preview == nil || (value.Preview.Kind == "widget") != public {
			t.Fatalf("calendar public=%t result=%+v error=%v calls=%d", public, value.Preview, err, calls)
		}
	}
}

func TestGooglePublishedPreviewAndMapResolution(t *testing.T) {
	for _, source := range []string{
		"https://docs.google.com/spreadsheets/d/e/" + googlePublishedSheetFixture + "/pubhtml?gid=1443541234&single=true&widget=true&headers=false",
		"https://docs.google.com/presentation/d/e/" + googleSlidesFixture + "/pub?start=false&loop=false&delayms=3000",
		"https://www.google.com/maps/d/viewer?mid=zkaGBGYwQzao.kWqUdP2InpAI",
		"https://www.google.com/maps/embed?pb=public_test_value",
	} {
		r := NewResolver(ResolverOptions{})
		r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
			res := response(200, "text/html", `<html><head><meta property="og:title" content="Public preview"></head><body>Public content</body></html>`)
			res.Request = req
			return res, nil
		})}
		value, err := r.ResolveSource(context.Background(), source)
		if err != nil || value.Preview == nil || value.Preview.Kind != "widget" || !strings.Contains(value.Preview.HTML, "<iframe") {
			t.Fatalf("public route failed: %+v %v", value.Preview, err)
		}
	}
}
