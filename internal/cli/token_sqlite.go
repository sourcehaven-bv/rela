//go:build sqlite

package cli

import (
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// tokenLockHint adds the way around the sqlite single-process lock to a
// `rela token` command that could not open the database because a server
// holds it.
func tokenLockHint(cmd string, err error) error {
	if firstKongToken(cmd) != "token" || !errors.Is(err, sqlitedb.ErrInUse) {
		return err
	}
	return fmt.Errorf("%w\n\nThe running rela server holds the database. Stop it, run this "+
		"`rela token` command, then start the server again", err)
}
