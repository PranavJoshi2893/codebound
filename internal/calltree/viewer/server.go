package viewer

import (
	"context"
	_ "embed"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"
)

//go:embed index.html
var indexHTML []byte

//go:embed viewer.js
var viewerJS []byte

func Handler(svg []byte) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = writer.Write(indexHTML)
	})
	mux.HandleFunc("GET /viewer.js", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = writer.Write(viewerJS)
	})
	mux.HandleFunc("GET /calltree.svg", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "image/svg+xml")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = writer.Write(svg)
	})
	return protectLocalViewer(mux)
}

// Binding to loopback alone does not stop DNS rebinding: a hostile website can
// resolve its own hostname to 127.0.0.1. Reject that Host and cross-site requests.
func protectLocalViewer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("X-Frame-Options", "DENY")
		writer.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		writer.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; img-src 'self'; style-src 'unsafe-inline'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
		host := request.Host
		if name, _, err := net.SplitHostPort(host); err == nil {
			host = name
		}
		switch strings.ToLower(host) {
		case "127.0.0.1", "localhost", "::1", "[::1]":
		default:
			http.Error(writer, "local viewer requires a loopback host", http.StatusForbidden)
			return
		}
		origin := request.Header.Get("Origin")
		if (origin != "" && origin != "http://"+request.Host) || request.Header.Get("Sec-Fetch-Site") == "cross-site" {
			http.Error(writer, "cross-site viewer access denied", http.StatusForbidden)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

// Serve owns listener and stops gracefully when ctx is cancelled.
func Serve(ctx context.Context, listener net.Listener, svg []byte) error {
	server := &http.Server{
		Handler:           Handler(svg),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       time.Minute,
		MaxHeaderBytes:    16 << 10,
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
