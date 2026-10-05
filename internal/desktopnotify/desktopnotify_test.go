package desktopnotify

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const testSchema = `version: "1.0"
entities:
  taak:
    label: Taak
    plural: taken
    id_prefix: "TAAK-"
    id_type: sequential
    display_property: title
    properties:
      title:
        type: string
      status:
        type: string
      vervaldatum:
        type: date
      tags:
        type: string
  note:
    label: Note
    plural: notes
    id_prefix: "NOTE-"
    id_type: sequential
    properties:
      title:
        type: string
relations:
  about:
    label: about
    inverse: hasNote
    from: [note]
    to: [taak]
`

// fixedNow is the clock every test evaluates today() against.
var fixedNow = time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)

// fileLoader serves desktop.yaml from memory; an absent entry is
// fs.ErrNotExist, like config.Loader.
type fileLoader struct {
	files map[string]string
	err   error
}

func (l fileLoader) Load(_ context.Context, name string) ([]byte, error) {
	if l.err != nil {
		return nil, l.err
	}
	s, ok := l.files[name]
	if !ok {
		return nil, &os.PathError{Op: "open", Path: name, Err: os.ErrNotExist}
	}
	return []byte(s), nil
}

func withYAML(src string) fileLoader {
	return fileLoader{files: map[string]string{ConfigFile: src}}
}

func testMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	meta, err := metamodel.Parse([]byte(testSchema))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	return meta
}

func load(t *testing.T, src string) *Config {
	t.Helper()
	cfg, err := Load(t.Context(), withYAML(src), testMeta(t), WithClock(func() time.Time { return fixedNow }))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

type taak struct {
	id, title, status, due string
}

func newStore(t *testing.T, taken ...taak) *memstore.MemStore {
	t.Helper()
	st := memstore.New()
	for _, tk := range taken {
		putTaak(t, st, tk)
	}
	return st
}

func putTaak(t *testing.T, st *memstore.MemStore, tk taak) {
	t.Helper()
	props := map[string]any{"title": tk.title, "status": tk.status}
	if tk.due != "" {
		props["vervaldatum"] = tk.due
	}
	e := &entity.Entity{ID: tk.id, Type: "taak", Properties: props}
	ctx := context.Background()
	if _, err := st.GetEntity(ctx, entity.Ref{ID: tk.id}); err == nil {
		if err := st.UpdateEntity(ctx, e); err != nil {
			t.Fatalf("UpdateEntity: %v", err)
		}
		return
	}
	if err := st.CreateEntity(ctx, e); err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}
}

func newKV(t *testing.T) state.KV {
	t.Helper()
	rfs, err := storage.NewRootedFS(storage.NewMemFS(), "/state")
	if err != nil {
		t.Fatalf("NewRootedFS: %v", err)
	}
	return state.NewFSKV(rfs)
}

func TestLoad_Errors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{"unknown top-level key", "notifs: []\n", "field notifs not found"},
		{"unknown rule key", "notifications:\n  - id: a\n    type: taak\n    condition: \"true\"\n    icon: x\n", "field icon not found"},
		{"malformed yaml", "notifications: [\n", "parse desktop.yaml"},
		{"missing id", "notifications:\n  - type: taak\n    condition: \"true\"\n", "id is required"},
		{"invalid id", "notifications:\n  - id: Due Today\n    type: taak\n    condition: \"true\"\n", "must match"},
		{
			"duplicate id",
			"notifications:\n  - id: a\n    type: taak\n    condition: \"true\"\n  - id: a\n    type: taak\n    condition: \"true\"\n",
			"duplicate id \"a\"",
		},
		{"missing type", "notifications:\n  - id: a\n    condition: \"true\"\n", "type is required"},
		{"unknown type", "notifications:\n  - id: a\n    type: ghost\n    condition: \"true\"\n", "unknown entity type \"ghost\""},
		{"missing condition", "notifications:\n  - id: a\n    type: taak\n", "condition is required"},
		{"bad condition", "notifications:\n  - id: a\n    type: taak\n    condition: \"entity.nope ==\"\n", "condition"},
		{"unknown property in condition", "notifications:\n  - id: a\n    type: taak\n    condition: \"entity.nope == 'x'\"\n", "condition"},
		{
			"current_user",
			"notifications:\n  - id: a\n    type: taak\n    condition: \"is_current_user(entity.status)\"\n",
			"current_user is not available in desktop.yaml",
		},
		{
			"related",
			"notifications:\n  - id: a\n    type: taak\n    condition: \"related(entity, 'hasNote', { title = 'x' })\"\n",
			"related() is not available",
		},
		{"bad title placeholder", "notifications:\n  - id: a\n    type: taak\n    condition: \"true\"\n    title: \"{{now}}\"\n", "unsupported placeholder"},
		{"title function call", "notifications:\n  - id: a\n    type: taak\n    condition: \"true\"\n    title: \"{{entity.title | upper}}\"\n", "unsupported placeholder"},
		{"unknown title property", "notifications:\n  - id: a\n    type: taak\n    condition: \"true\"\n    title: \"{{entity.ghost}}\"\n", "unknown property \"ghost\""},
		{"bad body placeholder", "notifications:\n  - id: a\n    type: taak\n    condition: \"true\"\n    body: \"x {{ entity.title }} {{\"\n", "unsupported placeholder"},
		{"badge unknown type", "badge:\n  type: ghost\n  condition: \"true\"\n", "badge: unknown entity type"},
		{"badge missing condition", "badge:\n  type: taak\n", "badge: condition is required"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(context.Background(), withYAML(tc.yaml), testMeta(t))
			if err == nil {
				t.Fatalf("Load succeeded, want error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not contain %q", err, tc.want)
			}
		})
	}
}

