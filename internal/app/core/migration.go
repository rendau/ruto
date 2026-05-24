package core

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	configCore "github.com/rendau/ruto/internal/config/core"
)

func runMigrations() {
	if configCore.Conf.PgDsn == "" {
		slog.Warn("PG-dsn is empty, migrations will not be applied")
		return
	}

	absPath, _ := filepath.Abs("./migrations")

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		slog.Warn("migrations folder does not exist, migrations will not be applied", "path", absPath)
		return
	}

	m, err := migrate.New("file://"+absPath, configCore.Conf.PgDsn)
	if err != nil {
		panic(err)
	}

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		panic(fmt.Errorf("migration up error: %w", err))
	}
}
