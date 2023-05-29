package web

import (
	"embed"
	"io/fs"
)

//go:embed auth-debug/*
var authDebug embed.FS

//go:embed redoc/*
var swagger embed.FS

func AuthDebugFiles() fs.FS {
	sub, _ := fs.Sub(authDebug, "auth-debug")

	return sub
}

func SwaggerFiles() fs.FS {
	sub, _ := fs.Sub(swagger, "redoc")

	return sub
}