func TestLoad_MissingFileAndEmpty(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		loader fileLoader
	}{
		{"missing file", fileLoader{}},
		{"empty file", withYAML("")},
		{"comment only", withYAML("# nothing yet\n")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := Load(context.Background(), tc.loader, testMeta(t))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if len(cfg.RuleIDs()) != 0 || cfg.HasBadge() {
				t.Fatalf("want no rules and no badge, got %v badge=%v", cfg.RuleIDs(), cfg.HasBadge())
			}
			res, err := cfg.Evaluate(context.Background(), newStore(t, taak{"TAAK-1", "a", "open", ""}), store.TrivialScope())
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if len(res.Matches) != 0 || res.Badge != 0 {
				t.Fatalf("want empty result, got %+v", res)
			}
		})
	}
}

func TestLoad_ReadErrorAndNilArgs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	meta := testMeta(t)
	readErr := errors.New("disk on fire")
	if _, err := Load(ctx, fileLoader{err: readErr}, meta); !errors.Is(err, readErr) {
		t.Fatalf("want read error, got %v", err)
	}
	if _, err := Load(ctx, nil, meta); err == nil {
		t.Fatal("nil loader accepted")
	}
	if _, err := Load(ctx, fileLoader{}, nil); err == nil {
		t.Fatal("nil metamodel accepted")
	}
	cfg := load(t, "")
	if _, err := cfg.Evaluate(ctx, nil, store.TrivialScope()); err == nil {
		t.Fatal("nil reader accepted")
	}
	if _, err := NewTracker(nil); err == nil {
		t.Fatal("nil KV accepted")
	}
}

const dueTodayYAML = `
notifications:
  - id: due-today
    type: taak
    condition: "entity.status == 'open' and entity.vervaldatum <= today()"
    title: "Due: {{entity.title}}"
    body: "{{entity.id}} ({{ entity.type }}) is {{entity.status}}{{entity.tags}}"
  - id: done
    type: taak
    condition: "entity.status == 'done'"
badge:
  type: taak
  condition: "entity.status == 'open'"
`

func TestEvaluate_MatchesAndBadge(t *testing.T) {
	t.Parallel()
	cfg := load(t, dueTodayYAML)
	if got := cfg.RuleIDs(); strings.Join(got, ",") != "due-today,done" {
		t.Fatalf("RuleIDs = %v", got)
	}
	if !cfg.HasBadge() {
		t.Fatal("HasBadge = false")
	}
	st := newStore(t,
		taak{"TAAK-1", "Overdue", "open", "2026-10-01"},
		taak{"TAAK-2", "Today", "open", "2026-10-04"},
		taak{"TAAK-3", "Later", "open", "2026-10-09"},
		taak{"TAAK-4", "Finished", "done", "2026-09-01"},
		taak{"TAAK-5", "", "open", ""},
	)
	if err := st.CreateEntity(context.Background(), &entity.Entity{
		ID: "NOTE-1", Type: "note", Properties: map[string]any{"title": "open"},
	}); err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}

	res, err := cfg.Evaluate(context.Background(), st, store.TrivialScope())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	want := []Match{
		{RuleID: "due-today", EntityID: "TAAK-1", EntityType: "taak", Title: "Due: Overdue", Body: "TAAK-1 (taak) is open"},
		{RuleID: "due-today", EntityID: "TAAK-2", EntityType: "taak", Title: "Due: Today", Body: "TAAK-2 (taak) is open"},
		{RuleID: "done", EntityID: "TAAK-4", EntityType: "taak", Title: "Finished", Body: ""},
	}
	if len(res.Matches) != len(want) {
		t.Fatalf("matches = %+v, want %+v", res.Matches, want)
	}
	for i := range want {
		if res.Matches[i] != want[i] {
			t.Errorf("match %d = %+v, want %+v", i, res.Matches[i], want[i])
		}
	}
	if res.Badge != 4 {
		t.Errorf("badge = %d, want 4 (open taken only, not the note)", res.Badge)
	}
	if strings.Join(res.Rules, ",") != "due-today,done" {
		t.Errorf("Rules = %v", res.Rules)
	}
}

