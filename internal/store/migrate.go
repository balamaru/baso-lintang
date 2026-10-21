package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"baso-lintang/db"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func Migrate(ctx context.Context, url, direction string) error {
	conn, err := sql.Open("pgx", url)
	if err != nil {
		return errors.New("database tidak tersedia")
	}
	defer conn.Close()
	goose.SetBaseFS(db.Migrations)
	goose.SetLogger(migrationLogger{})
	if err = goose.SetDialect("postgres"); err != nil {
		return err
	}
	switch direction {
	case "up":
		return goose.UpContext(ctx, conn, "migrations")
	case "down":
		return goose.DownContext(ctx, conn, "migrations")
	default:
		return errors.New("arah migrasi harus up atau down")
	}
}

type migrationLogger struct{}

func (migrationLogger) Printf(format string, args ...any) {
	slog.Info("migration", "message", fmt.Sprintf(format, args...))
}

func (migrationLogger) Fatalf(format string, args ...any) {
	slog.Error("migration failed", "message", fmt.Sprintf(format, args...))
}
