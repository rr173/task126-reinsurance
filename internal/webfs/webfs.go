package webfs

import (
	"embed"
	"io/fs"
)

//go:embed web
var webFS embed.FS

// Sub returns the embedded web directory as an fs.FS for http.FileServer.
func Sub() (fs.FS, error) {
	return fs.Sub(webFS, "web")
}
