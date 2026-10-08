package visibility_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// cannedHistory serves fixed snapshots per state ref (`ID` or `ID@face`).
type cannedHistory map[string][]store.VersionSnapshot

func (h cannedHistory) ListVersions(_ context.Context, ref entity.Ref) ([]store.VersionMeta, error) {
	var out []store.VersionMeta
	for _, s := range h[ref.String()] {
		out = append(out, s.VersionMeta)
	}
	return out, nil
}

func (h cannedHistory) GetVersion(_ context.Context, ref entity.Ref, n int) (*store.VersionSnapshot, error) {
	vs := h[ref.String()]
	if n < 1 || n > len(vs) {
		return nil, store.ErrNotFound
	}
	return &vs[n-1], nil
}

// historicalRedactor hides salary, and only on a historical subject, so a
// test can tell the snapshot was redacted with the marker.
type historicalRedactor struct{}

func (historicalRedactor) HiddenProperties(ctx context.Context, _ *entity.Entity) map[string]struct{} {
	if affordances.IsHistoricalSubject(ctx) {
		return map[string]struct{}{"salary": {}}
	}
	return nil
}

func version(n int, typ string, op store.VersionOp, face entity.Face) store.VersionSnapshot {
	return store.VersionSnapshot{
		VersionMeta: store.VersionMeta{Version: n, Op: op, Type: typ, Face: face,
			CreatedAt: time.Date(2026, 1, n, 0, 0, 0, 0, time.UTC)},
		Content:    "then",
		Properties: map[string]any{"title": "old", "salary": "1"},
	}
}

func historyFixture(t *testing.T) (*visibility.ScriptReader, *visibility.UnrestrictedReader, cannedHistory) {
	t.Helper()
	st := memstore.New()
	for _, id := range []string{"TKT-1", "TKT-2"} {
		e := entity.New(id, "ticket")
		e.SetString("title", "now")
		if err := st.CreateEntity(context.Background(), e); err != nil {
			t.Fatal(err)
		}
	}
	policy, err := visibility.NewPolicyReader(idGate{id: "TKT-1"}, historicalRedactor{}, st)
	if err != nil {
		t.Fatal(err)
	}
	script, err := visibility.NewScriptReader(policy, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	h := cannedHistory{"TKT-1": {
		version(1, "ticket", store.VersionOpCreate, ""),
		version(2, "other", store.VersionOpUpdate, ""),
		version(3, "ticket", store.VersionOpUpdate, "draft"),
		version(4, "ticket", store.VersionOpPurge, ""),
	}}
	world := visibility.WorldOf(store.TrivialScope())
	return script.WithWorld(world), visibility.Unrestricted(st).WithWorld(world), h
}

func TestHistory_WithoutHistoryIsUnsupported(t *testing.T) {
	script, unrestricted, _ := historyFixture(t)
	ctx := context.Background()
	for name, read := range map[string]func() error{
		"script list":       func() error { _, err := script.EntityVersions(ctx, "TKT-1"); return err },
		"script get":        func() error { _, _, err := script.EntityVersion(ctx, "TKT-1", 1); return err },
		"unrestricted list": func() error { _, err := unrestricted.EntityVersions(ctx, "TKT-1"); return err },
		"unrestricted get":  func() error { _, _, err := unrestricted.EntityVersion(ctx, "NOPE-1", 1); return err },
	} {
		if err := read(); !errors.Is(err, store.ErrHistoryUnsupported) {
			t.Errorf("%s: err = %v, want ErrHistoryUnsupported", name, err)
		}
	}
}

func TestHistory_ScriptReaderGatesAndRedacts(t *testing.T) {
	script, _, h := historyFixture(t)
	r := script.WithHistory(h)
	ctx := context.Background()

	metas, err := r.EntityVersions(ctx, "TKT-1")
	if err != nil || len(metas) != 4 {
		t.Fatalf("EntityVersions = %d, %v; want 4", len(metas), err)
	}
	if _, hiddenErr := r.EntityVersions(ctx, "TKT-2"); !errors.Is(hiddenErr, store.ErrNotFound) {
		t.Errorf("hidden entity: err = %v, want ErrNotFound", hiddenErr)
	}

	e, meta, err := r.EntityVersion(ctx, "TKT-1", 1)
	if err != nil {
		t.Fatalf("EntityVersion: %v", err)
	}
	if meta.Version != 1 || e.GetString("title") != "old" || e.Content != "then" || !e.UpdatedAt.Equal(meta.CreatedAt) {
		t.Errorf("version 1 = %+v (meta %+v)", e, meta)
	}
	if _, ok := e.Properties["salary"]; ok {
		t.Error("salary served: the snapshot was not redacted as a historical subject")
	}

	for n, why := range map[int]string{2: "other type", 3: "other face", 4: "purge row", 9: "no such version"} {
		if _, _, err := r.EntityVersion(ctx, "TKT-1", n); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("version %d (%s): err = %v, want ErrNotFound", n, why, err)
		}
	}
	if _, _, err := r.EntityVersion(ctx, "TKT-2", 1); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("hidden entity: err = %v, want ErrNotFound", err)
	}
}

func TestHistory_UnrestrictedServesWholeSnapshot(t *testing.T) {
	_, unrestricted, h := historyFixture(t)
	r := unrestricted.WithHistory(h)
	ctx := context.Background()
	if metas, err := r.EntityVersions(ctx, "TKT-1"); err != nil || len(metas) != 4 {
		t.Fatalf("EntityVersions = %d, %v", len(metas), err)
	}
	e, _, err := r.EntityVersion(ctx, "TKT-1", 1)
	if err != nil || e.GetString("salary") != "1" {
		t.Fatalf("EntityVersion = %+v, %v; want salary kept", e, err)
	}
}

func TestHistory_DenyReaderMisses(t *testing.T) {
	ctx := context.Background()
	if _, err := (visibility.DenyReader{}).EntityVersions(ctx, "TKT-1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("EntityVersions err = %v", err)
	}
	if _, _, err := (visibility.DenyReader{}).EntityVersion(ctx, "TKT-1", 1); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("EntityVersion err = %v", err)
	}
}
