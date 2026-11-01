package acl

import (
	"bytes"
	"context"
	"errors"
	"iter"
	"log/slog"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/testutil/logtest"
)

// failingRelations fails relation reads once armed: with the ctx's error
// if it has one, as a database driver does, else with errDown.
type failingRelations struct {
	*memstore.MemStore
	armed *bool
}

var errDown = errors.New("relations down")

func (s failingRelations) ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error] {
	if !*s.armed {
		return s.MemStore.ListRelations(ctx, q)
	}
	err := ctx.Err()
	if err == nil {
		err = errDown
	}
	return func(yield func(*entity.Relation, error) bool) { yield(nil, err) }
}

// A batch that fails because the request was canceled is not logged: the
// client is gone and the per-entity checks after it fail on the same ctx
// (TKT-T2I7P2). Any other failure still is.
func TestPrimeLocalRoles_CancelIsNotLogged(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cancel   bool
		wantWarn bool
	}{
		{"canceled", true, false},
		{"store failure", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logtest.SetDefault(t, slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
			st, ids := randomPrimeGraph(t, 3, 5)
			armed := false
			fs := failingRelations{MemStore: st, armed: &armed}
			d, err := NewDeclarative(primePolicy(), aclstore.MustNewGraph(fs), fs)
			if err != nil {
				t.Fatal(err)
			}
			p := principal.Principal{User: "alice", Tool: principal.ToolDataEntry}
			r, _ := d.ForIdentity(identityFor(context.Background(), t, d, p))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			armed = true
			PrimeLocalRoles(bindTo(ctx, r), ids)

			if got := strings.Contains(buf.String(), "batching local roles failed"); got != tc.wantWarn {
				t.Errorf("warning logged = %v, want %v: %q", got, tc.wantWarn, buf.String())
			}
		})
	}
}
