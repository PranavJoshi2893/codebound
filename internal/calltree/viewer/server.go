package viewer

import (
	"context"
	_ "embed"
	"errors"
	"net"
	"net/http"
	"time"
)

//go:embed index.html
var indexHTML []byte

func Handler(svg []byte) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = writer.Write(indexHTML)
	})
	mux.HandleFunc("GET /calltree.svg", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "image/svg+xml")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = writer.Write(svg)
	})
	return mux
}

// Serve owns listener and stops gracefully when ctx is cancelled.
func Serve(ctx context.Context, listener net.Listener, svg []byte) error {
	server := &http.Server{
		Handler:           Handler(svg),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       time.Minute,
	}
	result := make(chan error, 1)
	go func() { result <- server.Serve(listener) }()
	select {
	case err := <-result:
		return serverError(err)
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			_ = server.Close()
			<-result
			return err
		}
		return serverError(<-result)
	}
}

func serverError(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
