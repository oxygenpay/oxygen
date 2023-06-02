package cmd

import (
	"context"
	"database/sql"
	"log"
	"os"
	"time"

	//nolint:revive.
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/olekukonko/tablewriter"
	"github.com/oxygenpay/oxygen/internal/config"
	"github.com/oxygenpay/oxygen/scripts"
	migrate "github.com/rubenv/sql-migrate"
	"github.com/spf13/cobra"
)

const (
	dbDialect       = "postgres"
	dbSchema        = "public"
	migrationsTable = "migrations"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate DB",
	Long:  "Allows to use sql-migration commands: status, up, down",
	Run:   migration,
}

var migrateSelectedCommand string

func migration(_ *cobra.Command, _ []string) {
	cfg := resolveConfig()

	db := migrationConnection(cfg)
	source := scripts.MigrationFilesSource()
	migrationSet := &migrate.MigrationSet{
		SchemaName: dbSchema,
		TableName:  migrationsTable,
	}

	log.Printf("Using table %q.%q\n", dbSchema, migrationsTable)

	switch migrateSelectedCommand {
	case "up":
		log.Println("Running migrations")
		_, err := migrationSet.Exec(db, dbDialect, source, migrate.Up)
		if err != nil {
			log.Fatalf("Error while running migrations: %s\n", err.Error())
		}

		log.Println("Applied ✔")
		migrationStatus(db, migrationSet)

	case "down":
		log.Println("Rolling back migrations")
		_, err := migrationSet.Exec(db, dbDialect, source, migrate.Down)
		if err != nil {
			log.Fatalf("Error while running migrations: %s\n", err.Error())
		}

		log.Println("Rolled back ✔")
		migrationStatus(db, migrationSet)

	case "status":
		migrationStatus(db, migrationSet)

	default:
		log.Fatalf("Unknown --command %q\n", migrateSelectedCommand)
	}
}

func migrationConnection(cfg *config.Config) *sql.DB {
	db, err := sql.Open("pgx", cfg.Oxygen.Postgres.DataSource)
	if err != nil {
		log.Fatalf("unable to open DB connection: %s\n", err.Error())
	}

	if _, err = db.Conn(context.Background()); err != nil {
		log.Fatalf("unable to connect to DB: %s\n", err.Error())
	}

	return db
}

func migrationStatus(db *sql.DB, set *migrate.MigrationSet) {
	items, err := set.GetMigrationRecords(db, dbDialect)
	if err != nil {
		log.Fatalf("Status error: %s", err.Error())
	}

	t := tablewriter.NewWriter(os.Stdout)
	defer t.Render()

	t.SetHeader([]string{"Migration", "Timestamp"})
	for _, item := range items {
		t.Append([]string{item.Id, item.AppliedAt.Format(time.RFC3339)})
	}
}
