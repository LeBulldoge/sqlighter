package sqlighter

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/LeBulldoge/sqlighter/internal/os"
	"github.com/LeBulldoge/sqlighter/schema"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

type Tx = sqlx.Tx

type DB struct {
	db            *sqlx.DB
	configDir     string
	targetVersion int
	versionMap    schema.VersionMap
}

func New(configDir string, targetVersion int, versionMap schema.VersionMap) *DB {
	return &DB{
		configDir:     configDir,
		targetVersion: targetVersion,
		versionMap:    versionMap,
	}
}

func (m *DB) Open(ctx context.Context, pragmas ...string) error {
	var config string
	if m.configDir != "" {
		config = m.configDir
	} else {
		config = os.ConfigPath()
	}
	dbPath := filepath.Join(config, "storage.db")

	if !os.FileExists(dbPath) {
		err := os.CreateFile(dbPath)
		if err != nil {
			return err
		}
	}

	db, err := sqlx.Open("sqlite", dbPath)
	if err != nil {
		return err
	}

	db.SetMaxOpenConns(1)

	m.db = db

	err = m.Tx(ctx, func(ctx context.Context, tx *Tx) error {
		curVersion, err := schema.CurrentVersion(ctx, tx)
		if err != nil {
			return err
		}

		needSchemaUpdate := curVersion != m.targetVersion

		if needSchemaUpdate {
			err := schema.ApplyMigrations(ctx, tx, m.versionMap, curVersion, m.targetVersion)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		m.db.Close()
		return err
	}

	for _, pragma := range pragmas {
		_, err = m.db.ExecContext(ctx, "PRAGMA "+pragma)
		if err != nil {
			m.db.Close()
			return err
		}
	}

	return nil
}

func (m *DB) Close() error {
	_, err := m.db.Exec("PRAGMA optimize")
	if err != nil {
		return err
	}

	return m.db.Close()
}

func (m *DB) Tx(ctx context.Context, f func(context.Context, *Tx) error) error {
	tx, err := m.db.BeginTxx(ctx, nil)

	if err != nil {
		return err
	}

	err = f(ctx, tx)
	if err != nil {
		e := tx.Rollback()
		return errors.Join(err, e)
	}

	return tx.Commit()
}
