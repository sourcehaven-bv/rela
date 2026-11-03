package ingest

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"reflect"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/markdown"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/ingestspec"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// world is one fake store behind all three seams, so a row a create writes is
// what the next attempt's find and re-read see.
type world struct {
	rows []*entity.Entity

	createErrs []error // consumed one per call; a nil entry ⇒ success
	patchErrs  []error
	onConflict func() // called whenever an injected error is returned

	lists, reReads int
	created        []*entity.Entity
	createOpts     []entity.CreateOptions
	patches        []entity.Patch
	patchedRefs    []string
	queries        []store.EntityQuery
	nextID         int
}

func (w *world) GetAddress(context.Context, string) (*entity.Entity, error) {
	return nil, store.ErrNotFound
}

func (w *world) ListEntities(_ context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	w.lists++
	w.queries = append(w.queries, q)
	return func(yield func(*entity.Entity, error) bool) {
		for _, e := range w.rows {
			if e.Type == q.Type && !yield(e, nil) {
				return
			}
		}
	}
}

func (w *world) GetEntity(_ context.Context, ref entity.Ref) (*entity.Entity, error) {
	w.reReads++
	for _, e := range w.rows {
		if e.Ref() == ref {
			return e, nil
		}
	}
	return nil, store.ErrNotFound
}

func (w *world) injected(errs *[]error) error {
	if len(*errs) == 0 {
		return nil
	}
	err := (*errs)[0]
	*errs = (*errs)[1:]
	if err != nil && w.onConflict != nil {
		w.onConflict()
	}
	return err
}

func (w *world) CreateEntity(
	_ context.Context, e *entity.Entity, opts entity.CreateOptions,
) (*entity.CreateResult, error) {
	if err := w.injected(&w.createErrs); err != nil {
		return nil, err
	}
	w.nextID++
	e.ID = fmt.Sprintf("NEW-%d", w.nextID)
	e.Face = opts.Face
	w.rows = append(w.rows, e)
	w.created = append(w.created, e)
	w.createOpts = append(w.createOpts, opts)
	return &entity.CreateResult{Entity: e}, nil
}

func (w *world) PatchEntity(_ context.Context, ref string, p entity.Patch) (*entity.UpdateResult, error) {
	if err := w.injected(&w.patchErrs); err != nil {
		return nil, err
	}
	w.patchedRefs = append(w.patchedRefs, ref)
	w.patches = append(w.patches, p)
	return &entity.UpdateResult{}, nil
}

func (w *world) deps() Deps {
	return Deps{Reader: w, Writer: w, WritePrep: w, MaxAttempts: 3, Label: "test"}
}

type mapSource map[string]string

func (m mapSource) Resolve(ref string) string          { return m[ref] }
func (m mapSource) DefaultMatchRef(prop string) string { return "body." + prop }

// Kind reads the prefix: block.* is markdown, text.* is text, the rest raw.
func (m mapSource) Kind(ref string) ValueKind {
	switch {
	case strings.HasPrefix(ref, "block."):
		return ValueMarkdown
	case strings.HasPrefix(ref, "text."):
		return ValueText
	}
	return ValueRaw
}

func ticket(id, host string) *entity.Entity {
	return &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"host": host}, Content: "## Log\n"}
}

var findOrCreate = ingestspec.Mapping{
	Find: &ingestspec.Find{Type: "ticket", Match: []string{"host"}},
	CreateIfMissing: &ingestspec.Create{
		Properties: map[string]string{"title": "{{body.title}}"},
	},
	Then: []ingestspec.Step{{Set: map[string]string{"status": "{{body.status}}"}}},
}

var appendLog = ingestspec.Mapping{
	Find:            &ingestspec.Find{Type: "ticket", Match: []string{"host"}},
	CreateIfMissing: &ingestspec.Create{},
	Then: []ingestspec.Step{
		{AppendSection: &ingestspec.AppendSection{Section: "Log", Content: "- {{body.msg}}"}},
	},
}

