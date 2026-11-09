package push

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestPushTransportBlocksPrivateDestinations(t *testing.T) {
	client := safeHTTPClient()
	defer client.CloseIdleConnections()
	for _, raw := range []string{"https://127.0.0.1/", "https://[::1]/", "https://192.0.2.1/", "https://[64:ff9b::7f00:1]/"} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, raw, nil)
		if err != nil {
			t.Fatal(err)
		}
		res, err := client.Do(req)
		cancel()
		if err == nil {
			res.Body.Close()
			t.Errorf("accepted private/reserved push destination %s", raw)
		}
	}
}