func TestEvaluate_EvalErrorIsNoMatch(t *testing.T) {
	t.Parallel()
	cfg := load(t, "notifications:\n  - id: a\n    type: taak\n    condition: \"entity.vervaldatum <= today()\"\n")
	st := newStore(t, taak{"TAAK-1", "bad date", "open", "not-a-date"})
	res, err := cfg.Evaluate(context.Background(), st, store.TrivialScope())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(res.Matches) != 0 {
		t.Fatalf("matches = %+v, want none", res.Matches)
	}
	// Whether the binding rejects the value or treats it as absent is the
	// predicate engine's business; either way it must not match. When it
	// does reject, the error names the entity.
	for _, e := range res.EvalErrors {
		if !strings.Contains(e.Error(), "TAAK-1") {
			t.Errorf("eval error %q does not name the entity", e)
		}
	}
}

func TestRender(t *testing.T) {
	t.Parallel()
	meta := testMeta(t)
	def, _ := meta.GetEntityDef("taak")
	long := strings.Repeat("é", 300)
	tests := []struct {
		name     string
		src      string
		props    map[string]any
		limit    int
		newlines bool
		want     string
	}{
		{"literal only", "hello", nil, maxTitleRunes, false, "hello"},
		{"all fields", "{{entity.id}}/{{entity.type}}/{{entity.title}}/{{entity.status}}",
			map[string]any{"status": "open"}, maxTitleRunes, false, "TAAK-1/taak/Shown/open"},
		{"missing property is empty", "[{{entity.status}}]", nil, maxTitleRunes, false, "[]"},
		{"list property", "{{entity.tags}}", map[string]any{"tags": []any{"a", nil, 2}}, maxTitleRunes, false, "a, , 2"},
		{"string list property", "{{entity.tags}}", map[string]any{"tags": []string{"x", "y"}}, maxTitleRunes, false, "x, y"},
		{"non-string property", "{{entity.status}}", map[string]any{"status": 3}, maxTitleRunes, false, "3"},
		{"title drops newlines", "{{entity.status}}", map[string]any{"status": "a\nb\tc"}, maxTitleRunes, false, "a b c"},
		{"body keeps newlines", "{{entity.status}}", map[string]any{"status": "a\nb\x00c"}, maxBodyRunes, true, "a\nb c"},
		{"title truncated", "{{entity.status}}", map[string]any{"status": long}, maxTitleRunes, false,
			strings.Repeat("é", maxTitleRunes-1) + "…"},
		{"body truncated", "{{entity.status}}", map[string]any{"status": long}, maxBodyRunes, true,
			strings.Repeat("é", maxBodyRunes-1) + "…"},
		{"placeholder value is not re-expanded", "{{entity.status}}",
			map[string]any{"status": "{{entity.id}}"}, maxTitleRunes, false, "{{entity.id}}"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpl, err := parseTemplate(tc.src, def)
			if err != nil {
				t.Fatalf("parseTemplate: %v", err)
			}
			got := plainText(tmpl.render(fields{id: "TAAK-1", entityType: "taak", title: "Shown", props: tc.props}),
				tc.limit, tc.newlines)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTracker(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := load(t, dueTodayYAML)
	st := newStore(t,
		taak{"TAAK-1", "Existing", "open", "2026-10-01"},
		taak{"TAAK-2", "Later", "open", "2026-12-01"},
	)
	kv := newKV(t)
	first, err := NewTracker(kv)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	// A second tracker on the same KV stands in for the next app launch.
	second, err := NewTracker(kv)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	allOpen := load(t, `
notifications:
  - id: all-open
    type: taak
    condition: "entity.status == 'open'"
`)

	// Steps run in order; each depends on the state the previous one left.
	steps := []struct {
		name    string
		change  *taak
		cfg     *Config
		tracker *Tracker
		want    []string
	}{
		{name: "first run is silent", tracker: first},
		{name: "unchanged matches do not repeat", tracker: first},
		{
			name: "new match notifies", tracker: first,
			change: &taak{"TAAK-2", "Later", "open", "2026-10-02"}, want: []string{"due-today:TAAK-2"},
		},
		{name: "new match notifies only once", tracker: first},
		{
			// "done" had no matches on the first run, but it was recorded,
			// so its first match now is a change and notifies.
			name: "rule empty at first run notifies later matches", tracker: first,
			change: &taak{"TAAK-3", "Wrapped up", "done", ""}, want: []string{"done:TAAK-3"},
		},
		{
			name: "entity leaves the match", tracker: first,
			change: &taak{"TAAK-1", "Existing", "paused", "2026-10-01"},
		},
		{
			name: "re-match after leaving notifies again", tracker: first,
			change: &taak{"TAAK-1", "Existing", "open", "2026-10-01"}, want: []string{"due-today:TAAK-1"},
		},
		{name: "state persists across tracker instances", tracker: second},
		{
			name: "second tracker notifies new matches", tracker: second,
			change: &taak{"TAAK-4", "Fresh", "open", "2026-10-03"}, want: []string{"due-today:TAAK-4"},
		},
		{name: "newly added rule is silent on its first run", tracker: second, cfg: allOpen},
		{
			name: "newly added rule notifies after its first run", tracker: second, cfg: allOpen,
			change: &taak{"TAAK-5", "Another", "open", ""}, want: []string{"all-open:TAAK-5"},
		},
	}
	for _, step := range steps {
		if step.change != nil {
			putTaak(t, st, *step.change)
		}
		c := cfg
		if step.cfg != nil {
			c = step.cfg
		}
		res, err := c.Evaluate(ctx, st, store.TrivialScope())
		if err != nil {
			t.Fatalf("%s: Evaluate: %v", step.name, err)
		}
		fresh, err := step.tracker.Update(ctx, res)
		if err != nil {
			t.Fatalf("%s: Update: %v", step.name, err)
		}
		got := make([]string, len(fresh))
		for i, m := range fresh {
			got[i] = m.RuleID + ":" + m.EntityID
		}
		if strings.Join(got, ",") != strings.Join(step.want, ",") {
			t.Fatalf("%s: new = %v, want %v", step.name, got, step.want)
		}
	}
}

func TestTracker_CorruptStateStartsOver(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kv := newKV(t)
	if err := kv.Put(ctx, stateKeyPrefix+"a", []byte("{not json")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	tr, err := NewTracker(kv)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	res := Result{Rules: []string{"a"}, Matches: []Match{{RuleID: "a", EntityID: "X-1"}}}
	fresh, err := tr.Update(ctx, res)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(fresh) != 0 {
		t.Fatalf("corrupt state must restart silently, got %+v", fresh)
	}
	data, err := kv.Get(ctx, stateKeyPrefix+"a")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(data) != `{"seen":["X-1"]}` {
		t.Fatalf("state = %s", data)
	}
}

// failingKV fails Get or Put on a chosen key.
type failingKV struct {
	state.KV
	failGet, failPut string
}

var errKV = errors.New("kv down")

func (f failingKV) Get(ctx context.Context, key string) ([]byte, error) {
	if key == f.failGet {
		return nil, errKV
	}
	return f.KV.Get(ctx, key)
}

func (f failingKV) Put(ctx context.Context, key string, data []byte) error {
	if key == f.failPut {
		return errKV
	}
	return f.KV.Put(ctx, key, data)
}

func TestTracker_KVErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// Rule "a" is already known, so X-2 is new for it; rule "b" fails.
	seed := func(t *testing.T) state.KV {
		t.Helper()
		kv := newKV(t)
		if err := kv.Put(ctx, stateKeyPrefix+"a", []byte(`{"seen":["X-1"]}`)); err != nil {
			t.Fatalf("Put: %v", err)
		}
		return kv
	}
	res := Result{
		Rules: []string{"a", "b"},
		Matches: []Match{
			{RuleID: "a", EntityID: "X-1"}, {RuleID: "a", EntityID: "X-2"},
			{RuleID: "b", EntityID: "X-3"},
		},
	}
	tests := []struct {
		name             string
		failGet, failPut string
	}{
		{"get fails", stateKeyPrefix + "b", ""},
		{"put fails", "", stateKeyPrefix + "b"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tr, err := NewTracker(failingKV{KV: seed(t), failGet: tc.failGet, failPut: tc.failPut})
			if err != nil {
				t.Fatalf("NewTracker: %v", err)
			}
			fresh, err := tr.Update(ctx, res)
			if !errors.Is(err, errKV) {
				t.Fatalf("want kv error, got %v", err)
			}
			// Rule "a" was recorded before "b" failed, so its new match is
			// still returned for the caller to show.
			if len(fresh) != 1 || fresh[0].EntityID != "X-2" {
				t.Fatalf("fresh = %+v, want X-2 only", fresh)
			}
		})
	}
}

func TestTracker_ReservedRuleID(t *testing.T) {
	t.Parallel()
	// "con" is a Windows reserved name; the key prefix keeps it valid.
	if err := state.ValidateKey(stateKeyPrefix + "con"); err != nil {
		t.Fatalf("key for rule \"con\" rejected: %v", err)
	}
}
