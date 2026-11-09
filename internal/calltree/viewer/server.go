package viewer

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PranavJoshi2893/codebound/internal/calltree/diagram"
	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
)

func Handler(svg []byte, graph *model.Graph) http.Handler {
	index := diagram.IndexGraph(graph)
	graphJSON, jsonError := json.Marshal(describeGraph(index))
	views := newViewCache(func(ctx context.Context, options diagram.BrowserOptions) ([]byte, error) {
		return renderViewSVG(ctx, index, options)
	})
	mux := http.NewServeMux()
	registerAssets(mux)
	mux.HandleFunc("GET /calltree.svg", func(writer http.ResponseWriter, request *http.Request) {
		options := diagram.BrowserOptions{Focus: -1}
		switch request.URL.Query().Get("mode") {
		case "", "calls":
		case "contexts":
			options.Contexts = true
		default:
			http.Error(writer, "invalid graph mode", http.StatusBadRequest)
			return
		}
		if focus := request.URL.Query().Get("focus"); focus != "" {
			id, err := strconv.Atoi(focus)
			if err != nil || id < 0 || id >= len(index.Functions) {
				http.Error(writer, "invalid graph focus", http.StatusBadRequest)
				return
			}
			options.Focus = id
		}
		image := svg
		if options.Contexts || options.Focus >= 0 {
			if graph == nil {
				http.Error(writer, "graph data unavailable", http.StatusServiceUnavailable)
				return
			}
			rendered, err := views.image(request.Context(), options)
			if err != nil {
				http.Error(writer, "could not render this graph view", http.StatusInternalServerError)
				return
			}
			image = rendered
		}
		writer.Header().Set("Content-Type", "image/svg+xml")
		_, _ = writer.Write(image)
	})
	mux.HandleFunc("GET /graph.json", func(writer http.ResponseWriter, _ *http.Request) {
		if jsonError != nil {
			http.Error(writer, "graph details unavailable", http.StatusInternalServerError)
			return
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = writer.Write(graphJSON)
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
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; connect-src 'self'; img-src 'self' blob:; style-src 'self' 'unsafe-inline'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
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
func Serve(ctx context.Context, listener net.Listener, svg []byte, graph *model.Graph) error {
	server := &http.Server{
		Handler:           Handler(svg, graph),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      renderTimeout + 5*time.Second,
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
