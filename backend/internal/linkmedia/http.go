package linkmedia

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ircord/backend/internal/mediafetch"
)

// Every native fetch, including discovery endpoints and redirects, shares this
// public-only transport. Caller transports and cookie jars cannot bypass it.
func nativeHTTPClient() *http.Client {
	client := mediafetch.Client()
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) > 3 {
			return ErrUnavailable
		}
		if _, err := mediafetch.URL(next.URL.String()); err != nil {
			return err
		}
		if fileCapabilityURL(next.URL.String()) {
			return ErrRestricted
		}
		if len(via) > 0 && (next.URL.Scheme != via[0].URL.Scheme || !strings.EqualFold(next.URL.Host, via[0].URL.Host)) {
			for key := range next.Header {
				next.Header.Del(key)
			}
		}
		next.Header.Set("Accept-Encoding", "identity")
		return nil
	}
	return client
}

func (r *Resolver) fetch(ctx context.Context, raw string, accept string) (*http.Response, error) {
	if _, err := mediafetch.URL(raw); err != nil {
		return nil, err
	}
	if fileCapabilityURL(raw) {
		return nil, ErrRestricted
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", "IRCord-LinkPreview/1.0")
	res, err := r.http.Do(req)
	if err != nil {
		return nil, ErrUnavailable
	}
	if encoding := strings.TrimSpace(res.Header.Get("Content-Encoding")); encoding != "" && !strings.EqualFold(encoding, "identity") {
		res.Body.Close()
		return nil, ErrLimit // no unbounded HTTP decompression
	}
	return res, nil
}

func httpClient(client *http.Client) *http.Client {
	if client == nil {
		return mediafetch.Client()
	}
	copy := *client
	// Extraction is public-only. A supplied client must never bring an account
	// cookie jar into media probes, provider requests, or CDN source downloads.
	copy.Jar = nil
	if copy.Timeout == 0 {
		copy.Timeout = 20 * time.Second
	}
	return &copy
}

func readResponse(res *http.Response, provider string, limit int64) ([]byte, error) {
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusPartialContent {
		retry, _ := strconv.Atoi(res.Header.Get("Retry-After"))
		return nil, &HTTPError{Provider: provider, Status: res.StatusCode, RetryAfter: time.Duration(retry) * time.Second}
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, ErrLimit
	}
	return data, nil
}

func request(ctx context.Context, client *http.Client, req *http.Request, provider string, limit int64, out any) error {
	res, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmtUnavailable(provider)
	}
	data, err := readResponse(res, provider, limit)
	if err != nil {
		return err
	}
	if json.Unmarshal(data, out) != nil {
		return errors.New(provider + " returned invalid JSON")
	}
	return nil
}

func fmtUnavailable(provider string) error {
	return errors.Join(ErrUnavailable, errors.New(provider+" request failed"))
}
