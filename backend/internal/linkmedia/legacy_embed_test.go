package linkmedia

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestLegacyEmbedPayloadCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name, input, embed string
		invalid            bool
	}{
		{"absent", `{}`, `null`, false},
		{"null", `{"embed":null}`, `null`, false},
		{"empty", `{"embed":{}}`, `{"meta":{}}`, false},
		{"grouped", `{"embed":{"id":"id","url":"https://example.test","html":"<p>test</p>","rel":["app"],"meta":{"title":"Title","description":"Description","author":"Author","author_url":"https://example.test/author","site":"Site","canonical":"https://example.test/a","medium":"video","duration":1.5,"date":"today"},"links":{"player":[{"href":"https://example.test/embed","type":"text/html","rel":["player"],"html":"<iframe></iframe>","media":{"width":640}}]}}}`, `{"id":"id","url":"https://example.test","html":"<p>test</p>","rel":["app"],"meta":{"title":"Title","description":"Description","author":"Author","author_url":"https://example.test/author","site":"Site","canonical":"https://example.test/a","medium":"video","duration":1.5,"date":"today"},"links":{"player":[{"href":"https://example.test/embed","type":"text/html","rel":["player"],"html":"<iframe></iframe>","media":{"width":640}}]}}`, false},
		{"flat", `{"embed":{"links": [ {"rel":["image","ssl"],"href":"https://example.test/image"},{"href":"https://example.test/other"},{"rel":["image"],"type":"image/png"}]}}`, `{"meta":{},"links":{"image":[{"href":"https://example.test/image","rel":["image","ssl"]},{"type":"image/png","rel":["image"]}],"other":[{"href":"https://example.test/other"}]}}`, false},
		{"null links", `{"embed":{"links":null}}`, `{"meta":{}}`, false},
		{"empty flat links", `{"embed":{"links":[]}}`, `{"meta":{}}`, false},
		{"unknown field", `{"embed":{"unknown":"ignored"}}`, `{"meta":{}}`, false},
		{"invalid embed", `{"embed":"wrong"}`, ``, true},
		{"invalid metadata", `{"embed":{"meta":{"duration":"wrong"}}}`, ``, true},
		{"invalid grouped links", `{"embed":{"links":{"image":"wrong"}}}`, ``, true},
		{"invalid flat links", `{"embed":{"links":[42]}}`, ``, true},
		{"invalid json", `{"embed":`, ``, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var value Result
			err := json.Unmarshal([]byte(tc.input), &value)
			if tc.invalid {
				if err == nil {
					t.Fatal("invalid legacy payload accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(value.Embed)
			if err != nil {
				t.Fatal(err)
			}
			var got, want any
			if err = json.Unmarshal(encoded, &got); err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal([]byte(tc.embed), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("legacy encoding = %s, want %s", encoded, tc.embed)
			}
			result, err := json.Marshal(value)
			if err != nil || strings.Contains(string(result), `"embed":`) != (value.Embed != nil) {
				t.Fatalf("legacy omitempty = %s, error %v", result, err)
			}
		})
	}
}
