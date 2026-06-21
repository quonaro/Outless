package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var assets embed.FS

// FS returns the embedded frontend filesystem rooted at dist.
func FS() (http.FileSystem, error) {
	sub, err := fs.Sub(assets, "dist")
	if err != nil {
		return nil, err
	}
	return http.FS(sub), nil
}

// EmbeddedAssets exposes the raw embed.FS for testing.
func EmbeddedAssets() embed.FS {
	return assets
}
