package irc

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
)

// A fenced block as the composer sends it: fences, indentation, blank lines,
// trailing spaces, backticks and Unicode travel byte for byte.
const fencedCode = "look:\n```go\nfunc main() {\n\tfmt.Println(\"`hi` **not bold** 🐈\")  \n\n    // https://example.com/in_code\n}\n```\ndone"

// The frontend and the server must agree on what is code: nothing the browser
// draws as code may get a server link preview. Both test suites load this table.
func TestOutsideCodeMatchesTheFrontend(t *testing.T) {
	raw, err := os.ReadFile("../../../frontend/tests/fixtures/code-parity.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name, Text string
		URLs       []string
	}
	if err = json.Unmarshal(raw, &cases); err != nil || len(cases) == 0 {
		t.Fatalf("parity fixture: %v", err)
	}
	for _, tc := range cases {
		got := linkURL.FindAllString(outsideCode(tc.Text), -1)
		if !slices.Equal(got, tc.URLs) && !(len(got) == 0 && len(tc.URLs) == 0) {
			t.Errorf("%s: links outside code %q, want %q", tc.Name, got, tc.URLs)
		}
	}
	if raw := linkURL.FindString(outsideCode(fencedCode)); raw != "" {
		t.Fatalf("a link inside code would be previewed: %q", raw)
	}
}

func TestFencedCodeNeedsMultilineAndKeepsEveryLine(t *testing.T) {
	m, c, conv, _ := redactFixture(t, map[string]bool{"message-tags": true})
	c.pending = map[string]pending{}
	err := m.Send("alice", SendInput{ClientID: "code-1", ConversationID: conv.ID, Text: fencedCode})
	if err == nil || !strings.Contains(err.Error(), "multiline") {
		t.Fatalf("a multi-line code block without draft/multiline: %v", err)
	}
	if len(c.queue) != 0 {
		t.Fatal("an unsupported multi-line message reached the network")
	}

	c.caps["batch"], c.caps["draft/multiline"] = true, true
	c.capValues = map[string]string{"draft/multiline": "max-bytes=4096,max-lines=100"}
	if err = m.Send("alice", SendInput{ClientID: "code-2", ConversationID: conv.ID, Text: fencedCode}); err != nil {
		t.Fatal(err)
	}
	sent := <-c.queue
	if sent[0].Command != "BATCH" || sent[0].Params[1] != "draft/multiline" || sent[len(sent)-1].Command != "BATCH" {
		t.Fatalf("not one multiline batch: %v", sent)
	}
	var lines []string
	for _, e := range sent[1 : len(sent)-1] {
		if e.Command != "PRIVMSG" {
			t.Fatalf("unexpected line in batch: %v", e)
		}
		lines = append(lines, e.Last())
	}
	if got := strings.Join(lines, "\n"); got != fencedCode {
		t.Fatalf("wire text changed:\n%q\nwant\n%q", got, fencedCode)
	}
}
