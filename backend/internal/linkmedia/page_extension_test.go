package linkmedia

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// Pages whose last path segment contains a dot are ordinary pages, not files.
func TestDottedPageURLsResolveAsPages(t *testing.T) {
	for _, raw := range []string{
		"https://github.com/example/project/blob/main/README.md",
		"https://github.com/golang/go/releases/tag/go1.27.3",
		"https://en.wikipedia.org/wiki/Node.js",
		"https://www.npmjs.com/package/socket.io",
		"https://pkg.go.dev/golang.org/x/net@v0.62.0",
	} {
		t.Run(raw, func(t *testing.T) {
			r := NewResolver(ResolverOptions{})
			r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
				res := response(http.StatusOK, "text/html; charset=utf-8", `<!doctype html><html><head><title>A real page</title></head><body>Hello</body></html>`)
				res.Request = req
				return res, nil
			})}
			out, err := r.Resolve(context.Background(), raw)
			if err != nil {
				t.Fatal(err)
			}
			if out.Provider == "file" || out.Preview == nil || out.Preview.Kind == "file" || out.Preview.File != nil || out.Preview.Reason == "unsupported_type" {
				t.Fatalf("page was shown as a file: %+v %+v", out, out.Preview)
			}
			if out.Title != "A real page" && out.Preview.Title != "A real page" {
				t.Fatalf("page title missing: %+v %+v", out, out.Preview)
			}
		})
	}
}

// A dotted URL that serves a real file is still handled as a file.
func TestDottedFileURLStaysAFile(t *testing.T) {
	r := NewResolver(ResolverOptions{})
	r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		res := response(http.StatusOK, "application/pdf", "%PDF-1.7\nfixture")
		res.Request = req
		return res, nil
	})}
	out, err := r.Resolve(context.Background(), "https://example.com/docs/report.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if out.Provider != "file" || out.Preview == nil || out.Preview.Kind != "file" || out.Preview.File == nil || out.Preview.File.Name != "report.pdf" {
		t.Fatalf("file lost its file card: %+v %+v", out, out.Preview)
	}
}

// Every spelling of IRCord's file route is a capability. None may be fetched or
// copied by the public resolver.
func TestFileCapabilityVariantsAreNeverIngested(t *testing.T) {
	id, token := strings.Repeat("a", 32), strings.Repeat("b", 48)
	for _, raw := range []string{
		"https://other.example.test/files/" + id + "/" + token + "/photo.png",
		"https://other.example.test/sub/files/" + id + "/" + token + "/photo.png",
		"https://other.example.test/files/" + id + "/" + token + "/x/photo.png",
		"https://other.example.test/files/" + id + "/" + token + "/photo.png/",
		"https://other.example.test/files/" + id + "/" + token + "//photo.png",
	} {
		if !fileCapabilityURL(raw) {
			t.Fatalf("not recognized as a capability: %s", raw)
		}
		r := NewResolver(ResolverOptions{})
		r.http = noSourceNetwork(t)
		out, err := r.Resolve(context.Background(), raw)
		if err != nil || out.Preview == nil || out.Preview.Reason != "restricted" || len(out.Assets) != 0 {
			t.Fatalf("%s: %+v %v", raw, out, err)
		}
	}
	for _, raw := range []string{
		"https://cdn.shopify.com/s/files/1/0001/0002/products/photo.png",
		"https://www.drupal.org/sites/default/files/styles/photo.jpg",
		"https://other.example.test/files/" + id + "/" + token,
		"https://other.example.test/files/" + strings.Repeat("z", 32) + "/" + strings.Repeat("z", 48) + "/a.png",
	} {
		if fileCapabilityURL(raw) {
			t.Fatalf("recognized as a capability: %s", raw)
		}
	}
}

// Results stored for dotted page names as unsupported file cards must not be
// served after the fix, while media and document results are kept.
func TestDottedPagesLeaveStoredFileCards(t *testing.T) {
	old := resolutionVersion("https://example.com/article")
	for _, raw := range []string{
		"https://github.com/example/project/blob/main/README.md",
		"https://en.wikipedia.org/wiki/Node.js",
		"https://www.npmjs.com/package/socket.io",
		"https://pkg.go.dev/golang.org/x/net@v0.62.0",
	} {
		if v := resolutionVersion(raw); v == old {
			t.Fatalf("%s kept the stored version %s", raw, v)
		}
	}
	for _, raw := range []string{"https://example.com/photo.png", "https://example.com/report.pdf", "https://example.com/article"} {
		if v := resolutionVersion(raw); strings.HasPrefix(v, "native-file-v2") {
			t.Fatalf("%s lost its stored result: %s", raw, v)
		}
	}
}

func TestHTMLRejectionKeepsUnsupportedIdentity(t *testing.T) {
	r := NewResolver(ResolverOptions{})
	r.http = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		res := response(http.StatusOK, "text/html", `<!doctype html><title>x</title>`)
		res.Request = req
		return res, nil
	})}
	_, err := r.ingest(context.Background(), Asset{URL: "https://example.com/Node.js"}, "")
	if !errors.Is(err, ErrUnsupported) || !errors.Is(err, errHTMLDocument) || failureReason(err) != "unsupported_type" {
		t.Fatalf("html rejection: %v", err)
	}
}
