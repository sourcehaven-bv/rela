package config_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/config"
)

// mapLoader is a Loader over an in-memory name→bytes map, standing in for
// either layer. Absent names return an os.ErrNotExist-compatible error,
// which is the contract the Loader interface documents.
type mapLoader map[string][]byte

func (m mapLoader) Load(_ context.Context, name string) ([]byte, error) {
	data, ok := m[name]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: os.ErrNotExist}
	}
	return data, nil
}

func (m mapLoader) List(_ context.Context, dir string) ([]string, error) {
	var names []string
	for name := range m {
		base, ok := cutDir(name, dir)
		if ok {
			names = append(names, base)
		}
	}
	slices.Sort(names)
	return names, nil
}

// cutDir reports the base name of `name` when it sits directly under dir.
func cutDir(name, dir string) (string, bool) {
	prefix := dir + "/"
	if len(name) <= len(prefix) || name[:len(prefix)] != prefix {
		return "", false
	}
	base := name[len(prefix):]
	if strings.ContainsRune(base, '/') {
		return "", false
	}
	return base, true
}

// errLoader fails every call with a non-ErrNotExist error, standing in for a
// permission problem or a truncated read.
type errLoader struct{ err error }

func (e errLoader) Load(context.Context, string) ([]byte, error)   { return nil, e.err }
func (e errLoader) List(context.Context, string) ([]string, error) { return nil, e.err }

// mustLayered builds a layered loader, failing the test if construction does.
func mustLayered(t *testing.T, primary, secondary config.Loader) config.Loader {
	t.Helper()
	l, err := config.NewLayered(primary, secondary)
	if err != nil {
		t.Fatalf("NewLayered: %v", err)
	}
	return l
}

func TestLayered_Load_PrimaryWins(t *testing.T) {
	t.Parallel()
	l := mustLayered(t,
		mapLoader{"schema.yaml": []byte("from disk")},
		mapLoader{"schema.yaml": []byte("from database")},
	)
	got, err := l.Load(context.Background(), "schema.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Disk-first is the whole point: an operator editing a file must see
	// their edit, not the copy baked in at package time.
	if !bytes.Equal(got, []byte("from disk")) {
		t.Errorf("Load = %q, want the primary layer's bytes", got)
	}
}

func TestLayered_Load_FallsBackWhenAbsent(t *testing.T) {
	t.Parallel()
	l := mustLayered(t,
		mapLoader{},
		mapLoader{"schema.yaml": []byte("from database")},
	)
	got, err := l.Load(context.Background(), "schema.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !bytes.Equal(got, []byte("from database")) {
		t.Errorf("Load = %q, want the secondary layer's bytes", got)
	}
}

func TestLayered_Load_MissingInBothIsNotExist(t *testing.T) {
	t.Parallel()
	l := mustLayered(t, mapLoader{}, mapLoader{})
	_, err := l.Load(context.Background(), "schema.yaml")
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("error %v is not os.IsNotExist-compatible", err)
	}
}

func TestLayered_Load_NonNotExistErrorDoesNotFallBack(t *testing.T) {
	t.Parallel()
	boom := errors.New("permission denied")
	l := mustLayered(t,
		errLoader{err: boom},
		mapLoader{"schema.yaml": []byte("from database")},
	)

	// A permission error is not evidence the file is absent. Falling back
	// here would silently serve stale baked config in place of the file the
	// operator is actually looking at.
	_, err := l.Load(context.Background(), "schema.yaml")
	if !errors.Is(err, boom) {
		t.Errorf("Load error = %v, want the primary's error surfaced", err)
	}
}

func TestLayered_List_UnionsBothLayers(t *testing.T) {
	t.Parallel()
	l := mustLayered(t,
		mapLoader{"scripts/a.lua": nil, "scripts/shared.lua": []byte("from disk")},
		mapLoader{"scripts/b.lua": nil, "scripts/shared.lua": []byte("from database")},
	)
	got, err := l.List(context.Background(), "scripts")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// A union, not the Load precedence rule: a script present only in the
	// database is genuinely there, and a same-named file on disk must not
	// hide the rest of the directory. Duplicates collapse to one entry.
	want := []string{"a.lua", "b.lua", "shared.lua"}
	if !slices.Equal(got, want) {
		t.Errorf("List = %v, want %v", got, want)
	}

	// The other half of the claim: a name appearing in both layers still
	// resolves to the primary's bytes on the follow-up Load.
	data, err := l.Load(context.Background(), "scripts/shared.lua")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !bytes.Equal(data, []byte("from disk")) {
		t.Errorf("Load after union = %q, want the primary's bytes", data)
	}
}

