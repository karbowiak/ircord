package linkmedia

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"
	"time"
)

var socialReviewFixtures = []struct{ Name, URL string }{
	{"instagram-reel", "https://www.instagram.com/reel/DfX9cZwS4DI/"},
	{"instagram-gallery-one", "https://www.instagram.com/p/DfZcglGlDYM/"},
	{"instagram-gallery-two", "https://www.instagram.com/p/DfTNU4EHeRc/"},
	{"instagram-photo", "https://www.instagram.com/p/DXBJTJiDvgY/"},
	{"tiktok-one", "https://www.tiktok.com/@zaylento.yt/video/7703024247081291040?is_from_webapp=1&sender_device=pc"},
	{"tiktok-two", "https://www.tiktok.com/@john.erik.wagner1/video/7696385281938754838?is_from_webapp=1&sender_device=pc"},
}

func TestLiveSocialDeliveryHeaders(t *testing.T) {
	if flag := os.Getenv("IRCORD_TEST_SOCIAL_HEADERS"); flag != "true" && flag != "tiktok" {
		t.Skip("set IRCORD_TEST_SOCIAL_HEADERS=true for bounded anonymous delivery prefix probes")
	}
	y := NewYTDLP(YTDLPOptions{Timeout: 25 * time.Second})
	r := NewResolver(ResolverOptions{})
	for _, fixture := range []struct{ Name, URL string }{socialReviewFixtures[0], socialReviewFixtures[4], socialReviewFixtures[5]} {
		if os.Getenv("IRCORD_TEST_SOCIAL_HEADERS") == "tiktok" && fixture.Name == "instagram-reel" {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		value, err := y.Resolve(ctx, fixture.URL)
		if err != nil {
			t.Logf("%s source metadata unavailable: %s", fixture.Name, failureReason(err))
			cancel()
			continue
		}
		asset := value.Assets[0]
		res, err := r.fetchExtractionAsset(ctx, asset)
		report := map[string]any{"fixture": fixture.Name, "anonymousCookieOriginBound": asset.anonymousCookieOrigin != "", "authorizationForwarded": false}
		if err != nil {
			report["reason"] = failureReason(err)
		} else {
			prefix, readErr := io.ReadAll(io.LimitReader(res.Body, sniffBytes))
			res.Body.Close()
			report["status"], report["observedMime"], report["bytesRead"] = res.StatusCode, mediaType("", prefix), len(prefix)
			if readErr != nil {
				report["reason"] = "prefix_read_failed"
			}
		}
		cancel()
		encoded, _ := json.Marshal(report)
		t.Log(string(encoded))
	}
}

// Opt-in anonymous metadata probe. Failures are evidence, never live unit-test
// failures. Signed URLs, captions, headers, cookies and raw stderr stay private.
func TestLiveSocialMetadata(t *testing.T) {
	if os.Getenv("IRCORD_TEST_SOCIAL_METADATA") != "true" {
		t.Skip("set IRCORD_TEST_SOCIAL_METADATA=true for the six authorized anonymous metadata probes")
	}
	y := NewYTDLP(YTDLPOptions{Timeout: 25 * time.Second})
	i := NewInstagram(nil)
	for _, fixture := range socialReviewFixtures {
		canonical, provider, id, err := socialSource(fixture.URL)
		if err != nil {
			t.Fatal("invalid authorized fixture shape")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		value, err := y.Resolve(ctx, canonical)
		report := map[string]any{"fixture": fixture.Name, "primary": socialProbeSummary(value, err, provider, id)}
		if err != nil && provider == "instagram" {
			fallback, fallbackErr := i.Resolve(ctx, canonical)
			report["secondary"] = socialProbeSummary(fallback, fallbackErr, provider, id)
		}
		cancel()
		encoded, _ := json.Marshal(report)
		t.Log(string(encoded))
	}
}

func socialProbeSummary(value Result, err error, provider, id string) map[string]any {
	out := map[string]any{"assets": len(value.Assets)}
	if err != nil {
		reason := failureReason(err)
		if errors.Is(err, context.DeadlineExceeded) {
			reason = "extractor_timeout"
		}
		out["reason"] = reason
		return out
	}
	kinds := []string{}
	for _, asset := range value.Assets {
		kinds = append(kinds, asset.Kind)
	}
	out["kinds"], out["samePost"], out["validatedSourceMetadata"] = kinds, value.ID == id && sameSocialPost(value.URL, provider, id), validateSocialResult(value, provider, id) == nil
	return out
}
