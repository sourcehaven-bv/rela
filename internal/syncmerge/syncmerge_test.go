package syncmerge

import (
	"errors"
	"fmt"
	"maps"
	"math/rand/v2"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

const schema = `
version: "1.0"
types:
  status:
    values: [open, done]
entities:
  ticket:
    label: Ticket
    id_prefix: "T-"
    properties:
      title: {type: string}
      points: {type: integer}
      due: {type: date}
      at: {type: datetime}
      done: {type: boolean}
      tags: {type: string, list: true}
      state: {type: status}
      kind: {type: enum, values: [bug, task]}
      basecamp: {type: external_ref, system: basecamp}
      total: {type: integer, computed: "entity.points"}
  page:
    label: Page
    id_prefix: "P-"
    properties:
      content: {type: string}
`

func fields(t *testing.T, names ...string) []Field {
	t.Helper()
	m, err := metamodel.Parse([]byte(schema))
	if err != nil {
		t.Fatal(err)
	}
	fs, err := FieldsFor(m, "ticket", names)
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

func props(kv ...any) map[string]any {
	m := map[string]any{}
	for i := 0; i < len(kv); i += 2 {
		m[kv[i].(string)] = kv[i+1]
	}
	return m
}

// AC8: per-field classification, and the base-unknown rule.
func TestMerge_Classification(t *testing.T) {
	fs := fields(t, "title", "points", "done", "content")
	base := &State{Properties: props("title", "a", "points", int64(1), "done", false), Content: "body"}
	ours := State{Properties: props("title", "a", "points", int64(2), "done", true), Content: "body"}
	body := "new body"
	theirs := Theirs{Properties: props("title", "b", "points", int64(1), "done", "true"), Content: &body}

	r, err := Merge(fs, base, ours, theirs)
	if err != nil {
		t.Fatal(err)
	}
	if r.BaseUnknown || r.Retag {
		t.Fatalf("flags: %+v", r)
	}
	if r.Write["title"] != "b" || len(r.Write) != 1 {
		t.Fatalf("write = %v", r.Write)
	}
	if r.WriteContent == nil || *r.WriteContent != "new body" {
		t.Fatalf("content = %v", r.WriteContent)
	}
	if r.Push["points"] != int64(2) || len(r.Push) != 1 {
		t.Fatalf("push = %v", r.Push)
	}
	if len(r.Unchanged) != 1 || r.Unchanged[0] != "done" {
		t.Fatalf("unchanged = %v", r.Unchanged)
	}

	t.Run("conflict", func(t *testing.T) {
		r, err := Merge(fs, base, ours, Theirs{Properties: props("points", int64(3))})
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Conflicts) != 1 || r.Conflicts[0].Field != "points" || r.Conflicts[0].Base != int64(1) {
			t.Fatalf("conflicts = %+v", r.Conflicts)
		}
	})
	t.Run("absent theirs is skipped, Empty clears", func(t *testing.T) {
		r, err := Merge(fs, base, State{Properties: props("title", "a")}, Theirs{Properties: props("title", Empty)})
		if err != nil {
			t.Fatal(err)
		}
		if v, ok := r.Write["title"]; !ok || v != nil {
			t.Fatalf("write = %v, want a clear of title", r.Write)
		}
		if len(r.Push) != 0 || len(r.Conflicts) != 0 {
			t.Fatalf("absent fields must be skipped: %+v", r)
		}
	})
	t.Run("base unknown", func(t *testing.T) {
		r, err := Merge(fs, nil, ours, theirs)
		if err != nil {
			t.Fatal(err)
		}
		if !r.BaseUnknown || len(r.Write) != 0 || len(r.Push) != 0 || r.WriteContent != nil {
			t.Fatalf("base unknown must write and push nothing: %+v", r)
		}
		if len(r.Conflicts) != 3 {
			t.Fatalf("conflicts = %+v", r.Conflicts)
		}
		// A partial report proves nothing about the fields it left out, so
		// it never retags, even when every reported field agrees.
		r, err = Merge(fs, nil, ours, Theirs{Properties: props("title", "a")})
		if err != nil {
			t.Fatal(err)
		}
		if r.Retag || r.Complete {
			t.Fatalf("partial report without a base must not retag: %+v", r)
		}
		r, err = Merge(fs, nil, ours, Theirs{})
		if err != nil {
			t.Fatal(err)
		}
		if r.Retag || r.Complete {
			t.Fatalf("empty report without a base must not retag: %+v", r)
		}
		r, err = Merge(fs, nil, ours, Theirs{Properties: props("title", "a", "points", "2", "done", true),
			Content: new("body")})
		if err != nil {
			t.Fatal(err)
		}
		if !r.Retag || !r.Complete {
			t.Fatalf("complete converged report without a base must retag: %+v", r)
		}
	})
	t.Run("unreported local edit blocks retag", func(t *testing.T) {
		// points changed locally since base; theirs does not report it.
		r, err := Merge(fs, base, State{Properties: props("title", "a", "points", int64(2), "done", true),
			Content: "body"}, Theirs{Properties: props("done", true)})
		if err != nil {
			t.Fatal(err)
		}
		if r.Retag || r.Complete {
			t.Fatalf("unreported local edit must not retag: %+v", r)
		}
	})
	t.Run("converged since base retags", func(t *testing.T) {
		r, err := Merge(fs, base, ours, Theirs{Properties: props("title", "a", "points", 2, "done", true),
			Content: new("body")})
		if err != nil {
			t.Fatal(err)
		}
		if !r.Retag {
			t.Fatalf("want retag: %+v", r)
		}
		r, err = Merge(fs, base, *base, Theirs{Properties: props("title", "a")})
		if err != nil {
			t.Fatal(err)
		}
		if r.Retag {
			t.Fatal("ours equals base: nothing to retag")
		}
	})
}

// AC9: equality by meaning, and canonical write values.
func TestEqualAndCanonical(t *testing.T) {
	fs := fields(t, "title", "points", "due", "at", "done", "tags", "state", "kind", "basecamp")
	f := map[string]Field{}
	for _, x := range fs {
		f[x.Name] = x
	}
	content := Field{Name: "content", Kind: KindContent}
	equal := []struct {
		field string
		a, b  any
	}{
		{"title", "", nil},
		{"title", nil, []any{}},
		{"points", int64(3), "3"},
		{"points", 3.0, 3},
		{"due", "2026-10-08", "2026-10-08T00:00:00Z"},
		{"at", "2026-10-08T12:00:00+02:00", "2026-10-08T10:00:00Z"},
		{"done", true, "true"},
		{"tags", []any{"a", "b", "a"}, []string{"b", "a", "a"}},
		{"basecamp", map[string]any{"id": "1", "url": "https://a.test"}, map[string]any{"id": "1"}},
	}
	for _, tc := range equal {
		if !Equal(f[tc.field], tc.a, tc.b) {
			t.Errorf("%s: %#v != %#v", tc.field, tc.a, tc.b)
		}
	}
	if !Equal(content, "a\r\nb  \n\n", "a\nb") {
		t.Error("content normalization")
	}
	unequal := []struct {
		field string
		a, b  any
	}{
		{"title", "a", "A"},
		{"title", "", "x"},
		{"points", 3, 4},
		{"tags", []any{"a", "a"}, []any{"a"}},
		{"due", "2026-10-08", "2026-10-09"},
	}
	for _, tc := range unequal {
		if Equal(f[tc.field], tc.a, tc.b) {
			t.Errorf("%s: %#v == %#v", tc.field, tc.a, tc.b)
		}
	}

	canon := []struct {
		field string
		in    any
		want  string
	}{
		{"points", "7", "int64:7"},
		{"due", "2026-10-08T00:00:00Z", "string:2026-10-08"},
		{"at", "2026-10-08T12:00:00+02:00", "string:2026-10-08T10:00:00Z"},
		{"done", "false", "bool:false"},
		{"state", "done", "string:done"},
	}
	for _, tc := range canon {
		got, err := Canonical(f[tc.field], tc.in)
		if err != nil {
			t.Fatalf("%s: %v", tc.field, err)
		}
		if s := fmt.Sprintf("%T:%v", got, got); s != tc.want {
			t.Errorf("%s: %s, want %s", tc.field, s, tc.want)
		}
	}
	// D6: theirs values that do not fit raise.
	for field, bad := range map[string]any{"points": "lots", "state": "closed", "kind": "epic", "done": "maybe"} {
		if _, err := Merge([]Field{f[field]}, &State{}, State{}, Theirs{Properties: props(field, bad)}); !errors.Is(err, ErrField) {
			t.Errorf("%s=%v: err = %v", field, bad, err)
		}
	}
}

// AC10: hidden, undeclared and clashing fields are refused.
func TestMerge_Refusals(t *testing.T) {
	m, err := metamodel.Parse([]byte(schema))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ typ, field string }{
		{"ticket", "nope"}, {"ticket", "total"}, {"page", "content"},
	} {
		if _, err := FieldsFor(m, tc.typ, []string{tc.field}); !errors.Is(err, ErrField) {
			t.Errorf("%s.%s: err = %v", tc.typ, tc.field, err)
		}
	}
	fs := fields(t, "title")
	if _, err := Merge(fs, nil, State{Redacted: []string{"title"}}, Theirs{}); !errors.Is(err, ErrField) {
		t.Errorf("hidden in ours: %v", err)
	}
	if _, err := Merge(fs, &State{Redacted: []string{"title"}}, State{}, Theirs{}); !errors.Is(err, ErrField) {
		t.Errorf("hidden in base: %v", err)
	}
}

