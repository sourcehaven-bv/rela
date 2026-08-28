package memuserstate_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/userstate"
	"github.com/Sourcehaven-BV/rela/internal/userstate/memuserstate"
	"github.com/Sourcehaven-BV/rela/internal/userstate/userstatetest"
)

func TestConformance(t *testing.T) {
	t.Parallel()
	userstatetest.RunAll(t, func(t *testing.T) userstate.Store {
		t.Helper()
		s := memuserstate.New()
		t.Cleanup(func() { _ = s.Close() })
		return s
	})
}
