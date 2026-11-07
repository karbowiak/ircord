// Export the same OpenAPI contract as the live service, without connecting to IRC.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"ircord/backend/internal/api"
	"ircord/backend/internal/auth"
	"ircord/backend/internal/filehost"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/storage"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	dir, err := os.MkdirTemp("", "ircord-openapi-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	store, err := storage.Open(dir)
	if err != nil {
		return err
	}
	defer store.Close()
	files, err := filehost.New(filehost.Options{Store: store, Blob: filehost.Local{Root: filepath.Join(dir, "files")}, PublicURL: "http://localhost:8080"})
	if err != nil {
		return err
	}
	manager := irc.New(irc.Options{Store: store, Files: files})
	defer manager.Close()
	sessions := auth.New(store, auth.Options{Registration: true})
	defer sessions.Close()
	server := api.New(manager, api.Options{Store: store, Files: files, Auth: sessions})
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(server.API.OpenAPI())
}
