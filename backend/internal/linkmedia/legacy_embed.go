package linkmedia

import (
	"encoding/json"
	"strings"
)

// LegacyEmbed retains the persisted Iframely response format so cached embeds
// can be identified and rejected. The native resolver does not create them.
type LegacyEmbed struct {
	ID    string           `json:"id,omitempty"`
	URL   string           `json:"url,omitempty"`
	HTML  string           `json:"html,omitempty"`
	Rel   []string         `json:"rel,omitempty"`
	Meta  LegacyEmbedMeta  `json:"meta"`
	Links LegacyEmbedLinks `json:"links,omitempty"`
}

type LegacyEmbedMeta struct {
	Title       string  `json:"title,omitempty"`
	Description string  `json:"description,omitempty"`
	Author      string  `json:"author,omitempty"`
	AuthorURL   string  `json:"author_url,omitempty"`
	Site        string  `json:"site,omitempty"`
	Canonical   string  `json:"canonical,omitempty"`
	Medium      string  `json:"medium,omitempty"`
	Duration    float64 `json:"duration,omitempty"`
	Date        string  `json:"date,omitempty"`
}

type LegacyEmbedLink struct {
	Href  string                     `json:"href,omitempty"`
	Type  string                     `json:"type,omitempty"`
	Rel   []string                   `json:"rel,omitempty"`
	HTML  string                     `json:"html,omitempty"`
	Media map[string]json.RawMessage `json:"media,omitempty"`
}

// LegacyEmbedLinks accepts both grouped links and the older flat list.
type LegacyEmbedLinks map[string][]LegacyEmbedLink

func (l *LegacyEmbedLinks) UnmarshalJSON(data []byte) error {
	if strings.HasPrefix(strings.TrimSpace(string(data)), "[") {
		var flat []LegacyEmbedLink
		if err := json.Unmarshal(data, &flat); err != nil {
			return err
		}
		*l = LegacyEmbedLinks{}
		for _, link := range flat {
			key := "other"
			if len(link.Rel) > 0 {
				key = link.Rel[0]
			}
			(*l)[key] = append((*l)[key], link)
		}
		return nil
	}
	type grouped LegacyEmbedLinks
	return json.Unmarshal(data, (*grouped)(l))
}
