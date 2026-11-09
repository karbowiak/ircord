package irc

import (
	"strconv"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
)

const (
	// pingInterval is how often the client pings the server to keep the
	// connection alive; each reply is a latency sample.
	pingInterval = 20 * time.Second
	// latencyWindow is how far back samples are kept.
	latencyWindow = 30 * time.Minute
	// maxLatencySamples bounds a network's samples whatever the server sends.
	maxLatencySamples = int(latencyWindow/pingInterval) + 30
	// maxRoundTrip discards replies to pings that aren't ours or are stale.
	maxRoundTrip = 2 * time.Minute
)

// recordLatency times a keep-alive reply. girc's PING token is the send time
// in Unix nanoseconds, which the server echoes in its PONG, so the round trip
// needs no shared state with the ping loop. Other PONGs (a token typed in a
// raw command) don't parse as a recent time and are ignored.
func (c *connection) recordLatency(e girc.Event, now time.Time) {
	if e.Command != "PONG" || len(e.Params) == 0 {
		return
	}
	sent, err := strconv.ParseInt(e.Last(), 10, 64)
	if err != nil {
		return
	}
	rtt := now.Sub(time.Unix(0, sent))
	if rtt < 0 || rtt > maxRoundTrip {
		return
	}
	sample := model.LatencySample{At: now.UTC(), Milliseconds: float64(rtt.Microseconds()) / 1000}
	w := c.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	w.latency[c.config.ID] = recentSamples(append(w.latency[c.config.ID], sample), now)
}

// recentSamples drops samples older than the window, keeping at most the cap.
func recentSamples(samples []model.LatencySample, now time.Time) []model.LatencySample {
	start := 0
	for start < len(samples) && now.Sub(samples[start].At) > latencyWindow {
		start++
	}
	samples = samples[start:]
	if len(samples) > maxLatencySamples {
		samples = samples[len(samples)-maxLatencySamples:]
	}
	return samples
}

// ConnectionLatency returns a network's keep-alive round trips over the last
// half hour, oldest first. They span reconnects; a gap means no replies.
func (m *Manager) ConnectionLatency(owner, network string) (model.ConnectionLatency, error) {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.networks[network]; !ok {
		return model.ConnectionLatency{}, ErrNotFound
	}
	samples := recentSamples(w.latency[network], time.Now())
	return model.ConnectionLatency{
		NetworkID:       network,
		IntervalSeconds: int(pingInterval / time.Second),
		WindowSeconds:   int(latencyWindow / time.Second),
		Samples:         append([]model.LatencySample{}, samples...),
	}, nil
}
