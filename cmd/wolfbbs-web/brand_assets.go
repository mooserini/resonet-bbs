package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed assets/fonts/*.woff assets/icons/*
var brandAssetFS embed.FS

// brandAssetHandler serves the embedded brand fonts and icons under /assets/.
func brandAssetHandler() http.Handler {
	sub, err := fs.Sub(brandAssetFS, "assets")
	if err != nil {
		panic(err)
	}
	files := http.StripPrefix("/assets/", http.FileServer(http.FS(sub)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=604800")
		files.ServeHTTP(w, r)
	})
}

// brandIconLinks is injected into every page head.
const brandIconLinks = `<link rel="icon" href="/favicon.ico" sizes="any">` +
	`<link rel="icon" type="image/png" sizes="32x32" href="/assets/icons/icon-32.png">` +
	`<link rel="icon" type="image/png" sizes="192x192" href="/assets/icons/icon-192.png">` +
	`<link rel="apple-touch-icon" sizes="180x180" href="/assets/icons/icon-180.png">`

// faviconHandler answers the /favicon.ico and /apple-touch-icon.png paths
// browsers request on their own, even on pages without link tags.
func faviconHandler(name, contentType string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := brandAssetFS.ReadFile("assets/icons/" + name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "public, max-age=604800")
		_, _ = w.Write(data)
	})
}
