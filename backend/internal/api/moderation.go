package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/storage"
)

// Time ranges offered for deleting one person's recent messages.
var purgeRanges = map[string]time.Duration{"1h": time.Hour, "24h": 24 * time.Hour, "7d": irc.PurgeMaxAge}
var purgeRangeNames = map[string]string{"1h": "last hour", "24h": "last 24 hours", "7d": "last 7 days"}

type RecentMessagesPreview struct {
	Nick      string `json:"nick"`
	Count     int    `json:"count" doc:"Messages that would be deleted, at most limit."`
	Limit     int    `json:"limit"`
	Authority string `json:"authority" enum:"operator,owner"`
}

// auditDeletion records a server owner's deletion of one message. The message
// text is never recorded.
func (s *Server) auditDeletion(ctx context.Context, r irc.RedactResult, reason string, err error, files int) {
	if s.options.Store == nil {
		return
	}
	who := r.Sender
	if r.Original.SenderAccount != "" && !strings.EqualFold(r.Original.SenderAccount, who) {
		who += " (" + r.Original.SenderAccount + ")"
	}
	outcome := "deleted"
	switch {
	case errors.Is(err, irc.ErrRedactRefused):
		outcome = "refused: " + strings.TrimPrefix(strings.TrimPrefix(err.Error(), irc.ErrRedactRefused.Error()), ": ")
	case errors.Is(err, irc.ErrRedactUnconfirmed):
		outcome = "unconfirmed"
	case err != nil:
		outcome = "failed"
	case files == 1:
		outcome = "deleted with 1 file"
	case files > 1:
		outcome = fmt.Sprintf("deleted with %d files", files)
	}
	s.audit(principal(ctx), "message-delete", strings.Join([]string{r.Channel, who, r.Original.UpstreamID}, " · "), reason, outcome)
}

func (s *Server) registerModeration() {
	huma.Register(s.API, huma.Operation{OperationID: "recent-messages-preview", Method: "GET", Path: "/api/v1/conversations/{id}/members/{memberId}/recent-messages", Summary: "Count a person's recent messages in a channel", Description: "For channel operators and, on the associated server, server owners. Loads the channel's history back to the start of the range and counts the messages this member sent, matched by account when known. Nothing is deleted."}, func(ctx context.Context, in *struct {
		ID       string `path:"id"`
		MemberID string `path:"memberId"`
		Range    string `query:"range" enum:"1h,24h,7d" default:"24h"`
	}) (*Output[RecentMessagesPreview], error) {
		selection, err := s.Manager.RecentMessages(ctx, owner(ctx), in.ID, in.MemberID, purgeRanges[in.Range])
		return result(RecentMessagesPreview{Nick: selection.Nick, Count: len(selection.Messages), Limit: irc.PurgeMaxMessages, Authority: selection.Authority}, err)
	})
	huma.Register(s.API, huma.Operation{OperationID: "delete-recent-messages", Method: "POST", Path: "/api/v1/conversations/{id}/members/{memberId}/recent-messages/deletion", DefaultStatus: 202, Summary: "Delete a person's recent messages in a channel", Description: "Starts a background job that deletes the messages the preview counts, one REDACT at a time, each confirmed by the network before the next. Poll the returned job for progress. Server-owner jobs are recorded in the owner audit log without message text."}, func(ctx context.Context, in *struct {
		ID       string `path:"id"`
		MemberID string `path:"memberId"`
		Body     struct {
			Range  string `json:"range" enum:"1h,24h,7d"`
			Reason string `json:"reason,omitempty" maxLength:"300"`
		}
	}) (*Output[irc.PurgeJob], error) {
		actor := principal(ctx)
		reason, span := strings.TrimSpace(in.Body.Reason), in.Body.Range
		job, err := s.Manager.StartPurge(ctx, owner(ctx), in.ID, in.MemberID, purgeRanges[span], reason, func(job irc.PurgeJob) {
			if job.Authority != irc.AuthorityOwner || s.options.Store == nil {
				return
			}
			outcome := fmt.Sprintf("deleted %d of %d", job.Deleted, job.Total)
			_ = s.options.Store.RecordOwnerEvent(storage.OwnerEvent{Actor: actor.Account.Username, Action: "messages-delete-recent", Target: strings.Join([]string{job.Channel, job.Nick, purgeRangeNames[span]}, " · "), Reason: reason, Outcome: outcome})
		})
		return result(job, err)
	})
	huma.Register(s.API, huma.Operation{OperationID: "recent-messages-deletion", Method: "GET", Path: "/api/v1/recent-messages-deletions/{jobId}", Summary: "Progress of a recent-messages deletion"}, func(ctx context.Context, in *struct {
		JobID string `path:"jobId"`
	}) (*Output[irc.PurgeJob], error) {
		job, err := s.Manager.Purge(owner(ctx), in.JobID)
		return result(job, err)
	})
}
