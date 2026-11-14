package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/auth"
	"ircord/backend/internal/filehost"
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
	Partial   bool   `json:"partial,omitempty" doc:"Older history in the range could not be loaded, so there may be more messages than count."`
}

// finishRedaction turns a deletion into the response. For a server owner on the
// associated server it deletes the hosted files when asked and writes the audit
// records; for the author it lists their own deletable uploads. Channel
// operators get neither.
func (s *Server) finishRedaction(ctx context.Context, r irc.RedactResult, err error, reason string, deleteFiles bool) (RedactResponse, error) {
	if err != nil {
		if r.OwnerActing {
			s.auditDeletion(ctx, r, reason, err, nil)
		}
		return RedactResponse{}, err
	}
	msg, attachments := r.Message, r.Attachments
	msg.EmbedGeometry = nil
	out := RedactResponse{Message: msg, Authority: r.Authority, AttachmentRetained: len(attachments) > 0}
	if r.OwnerActing {
		var files []filehost.HostedFile
		if deleteFiles && s.options.Files != nil {
			kept := 0
			for _, attachment := range attachments {
				f, removed, e := s.options.Files.DeleteHostedURL(ctx, attachment.URL)
				if e != nil {
					slog.Error("Deleting a moderated message's file failed", "error", e)
				}
				if removed {
					files = append(files, f)
				} else {
					kept++
				}
			}
			out.FilesDeleted, out.AttachmentRetained = len(files), kept > 0
		}
		s.auditDeletion(ctx, r, reason, nil, files)
		return out, nil
	}
	if s.options.Files != nil && r.Authority == irc.AuthorityAuthor {
		for _, attachment := range attachments {
			if id := s.options.Files.DeletableUpload(owner(ctx), attachment.URL); id != "" {
				if len(attachments) == 1 {
					out.UploadID = id
				} else {
					out.UploadIDs = append(out.UploadIDs, id)
				}
			}
		}
	}
	return out, nil
}

// auditDeletion records a server owner's deletion of one message, and one row
// for each uploaded file deleted with it naming the file and its uploader. The
// message text is never recorded.
func (s *Server) auditDeletion(ctx context.Context, r irc.RedactResult, reason string, err error, files []filehost.HostedFile) {
	if s.options.Store == nil {
		return
	}
	reason = strings.TrimSpace(reason)
	outcome := "deleted"
	switch {
	case errors.Is(err, irc.ErrAlreadyDeleted):
		outcome = "already deleted"
	case errors.Is(err, irc.ErrRedactRefused):
		outcome = "refused: " + strings.TrimPrefix(strings.TrimPrefix(err.Error(), irc.ErrRedactRefused.Error()), ": ")
	case errors.Is(err, irc.ErrRedactUnconfirmed):
		outcome = "unconfirmed"
	case err != nil:
		outcome = "failed"
	case len(files) == 1:
		outcome = "deleted with 1 file"
	case len(files) > 1:
		outcome = fmt.Sprintf("deleted with %d files", len(files))
	}
	actor := principal(ctx)
	s.audit(actor, "message-delete", strings.Join([]string{r.Channel, withAccount(r.Sender, r.Original.SenderAccount), r.Original.UpstreamID}, " · "), reason, outcome)
	for _, f := range files {
		uploader := f.Owner
		if u, e := s.options.Store.AdminUser(ctx, f.Owner); e == nil {
			uploader = u.Username
		}
		s.audit(actor, "file-delete", f.ID+" · uploaded by "+uploader, reason, "deleted")
	}
}

// auditPurge records a server owner's recent-messages deletion when it starts
// and when it finishes, without message text.
func (s *Server) auditPurge(actor auth.Principal, job irc.PurgeJob, span, reason string) {
	if !job.OwnerActing || s.options.Store == nil {
		return
	}
	outcome := fmt.Sprintf("started: %d selected", job.Total)
	if job.State == "done" {
		outcome = fmt.Sprintf("deleted %d of %d", job.Deleted, job.Total)
		if job.TooOld > 0 {
			outcome += fmt.Sprintf(", %d too old", job.TooOld)
		}
		if job.Failed > 0 {
			outcome += fmt.Sprintf(", %d failed", job.Failed)
		}
	}
	_ = s.options.Store.RecordOwnerEvent(storage.OwnerEvent{Actor: actor.Account.Username, Action: "messages-delete-recent", Target: strings.Join([]string{job.Channel, withAccount(job.Nick, job.Account), purgeRangeNames[span]}, " · "), Reason: strings.TrimSpace(reason), Outcome: outcome})
}

// withAccount adds the IRC account to a nick when it differs.
func withAccount(nick, account string) string {
	if account != "" && !strings.EqualFold(account, nick) {
		return nick + " (" + account + ")"
	}
	return nick
}

func (s *Server) registerModeration() {
	huma.Register(s.API, huma.Operation{OperationID: "recent-messages-preview", Method: "GET", Path: "/api/v1/conversations/{id}/members/{memberId}/recent-messages", Summary: "Count a person's recent messages in a channel", Description: "For channel operators and, on the associated server, server owners. Loads the channel's history back to the start of the range and counts the messages this member sent, matched by account when known. Nothing is deleted."}, func(ctx context.Context, in *struct {
		ID       string `path:"id"`
		MemberID string `path:"memberId"`
		Range    string `query:"range" enum:"1h,24h,7d" default:"24h"`
	}) (*Output[RecentMessagesPreview], error) {
		selection, err := s.Manager.RecentMessages(ctx, owner(ctx), in.ID, in.MemberID, purgeRanges[in.Range])
		return result(RecentMessagesPreview{Nick: selection.Nick, Count: len(selection.Messages), Limit: irc.PurgeMaxMessages, Authority: selection.Authority, Partial: selection.Partial}, err)
	})
	huma.Register(s.API, huma.Operation{OperationID: "delete-recent-messages", Method: "POST", Path: "/api/v1/conversations/{id}/members/{memberId}/recent-messages/deletion", DefaultStatus: 202, Summary: "Delete a person's recent messages in a channel", Description: "Starts a background job that deletes the messages the preview counts, one REDACT at a time, each confirmed by the network before the next. Poll the returned job for progress. Server owners' jobs on the associated server are recorded in the owner audit log when they start and finish, without message text."}, func(ctx context.Context, in *struct {
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
			s.auditPurge(actor, job, span, reason)
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
