package schema

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"
)

type VersionMap map[int]Version

type Version struct {
	Up   string
	Down string
}

func FromFS(fsys fs.ReadDirFS, dir string) (VersionMap, error) {
	entries, err := fsys.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	versionMap := make(VersionMap)

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		name = strings.TrimSuffix(name, ".sql")
		parts := strings.Split(name, "_")

		path := filepath.Join(dir, entry.Name())

		contents, err := fs.ReadFile(fsys, path)
		if err != nil {
			return nil, err
		}

		versionInt, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, err
		}

		direction := parts[len(parts)-1]

		version := versionMap[versionInt]
		switch direction {
		case "up":
			version.Up = string(contents)
		case "down":
			version.Down = string(contents)
		default:
			return nil, fmt.Errorf("invalid migration file format: %s (expected: 000_name_up/down.sql", entry.Name())
		}

		versionMap[versionInt] = version
	}

	return versionMap, nil
}

func CurrentVersion(ctx context.Context, tx *sqlx.Tx) (int, error) {
	var version int
	err := tx.GetContext(ctx, &version, "PRAGMA user_version")
	return version, err
}

func setVersion(ctx context.Context, tx *sqlx.Tx, version int) error {
	_, err := tx.ExecContext(ctx, "PRAGMA user_version = "+strconv.Itoa(version))

	return err
}

func ApplyMigrations(ctx context.Context, tx *sqlx.Tx, versionMap VersionMap, fromVer int, toVer int) error {
	if fromVer == toVer {
		return fmt.Errorf("current version: %d equals to target version: %d", fromVer, toVer)
	}

	var err error
	if fromVer < toVer {
		err = migrateUp(ctx, tx, versionMap, fromVer, toVer)
	} else {
		err = migrateDown(ctx, tx, versionMap, fromVer, toVer)
	}

	if err != nil {
		return err
	}

	return setVersion(ctx, tx, toVer)
}

func runMigration(ctx context.Context, tx *sqlx.Tx, migration string) error {
	_, err := tx.ExecContext(ctx, migration)
	return err
}

func migrateUp(ctx context.Context, tx *sqlx.Tx, versionMap VersionMap, curVersion int, targetVersion int) error {
	slog.Info("upgrading database schema version", "from", curVersion, "to", targetVersion)
	for v := curVersion + 1; v <= targetVersion; v++ {
		migration := versionMap[v]
		if migration.Up == "" {
			return fmt.Errorf("cannot migrate database further up than v%d, should never happen", v)
		}

		err := runMigration(ctx, tx, migration.Up)
		if err != nil {
			return fmt.Errorf("error migrating database from v%d to v%d: %w", v-1, v, err)
		}
	}

	return nil
}

func migrateDown(ctx context.Context, tx *sqlx.Tx, versionMap VersionMap, curVersion int, targetVersion int) error {
	slog.Info("downgrading database schema version", "from", curVersion, "to", targetVersion)
	for v := curVersion; v > targetVersion; v-- {
		migration := versionMap[v]
		if migration.Down == "" {
			return fmt.Errorf("cannot migrate database further down than v%d", v)
		}

		err := runMigration(ctx, tx, migration.Down)
		if err != nil {
			return fmt.Errorf("error migrating database from v%d to v%d: %w", v, v-1, err)
		}
	}

	return nil
}
