package linkmedia

import (
	"context"
	"net/http"
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

func TestDirectFileRoutingHint(t *testing.T) {
	if DirectFileCandidate("https://example.com/article") {
		t.Fatal("extensionless page routed as a file")
	}
}
