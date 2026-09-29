package mcp

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const facedDeleteMetaYAML = `version: "1.0"
entities:
  policy:
    label: Policy
    id_prefix: "POL-"
    faces:
      draft: {label: Draft}
      published: {label: Published}
    properties:
      title: {type: string}
  control:
    label: Control
    id_prefix: "CTL-"
    properties:
      title: {type: string}
relations:
  implements:
    from: [policy]
    to: [control]
    scope: content
`

// facedDeleteServer seeds POL-1 at draft and published, CTL-1, and an
// implements edge from each face to CTL-1.
func facedDeleteServer(t *testing.T) (*Server, *memstore.MemStore) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(facedDeleteMetaYAML))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	st := memstore.New()
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		{ID: "POL-1", Type: "policy", Face: "draft", Properties: map[string]any{"title": "d"}},
		{ID: "POL-1", Type: "policy", Face: "published", Properties: map[string]any{"title": "p"}},
		{ID: "CTL-1", Type: "control", Properties: map[string]any{"title": "c"}},
	} {
		seedEntity(t, st, e)
	}
	for _, face := range []entity.Face{"draft", "published"} {
		if _, err := st.CreateRelation(ctx, "POL-1", "implements", "CTL-1",
			&store.RelationData{FromFace: face}); err != nil {
			t.Fatalf("seed edge: %v", err)
		}
	}
	srv := &Server{logger: slog.New(slog.DiscardHandler)}
	setDeps(srv, newTestDeps(t, meta, st))
	return srv, st
}

func storedFaces(t *testing.T, st store.Store, id string) map[entity.Face]bool {
	t.Helper()
	faces := map[entity.Face]bool{}
	q := store.EntityQuery{IDs: []string{id}, Faces: store.AllFaces()}
	for e, err := range st.ListEntities(context.Background(), q) {
		if err != nil {
			t.Fatalf("ListEntities: %v", err)
		}
		faces[e.Face] = true
	}
	return faces
}

// delete_entity takes `ID@face` for one face and a bare id for the family;
// both used to answer "not found" on a faced type (BUG-J3PBFN).
func TestHandleDeleteEntity_FacedEntity(t *testing.T) {
	s, st := facedDeleteServer(t)
	ctx := context.Background()

	// cascade guards the family delete only.
	res, err := s.handleDeleteEntity(ctx, makeToolRequest(map[string]any{"id": "POL-1"}))
	if err != nil || !isErrorResult(res) || !strings.Contains(getResultText(t, res), "has 2 relation(s)") {
		t.Fatalf("family delete without cascade = %v, %v; want the two-edge refusal", res, err)
	}

	// A face's tailed edges are its content, so they go without cascade.
	res, err = s.handleDeleteEntity(ctx, makeToolRequest(map[string]any{"id": "POL-1@draft"}))
	if err != nil || isErrorResult(res) {
		t.Fatalf("delete POL-1@draft: %v %s", err, getResultText(t, res))
	}
	if got := getResultText(t, res); got != "Deleted POL-1@draft and 1 relation(s)" {
		t.Errorf("result = %q, want the face and its one edge", got)
	}
	if got := storedFaces(t, st, "POL-1"); len(got) != 1 || !got["published"] {
		t.Fatalf("faces after the face delete = %v, want [published]", got)
	}
	if n, countErr := st.CountRelations(ctx, store.RelationQuery{From: "POL-1"}); countErr != nil || n != 1 {
		t.Fatalf("edges after the face delete = %d (%v), want the published one", n, countErr)
	}

	res, err = s.handleDeleteEntity(ctx, makeToolRequest(map[string]any{"id": "POL-1", "cascade": true}))
	if err != nil || isErrorResult(res) {
		t.Fatalf("delete POL-1: %v %s", err, getResultText(t, res))
	}
	if got := storedFaces(t, st, "POL-1"); len(got) != 0 {
		t.Fatalf("faces after the family delete = %v, want none", got)
	}
}

// delete_relation removes the edge on the named tail only.
func TestHandleDeleteRelation_FaceTail(t *testing.T) {
	s, st := facedDeleteServer(t)
	ctx := context.Background()

	res, err := s.handleDeleteRelation(ctx, makeToolRequest(map[string]any{
		"from": "POL-1@published", "type": "implements", "to": "CTL-1",
	}))
	if err != nil || isErrorResult(res) {
		t.Fatalf("delete relation: %v %s", err, getResultText(t, res))
	}
	for face, want := range map[entity.Face]int{"draft": 1, "published": 0} {
		n, countErr := st.CountRelations(ctx, store.RelationQuery{From: "POL-1", FromFace: &face})
		if countErr != nil || n != want {
			t.Errorf("edges on %s = %d (%v), want %d", face, n, countErr, want)
		}
	}

	// The bare id names the identity tail, which holds no edge here.
	res, err = s.handleDeleteRelation(ctx, makeToolRequest(map[string]any{
		"from": "POL-1", "type": "implements", "to": "CTL-1",
	}))
	if err != nil || !isErrorResult(res) {
		t.Errorf("identity-tail delete = %v, %v; want relation not found", res, err)
	}
}

// Deleting the last face removes the entity, so cascade guards it like a
// family delete, and the count covers every incident edge (RR-2466U1).
func TestHandleDeleteEntity_LastFaceNeedsCascade(t *testing.T) {
	s, st := facedDeleteServer(t)
	ctx := context.Background()
	if _, err := st.DeleteFace(ctx, entity.Ref{ID: "POL-1", Face: "published"}); err != nil {
		t.Fatalf("delete published: %v", err)
	}

	res, err := s.handleDeleteEntity(ctx, makeToolRequest(map[string]any{"id": "POL-1@draft"}))
	if err != nil || !isErrorResult(res) || !strings.Contains(getResultText(t, res), "has 1 relation(s)") {
		t.Fatalf("last-face delete without cascade = %v, %v; want the refusal", res, err)
	}
	if got := storedFaces(t, st, "POL-1"); !got["draft"] {
		t.Fatalf("POL-1@draft must survive the refusal, faces = %v", got)
	}

	res, err = s.handleDeleteEntity(ctx, makeToolRequest(map[string]any{"id": "POL-1@draft", "cascade": true}))
	if err != nil || isErrorResult(res) {
		t.Fatalf("delete POL-1@draft with cascade: %v %s", err, getResultText(t, res))
	}
	if got := getResultText(t, res); got != "Deleted POL-1@draft and 1 relation(s)" {
		t.Errorf("result = %q", got)
	}
}
