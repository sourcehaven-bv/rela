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

// cannedTags resolves tags from a fixed map keyed `ref|name` to an ordinal
// in hist, and records which refs it was asked about.
type cannedTags struct {
	tags  map[string]int
	hist  cannedHistory
	asked []string
}

func (c *cannedTags) VersionByTag(
	ctx context.Context, ref entity.Ref, name store.VersionTagName,
) (*store.VersionSnapshot, error) {
	c.asked = append(c.asked, ref.String())
	n, ok := c.tags[ref.String()+"|"+name.String()]
	if !ok {
		return nil, store.ErrNotFound
	}
	return c.hist.GetVersion(ctx, ref, n)
}

func tagName(t *testing.T, s string) store.VersionTagName {
	t.Helper()
	n, err := store.ParseVersionTagName(s)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestVersionByTag_WithoutTagsIsUnsupported(t *testing.T) {
	script, unrestricted, h := historyFixture(t)
	ctx := context.Background()
	name := tagName(t, "reviewed")
	for label, read := range map[string]func() error{
		"script, no tags": func() error { _, _, err := script.WithHistory(h).VersionByTag(ctx, "TKT-1", name); return err },
		"script, no history": func() error {
			_, _, err := script.WithVersionTags(&cannedTags{}).VersionByTag(ctx, "TKT-1", name)
			return err
		},
		"unrestricted, no tags": func() error { _, _, err := unrestricted.WithHistory(h).VersionByTag(ctx, "TKT-1", name); return err },
	} {
		if err := read(); !errors.Is(err, store.ErrHistoryUnsupported) {
			t.Errorf("%s: err = %v, want ErrHistoryUnsupported", label, err)
		}
	}
}

// TestVersionByTag_ScriptReaderGatesAndRedacts pins that a hidden entity is
// indistinguishable from a missing one, and from a missing tag: the same
// error, and the tag lookup never runs for the hidden entity.
func TestVersionByTag_ScriptReaderGatesAndRedacts(t *testing.T) {
	script, _, h := historyFixture(t)
	h["TKT-2"] = []store.VersionSnapshot{version(1, "ticket", store.VersionOpCreate, "")}
	tags := &cannedTags{tags: map[string]int{
		"TKT-1|reviewed": 1,
		"TKT-1|purged":   4,
		"TKT-2|reviewed": 1,
	}, hist: h}
	r := script.WithHistory(h).WithVersionTags(tags)
	ctx := context.Background()

	e, meta, err := r.VersionByTag(ctx, "TKT-1", tagName(t, "reviewed"))
	if err != nil {
		t.Fatalf("VersionByTag: %v", err)
	}
	if meta.Version != 1 || e.GetString("title") != "old" {
		t.Errorf("tagged version = %+v (meta %+v)", e, meta)
	}
	if _, ok := e.Properties["salary"]; ok {
		t.Error("salary served: the tagged snapshot was not redacted as a historical subject")
	}

	tags.asked = nil
	misses := []struct {
		name, addr, tag string
	}{
		{"hidden entity", "TKT-2", "reviewed"},
		{"missing entity", "NOPE-1", "reviewed"},
		{"missing tag", "TKT-1", "absent"},
		{"tag on an unservable version", "TKT-1", "purged"},
	}
	var first error
	for _, m := range misses {
		_, _, err := r.VersionByTag(ctx, m.addr, tagName(t, m.tag))
		if !errors.Is(err, store.ErrNotFound) {
			t.Errorf("%s: err = %v, want ErrNotFound", m.name, err)
			continue
		}
		if first == nil {
			first = err
		} else if err.Error() != first.Error() {
			t.Errorf("%s: err %q differs from %q", m.name, err, first)
		}
	}
	for _, ref := range tags.asked {
		if ref == "TKT-2" {
			t.Error("the tag lookup ran for an entity the gate hides")
		}
	}
}

func TestVersionByTag_UnrestrictedAndDeny(t *testing.T) {
	_, unrestricted, h := historyFixture(t)
	tags := &cannedTags{tags: map[string]int{"TKT-1|reviewed": 1}, hist: h}
	ctx := context.Background()
	e, _, err := unrestricted.WithHistory(h).WithVersionTags(tags).VersionByTag(ctx, "TKT-1", tagName(t, "reviewed"))
	if err != nil || e.GetString("salary") != "1" {
		t.Fatalf("unrestricted VersionByTag = %+v, %v; want salary kept", e, err)
	}
	if _, _, err := (visibility.DenyReader{}).VersionByTag(ctx, "TKT-1", tagName(t, "reviewed")); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("DenyReader err = %v", err)
	}
}
