package test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/oxygenpay/oxygen/internal/db/connection/pg"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/rs/zerolog"
)

type Database struct {
	genericConnection *pg.Connection
	context           context.Context
	connection        *pg.Connection
	name              string
}

const (
	dbHost                 = "127.0.0.1"
	dbUser                 = "postgres"
	dbPassword             = ""
	maxConnections         = 64
	migrationsRelativePath = "../../scripts/migrations/"
)

func NewDB(ctx context.Context) *Database {
	// connect to database
	nopLogger := zerolog.Nop()
	cfg := pg.Config{
		DataSource: fmt.Sprintf(
			"user=%s password=%s host=%s sslmode=disable pool_max_conns=%d",
			dbUser, dbPassword, dbHost, maxConnections,
		),
	}

	conn, err := pg.Open(ctx, cfg, &nopLogger)
	if err != nil {
		panic("unable to open database: " + err.Error())
	}

	db := &Database{
		context:           ctx,
		genericConnection: conn,
		name:              "oxygen_test_" + util.Strings.Random(8),
	}

	// create tmp database
	if _, errDB := conn.Exec(ctx, "create database "+db.name); errDB != nil {
		panic("unable to create test database: " + err.Error())
	}

	// connect to the tmp database
	tmpConnectionCfg := pg.Config{
		DataSource: fmt.Sprintf(
			"user=%s password=%s host=%s dbname=%s sslmode=disable pool_max_conns=%d",
			dbUser, dbPassword, dbHost, db.name, maxConnections,
		),
	}

	tmpConn, err := pg.Open(ctx, tmpConnectionCfg, &nopLogger)
	if err != nil {
		panic("unable to open test database: " + err.Error())
	}

	db.connection = tmpConn

	db.applyMigrations()

	return db
}

func (db *Database) Conn() *pg.Connection {
	return db.connection
}

func (db *Database) Context() context.Context {
	return db.context
}

func (db *Database) Name() string {
	return db.name
}

func (db *Database) TearDown() {
	if err := db.connection.Shutdown(); err != nil {
		panic("unable to close test database: " + err.Error())
	}

	if _, err := db.genericConnection.Exec(db.context, "drop database "+db.name); err != nil {
		panic("unable to drop test database: " + err.Error())
	}

	if err := db.genericConnection.Shutdown(); err != nil {
		panic("unable to close test generic database: " + err.Error())
	}
}

//nolint:dogsled
func (db *Database) applyMigrations() {
	// locate migrations directory relative to this very file
	_, filename, _, _ := runtime.Caller(1)
	currentDir := path.Dir(filename)

	migrationsDirectory := path.Join(currentDir, migrationsRelativePath)

	// get all filenames
	migrations, err := os.ReadDir(migrationsDirectory)
	if err != nil {
		db.TearDown()
		panic("unable to open migrations directory: " + err.Error())
	}

	// apply migrations
	for _, file := range migrations {
		db.applySingleMigration(migrationsDirectory + "/" + file.Name())
	}
}

func (db *Database) applySingleMigration(filename string) {
	// open file
	f, err := os.Open(filename)
	if err != nil {
		panic("unable to open " + filename)
	}
	defer f.Close()

	// read all
	bytes, err := io.ReadAll(f)
	if err != nil {
		panic("unable to read " + filename)
	}

	text := string(bytes)

	// cut "down" command
	lastIndex := strings.Index(text, "-- +migrate Down")
	if lastIndex == -1 {
		panic("invalid migration: " + filename)
	}

	migrationSQL := text[:lastIndex]

	// apply migration
	if _, err := db.connection.Exec(db.context, migrationSQL); err != nil {
		panic("unable to apply migration: " + filename)
	}
}