func TestLayered_List_DoesNotMutateLayerSlices(t *testing.T) {
	t.Parallel()
	// A layer's returned slice belongs to that layer. Appending the other
	// layer's names into its spare capacity would write through to a slice
	// the layer may still be holding — a corruption that no test would show
	// until a caching backend retains what it returned.
	primary := &sliceLoader{names: append(make([]string, 0, 8), "a.lua", "shared.lua")}
	l := mustLayered(t, primary, &sliceLoader{names: []string{"z.lua"}})

	if _, err := l.List(context.Background(), "scripts"); err != nil {
		t.Fatalf("List: %v", err)
	}
	backing := primary.names[:cap(primary.names)]
	if backing[2] != "" {
		t.Errorf("List wrote %q into the primary's spare capacity", backing[2])
	}
}

// sliceLoader returns a caller-owned slice from List, so aliasing is visible.
type sliceLoader struct {
	mapLoader
	names []string
}

func (s *sliceLoader) List(context.Context, string) ([]string, error) { return s.names, nil }

func TestLayered_List_SurfacesErrors(t *testing.T) {
	t.Parallel()
	boom := errors.New("permission denied")
	for _, tc := range []struct {
		name              string
		primary, fallback config.Loader
	}{
		{"primary fails", errLoader{err: boom}, mapLoader{}},
		{"secondary fails", mapLoader{}, errLoader{err: boom}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := mustLayered(t, tc.primary, tc.fallback)
			if _, err := l.List(context.Background(), "scripts"); !errors.Is(err, boom) {
				t.Errorf("List error = %v, want %v", err, boom)
			}
		})
	}
}

func TestNewLayered_RejectsNil(t *testing.T) {
	t.Parallel()
	// Constructors reject nil required collaborators rather than deferring
	// the failure to a downstream symptom. An error, not a panic: this is
	// built at a wiring site that already threads errors.
	for _, tc := range []struct {
		name              string
		primary, fallback config.Loader
	}{
		{"nil primary", nil, mapLoader{}},
		{"nil secondary", mapLoader{}, nil},
		{"both nil", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l, err := config.NewLayered(tc.primary, tc.fallback)
			if err == nil {
				t.Fatal("NewLayered should reject a nil loader")
			}
			if l != nil {
				t.Errorf("NewLayered returned %v alongside an error, want nil", l)
			}
		})
	}
}

func TestLayered_Subscribe_ForwardsToCapableLayer(t *testing.T) {
	t.Parallel()
	// mapLoader is not a Subscriber, so neither layer can watch.
	l := mustLayered(t, mapLoader{}, mapLoader{})
	sub, ok := l.(config.Subscriber)
	if !ok {
		t.Fatal("layered should satisfy config.Subscriber so the capability is not silently lost")
	}
	if _, err := sub.Subscribe(context.Background(), "x.yaml", func() {}); err == nil {
		t.Error("Subscribe should report that no layer can watch")
	}

	// With a watching primary, the call is forwarded rather than refused.
	watcher := &countingSubscriber{}
	l = mustLayered(t, watcher, mapLoader{})
	stop, err := l.(config.Subscriber).Subscribe(context.Background(), "x.yaml", func() {})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if watcher.calls != 1 {
		t.Errorf("primary Subscribe called %d times, want 1", watcher.calls)
	}
	stop()
}

// countingSubscriber is a Loader that also implements Subscriber, so the
// decorator's forwarding can be observed.
type countingSubscriber struct {
	mapLoader
	calls int
}

func (c *countingSubscriber) Subscribe(
	context.Context, string, func(),
) (func(), error) {
	c.calls++
	return func() {}, nil
}
