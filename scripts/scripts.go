package scripts

import (
	"embed"

	migrate "github.com/rubenv/sql-migrate"
)

//go:embed migrations/*
var migrations embed.FS

func MigrationFilesSource() *migrate.EmbedFileSystemMigrationSource {
	return &migrate.EmbedFileSystemMigrationSource{
		FileSystem: migrations,
		Root:       "migrations",
	}
}
