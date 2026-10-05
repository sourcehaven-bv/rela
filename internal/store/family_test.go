package store_test

import (
	"context"
	"errors"
	"iter"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// familyLister yields its rows for any query and records the query, so a
// test sees both what the helper asked for and what it kept.
type familyLister struct {
	rows []*entity.Entity
	err  error
	got  store.EntityQuery
}

func (l *familyLister) ListEntities(_ context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	l.got = q
	return func(yield func(*entity.Entity, error) bool) {
		if l.err != nil {
			yield(nil, l.err)
			return
		}
		for _, e := range l.rows {
			if !yield(e, nil) {
				return
			}
		}
	}
}

func TestFamily_KeepsOnlyTheExactID(t *testing.T) {
	l := &familyLister{rows: []*entity.Entity{
		{ID: "DOC-1", Type: "doc", Face: "draft"},
		{ID: "doc-1", Type: "doc", Face: "draft"}, // case-folded match
		{ID: "DOC-1", Type: "doc", Face: "published"},
	}}
	rows, err := store.Family(context.Background(), l, "DOC-1")
	if err != nil {
		t.Fatalf("Family: %v", err)
	}
	var faces []entity.Face
	for _, e := range rows {
		faces = append(faces, e.Face)
	}
	if !slices.Equal(faces, []entity.Face{"draft", "published"}) {
		t.Errorf("faces = %v, want [draft published]", faces)
	}
	if !l.got.Faces.IsAll() || !slices.Equal(l.got.IDs, []string{"DOC-1"}) {
		t.Errorf("query = %+v, want IDs [DOC-1] over every face", l.got)
	}

	headers, err := store.FamilyHeaders(context.Background(), l, "DOC-1")
	if err != nil {
		t.Fatalf("FamilyHeaders: %v", err)
	}
	if len(headers) != 2 {
		t.Errorf("headers = %d, want 2", len(headers))
	}
}

func TestFamily_ReturnsTheReadError(t *testing.T) {
	boom := errors.New("boom")
	l := &familyLister{err: boom}
	if _, err := store.Family(context.Background(), l, "X"); !errors.Is(err, boom) {
		t.Errorf("Family err = %v, want boom", err)
	}
	if _, err := store.FamilyHeaders(context.Background(), l, "X"); !errors.Is(err, boom) {
		t.Errorf("FamilyHeaders err = %v, want boom", err)
	}
}

func TestFamily_NoRowIsEmptyNotAnError(t *testing.T) {
	rows, err := store.FamilyHeaders(context.Background(), &familyLister{}, "X")
	if err != nil || len(rows) != 0 {
		t.Errorf("FamilyHeaders = %v, %v; want empty, nil", rows, err)
	}
}