func TestRun_Workflows(t *testing.T) {
	src := mapSource{"body.host": "web1", "body.title": "down", "body.status": "open"}

	for _, tc := range []struct {
		name       string
		rows       []*entity.Entity
		mapping    ingestspec.Mapping
		src        mapSource
		wantAction string
		wantID     string
	}{
		{
			name: "match updates the lowest id", mapping: findOrCreate, src: src,
			rows:       []*entity.Entity{ticket("T-2", "web1"), ticket("T-1", "web1"), ticket("T-0", "web2")},
			wantAction: ActionUpdated, wantID: "T-1",
		},
		{
			name: "miss creates", mapping: findOrCreate, src: src,
			rows:       []*entity.Entity{ticket("T-0", "web2")},
			wantAction: ActionCreated, wantID: "NEW-1",
		},
		{
			name:       "empty match value creates rather than matching blanks",
			mapping:    findOrCreate,
			src:        mapSource{"body.title": "x"},
			rows:       []*entity.Entity{{ID: "T-9", Type: "ticket", Properties: map[string]any{}}},
			wantAction: ActionCreated, wantID: "NEW-1",
		},
		{
			name: "miss without create is no_match",
			mapping: ingestspec.Mapping{
				Find: &ingestspec.Find{Type: "ticket", Match: []string{"host"}},
				Then: findOrCreate.Then,
			},
			src:        src,
			wantAction: ActionNoMatch,
		},
		{
			name: "no find always creates",
			mapping: ingestspec.Mapping{CreateIfMissing: &ingestspec.Create{
				Type: "ticket", Content: "{{body.title}}",
			}},
			src:        src,
			wantAction: ActionCreated, wantID: "NEW-1",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := &world{rows: tc.rows}
			res, err := Run(context.Background(), w.deps(), tc.mapping, tc.src)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if res.Action != tc.wantAction || res.EntityID != tc.wantID {
				t.Errorf("result = %+v, want %s %s", res, tc.wantAction, tc.wantID)
			}
			if got, want := len(w.created) > 0, tc.wantAction == ActionCreated; got != want {
				t.Errorf("created = %v, want %v", got, want)
			}
		})
	}
}

func TestRun_CreateProperties(t *testing.T) {
	w := &world{}
	m := ingestspec.Mapping{
		Find: &ingestspec.Find{
			Type: "ticket", Match: []string{"host", "service", "title"},
			Values: map[string]string{"service": "{{body.svc}}"},
		},
		CreateIfMissing: &ingestspec.Create{
			Template:   "alert",
			Properties: map[string]string{"title": "explicit {{body.title}}"},
		},
	}
	src := mapSource{"body.host": "web1", "body.svc": "http", "body.title": "down"}
	if _, err := Run(context.Background(), w.deps(), m, src); err != nil {
		t.Fatal(err)
	}
	props := w.created[0].Properties
	if props["host"] != src["body.host"] || props["service"] != src["body.svc"] {
		t.Errorf("match values not seeded: %v", props)
	}
	if want := "explicit " + src["body.title"]; props["title"] != want {
		t.Errorf("title = %v, want the explicit create property %q", props["title"], want)
	}
	if got := w.createOpts[0].Variant; got != m.CreateIfMissing.Template {
		t.Errorf("Variant = %q, want %q", got, m.CreateIfMissing.Template)
	}
}

// TestRun_FlattensEveryTemplatedField pins that every place a template is
// expanded flattens the source's values, not only append_section.
func TestRun_FlattensEveryTemplatedField(t *testing.T) {
	const evil = "a\n## Forged\x00"
	const flat = "a ## Forged"
	w := &world{}
	m := ingestspec.Mapping{
		Find: &ingestspec.Find{Type: "ticket", Match: []string{"host"}},
		CreateIfMissing: &ingestspec.Create{
			Properties: map[string]string{"title": "{{body.v}}"},
			Content:    "## Log\n{{body.v}}",
		},
		Then: []ingestspec.Step{{Set: map[string]string{"status": "{{body.v}}"}}},
	}
	if _, err := Run(context.Background(), w.deps(), m, mapSource{"body.host": evil, "body.v": evil}); err != nil {
		t.Fatal(err)
	}
	created := w.created[0]
	for name, got := range map[string]any{
		"seeded match value": created.Properties["host"],
		"create property":    created.Properties["title"],
		"set step":           w.patches[0].Properties["status"],
	} {
		if got != flat {
			t.Errorf("%s = %q, want %q", name, got, flat)
		}
	}
	if want := "## Log\n" + flat; created.Content != want {
		t.Errorf("content = %q, want %q", created.Content, want)
	}
}

