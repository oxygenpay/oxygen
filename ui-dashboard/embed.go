package uidashboard

import (
	"embed"
	"io/fs"
)

//go:embed dist/*
var dist embed.FS

func Files() fs.FS {
	sub, _ := fs.Sub(dist, "dist")

	return sub
}
