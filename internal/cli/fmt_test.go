package cli

import (
	"context"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// recordingFormatter is a store that formats nothing and records which rows
// `rela fmt` asked it to format.
type recordingFormatter struct {
	store.Store
	refs []string
}

func (r *recordingFormatter) FormatEntity(_ context.Context, ref entity.Ref, _ bool) (bool, error) {
	r.refs = append(r.refs, ref.String())
	return false, nil
}

func (r *recordingFormatter) FormatRelation(context.Context, entity.RelationKey, bool) (bool, error) {
	return false, nil
}

// `rela fmt` formats every face: each is its own file. It used to format the
// zero face of each id, which a faced type does not store.
func TestFmtCmd_FormatsEveryFace(t *testing.T) {
	meta, err := metamodel.Parse([]byte(facedCLIMetaYAML))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	b, err := newCLIBundles(appbuildtest.New(meta))
	if err != nil {
		t.Fatalf("build cli services: %v", err)
	}
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		{ID: "POL-1", Type: "policy", Face: "draft"},
		{ID: "POL-1", Type: "policy", Face: "published"},
		{ID: "CTL-1", Type: "control"},
	} {
		if cErr := b.read.Store.CreateEntity(ctx, e); cErr != nil {
			t.Fatalf("seed: %v", cErr)
		}
	}
	rec := &recordingFormatter{Store: b.read.Store}
	b.read.Store = rec
	withOutput(t, output.FormatTable)

	if err := (&FmtCmd{Check: true}).Run(ctx, b.read); err != nil {
		t.Fatalf("fmt: %v", err)
	}
	slices.Sort(rec.refs)
	want := []string{"CTL-1", "POL-1@draft", "POL-1@published"}
	if !slices.Equal(rec.refs, want) {
		t.Errorf("formatted %v, want %v", rec.refs, want)
	}
}
