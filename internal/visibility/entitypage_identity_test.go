package visibility_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// Without a bound identity the page read fails closed: it serves no row and
// no total, and says why.
func TestScriptReader_ListEntityPage_NoIdentityFailsClosed(t *testing.T) {
	t.Parallel()
	f := newPageFixture(t, 5)
	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolMCP})
	page, err := f.sr.ListEntityPage(ctx, visibility.PageQuery{Type: "ticket", Faces: store.AllFaces(), Limit: 5})
	if len(page.Rows) != 0 || page.Total != 0 {
		t.Fatalf("an unbound read served %d rows of %d", len(page.Rows), page.Total)
	}
	if !errors.Is(err, acl.ErrNoIdentity) {
		t.Errorf("err = %v, want acl.ErrNoIdentity", err)
	}
}
