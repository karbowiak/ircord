package api

import (
	"context"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/storage"
)

// The Hash Field high score table (Settings → About): one table shared by every account on the
// instance. A client submits its own score, so the server bounds what it accepts and how often.
const (
	hashFieldMaxScore = 10_000_000
	hashFieldMaxName  = 24
	// hashFieldGap is the shortest time between two submissions from one account; a game with
	// three ships and a spawn shield cannot end faster.
	hashFieldGap = 10 * time.Second
	// hashFieldHourly caps one account's submissions per hour.
	hashFieldHourly = 20
	// hashFieldAccounts bounds the limiter's memory.
	hashFieldAccounts = 10_000
)

type HashFieldEntry struct {
	Rank  int    `json:"rank" doc:"Place in the table, starting at 1"`
	Name  string `json:"name"`
	Score int64  `json:"score"`
}
type HashFieldTable struct {
	Items []HashFieldEntry `json:"items" doc:"The top entries, best first"`
}
type HashFieldResult struct {
	Placed bool             `json:"placed" doc:"Whether the score made the table"`
	Rank   int              `json:"rank" doc:"The new entry's place, or 0 when it did not place"`
	Items  []HashFieldEntry `json:"items" doc:"The table after the submission, best first"`
}
type HashFieldScoreInput struct {
	Body struct {
		Name  string `json:"name" minLength:"1" maxLength:"64" doc:"Name shown in the table; trimmed, at most 24 characters"`
		Score int64  `json:"score" minimum:"10" maximum:"10000000" multipleOf:"10" doc:"Final score of one game"`
	}
}

type hashFieldLimiter struct {
	mu       sync.Mutex
	accounts map[string][]time.Time
}

// allow records a submission for the account when it is within the limits.
func (l *hashFieldLimiter) allow(account string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.accounts == nil {
		l.accounts = map[string][]time.Time{}
	}
	hour := now.Add(-time.Hour)
	recent := l.accounts[account][:0]
	for _, at := range l.accounts[account] {
		if at.After(hour) {
			recent = append(recent, at)
		}
	}
	if len(recent) > 0 && now.Sub(recent[len(recent)-1]) < hashFieldGap || len(recent) >= hashFieldHourly {
		l.accounts[account] = recent
		return false
	}
	if _, known := l.accounts[account]; !known && len(l.accounts) >= hashFieldAccounts {
		for key, times := range l.accounts {
			if len(times) == 0 || !times[len(times)-1].After(hour) {
				delete(l.accounts, key)
			}
		}
		if len(l.accounts) >= hashFieldAccounts {
			return false
		}
	}
	l.accounts[account] = append(recent, now)
	return true
}

// The reasons a submitted name is refused.
const (
	hashFieldNameLength     = "enter a name of 1 to 24 characters"
	hashFieldNameCharacters = "that name has characters the table can't show"
)

// hashFieldRune reports whether a name may contain r: letters, marks, numbers, punctuation,
// symbols and spaces, plus the zero-width joiner that builds emoji sequences. Controls, other
// format characters (bidi overrides), private use, unassigned code points and U+FFFD, which
// stands for invalid input, are refused rather than shown.
func hashFieldRune(r rune) bool {
	if r == unicode.ReplacementChar {
		return false
	}
	return r == '\u200d' || unicode.IsSpace(r) || unicode.In(r, unicode.L, unicode.M, unicode.N, unicode.P, unicode.S, unicode.Zs)
}

// hashFieldName trims a submitted name and folds runs of whitespace. It returns the name, or
// the reason it is refused.
func hashFieldName(raw string) (string, string) {
	for _, r := range raw {
		if !hashFieldRune(r) {
			return "", hashFieldNameCharacters
		}
	}
	name := strings.Join(strings.Fields(raw), " ")
	if name == "" || len([]rune(name)) > hashFieldMaxName {
		return "", hashFieldNameLength
	}
	return name, ""
}

func hashFieldEntries(rows []storage.HashFieldScore) []HashFieldEntry {
	items := make([]HashFieldEntry, len(rows))
	for i, row := range rows {
		items[i] = HashFieldEntry{Rank: i + 1, Name: row.Name, Score: row.Score}
	}
	return items
}

func (s *Server) registerHashField() {
	huma.Register(s.API, huma.Operation{OperationID: "get-hash-field-scores", Method: "GET", Path: "/api/v1/hash-field/scores", Summary: "List Hash Field high scores", Description: "The instance-wide top 10 of the Hash Field game in Settings → About."}, func(ctx context.Context, _ *struct{}) (*Output[HashFieldTable], error) {
		if s.options.Store == nil {
			return nil, huma.Error503ServiceUnavailable("high scores are not available")
		}
		rows, err := s.options.Store.HashFieldScores()
		if err != nil {
			return nil, huma.Error503ServiceUnavailable("high scores are not available")
		}
		return &Output[HashFieldTable]{Body: HashFieldTable{Items: hashFieldEntries(rows)}}, nil
	})
	huma.Register(s.API, huma.Operation{OperationID: "submit-hash-field-score", Method: "POST", Path: "/api/v1/hash-field/scores", Summary: "Submit a Hash Field score", Description: "Enters a finished game's score into the instance-wide top 10. A score that does not place is not stored. One account may submit once every 10 seconds and 20 times an hour."}, func(ctx context.Context, in *HashFieldScoreInput) (*Output[HashFieldResult], error) {
		if s.options.Store == nil {
			return nil, huma.Error503ServiceUnavailable("high scores are not available")
		}
		name, problem := hashFieldName(in.Body.Name)
		if problem != "" {
			return nil, huma.Error422UnprocessableEntity(problem)
		}
		if in.Body.Score <= 0 || in.Body.Score > hashFieldMaxScore || in.Body.Score%10 != 0 {
			return nil, huma.Error422UnprocessableEntity("that is not a Hash Field score")
		}
		if !s.hashField.allow(owner(ctx), time.Now()) {
			return nil, huma.Error429TooManyRequests("too many scores; try again shortly")
		}
		id, rows, err := s.options.Store.AddHashFieldScore(name, in.Body.Score)
		if err != nil {
			return nil, huma.Error503ServiceUnavailable("high scores are not available")
		}
		out := HashFieldResult{Placed: id != 0, Items: hashFieldEntries(rows)}
		for i, row := range rows {
			if row.ID == id {
				out.Rank = i + 1
			}
		}
		return &Output[HashFieldResult]{Body: out}, nil
	})
}
