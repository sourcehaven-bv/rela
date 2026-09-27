package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"maps"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// boundWorldKey marks the ctx a fakeWorlds selection returns.
type boundWorldKey struct{}

// fakeWorlds declares `current` and `review`; the caller may read `current`
// only.
type fakeWorlds struct {
	readErr error
}

func (fakeWorlds) SelectWorld(ctx context.Context, name string) (context.Context, error) {
	switch name {
	case "current", defaultWorldName:
		return context.WithValue(ctx, boundWorldKey{}, name), nil
	case "review":
		return ctx, errors.New(`world "review" is not readable by you`)
	}
	return ctx, errors.New(`no such world "` + name + `"`)
}

func (f fakeWorlds) WorldReadable(_ context.Context, name string) (bool, error) {
	if f.readErr != nil {
		return false, f.readErr
	}
	return name == "current", nil
}

func (fakeWorlds) DefaultWorld() string { return "current" }

// worldRecorder records the world bound on the ctx of each read.
type worldRecorder struct {
	GraphReader
	seen *[]any
}

func (r worldRecorder) GetEntity(ctx context.Context, id string) (*entity.Entity, error) {
	*r.seen = append(*r.seen, ctx.Value(boundWorldKey{}))
	return r.GraphReader.GetEntity(ctx, id)
}

func (r worldRecorder) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	*r.seen = append(*r.seen, ctx.Value(boundWorldKey{}))
	return r.GraphReader.ListEntities(ctx, q)
}

// searchRecorder records the world bound on the ctx of each search.
type searchRecorder struct {
	search.Searcher
	seen *[]any
}

func (r searchRecorder) Search(ctx context.Context, q search.Query) iter.Seq2[search.Hit, error] {
	*r.seen = append(*r.seen, ctx.Value(boundWorldKey{}))
	return r.Searcher.Search(ctx, q)
}

func worldMeta(meta *metamodel.Metamodel) *metamodel.Metamodel {
	meta.Worlds = map[string]metamodel.WorldDef{
		"current": {Select: []string{"adopted", "concept"}, Otherwise: metamodel.OtherwiseExclude},
		"review": {
			Select:    []string{"concept"},
			Overrides: map[string][]string{"requirement": {"adopted"}},
			Otherwise: metamodel.OtherwiseDefault,
		},
	}
	return meta
}

