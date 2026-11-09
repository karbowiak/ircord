package linkmedia

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"ircord/backend/internal/cdn"
)

// FromEnv constructs the backend clients without doing network I/O, starting
// background work, or attaching them to chat. The process must load its local
// .env before calling this, as it does for the other backend configuration.
func FromEnv() (*Resolver, error) { return fromEnv(os.Getenv) }

func fromEnv(get func(string) string) (*Resolver, error) {
	duration := func(name string, fallback time.Duration) (time.Duration, error) {
		if get(name) == "" {
			return fallback, nil
		}
		value, err := time.ParseDuration(get(name))
		if err != nil || value <= 0 {
			return 0, fmt.Errorf("%s must be a positive duration", name)
		}
		return value, nil
	}
	timeout, err := duration("IRCORD_LINKMEDIA_TIMEOUT", 75*time.Second)
	if err != nil {
		return nil, err
	}
	maxBytes := int64(50 << 20)
	if value := get("IRCORD_LINKMEDIA_MAX_FILE_BYTES"); value != "" {
		maxBytes, err = strconv.ParseInt(value, 10, 64)
		if err != nil || maxBytes <= 0 || maxBytes == math.MaxInt64 {
			return nil, errors.New("IRCORD_LINKMEDIA_MAX_FILE_BYTES must be a positive byte count below MaxInt64")
		}
	}
	o := ResolverOptions{Timeout: timeout, MaxFileBytes: maxBytes}
	if get("IRCORD_REDDIT_CLIENT_ID") != "" || get("IRCORD_REDDIT_CLIENT_SECRET") != "" {
		for _, name := range []string{"IRCORD_REDDIT_CLIENT_ID", "IRCORD_REDDIT_CLIENT_SECRET"} {
			if get(name) == "" {
				return nil, fmt.Errorf("%s is required when Reddit is configured", name)
			}
		}
		o.Reddit = NewReddit(RedditOptions{ClientID: get("IRCORD_REDDIT_CLIENT_ID"), ClientSecret: get("IRCORD_REDDIT_CLIENT_SECRET"), UserAgent: get("IRCORD_REDDIT_USER_AGENT")})
	}
	// Instagram, TikTok, yt-dlp, and Iframely remain inactive.
	var storageConfigured bool
	for _, name := range []string{"ENDPOINT", "BUCKET", "ACCESS_KEY", "SECRET_KEY", "SESSION_TOKEN", "PUBLIC_URL", "PREFIX", "REGION"} {
		if get("IRCORD_CDN_"+name) != "" {
			storageConfigured = true
		}
	}
	if storageConfigured {
		for _, name := range []string{"ENDPOINT", "BUCKET", "ACCESS_KEY", "SECRET_KEY", "PUBLIC_URL"} {
			if get("IRCORD_CDN_"+name) == "" {
				return nil, fmt.Errorf("IRCORD_CDN_%s is required when CDN storage is configured", name)
			}
		}
		prefix, region := get("IRCORD_CDN_PREFIX"), get("IRCORD_CDN_REGION")
		if prefix == "" {
			prefix = "media"
		}
		if region == "" {
			region = "us-east-1"
		}
		o.CDN, err = cdn.New(cdn.Options{Endpoint: get("IRCORD_CDN_ENDPOINT"), Bucket: get("IRCORD_CDN_BUCKET"), Region: region, Prefix: strings.Trim(prefix, "/"), PublicURL: get("IRCORD_CDN_PUBLIC_URL"), AccessKey: get("IRCORD_CDN_ACCESS_KEY"), SecretKey: get("IRCORD_CDN_SECRET_KEY"), SessionToken: get("IRCORD_CDN_SESSION_TOKEN")})
		if err != nil {
			return nil, err
		}
	}
	return NewResolver(o), nil
}
