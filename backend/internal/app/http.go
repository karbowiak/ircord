package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

func serve(ctx context.Context, server *http.Server) error {
	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("IRCord API listening", "address", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
	case err := <-serverErrors:
		if err != http.ErrServerClosed {
			return err
		}
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdown)
}