// Swapping ours and theirs swaps write and push and keeps conflicts.
func FuzzMerge_SwapSymmetry(f *testing.F) {
	f.Add("a", "a", "b")
	f.Add("a", "b", "c")
	f.Add("", "x", "")
	f.Fuzz(func(t *testing.T, b, o, th string) {
		fs := []Field{{Name: "x", Kind: KindString}}
		base := &State{Properties: props("x", b)}
		r1, err := Merge(fs, base, State{Properties: props("x", o)}, Theirs{Properties: props("x", th)})
		if err != nil {
			t.Fatal(err)
		}
		r2, err := Merge(fs, base, State{Properties: props("x", th)}, Theirs{Properties: props("x", o)})
		if err != nil {
			t.Fatal(err)
		}
		norm := func(v any) any {
			if isEmpty(v) {
				return nil
			}
			return v
		}
		if len(r1.Conflicts) != len(r2.Conflicts) || len(r1.Write) != len(r2.Push) || len(r1.Push) != len(r2.Write) {
			t.Fatalf("asymmetric: %+v vs %+v", r1, r2)
		}
		for k, v := range r1.Write {
			if norm(r2.Push[k]) != norm(v) {
				t.Fatalf("write %v vs push %v", r1.Write, r2.Push)
			}
		}
	})
}

