// Package studio embeds Osprey Studio, the browser UI for authoring rules and
// typologies and for sending test transactions to the engine.
//
// The UI is plain HTML, CSS, and JavaScript with no build step. It calls the
// same HTTP API as any other client, so it has no special access.
package studio

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var files embed.FS

// Prefix is the URL path where the server mounts the studio.
const Prefix = "/studio/"

// Handler serves the embedded studio files under Prefix.
func Handler() http.Handler {
	sub, err := fs.Sub(files, "static")
	if err != nil {
		// The embed directive guarantees "static" exists; fail loudly if not.
		panic(err)
	}
	fileServer := http.StripPrefix(Prefix, http.FileServerFS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		// Same-origin scripts and API calls only. The studio has no inline
		// script and loads nothing from a CDN.
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-cache")
		fileServer.ServeHTTP(w, r)
	})
}
