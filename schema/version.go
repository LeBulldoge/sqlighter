package schema

import (
	"context"

	"github.com/jmoiron/sqlx"
)

var versionMap = map[int](func() Migration){
	1: version1,
}

// The initial schema
func version1() Migration {
	up := func(ctx context.Context, tx *sqlx.Tx) error {

		const schema = `CREATE TABLE Polls (
  id     TEXT   NOT NULL PRIMARY KEY
                UNIQUE,
  owner  TEXT   NOT NULL,
  title  TEXT   NOT NULL
);

CREATE TABLE PollOptions (
  id      INTEGER NOT NULL,
  poll_id TEXT    NOT NULL
                  REFERENCES Polls (id) ON DELETE CASCADE,
  name    TEXT    NOT NULL,
  UNIQUE(poll_id, name)
  FOREIGN KEY (
      poll_id
  )
  REFERENCES Poll (id),
  PRIMARY KEY (
      id AUTOINCREMENT
  )
);

CREATE TABLE Votes (
  option_id INTEGER NOT NULL
                    REFERENCES PollOptions (id) ON DELETE CASCADE,
  voter_id  TEXT    NOT NULL
);`

		_, err := tx.ExecContext(ctx, schema)
		if err != nil {
			return err
		}

		return nil
	}

	return Migration{up: up}
}
