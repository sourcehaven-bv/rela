//go:build postgres

package appbuild

import (
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/flowrun/pgflowrun"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// storeFlowRunsFor returns the run store in the tenant's database, so every
// process of a deployment sees the same runs.
func storeFlowRunsFor(st store.Store) flowrun.Store {
	db := pgstore.HandleFor(st)
	if db == nil {
		slog.Error("flows: store is not a pgstore; flow runs are kept on this node")
		return nil
	}
	s, err := pgflowrun.New(db)
	if err != nil { // coverage-ignore: unreachable: db is non-nil here
		panic(err)
	}
	return s
}