// TestRun_BlockKeepsLinesOnlyInCreateContent pins that a source's block value
// keeps its line breaks in a new entity's body and nowhere else, and still
// loses NUL there.
func TestRun_BlockKeepsLinesOnlyInCreateContent(t *testing.T) {
	const block = "para one\n\npara\x00 two"
	w := &world{}
	m := ingestspec.Mapping{
		Find: &ingestspec.Find{Type: "ticket", Match: []string{"host"}},
		CreateIfMissing: &ingestspec.Create{
			Properties: map[string]string{"title": "{{block.v}}"},
			Content:    "{{block.v}}\n{{body.v}}",
		},
		Then: []ingestspec.Step{{Set: map[string]string{"status": "{{block.v}}"}}},
	}
	src := mapSource{"body.host": "h", "block.v": block, "body.v": "x\ny"}
	if _, err := Run(context.Background(), w.deps(), m, src); err != nil {
		t.Fatal(err)
	}
	created := w.created[0]
	if want := "para one\n\npara two\nx y"; created.Content != want {
		t.Errorf("content = %q, want %q", created.Content, want)
	}
	if got := created.Properties["title"]; got != "para one  para two" {
		t.Errorf("title = %q, want the block flattened", got)
	}
	if got := w.patches[0].Properties["status"]; got != "para one  para two" {
		t.Errorf("set step = %q, want the block flattened", got)
	}
}

// TestMatchValues pins the identity a caller reads before running a mapping.
func TestMatchValues(t *testing.T) {
	m := ingestspec.Mapping{Find: &ingestspec.Find{
		Type: "t", Match: []string{"a", "b"}, Values: map[string]string{"b": "x-{{body.y}}"},
	}}
	got := MatchValues(m, mapSource{"body.a": "1\n2", "body.y": "3"})
	if len(got) != 2 || got["a"] != "1 2" || got["b"] != "x-3" {
		t.Errorf("MatchValues = %v", got)
	}
	if got := MatchValues(ingestspec.Mapping{}, mapSource{}); got != nil {
		t.Errorf("MatchValues without find = %v, want nil", got)
	}
}

// TestRun_DefaultMatchValueIsFlattened pins that a default match value is
// resolved and flattened, never treated as template text.
func TestRun_DefaultMatchValueIsFlattened(t *testing.T) {
	w := &world{}
	src := mapSource{"body.host": "x{{body.secret}}\n## Injected", "body.secret": "leak"}
	if _, err := Run(context.Background(), w.deps(), findOrCreate, src); err != nil {
		t.Fatal(err)
	}
	if got := w.created[0].Properties["host"]; got != "x{{body.secret}} ## Injected" {
		t.Errorf("host = %q, want the literal value on one line", got)
	}
}

func TestRun_StepsMakeOnePatchAtTheFace(t *testing.T) {
	stored := ticket("T-1", "web1")
	stored.Face = "published"
	w := &world{rows: []*entity.Entity{stored}}
	m := appendLog
	m.Find = &ingestspec.Find{Type: "ticket", Face: "published", Match: []string{"host"}}
	m.Then = append([]ingestspec.Step{{Set: map[string]string{"status": "open"}}}, m.Then...)

	if _, err := Run(context.Background(), w.deps(), m, mapSource{"body.host": "web1", "body.msg": "x"}); err != nil {
		t.Fatal(err)
	}
	if len(w.patches) != 1 {
		t.Fatalf("patches = %d, want one write for both steps", len(w.patches))
	}
	if got, want := w.patchedRefs[0], stored.Ref().String(); got != want {
		t.Errorf("patched %q, want %q", got, want)
	}
	p := w.patches[0]
	if p.Properties["status"] != "open" || p.Content == nil {
		t.Errorf("patch = %+v, want status and content", p)
	}
	if p.ExpectedVersion != string(store.VersionOf(stored)) {
		t.Errorf("ExpectedVersion = %q, want the re-read base's version", p.ExpectedVersion)
	}
	if got := w.queries[0].Faces.String(); got != store.AtFaces("published").String() {
		t.Errorf("find faces = %s, want the configured face", got)
	}
}

func TestRun_FacelessFindReadsTheWorld(t *testing.T) {
	w := &world{}
	d := w.deps()
	d.World = store.TrivialScope()
	if _, err := Run(context.Background(), d, findOrCreate, mapSource{"body.host": "web1"}); err != nil {
		t.Fatal(err)
	}
	if got, want := w.queries[0].Faces.String(), store.InWorld(d.World).String(); got != want {
		t.Errorf("find faces = %s, want %s", got, want)
	}
	if got := w.createOpts[0].Face; got != "" {
		t.Errorf("create face = %q, want none for a faceless find", got)
	}
}