// D12: any interleaving of user edits with sync rounds reaches a fixed point
// within two rounds once edits stop, and no edit is lost: each field either
// converged on its most recent edit or is reported as a conflict. While edits
// land, the other system omits fields from its report at random; the loop
// tags only after a complete report (finding 1), so an unreported local edit
// is pushed in a later round rather than folded into the base.
func TestConvergence(t *testing.T) {
	fs := []Field{{Name: "a", Kind: KindString}, {Name: "b", Kind: KindString}, {Name: "c", Kind: KindString}}
	names := []string{"a", "b", "c"}
	for seed := range uint64(2000) {
		rng := rand.New(rand.NewPCG(seed, 7))
		local, remote := map[string]any{}, map[string]any{}
		var base map[string]any // nil: never synced
		latest := map[string]any{}
		edits := 0
		edit := func() {
			f := names[rng.IntN(len(names))]
			v := fmt.Sprintf("v%d", edits)
			edits++
			if rng.IntN(2) == 0 {
				local[f] = v
			} else {
				remote[f] = v
			}
			latest[f] = v
		}
		// round runs one sync. midEdit, when set, lands a user edit between
		// the read and the write (1) or between the write and the tag (2).
		// partial lets the other system leave fields out of its report.
		round := func(midEdit int, partial bool) (changed bool, conflicts map[string]bool) {
			conflicts = map[string]bool{}
			for range 3 {
				read, readRemote := maps.Clone(local), maps.Clone(remote)
				var bs *State
				if base != nil {
					bs = &State{Properties: maps.Clone(base)}
				}
				// A reported field that is missing remotely is a clear.
				theirs := map[string]any{}
				for _, f := range names {
					if partial && rng.IntN(3) == 0 {
						continue
					}
					theirs[f] = Empty
					if v, ok := remote[f]; ok {
						theirs[f] = v
					}
				}
				r, err := Merge(fs, bs, State{Properties: read}, Theirs{Properties: theirs})
				if err != nil {
					t.Fatal(err)
				}
				if r.Complete != (len(theirs) == len(names)) {
					t.Fatalf("seed %d: Complete=%v with %d of %d fields reported", seed, r.Complete, len(theirs), len(names))
				}
				if midEdit == 1 {
					edit()
					midEdit = 0
				}
				// update_entity with expect: refused when local moved.
				if len(r.Write) > 0 && !maps.Equal(read, local) {
					continue
				}
				for k, v := range r.Write {
					changed = true
					if v == nil {
						delete(local, k)
					} else {
						local[k] = v
					}
				}
				// The token the tag expects: of the row written, or of the
				// row read when nothing was written (D1).
				written := maps.Clone(local)
				if len(r.Write) == 0 {
					written = read
				}
				// The push is conditional on the remote state read (an
				// ETag / If-Match); a moved remote means a new merge.
				if len(r.Push) > 0 && !maps.Equal(readRemote, remote) {
					continue
				}
				for k, v := range r.Push {
					changed = true
					if isEmpty(v) {
						delete(remote, k)
					} else {
						remote[k] = v
					}
				}
				if midEdit == 2 {
					edit()
				}
				for _, c := range r.Conflicts {
					conflicts[c.Field] = true
				}
				// The guide's rule: tag only when there are no conflicts and
				// the report was complete, and only while local is still the
				// row the tag's token describes (expect).
				if len(r.Conflicts) == 0 && r.Complete && (len(r.Write) > 0 || len(r.Push) > 0 || r.Retag) &&
					maps.Equal(written, local) {

					if !maps.Equal(base, written) {
						changed = true
					}
					base = written
				}
				return changed, conflicts
			}
			return true, conflicts
		}
		for range 1 + rng.IntN(6) {
			if rng.IntN(3) == 0 {
				round(1+rng.IntN(2), true)
			} else {
				edit()
			}
			if rng.IntN(2) == 0 {
				round(0, rng.IntN(2) == 0)
			}
		}
		// Edits stopped: the first complete round may still write, push and
		// tag; the second must change nothing.
		round(0, false)
		changed, conflicts := round(0, false)
		if changed {
			t.Fatalf("seed %d: no fixed point within two rounds: local=%v remote=%v base=%v",
				seed, local, remote, base)
		}
		for _, f := range names {
			if conflicts[f] {
				continue
			}
			if local[f] != remote[f] || (latest[f] != nil && local[f] != latest[f]) {
				t.Fatalf("seed %d: field %s lost an edit: local=%v remote=%v latest=%v",
					seed, f, local[f], remote[f], latest[f])
			}
		}
	}
}
