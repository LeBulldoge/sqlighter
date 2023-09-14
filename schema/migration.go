package schema

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/jmoiron/sqlx"
)

type VersionMap map[int]Version

type Version struct {
	Up   string
	Down string
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
			return fmt.Errorf("error migrating database from v%d to v%d: %w", curVersion, v, err)
		}
	}

	return nil
}

func migrateDown(ctx context.Context, tx *sqlx.Tx, versionMap VersionMap, curVersion int, targetVersion int) error {
	slog.Info("downgrading database schema version", "from", targetVersion, "to", curVersion)
	for v := curVersion; v > targetVersion; v-- {
		migration := versionMap[v]
		if migration.Down == "" {
			return fmt.Errorf("cannot migrate database further down than v%d", v)
		}

		err := runMigration(ctx, tx, migration.Down)
		if err != nil {
			return fmt.Errorf("error migrating database from v%d to v%d: %w", curVersion, v, err)
		}
	}

	return nil
}
