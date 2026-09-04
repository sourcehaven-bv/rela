package docs

import (
	"context"
	"strings"
	"testing"
)

// edit() applies to the in-memory resolver store, so a later echo island sees
// the changed value — the same one-recorder-both-stores property create() has.
func TestBuild_EditAppliesToResolverStore(t *testing.T) {
	t.Parallel()
	src := "```rela\n" +
		"create(\"risico\", {id=\"R-1\", titel=\"before\", kans=1, impact=1})\n" +
		"edit(\"R-1\", {titel=\"after\"})\n" +
		"entity{id=\"R-1\", fields={\"titel\"}}\n" +
		"```\n"
	out := build(t, src, Options{})
	if !strings.Contains(out, "after") {
		t.Errorf("edit did not reach the resolver store:\n%s", out)
	}
	if strings.Contains(out, "before") {
		t.Errorf("edit left the pre-edit value visible:\n%s", out)
	}
}

// An edit is recorded as a seed op so a screenshot/api temp project replays it.
func TestBuild_EditIsRecordedAsSeedOp(t *testing.T) {
	t.Parallel()
	src := "```rela\n" +
		"create(\"risico\", {id=\"R-1\", titel=\"before\", kans=1, impact=1})\n" +
		"edit(\"R-1\", {titel=\"after\"})\n" +
		"```\n"

	// Build through a capturer stub so the recorded seed is observable.
	var seen []SeedOp
	opts := Options{Meta: fixtureMeta(t), Capturer: seedSpy{onCapture: func(s []SeedOp) { seen = s }}}
	src += "```rela\nscreenshot{view=\"entity\", type=\"risico\", entity=\"R-1\", out=\"x.png\"}\n```\n"
	if _, err := Build(context.Background(), src, opts); err != nil {
		t.Fatalf("Build: %v", err)
	}

	var edits int
	for _, op := range seen {
		if op.Kind == "edit" {
			edits++
			if op.ID != "R-1" || op.Properties["titel"] != "after" {
				t.Errorf("unexpected edit op: %+v", op)
			}
		}
	}
	if edits != 1 {
		t.Errorf("want 1 recorded edit op, got %d (seed: %+v)", edits, seen)
	}
}

// A call that asserts nothing is an error — the house rule the assertion verbs
// already follow. An edit naming no change would still write on some backends
// and not others, so the manual's timeline would differ by backend.
func TestBuild_EditWithNoChangeIsRefused(t *testing.T) {
	t.Parallel()
	src := "```rela\n" +
		"create(\"risico\", {id=\"R-1\", titel=\"x\", kans=1, impact=1})\n" +
		"edit(\"R-1\")\n" +
		"```\n"
	_, err := Build(context.Background(), src, Options{Meta: fixtureMeta(t)})
	if err == nil {
		t.Fatal("want an error for an edit that changes nothing")
	}
	if !strings.Contains(err.Error(), "changes nothing") {
		t.Errorf("error should say the edit changes nothing, got: %v", err)
	}
}

// Editing an entity the manual never created is an author error, not a silent
// no-op: the figure would show an entity that does not exist.
func TestBuild_EditUnknownEntityIsRefused(t *testing.T) {
	t.Parallel()
	_, err := Build(context.Background(),
		"```rela\nedit(\"NOPE-1\", {titel=\"x\"})\n```\n", Options{Meta: fixtureMeta(t)})
	if err == nil {
		t.Fatal("want an error for editing an unseeded entity")
	}
	if !strings.Contains(err.Error(), "no such seeded entity") {
		t.Errorf("error should name the missing entity, got: %v", err)
	}
}

// seedSpy is a Capturer that records the seed it was handed.
type seedSpy struct{ onCapture func([]SeedOp) }

func (s seedSpy) Capture(_ context.Context, spec CaptureSpec) (string, error) {
	s.onCapture(spec.Seed)
	return spec.OutPath, nil
}
func (seedSpy) Close() error { return nil }

// `await_versions` only means something on the history view. Silently ignoring
// it elsewhere would let an author believe a capture waits when it does not.
func TestBuild_AwaitVersionsOnlyOnHistoryView(t *testing.T) {
	t.Parallel()
	src := "```rela\n" +
		"create(\"risico\", {id=\"R-1\", titel=\"x\", kans=1, impact=1})\n" +
		"screenshot{view=\"entity\", type=\"risico\", entity=\"R-1\", await_versions=2, out=\"x.png\"}\n" +
		"```\n"
	_, err := Build(context.Background(), src,
		Options{Meta: fixtureMeta(t), Capturer: seedSpy{onCapture: func([]SeedOp) {}}})
	if err == nil {
		t.Fatal("want an error for await_versions on a non-history view")
	}
	if !strings.Contains(err.Error(), "await_versions") {
		t.Errorf("error should name await_versions, got: %v", err)
	}
}
