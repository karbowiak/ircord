package linkmedia

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"ircord/backend/internal/mediafetch"
)

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
