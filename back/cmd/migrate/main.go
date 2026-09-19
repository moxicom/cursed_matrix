// Command migrate applies the embedded goose migrations.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"
	_ "time/tzdata"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/moxicom/cursed_matrix/back/internal/config"
	"github.com/moxicom/cursed_matrix/back/internal/migrations"
)

// version is stamped at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "", "path to the YAML configuration file")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		return fmt.Errorf("usage: migrate [-config PATH] <up|down|reset|status|version|up-to VERSION>")
	}
	command := args[0]

	cfg, err := config.LoadForMigrations(config.ConfigPath(*configPath))
	if err != nil {
		return err
	}
	dsn := cfg.DatabaseURL()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			fmt.Fprintln(os.Stderr, "migrate: closing the connection:", closeErr)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping: %w", err)
	}

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("dialect: %w", err)
	}

	fmt.Fprintf(os.Stderr, "migrate %s\n", version)

	switch command {
	case "up":
		return goose.UpContext(ctx, db, ".")
	case "down":
		return goose.DownContext(ctx, db, ".")
	case "reset":
		return goose.DownToContext(ctx, db, ".", 0)
	case "status":
		return goose.StatusContext(ctx, db, ".")
	case "version":
		return goose.VersionContext(ctx, db, ".")
	case "up-to":
		if len(args) < 2 {
			return fmt.Errorf("up-to needs a version")
		}
		version, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("version: %w", err)
		}
		return goose.UpToContext(ctx, db, ".", version)
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}