func newWorldServer(t *testing.T, worlds WorldSelector) (srv *Server, seen *[]any) {
	t.Helper()
	meta, st := makeTestFixture(t)
	deps := newTestDeps(t, worldMeta(meta), st)
	seen = &[]any{}
	deps.Store = worldRecorder{GraphReader: deps.Store, seen: seen}
	deps.Searcher = searchRecorder{Searcher: deps.Searcher, seen: seen}
	deps.Worlds = worlds
	var err error
	srv, err = NewServer(deps, "test",
		WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv, seen
}

// Each read tool binds the named world for its reads, and refuses a world the
// selector refuses, with the selector's reason.
func TestReadTools_WorldArgument(t *testing.T) {
	t.Parallel()
	calls := map[string]string{
		"list_entities":   `{"type":"requirement"%s}`,
		"show_entity":     `{"id":"REQ-001"%s}`,
		"search_entities": `{"query":"requirement"%s}`,
	}
	for tool, args := range calls {
		t.Run(tool+" reads in the named world", func(t *testing.T) {
			t.Parallel()
			srv, seen := newWorldServer(t, fakeWorlds{})
			text, isErr := callTool(t, srv, tool, strings.Replace(args, "%s", `,"world":"current"`, 1))
			if isErr {
				t.Fatalf("%s: %s", tool, text)
			}
			if len(*seen) == 0 {
				t.Fatal("no read reached the store")
			}
			for _, w := range *seen {
				if w != "current" {
					t.Errorf("a read ran with world %v, want current", w)
				}
			}
		})
		t.Run(tool+" without a world binds none", func(t *testing.T) {
			t.Parallel()
			srv, seen := newWorldServer(t, fakeWorlds{})
			if text, isErr := callTool(t, srv, tool, strings.Replace(args, "%s", "", 1)); isErr {
				t.Fatalf("%s: %s", tool, text)
			}
			for _, w := range *seen {
				if w != nil {
					t.Errorf("a read ran with world %v, want none", w)
				}
			}
		})
		for world, want := range map[string]string{"review": "not readable", "nope": "no such world"} {
			t.Run(tool+" refuses "+world, func(t *testing.T) {
				t.Parallel()
				srv, seen := newWorldServer(t, fakeWorlds{})
				text, isErr := callTool(t, srv, tool, strings.Replace(args, "%s", `,"world":"`+world+`"`, 1))
				if !isErr || !strings.Contains(text, want) {
					t.Errorf("got %q (error %v), want an error containing %q", text, isErr, want)
				}
				if len(*seen) != 0 {
					t.Error("a refused world still reached the store")
				}
			})
		}
		t.Run(tool+" without a selector refuses a named world", func(t *testing.T) {
			t.Parallel()
			srv, _ := newWorldServer(t, nil)
			text, isErr := callTool(t, srv, tool, strings.Replace(args, "%s", `,"world":"current"`, 1))
			if !isErr || !strings.Contains(text, "does not resolve worlds") {
				t.Errorf("got %q (error %v), want a refusal", text, isErr)
			}
			if text, isErr := callTool(t, srv, tool, strings.Replace(args, "%s", `,"world":"default"`, 1)); isErr {
				t.Errorf("world default refused without a selector: %s", text)
			}
		})
	}
}

func TestListWorlds(t *testing.T) {
	t.Parallel()
	decode := func(t *testing.T, text string) worldListJSON {
		t.Helper()
		var out worldListJSON
		if err := json.Unmarshal([]byte(text), &out); err != nil {
			t.Fatalf("decode %s: %v", text, err)
		}
		return out
	}
	readable := func(out worldListJSON) map[string]bool {
		m := map[string]bool{}
		for _, w := range out.Worlds {
			m[w.Name] = w.Readable
		}
		return m
	}

	t.Run("with a selector", func(t *testing.T) {
		t.Parallel()
		srv, _ := newWorldServer(t, fakeWorlds{})
		text, isErr := callTool(t, srv, "list_worlds", `{}`)
		if isErr {
			t.Fatal(text)
		}
		out := decode(t, text)
		if out.DefaultWorld != "current" || out.Note != "" {
			t.Errorf("default_world = %q, note = %q; want current and no note", out.DefaultWorld, out.Note)
		}
		want := map[string]bool{defaultWorldName: true, "current": true, "review": false}
		if got := readable(out); !maps.Equal(got, want) {
			t.Errorf("readable = %v, want %v", got, want)
		}
		for _, w := range out.Worlds {
			if w.Name == "review" && (w.Otherwise != "default" || len(w.Overrides["requirement"]) != 1) {
				t.Errorf("review = %+v, want its otherwise and overrides", w)
			}
		}
	})
	t.Run("a failed grant check reports not readable", func(t *testing.T) {
		t.Parallel()
		srv, _ := newWorldServer(t, fakeWorlds{readErr: errors.New("down")})
		text, _ := callTool(t, srv, "list_worlds", `{}`)
		if got := readable(decode(t, text)); got["current"] || !got[defaultWorldName] {
			t.Errorf("readable = %v, want only default", got)
		}
	})
	t.Run("without a selector", func(t *testing.T) {
		t.Parallel()
		srv, _ := newWorldServer(t, nil)
		text, _ := callTool(t, srv, "list_worlds", `{}`)
		out := decode(t, text)
		if out.DefaultWorld != defaultWorldName || out.Note == "" {
			t.Errorf("default_world = %q, note = %q; want default and a note", out.DefaultWorld, out.Note)
		}
		if got := readable(out); got["current"] || got["review"] || !got[defaultWorldName] {
			t.Errorf("readable = %v, want only default", got)
		}
	})
}

// faceReader serves the faces in rows by their `ID@face` address.
type faceReader struct {
	GraphReader
	rows map[string]*entity.Entity
}

func (r faceReader) GetEntity(_ context.Context, ref string) (*entity.Entity, error) {
	if e, ok := r.rows[ref]; ok {
		return e, nil
	}
	return nil, store.ErrNotFound
}

func TestOtherFaces(t *testing.T) {
	t.Parallel()
	meta := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"policy": {Faces: map[string]metamodel.FaceDef{
			"concept": {Label: "Concept"}, "adopted": {}, "retired": {},
		}},
		"task": {},
	}}
	adopted := &entity.Entity{ID: "POL-1", Type: "policy", Face: "adopted"}
	concept := &entity.Entity{ID: "POL-1", Type: "policy", Face: "concept"}
	tests := []struct {
		name string
		rows map[string]*entity.Entity
		e    *entity.Entity
		want []faceJSON
	}{
		{
			name: "lists a readable other face with its label and ref",
			rows: map[string]*entity.Entity{"POL-1@adopted": adopted, "POL-1@concept": concept},
			e:    adopted,
			want: []faceJSON{{Face: "concept", Label: "Concept", Ref: "POL-1@concept"}},
		},
		{
			name: "omits the label when it repeats the face name",
			rows: map[string]*entity.Entity{"POL-1@adopted": adopted, "POL-1@concept": concept},
			e:    concept,
			want: []faceJSON{{Face: "adopted", Ref: "POL-1@adopted"}},
		},
		{
			// The reader withholds a face the grant does not cover, exactly
			// as it withholds a face that does not exist.
			name: "omits a face the reader does not return",
			rows: map[string]*entity.Entity{"POL-1@adopted": adopted},
			e:    adopted,
		},
		{name: "a faceless type has no other faces", e: &entity.Entity{ID: "TSK-1", Type: "task"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := otherFaces(context.Background(), faceReader{rows: tc.rows}, meta, tc.e)
			if len(got) != len(tc.want) {
				t.Fatalf("otherFaces = %+v, want %+v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("otherFaces[%d] = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}
