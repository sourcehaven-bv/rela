package entitymanager_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

func TestConvertEntityPreservesIdentityLineageAndMapsProperties(t *testing.T) {
	const schema = `version: "1"
entities:
  note:
    label: Note
    id_prefix: "NOTE-"
    id_type: manual
    properties:
      title: {type: string, required: true}
      body: {type: string}
  task:
    label: Task
    id_prefix: "TASK-"
    id_type: sequential
    properties:
      title: {type: string, required: true}
      description: {type: string}
relations:
  relates-to:
    label: Relates to
    from: [note, task]
    to: [note, task]
conversions:
  note-to-task:
    from: note
    to: task
    fields:
      title: "{{source.title}}"
      description: "{{source.body}}"
    relations:
      relates-to: relates-to
  note-to-task-dropping-relation:
    from: note
    to: task
    fields:
      title: "{{source.title}}"
    relations:
      relates-to: drop
`
	meta, parseErr := metamodel.Parse([]byte(schema))
	if parseErr != nil {
		t.Fatalf("parse schema: %v", parseErr)
	}
	transitions, transitionsErr := statemachine.Compile(meta)
	if transitionsErr != nil {
		t.Fatalf("compile transitions: %v", transitionsErr)
	}
	st := memstore.New()
	hooks := &conversionHooks{}
	auditor := audit.NewMemory()
	mgr, err := entitymanager.New(entitymanager.Deps{Store: st, Meta: meta, Templater: nopTemplater{}, Audit: auditor, ACL: acl.NopACL{}, Transitions: transitions, FieldGate: entitymanager.AllowAllFieldGate{}, AliasRewriter: hooks, VersionRecorder: hooks})
	if err != nil {
		t.Fatalf("manager: %v", err)
	}
	source := entity.New("NOTE-1", "note")
	source.SetString("title", "Quick note")
	source.SetString("body", "Keep this detail")
	source.Content = "Original body"
	if _, err := mgr.CreateEntity(context.Background(), source, entity.CreateOptions{ID: "NOTE-1"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	offers := entitymanager.ConversionsForSource(context.Background(), mgr, "note", "NOTE-1")
	if len(offers) != 2 || !reflect.DeepEqual(offers[0].MappedFields, []string{"description", "title"}) || offers[0].ContentMapped {
		t.Fatalf("conversion offer mapping metadata=%+v", offers)
	}
	for _, req := range []entitymanager.ConversionRequest{
		{Definition: "note-to-task", SourceID: "NOTE-1", Properties: map[string]any{"title": "client override"}},
	} {
		if _, err := entitymanager.PreviewConversion(context.Background(), mgr, req); !errors.Is(err, entitymanager.ErrConversionInvalid) {
			t.Fatalf("preview accepted an override of an operator-mapped field: %v", err)
		}
		if _, err := entitymanager.ConvertEntity(context.Background(), mgr, req); !errors.Is(err, entitymanager.ErrConversionInvalid) {
			t.Fatalf("conversion accepted an override of an operator-mapped field: %v", err)
		}
	}
	peer := entity.New("NOTE-2", "note")
	peer.SetString("title", "Peer")
	if err := st.CreateEntity(context.Background(), peer); err != nil {
		t.Fatalf("create relation peer: %v", err)
	}
	if _, err := st.CreateRelation(context.Background(), entity.RelationKey{From: "NOTE-1", Type: "relates-to", To: "NOTE-2"}, &store.RelationData{}); err != nil {
		t.Fatalf("create source relation: %v", err)
	}
	preview, err := entitymanager.PreviewConversion(context.Background(), mgr, entitymanager.ConversionRequest{Definition: "note-to-task", SourceID: "NOTE-1"})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(preview.Required) != 0 {
		t.Fatalf("required fields: %v", preview.Required)
	}
	result, err := entitymanager.ConvertEntity(context.Background(), mgr, entitymanager.ConversionRequest{Definition: "note-to-task", SourceID: "NOTE-1"})
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if result.OldID != "NOTE-1" || result.Entity.ID != "TASK-001" || result.Entity.Type != "task" {
		t.Fatalf("conversion identity = old %q, entity %#v", result.OldID, result.Entity)
	}
	if got := result.Entity.GetString("title"); got != "Quick note" {
		t.Errorf("title=%q", got)
	}
	if got := result.Entity.GetString("description"); got != "Keep this detail" {
		t.Errorf("description=%q", got)
	}
	if result.Entity.Content != "Original body" {
		t.Errorf("content=%q", result.Entity.Content)
	}
	if _, err := st.GetEntity(context.Background(), entity.Ref{ID: "NOTE-1"}); err == nil {
		t.Fatal("old id unexpectedly resolves directly in store")
	}
	if _, err := st.GetRelation(context.Background(), entity.RelationKey{From: "TASK-001", Type: "relates-to", To: "NOTE-2"}); err != nil {
		t.Fatalf("relation not preserved through conversion: %v", err)
	}
	if !reflect.DeepEqual(hooks.renamed, [][2]string{{"NOTE-1", "TASK-001"}}) {
		t.Fatalf("alias rewrites=%v", hooks.renamed)
	}
	if len(hooks.versions) != 1 || hooks.versions[0].Op != store.VersionOpRename || hooks.versions[0].PrevID != "NOTE-1" || hooks.versions[0].Type != "task" {
		t.Fatalf("conversion history=%+v", hooks.versions)
	}
	var conversionAudit *audit.Record
	for _, record := range auditor.Records() {
		if record.Op == audit.OpConvertEntity {
			copiedRecord := record
			conversionAudit = &copiedRecord
		}
	}
	if conversionAudit == nil || conversionAudit.Before.Type != "note" || conversionAudit.After.Type != "task" {
		t.Fatalf("conversion audit=%+v", conversionAudit)
	}
	second := entity.New("NOTE-3", "note")
	second.SetString("title", "Another note")
	if _, err := mgr.CreateEntity(context.Background(), second, entity.CreateOptions{ID: "NOTE-3"}); err != nil {
		t.Fatalf("create second note: %v", err)
	}
	if _, err := st.CreateRelation(context.Background(), entity.RelationKey{From: "NOTE-3", Type: "relates-to", To: "NOTE-2"}, &store.RelationData{}); err != nil {
		t.Fatalf("create second relation: %v", err)
	}
	preview, err = entitymanager.PreviewConversion(context.Background(), mgr, entitymanager.ConversionRequest{Definition: "note-to-task-dropping-relation", SourceID: "NOTE-3"})
	if err != nil || !reflect.DeepEqual(preview.RelationsToDrop, []string{"relates-to"}) {
		t.Fatalf("drop preview=%+v err=%v", preview, err)
	}
	if _, err = entitymanager.ConvertEntity(context.Background(), mgr, entitymanager.ConversionRequest{Definition: "note-to-task-dropping-relation", SourceID: "NOTE-3"}); !errors.Is(err, entitymanager.ErrConversionInvalid) {
		t.Fatalf("unconfirmed relation drop error=%v", err)
	}
	dropped, err := entitymanager.ConvertEntity(context.Background(), mgr, entitymanager.ConversionRequest{Definition: "note-to-task-dropping-relation", SourceID: "NOTE-3", DropRelations: []string{"relates-to"}})
	if err != nil {
		t.Fatalf("confirmed drop convert: %v", err)
	}
	if dropped.RelationsDropped != 1 {
		t.Fatalf("dropped relation count=%d", dropped.RelationsDropped)
	}
	if _, err := st.GetRelation(context.Background(), entity.RelationKey{From: dropped.Entity.ID, Type: "relates-to", To: "NOTE-2"}); err == nil {
		t.Fatal("dropped relation still exists")
	}
}

func TestConvertEntityMapsFaceFamilyAndRelations(t *testing.T) {
	const schema = `version: "1"
entities:
  note:
    label: Note
    id_prefix: "NOTE-"
    id_type: manual
    faces: {draft: {label: Draft}, final: {label: Final}}
    properties: {title: {type: string, required: true}}
  task:
    label: Task
    id_prefix: "TASK-"
    id_type: manual
    faces: {active: {label: Active}, done: {label: Done}}
    properties: {title: {type: string, required: true}}
  peer:
    label: Peer
    id_prefix: "PEER-"
    id_type: manual
    properties: {title: {type: string}}
relations:
  relates-to:
    label: Relates to
    from: [note, task]
    to: [peer]
conversions:
  note-to-task:
    from: note
    to: task
    fields: {title: "{{source.title}}"}
    relations: {relates-to: relates-to}
    faces:
      draft: [active, done]
      final: [active]
`
	meta, parseErr := metamodel.Parse([]byte(schema))
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	transitions, transitionsErr := statemachine.Compile(meta)
	if transitionsErr != nil {
		t.Fatal(transitionsErr)
	}
	st := memstore.New()
	hooks := &conversionHooks{}
	mgr, err := entitymanager.New(entitymanager.Deps{Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.NewMemory(), ACL: acl.NopACL{}, Transitions: transitions, FieldGate: entitymanager.AllowAllFieldGate{}, AliasRewriter: hooks, VersionRecorder: hooks})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		face  entity.Face
		title string
	}{{"draft", "Working"}, {"final", "Approved"}} {
		e := entity.New("NOTE-FACE", "note")
		e.SetString("title", row.title)
		if _, err := mgr.CreateEntity(context.Background(), e, entity.CreateOptions{ID: e.ID, Face: row.face}); err != nil {
			t.Fatal(err)
		}
	}
	peer := entity.New("PEER-1", "peer")
	if err := st.CreateEntity(context.Background(), peer); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateRelation(context.Background(), entity.RelationKey{From: "NOTE-FACE", FromFace: "draft", Type: "relates-to", To: "PEER-1"}, &store.RelationData{}); err != nil {
		t.Fatal(err)
	}
	preview, err := entitymanager.PreviewConversion(context.Background(), mgr, entitymanager.ConversionRequest{Definition: "note-to-task", SourceID: "NOTE-FACE"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(preview.RequiredFaces, []string{"draft"}) {
		t.Fatalf("required face choices %v", preview.RequiredFaces)
	}
	req := entitymanager.ConversionRequest{Definition: "note-to-task", SourceID: "NOTE-FACE", SourceFace: "draft", TargetID: "TASK-FACE", FaceMapping: map[string]string{"draft": "done"}}
	result, err := entitymanager.ConvertEntity(context.Background(), mgr, req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Entity.Face != "done" {
		t.Fatalf("result face %q", result.Entity.Face)
	}
	for _, face := range []entity.Face{"done", "active"} {
		if _, err := st.GetEntity(context.Background(), entity.Ref{ID: "TASK-FACE", Face: face}); err != nil {
			t.Fatalf("target face %q missing: %v", face, err)
		}
	}
	if _, err := st.GetRelation(context.Background(), entity.RelationKey{From: "TASK-FACE", FromFace: "done", Type: "relates-to", To: "PEER-1"}); err != nil {
		t.Fatalf("relation did not follow face: %v", err)
	}
	if len(hooks.versions) != 2 {
		t.Fatalf("conversion versions: %+v", hooks.versions)
	}
	seenPrev := map[entity.Face]bool{}
	for _, version := range hooks.versions {
		if version.PrevID != "NOTE-FACE" {
			t.Fatalf("missing id predecessor: %+v", version)
		}
		seenPrev[version.PrevFace] = true
	}
	if !seenPrev["draft"] || !seenPrev["final"] {
		t.Fatalf("cross-face predecessors: %v", seenPrev)
	}
	if len(hooks.faceMoves) != 2 {
		t.Fatalf("comment face moves: %v", hooks.faceMoves)
	}
}

type conversionHooks struct {
	renamed   [][2]string
	versions  []entitymanager.VersionRecord
	faceMoves [][2]entity.Face
}

func (h *conversionHooks) EntityRenamed(_ context.Context, oldID, newID string) error {
	h.renamed = append(h.renamed, [2]string{oldID, newID})
	return nil
}
func (*conversionHooks) EntityDeleted(context.Context, string) error                  { return nil }
func (*conversionHooks) EntityFaceDeleted(context.Context, string, entity.Face) error { return nil }
func (h *conversionHooks) EntityFaceMoved(_ context.Context, _, _ string, from, to entity.Face) error {
	h.faceMoves = append(h.faceMoves, [2]entity.Face{from, to})
	return nil
}
func (h *conversionHooks) RecordVersion(_ context.Context, v entitymanager.VersionRecord) error {
	h.versions = append(h.versions, v)
	return nil
}
