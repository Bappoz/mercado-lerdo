// Package web embeds the built UI (web/dist, produced by `npm run build`).
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist is the built SPA. Before `npm run build` it holds only a placeholder
// and has no index.html.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // "dist" is a literal that the embed directive guarantees
	}
	return sub
}
