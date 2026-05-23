package memstore_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// WithObserver(nil) must drop silently; the first write must not
// nil-deref. Mirrors app.FSFactory.AddObserver behavior. Locking this
// in guards the contract relied on by the appbuild seam, which passes
// the result of an optional observer factory directly.
func TestWithObserver_NilDroppedSilently(t *testing.T) {
	s := memstore.New(memstore.WithObserver(nil))
	if err := s.CreateEntity(context.Background(), &entity.Entity{ID: "X-1", Type: "x"}); err != nil {
		t.Fatalf("CreateEntity with nil observer should not error: %v", err)
	}
}
