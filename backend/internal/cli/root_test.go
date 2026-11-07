package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestLocalBackendCommandsAndCompatibleStartup(t *testing.T) {
	starts := 0
	serve := func(context.Context) error { starts++; return nil }
	for _, args := range [][]string{nil, {"serve"}} {
		if code := Execute(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}, serve); code != 0 {
			t.Fatal("compatible server invocation failed", code)
		}
	}
	if starts != 2 {
		t.Fatal("server was not started")
	}
	for _, args := range [][]string{{"--help"}, {"version"}, {"completion", "zsh"}, {"serve", "--help"}} {
		if code := Execute(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}, serve); code != 0 {
			t.Fatal("offline backend command failed", code)
		}
	}
	if starts != 2 {
		t.Fatal("offline backend commands started the server")
	}
	var out, errOut bytes.Buffer
	if code := Execute(context.Background(), []string{"--json", "version"}, &out, &errOut, serve); code != 0 || !json.Valid(out.Bytes()) || errOut.Len() != 0 {
		t.Fatal("backend version JSON failed")
	}
	if code := Execute(context.Background(), []string{"status"}, &bytes.Buffer{}, &bytes.Buffer{}, serve); code != 2 {
		t.Fatal("remote command unexpectedly present in backend CLI")
	}
	if code := Execute(context.Background(), nil, &bytes.Buffer{}, &bytes.Buffer{}, func(context.Context) error { return errors.New("startup failed") }); code != 1 {
		t.Fatal("startup error exit code is incorrect", code)
	}
}
