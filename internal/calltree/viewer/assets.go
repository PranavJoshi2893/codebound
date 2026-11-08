package viewer

import (
	"embed"
	"net/http"
)

//go:embed assets/index.html
var indexHTML []byte

//go:embed assets/*.js assets/*.css
var staticAssets embed.FS

func registerAssets(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", staticResponse("text/html; charset=utf-8", indexHTML))
	for _, asset := range []struct{ name, contentType string }{
		{"camera.js", "text/javascript; charset=utf-8"},
		{"graph.js", "text/javascript; charset=utf-8"},
		{"explorer.js", "text/javascript; charset=utf-8"},
		{"viewer.css", "text/css; charset=utf-8"},
	} {
		// These fixed paths are required embedded build inputs, not user input.
		content, err := staticAssets.ReadFile("assets/" + asset.name)
		if err != nil {
			panic(err)
		}
		mux.HandleFunc("GET /assets/"+asset.name, staticResponse(asset.contentType, content))
	}
}

func staticResponse(contentType string, content []byte) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", contentType)
		_, _ = writer.Write(content)
	}
}
