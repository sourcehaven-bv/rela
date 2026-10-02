package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// fakeWorlds declares `current` and `review`; the caller may read `current`
// only. `current` selects a face no fixture row has, so a read that runs in
// it finds nothing, and a read that ignores it finds the fixture's rows.
type fakeWorlds struct {
	readErr error
}

func (fakeWorlds) SelectWorld(_ context.Context, name string) (store.WorldScope, error) {
	switch name {
	case "current":
		return store.NewWorldScope(map[string]store.TypeResolution{
			"requirement": {Chain: []entity.Face{"adopted"}, Fallback: store.FallbackExclude},
		}), nil
	case "review":
		return store.WorldScope{}, errors.New(`world "review" is not readable by you`)
	}
	return store.WorldScope{}, errors.New(`no such world "` + name + `"`)
}

func (f fakeWorlds) WorldReadable(_ context.Context, name string) (bool, error) {
	if f.readErr != nil {
		return false, f.readErr
	}
	return name == "current", nil
}

func (fakeWorlds) DefaultWorld() string { return "current" }

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

func newWorldServer(t *testing.T, worlds WorldSelector, meta func(*metamodel.Metamodel) *metamodel.Metamodel) *Server {
	t.Helper()
	m, st := makeTestFixture(t)
	// A title search_entities can find, as in the golden fixture.
	req, err := st.GetEntity(context.Background(), entity.Ref{ID: "REQ-001"})
	if err != nil {
		t.Fatal(err)
	}
	req.Properties["title"] = "First requirement"
	if err = st.UpdateEntity(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	deps := newTestDeps(t, meta(m), st)
	deps.Worlds = worlds
	srv, err := NewServer(deps, "test",
		WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv
}

func noWorlds(m *metamodel.Metamodel) *metamodel.Metamodel { return m }

// Each read tool reads in the named world, and refuses a world the selector
// refuses, with the selector's reason. REQ-001 has no `adopted` face, so it
// is visible without a world and absent in `current`.
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
			srv := newWorldServer(t, fakeWorlds{}, worldMeta)
			text, isErr := callTool(t, srv, tool, strings.Replace(args, "%s", `,"world":"current"`, 1))
			if strings.Contains(text, `"REQ-001"`) && !isErr {
				t.Errorf("%s in current served REQ-001, which has no adopted face: %s", tool, text)
			}
		})
		t.Run(tool+" without a world reads in the default world", func(t *testing.T) {
			t.Parallel()
			srv := newWorldServer(t, fakeWorlds{}, noWorlds)
			text, isErr := callTool(t, srv, tool, strings.Replace(args, "%s", "", 1))
			if isErr || !strings.Contains(text, "REQ-001") {
				t.Errorf("%s = %q (error %v), want REQ-001", tool, text, isErr)
			}
		})
		for world, want := range map[string]string{"review": "not readable", "nope": "no such world"} {
			t.Run(tool+" refuses "+world, func(t *testing.T) {
				t.Parallel()
				srv := newWorldServer(t, fakeWorlds{}, worldMeta)
				text, isErr := callTool(t, srv, tool, strings.Replace(args, "%s", `,"world":"`+world+`"`, 1))
				if !isErr || !strings.Contains(text, want) {
					t.Errorf("got %q (error %v), want an error containing %q", text, isErr, want)
				}
			})
		}
		t.Run(tool+" without a selector accepts only the default world", func(t *testing.T) {
			t.Parallel()
			srv := newWorldServer(t, nil, noWorlds)
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
		srv := newWorldServer(t, fakeWorlds{}, worldMeta)
		text, isErr := callTool(t, srv, "list_worlds", `{}`)
		if isErr {
			t.Fatal(text)
		}
		out := decode(t, text)
		if out.DefaultWorld != "current" || out.Note != "" {
			t.Errorf("default_world = %q, note = %q; want current and no note", out.DefaultWorld, out.Note)
		}
		want := map[string]bool{"current": true, "review": false}
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
		srv := newWorldServer(t, fakeWorlds{readErr: errors.New("down")}, worldMeta)
		text, _ := callTool(t, srv, "list_worlds", `{}`)
		if got := readable(decode(t, text)); got["current"] || got["review"] {
			t.Errorf("readable = %v, want none", got)
		}
	})
	t.Run("without a selector", func(t *testing.T) {
		t.Parallel()
		srv := newWorldServer(t, nil, worldMeta)
		text, _ := callTool(t, srv, "list_worlds", `{}`)
		out := decode(t, text)
		// The schema declares worlds, so its first declared one is the
		// default and the generated `default` world does not exist.
		if out.DefaultWorld != "current" || out.Note == "" {
			t.Errorf("default_world = %q, note = %q; want current and a note", out.DefaultWorld, out.Note)
		}
		if got := readable(out); got["current"] || got["review"] || len(got) != 2 {
			t.Errorf("readable = %v, want current and review, neither readable", got)
		}
	})
}

// faceReader reports the faces in rows as the readable family.
type faceReader struct {
	GraphReader
	rows map[string]*entity.Entity
}

func (r faceReader) Family(_ context.Context, id string) (visibility.Family, bool, error) {
	fam := visibility.Family{ID: id}
	for _, e := range r.rows {
		if e.ID == id {
			fam.Type = e.Type
			fam.Faces = append(fam.Faces, e.Face)
		}
	}
	slices.Sort(fam.Faces)
	return fam, len(fam.Faces) > 0, nil
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
