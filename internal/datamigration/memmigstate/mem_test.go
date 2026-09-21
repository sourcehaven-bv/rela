package memmigstate_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	"github.com/Sourcehaven-BV/rela/internal/datamigration/memmigstate"
	"github.com/Sourcehaven-BV/rela/internal/datamigration/migstatetest"
)

func TestConformance(t *testing.T) {
	migstatetest.RunAll(t, func(t *testing.T) datamigration.StateStore {
		t.Helper()
		return memmigstate.New()
	})
}
