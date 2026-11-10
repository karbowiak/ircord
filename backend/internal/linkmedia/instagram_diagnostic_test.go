package linkmedia

import (
	"context"
	"os"
	"testing"
	"time"
)

// Explicit opt-in for the reported public post. Sources use anonymous public
// egress, while object writes and manifests use disposable test storage only.
// Never print signed URLs, captions, headers, cookies or raw extractor errors.
func TestLiveInstagramReportedPost(t *testing.T) {
	if os.Getenv("IRCORD_TEST_INSTAGRAM_REPORTED_POST") != "true" {
		t.Skip("set IRCORD_TEST_INSTAGRAM_REPORTED_POST=true for the reported anonymous post probe")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	raw := "https://www.instagram.com/p/DfhCCWHlLFS/?img_index=6&dlrf=MTMwc2w3djJ1MTBxNA=="
	canonical, provider, id, err := socialSource(raw)
	if err != nil {
		t.Fatal(err)
	}
	value, err := NewYTDLP(YTDLPOptions{Timeout: 35 * time.Second}).Resolve(ctx, canonical)
	t.Logf("anonymous extractor: %v", socialProbeSummary(value, err, provider, id))
	if err != nil {
		t.Fatal("anonymous metadata unavailable: " + failureReason(err))
	}
	puts, deletes := 0, 0
	r, _ := newSocialTestResolver(t, &puts, &deletes)
	r.options.Downloader = connectorFunc(func(context.Context, string) (Result, error) { return value, nil })
	result, err := r.ResolveSource(ctx, raw)
	if err != nil || len(result.Assets) != 9 || result.URL != raw || puts != 9 || deletes != 0 {
		reason := "unexpected_result"
		if result.Preview != nil {
			reason = result.Preview.Reason
		}
		t.Fatalf("full gallery delivery: assets=%d puts=%d deletes=%d reason=%s error=%t", len(result.Assets), puts, deletes, reason, err != nil)
	}
	var downloaded, stored int64
	for index, asset := range result.Assets {
		if asset.Kind != "image" || !r.IsDeliveryURL(asset.URL) || asset.SourceURL != "" || asset.Headers != nil || time.Until(asset.ExpiresAt) < 89*24*time.Hour {
			t.Fatalf("asset %d lost native delivery or source privacy", index+1)
		}
		downloaded += asset.downloadBytes
		stored += asset.Size
		t.Logf("asset %d: decoded=%dx%d storedBytes=%d downloadedBytes=%d", index+1, asset.Width, asset.Height, asset.Size, asset.downloadBytes)
	}
	t.Logf("completeGallery=true assets=%d storedBytes=%d downloadedBytes=%d withinAggregateBudget=%t", len(result.Assets), stored, downloaded, downloaded <= maxNativeResultBytes)
}