func TestRun_RetriesConflicts(t *testing.T) {
	unique := store.UniquePropertyError{Property: "host"}
	cas := &store.VersionConflictError{ID: "T-1"}

	t.Run("create loser re-finds the winner and updates it", func(t *testing.T) {
		w := &world{createErrs: []error{unique}}
		// The racing winner's row lands as the loser's create fails.
		w.onConflict = func() { w.rows = append(w.rows, ticket("T-1", "web1")) }
		res, err := Run(context.Background(), w.deps(), appendLog, mapSource{"body.host": "web1", "body.msg": "x"})
		if err != nil {
			t.Fatal(err)
		}
		if res.Action != ActionUpdated || res.EntityID != "T-1" {
			t.Errorf("result = %+v, want updated T-1", res)
		}
		if w.lists != 2 || len(w.created) != 0 {
			t.Errorf("lists = %d, creates = %d; want a second find and no second create", w.lists, len(w.created))
		}
	})

	t.Run("lost CAS re-reads the base", func(t *testing.T) {
		w := &world{rows: []*entity.Entity{ticket("T-1", "web1")}, patchErrs: []error{cas}}
		if _, err := Run(context.Background(), w.deps(), appendLog,
			mapSource{"body.host": "web1", "body.msg": "x"}); err != nil {
			t.Fatal(err)
		}
		if w.reReads != 2 || len(w.patches) != 1 {
			t.Errorf("re-reads = %d, patches = %d; want 2 and 1", w.reReads, len(w.patches))
		}
	})

	t.Run("budget exhausted", func(t *testing.T) {
		w := &world{createErrs: []error{unique, unique, unique, nil}}
		_, err := Run(context.Background(), w.deps(), findOrCreate, mapSource{"body.host": "web1"})
		if !errors.Is(err, ErrConflictExhausted) || !errors.As(err, new(store.UniquePropertyError)) {
			t.Errorf("err = %v, want ErrConflictExhausted wrapping the last conflict", err)
		}
		if w.lists != 3 || len(w.createErrs) != 1 {
			t.Errorf("attempts = %d, unused errors = %d; want exactly MaxAttempts", w.lists, len(w.createErrs))
		}
	})

	t.Run("cancelled between attempts", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		w := &world{createErrs: []error{unique}, onConflict: cancel}
		_, err := Run(ctx, w.deps(), findOrCreate, mapSource{"body.host": "web1"})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
		if w.lists != 1 {
			t.Errorf("attempts = %d, want no attempt after the cancel", w.lists)
		}
	})
}

func TestRun_NonConflictIsNotRetried(t *testing.T) {
	rejected := errors.New("status must be one of open, closed")
	w := &world{createErrs: []error{rejected, nil}}
	_, err := Run(context.Background(), w.deps(), findOrCreate, mapSource{"body.host": "web1"})
	if !errors.Is(err, rejected) {
		t.Fatalf("err = %v, want the rejection", err)
	}
	if w.lists != 1 {
		t.Errorf("attempts = %d, want 1", w.lists)
	}
}

type failingReader struct{ world }

func (*failingReader) ListEntities(context.Context, store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) { yield(nil, errors.New("store down")) }
}

