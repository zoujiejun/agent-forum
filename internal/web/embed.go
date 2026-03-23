package web

import (
	"embed"
	"io/fs"
)

//go:embed all:static
var embeddedFiles embed.FS

// FrontendFS returns the embedded filesystem for the frontend.
// It returns nil if the static directory is not present (e.g., during development
// when frontend/dist has not been built yet).
func FrontendFS() fs.FS {
	sub, err := fs.Sub(embeddedFiles, "static")
	if err != nil {
		return nil
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil
	}
	return sub
}

// ReadFile reads a file from the embedded frontend filesystem.
func ReadFile(name string) ([]byte, error) {
	fsys := FrontendFS()
	if fsys == nil {
		return nil, fsErrNotEmbedded
	}
	return fs.ReadFile(fsys, name)
}

// Open implements fs.FS by opening the named file.
func Open(name string) (fs.File, error) {
	fsys := FrontendFS()
	if fsys == nil {
		return nil, fsErrNotEmbedded
	}
	return fsys.Open(name)
}

type notEmbeddedError struct{}

func (e notEmbeddedError) Error() string { return "frontend assets not embedded (run 'make build' first)" }

var fsErrNotEmbedded = notEmbeddedError{}
