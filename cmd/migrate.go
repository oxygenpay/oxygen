package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v4"
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

var migrateCommand = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate DB",
	Long:  "Allows to use sql-migration commands: status, up, down",
	Run:   migration,
}

var migrateSelectedCommand string

func migration(_ *cobra.Command, _ []string) {
	performMigration(context.Background(), resolveConfig(), migrateSelectedCommand, false)
}

func performMigration(ctx context.Context, cfg *config.Config, command string, silent bool) {
	db := migrationConnection(ctx, cfg)
	source := scripts.MigrationFilesSource()
	migrationSet := &migrate.MigrationSet{
		SchemaName: dbSchema,
		TableName:  migrationsTable,
	}

	log.Printf("Using table %q.%q\n", dbSchema, migrationsTable)

	switch command {
	case "up":
		log.Println("Running migrations")
		_, err := migrationSet.Exec(db, dbDialect, source, migrate.Up)
		if err != nil {
			log.Fatalf("Error while running migrations: %s\n", err.Error())
		}

		log.Println("Applied migrations ✔")

		if !silent {
			migrationStatus(db, migrationSet)
		}
	case "down":
		log.Println("Rolling back migrations")
		_, err := migrationSet.Exec(db, dbDialect, source, migrate.Down)
		if err != nil {
			log.Fatalf("Error while running migrations: %s\n", err.Error())
		}

		log.Println("Rolled back migrations ✔")
		if !silent {
			migrationStatus(db, migrationSet)
		}

	case "status":
		migrationStatus(db, migrationSet)

	default:
		log.Fatalf("Unknown --command %q\n", migrateSelectedCommand)
	}

	if err := db.Close(); err != nil {
		log.Fatalf("Unable to close migration db connection: %s", err.Error())
	}
}

func migrationConnection(ctx context.Context, cfg *config.Config) *sql.DB {
	dataSource, err := parseConn(cfg.Oxygen.Postgres.DataSource)
	if err != nil {
		log.Fatalf("unable to parse DB connection: %s\n", err.Error())
	}

	db, err := sql.Open("pgx", dataSource)
	if err != nil {
		log.Fatalf("unable to open DB connection: %s\n", err.Error())
	}

	if _, err = db.Conn(ctx); err != nil {
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

// parseConn strip irrelevant pgx pool configuration params.
func parseConn(raw string) (string, error) {
	connCfg, err := pgx.ParseConfig(raw)
	if err != nil {
		log.Fatalf("unable to parse pg config: %s\n", err.Error())
		return "", err
	}

	sslMode := "disable"
	if connCfg.Config.TLSConfig != nil {
		sslMode = "require"
	}

	dataSource := fmt.Sprintf(
		"postgres://%s:%s@%s/%s?sslmode=%s",
		connCfg.User,
		connCfg.Password,
		connCfg.Host,
		connCfg.Database,
		sslMode,
	)

	return dataSource, nil
}