func TestRun_Errors(t *testing.T) {
	ok := (&world{}).deps()
	failing := ok
	failing.Reader = &failingReader{}
	without := func(edit func(*Deps)) Deps {
		d := ok
		edit(&d)
		return d
	}

	for _, tc := range []struct {
		name      string
		cancelled bool
		d         Deps
		m         ingestspec.Mapping
		want      string
	}{
		{"nil reader", false, without(func(d *Deps) { d.Reader = nil }), findOrCreate,
			"no visible reader"},
		{"nil writer", false, without(func(d *Deps) { d.Writer = nil }), findOrCreate,
			"no writer"},
		{"nil write-prep", false, without(func(d *Deps) { d.WritePrep = nil }), findOrCreate,
			"no write-prep"},
		{"zero attempts", false, without(func(d *Deps) { d.MaxAttempts = 0 }), findOrCreate,
			"MaxAttempts 0"},
		{"list error", false, failing, findOrCreate, "store down"},
		{"cancelled", true, ok, findOrCreate, "context canceled"},
		{"no create type", false, ok,
			ingestspec.Mapping{CreateIfMissing: &ingestspec.Create{}}, "no resolvable entity type"},
		{"step without action", false, ok,
			ingestspec.Mapping{CreateIfMissing: &ingestspec.Create{Type: "ticket"}, Then: []ingestspec.Step{{}}},
			"then[0] declares no action"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			_, err := Run(ctx, tc.d, tc.m, mapSource{"body.host": "web1"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestRun_AppendBaseMissing(t *testing.T) {
	d := (&world{}).deps()
	d.WritePrep = &world{} // the created row is not where the re-read looks
	_, err := Run(context.Background(), d, appendLog, mapSource{"body.host": "web1"})
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("err = %v, want the re-read's not-found", err)
	}
}

// TestReader_RawStoreDoesNotSatisfy pins the type-level guard: the raw store
// must not be passable as the find's gated reader.
func TestReader_RawStoreDoesNotSatisfy(t *testing.T) {
	if reflect.TypeFor[store.Store]().Implements(reflect.TypeFor[Reader]()) {
		t.Fatal("store.Store satisfies ingest.Reader; a raw store could replace the ACL-gated reader")
	}
}

func TestIsConflict(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"unique", store.UniquePropertyError{}, true},
		{"version conflict", &store.VersionConflictError{}, true},
		{"store conflict", store.ErrConflict, true},
		{"other", errors.New("nope"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsConflict(tc.err); got != tc.want {
				t.Errorf("IsConflict = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestRun_TextIsEscapedOnlyInBodies pins where a text value is escaped: in
// the content of a new entity and in an appended section, where it would
// otherwise render as markup, but not in a property, which is not markdown.
// A raw value is never escaped.
func TestRun_TextIsEscapedOnlyInBodies(t *testing.T) {
	const text = "![x](https://t/p.gif) *hi*"
	const escaped = `\!\[x\](https://t/p.gif) \*hi\*`
	w := &world{}
	m := ingestspec.Mapping{
		Find: &ingestspec.Find{Type: "ticket", Match: []string{"host"}},
		CreateIfMissing: &ingestspec.Create{
			Properties: map[string]string{"title": "{{text.v}}"},
			Content:    "{{text.v}} {{body.v}}\n\n## Log\n",
		},
		Then: []ingestspec.Step{{AppendSection: &ingestspec.AppendSection{Section: "Log", Content: "- {{text.v}} {{body.v}}"}}},
	}
	src := mapSource{"body.host": "h", "text.v": text, "body.v": "**raw**"}
	if _, err := Run(context.Background(), w.deps(), m, src); err != nil {
		t.Fatal(err)
	}
	created := w.created[0]
	if !strings.HasPrefix(created.Content, escaped+" **raw**") {
		t.Errorf("content = %q, want the text escaped and the raw value as it is", created.Content)
	}
	if got := created.Properties["title"]; got != text {
		t.Errorf("title = %q, want the text unchanged", got)
	}
	appended := *w.patches[0].Content
	if !strings.Contains(appended, "- "+escaped+" **raw**") {
		t.Errorf("appended = %q, want the text escaped and the raw value as it is", appended)
	}
}

func TestEscapeMarkdown(t *testing.T) {
	for in, want := range map[string]string{
		"":                      ``,
		"plain text 1.2":        "plain text 1.2",
		`a\b`:                   `a\\b`,
		`\*`:                    `\\\*`,
		"<img src=x>":           `\<img src\=x\>`,
		"===":                   `\=\=\=`,
		"--- + x":               `\-\-\- \+ x`,
		"&#33;[x](y)":           `\&\#33;\[x\](y)`,
		"# h | t ~~s~~ `c` _e_": `\# h \| t \~\~s\~\~ \` + "`" + `c\` + "`" + ` \_e\_`,
	} {
		if got := EscapeMarkdown(in); got != want {
			t.Errorf("EscapeMarkdown(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestRun_TextCannotForgeASetextHeading pins that a text value alone on an
// appended line cannot turn the line above it into a heading, which would end
// the section and send every later append above it.
func TestRun_TextCannotForgeASetextHeading(t *testing.T) {
	for _, title := range []string{"===", "---"} {
		content := markdown.AppendToSection("## Log\nfirst entry\n", "Log",
			interpolateBody(mapSource{"text.v": title}, "{{text.v}}", false))
		content = markdown.AppendToSection(content, "Log", "later")
		if !strings.HasSuffix(strings.TrimSpace(content), "later") {
			t.Errorf("title %q: content = %q, want later appended last in Log", title, content)
		}
	}
}
