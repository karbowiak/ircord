package api

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"path/filepath"
	"strings"
	"testing"

	"ircord/backend/internal/auth"
	"ircord/backend/internal/filehost"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

type moderationFixture struct {
	s      *Server
	store  *storage.Store
	files  *filehost.Service
	ctx    context.Context
	upload func() string
}

func newModerationFixture(t *testing.T) moderationFixture {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.Open(filepath.Join(dir, "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	for _, account := range []string{"owner", "uploader"} {
		if _, err = store.CreateAccount(account, account); err != nil {
			t.Fatal(err)
		}
	}
	files, err := filehost.New(filehost.Options{Store: store, Blob: filehost.Local{Root: filepath.Join(dir, "files")}, PublicURL: "https://chat.example"})
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	_ = png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	ctx := context.WithValue(context.Background(), principalKey{}, auth.Principal{Account: storage.Account{ID: "owner", Username: "owner"}})
	ctx = context.WithValue(ctx, ownerKey{}, "owner")
	f := moderationFixture{s: &Server{options: Options{Store: store, Files: files}}, store: store, files: files, ctx: ctx}
	f.upload = func() string {
		t.Helper()
		u, err := files.Upload(context.Background(), "uploader", "network", "cat.png", data.Bytes(), files.LocalProvider())
		if err != nil {
			t.Fatal(err)
		}
		a, err := files.Attachment("uploader", "network", u.ID)
		if err != nil {
			t.Fatal(err)
		}
		return a.URL
	}
	return f
}

func (f moderationFixture) events(t *testing.T) []storage.OwnerEvent {
	t.Helper()
	events, err := f.store.OwnerEvents(context.Background(), 50)
	if err != nil {
		t.Fatal(err)
	}
	return events
}

func deletion(authority string, owner bool, urls ...string) irc.RedactResult {
	r := irc.RedactResult{Authority: authority, OwnerActing: owner, Channel: "#general", Sender: "spammer", Message: irc.Message{Deleted: true, Text: "Message deleted"}, Original: irc.Message{UpstreamID: "msg-1", SenderAccount: "spamacct", Text: "the abusive text"}}
	for _, u := range urls {
		r.Attachments = append(r.Attachments, model.Attachment{URL: u})
	}
	return r
}

func TestOperatorDeletionKeepsFilesAndWritesNoAudit(t *testing.T) {
	f := newModerationFixture(t)
	url := f.upload()
	out, err := f.s.finishRedaction(f.ctx, deletion(irc.AuthorityOperator, false, url), nil, "spam", true)
	if err != nil || out.FilesDeleted != 0 || !out.AttachmentRetained || out.UploadID != "" {
		t.Fatalf("operator: %+v %v", out, err)
	}
	if f.files.DeletableUpload("uploader", url) == "" {
		t.Fatal("an operator's deletion removed someone's upload")
	}
	if events := f.events(t); len(events) != 0 {
		t.Fatalf("an operator's deletion was audited: %+v", events)
	}
}

func TestServerOwnerDeletionDeletesHostedFilesAndAudits(t *testing.T) {
	f := newModerationFixture(t)
	url := f.upload()
	// A server owner who is also a channel operator: the deletion used their own
	// connection, but files and audit follow ownership.
	r := deletion(irc.AuthorityOperator, true, url, "https://elsewhere.example/a.png")
	out, err := f.s.finishRedaction(f.ctx, r, nil, " spam ", true)
	if err != nil || out.FilesDeleted != 1 || !out.AttachmentRetained {
		t.Fatalf("owner: %+v %v", out, err)
	}
	if f.files.DeletableUpload("uploader", url) != "" {
		t.Fatal("the hosted upload is still there")
	}
	events := f.events(t)
	if len(events) != 2 {
		t.Fatalf("audit rows: %+v", events)
	}
	file, deleted := events[0], events[1]
	if deleted.Actor != "owner" || deleted.Action != "message-delete" || deleted.Target != "#general · spammer (spamacct) · msg-1" || deleted.Reason != "spam" || deleted.Outcome != "deleted with 1 file" {
		t.Fatalf("message row: %+v", deleted)
	}
	if file.Action != "file-delete" || !strings.HasSuffix(file.Target, "uploaded by uploader") || file.Outcome != "deleted" {
		t.Fatalf("file row: %+v", file)
	}
	for _, e := range events {
		if strings.Contains(e.Target+e.Reason+e.Outcome, "abusive") {
			t.Fatalf("the audit log kept message text: %+v", e)
		}
	}
}

func TestServerOwnerDeletionAuditsRefusalsAndKeepsFilesUnlessAsked(t *testing.T) {
	f := newModerationFixture(t)
	url := f.upload()
	if _, err := f.s.finishRedaction(f.ctx, deletion(irc.AuthorityOwner, true, url), errors.Join(irc.ErrRedactRefused, errors.New("This message does not exist or is too old")), "", true); !errors.Is(err, irc.ErrRedactRefused) {
		t.Fatal(err)
	}
	if _, err := f.s.finishRedaction(f.ctx, deletion(irc.AuthorityOwner, true, url), irc.ErrAlreadyDeleted, "", true); !errors.Is(err, irc.ErrAlreadyDeleted) {
		t.Fatal(err)
	}
	out, err := f.s.finishRedaction(f.ctx, deletion(irc.AuthorityOwner, true, url), nil, "", false)
	if err != nil || out.FilesDeleted != 0 || !out.AttachmentRetained {
		t.Fatalf("without deleteFiles: %+v %v", out, err)
	}
	if f.files.DeletableUpload("uploader", url) == "" {
		t.Fatal("a refused or unrequested deletion removed the upload")
	}
	events := f.events(t)
	if len(events) != 3 || !strings.HasPrefix(events[2].Outcome, "refused") || events[1].Outcome != "already deleted" || events[0].Outcome != "deleted" {
		t.Fatalf("audit rows: %+v", events)
	}
}

func TestServerOwnerRecentDeletionIsAuditedAtStartAndEnd(t *testing.T) {
	f := newModerationFixture(t)
	actor := auth.Principal{Account: storage.Account{ID: "owner", Username: "owner"}}
	job := irc.PurgeJob{Channel: "#general", Nick: "spammer", Account: "spamacct", OwnerActing: true, State: "running", Total: 5}
	f.s.auditPurge(actor, job, "24h", "spam")
	job.State, job.Deleted, job.TooOld, job.Failed = "done", 3, 1, 1
	f.s.auditPurge(actor, job, "24h", "spam")
	f.s.auditPurge(actor, irc.PurgeJob{Channel: "#general", Nick: "x", State: "done"}, "1h", "")
	events := f.events(t)
	if len(events) != 2 {
		t.Fatalf("rows: %+v", events)
	}
	done, started := events[0], events[1]
	if started.Action != "messages-delete-recent" || started.Target != "#general · spammer (spamacct) · last 24 hours" || started.Outcome != "started: 5 selected" || started.Reason != "spam" {
		t.Fatalf("start row: %+v", started)
	}
	if done.Outcome != "deleted 3 of 5, 1 too old, 1 failed" {
		t.Fatalf("finish row: %+v", done)
	}
}
