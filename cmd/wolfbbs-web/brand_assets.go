package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed assets/fonts/*.woff
var brandAssetFS embed.FS

// brandAssetHandler serves the embedded brand fonts under /assets/fonts/.
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
