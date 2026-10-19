package appbuild_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

const promoteMetamodel = `version: "1.0"
entities:
  doc:
    label: Doc
    plural: docs
    id_prefix: "DOC-"
    id_type: sequential
    properties:
      title: {type: string}
  secret:
    label: Secret
    plural: secrets
    id_prefix: "SEC-"
    id_type: sequential
    properties:
      title: {type: string}
  minute:
    label: Minute
    plural: minutes
    id_prefix: "MIN-"
    id_type: sequential
    properties:
      title: {type: string, required: true}
      status: {type: string}
      code: {type: string, unique: true}
      source: {type: string}
      phase: {type: minute-phase}
  note:
    label: Note
    plural: notes
    id_prefix: "NOTE-"
    id_type: sequential
    faces:
      draft: {}
      published: {}
    properties:
      title: {type: string}
types:
  minute-phase:
    values: [open, closed]
    initial: open
    transitions:
      - from: open
        to: closed
relations:
  about:
    label: About
    from: [minute]
    to: [doc]
automations:
  - name: dup-code
    on: {entity: minute, created: true, condition: "entity.title == 'dup'"}
    do:
      - set: code
        value: "X1"
`

// alice may create minutes but write only some of their fields; bob may
// read everything and create nothing; carol may create minutes but cannot
// read secrets.
const promotePolicy = `roles:
  writer:
    read: ["*"]
    create: [doc, secret, minute, "note@draft"]
    fields:
      minute:
        - field: title
        - field: code
        - field: source
        - field: phase
  reader:
    read: ["*"]
  limited:
    read: [doc, minute]
    create: [minute]
    fields:
      minute:
        - field: title
        - field: source
assignments:
  alice: writer
  bob: reader
  carol: limited
`

const promoteRealtime = `version: 1
channel_types:
  room:
    attach_to: [doc]
    roles:
      member: { from: attached }
    message_types:
      proposal:
        properties:
          summary: { type: string, required: true }
          status: { type: string }
          code: { type: string }
          source: { type: ref, to: [doc, secret] }
          phase: { type: minute-phase }
        post: [member]
        actions:
          accept:
            roles: [member]
            promote_to:
              type: minute
              map: { title: summary, status: status, code: code, source: source, phase: phase }
              relate: { type: about }
          draft:
            roles: [member]
            promote_to: { type: note, face: draft, map: { title: summary } }
`

// promoteServices assembles the promote project on the build's backend
// and creates the anchor doc as alice.
func promoteServices(t *testing.T) (svc *appbuild.Services, root string) {
	t.Helper()
	return promoteServicesWith(t, promoteMetamodel)
}

// promoteServicesWith is promoteServices over the metamodel meta.
func promoteServicesWith(t *testing.T, meta string) (svc *appbuild.Services, root string) {
	t.Helper()
	root = t.TempDir()
	writeMetamodelBody(t, root, meta)
	writePolicy(t, root, promotePolicy)
	require.NoError(t, os.WriteFile(filepath.Join(root, "realtime.yaml"), []byte(promoteRealtime), 0o600))
	svc, err := appbuild.Discover(root, script.NewEngine(), backendtest.Options(t)...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	e := entity.New("", "doc")
	e.SetString("title", "One")
	res, err := svc.EntityManager().CreateEntity(promoteAs(t, svc, "alice"), e, entity.CreateOptions{})
	require.NoError(t, err)
	require.Equal(t, promoteRoom.Anchor, res.Entity.ID)
	return svc, root
}

func promoteAs(t *testing.T, svc *appbuild.Services, name string) context.Context {
	t.Helper()
	return boundAs(t, svc, principal.Principal{User: name, Tool: principal.ToolDataEntry})
}

var promoteRoom = realtime.RawAddress{EntityType: "doc", Anchor: "DOC-001", Type: "room"}

// postProposal posts a proposal as alice and returns its seq.
func postProposal(t *testing.T, svc *appbuild.Services, props map[string]any) int64 {
	t.Helper()
	res, err := appbuild.ChannelService(svc).Post(promoteAs(t, svc, "alice"), promoteRoom,
		realtime.PostInput{Type: "proposal", Props: props})
	require.NoError(t, err)
	return res.Message.Seq
}

// actionStatus reads the action status of message seq as alice.
func actionStatus(t *testing.T, svc *appbuild.Services, seq int64) channels.ActionStatus {
	t.Helper()
	page, err := appbuild.ChannelService(svc).Messages(promoteAs(t, svc, "alice"), promoteRoom, channels.Page{})
	require.NoError(t, err)
	for _, m := range page.Messages {
		if m.Seq == seq {
			if m.Action == nil {
				return channels.ActionNone
			}
			return m.Action.Status
		}
	}
	t.Fatalf("message %d not found", seq)
	return channels.ActionNone
}

// minutes counts the stored minute entities.
func minutes(t *testing.T, svc *appbuild.Services) int {
	t.Helper()
	n := 0
	for _, err := range svc.Store().ListEntities(context.Background(), store.EntityQuery{Type: "minute", Faces: store.AllFaces()}) {
		require.NoError(t, err)
		n++
	}
	return n
}

// auditRecords reads the audit log of the project at root.
func auditRecords(t *testing.T, root string) []audit.Record {
	t.Helper()
	var out []audit.Record
	files, err := filepath.Glob(filepath.Join(root, ".rela", "audit", "*"))
	require.NoError(t, err)
	for _, name := range files {
		f, err := os.Open(name)
		require.NoError(t, err)
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var rec audit.Record
			require.NoError(t, json.Unmarshal(sc.Bytes(), &rec))
			out = append(out, rec)
		}
		require.NoError(t, f.Close())
	}
	return out
}

// TestPromote_RealWiring pins a promote through the assembled services: the
// entity is created as the caller with its relation to the anchor, the
// action is done and names the entity, and the graph audit records carry
// the channel origin.
func TestPromote_RealWiring(t *testing.T) {
	svc, root := promoteServices(t)
	cs := appbuild.ChannelService(svc)
	seq := postProposal(t, svc, map[string]any{"summary": "Ship it"})

	res, err := cs.RunAction(promoteAs(t, svc, "alice"), promoteRoom, seq, "accept")
	require.NoError(t, err)
	require.Empty(t, res.Warnings)
	require.NotNil(t, res.Message.Action)
	require.Equal(t, channels.ActionDone, res.Message.Action.Status)
	promoted := res.Message.Action.Promoted
	require.NotNil(t, promoted)
	require.Equal(t, "minute", promoted.Type)
	require.Equal(t, "Ship it", promoted.Title)

	got, err := svc.Store().GetEntity(context.Background(), entity.Ref{ID: promoted.ID})
	require.NoError(t, err)
	require.Equal(t, "Ship it", got.Properties["title"])
	_, err = svc.Store().GetRelation(context.Background(),
		entity.RelationKey{From: promoted.ID, Type: "about", To: promoteRoom.Anchor})
	require.NoError(t, err)

	origin := "channel:" + promoteRoom.Anchor + "/room#1/accept"
	var ops []string
	for _, rec := range auditRecords(t, root) {
		if rec.TriggeredBy == origin {
			require.Equal(t, "alice", rec.Principal.User)
			ops = append(ops, rec.Op)
		}
	}
	require.ElementsMatch(t, []string{audit.OpCreateEntity, audit.OpCreateRelation}, ops)
}

// TestPromote_ErrorClasses pins how each refusal of the entity create
// answers, and that the action is released because nothing was written.
func TestPromote_ErrorClasses(t *testing.T) {
	svc, _ := promoteServices(t)
	cs := appbuild.ChannelService(svc)

	first := postProposal(t, svc, map[string]any{"summary": "a", "code": "X1"})
	_, err := cs.RunAction(promoteAs(t, svc, "alice"), promoteRoom, first, "accept")
	require.NoError(t, err)

	cases := []struct {
		name   string
		user   string
		props  map[string]any
		status int
		typ    string
	}{
		{"create ACL", "bob", map[string]any{"summary": "b"}, http.StatusForbidden, realtime.TypePromoteDenied},
		{"field grant", "alice", map[string]any{"summary": "c", "status": "open"},
			http.StatusForbidden, realtime.TypePromoteDenied},
		{"unique", "alice", map[string]any{"summary": "d", "code": "X1"},
			http.StatusUnprocessableEntity, realtime.TypePromoteInvalid},
		{"illegal entry", "alice", map[string]any{"summary": "e", "phase": "closed"},
			http.StatusUnprocessableEntity, realtime.TypePromoteInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := minutes(t, svc)
			seq := postProposal(t, svc, tc.props)
			_, err := cs.RunAction(promoteAs(t, svc, tc.user), promoteRoom, seq, "accept")
			d, ok := errors.AsType[*realtime.DenialError](err)
			require.Truef(t, ok, "want a *DenialError, got %v", err)
			require.Equal(t, tc.status, d.Status, d.Error())
			require.Equal(t, tc.typ, d.Type)
			require.NotContains(t, d.Error(), "MIN-", "a refusal names no entity id")
			require.Equal(t, before, minutes(t, svc))
			require.Equal(t, channels.ActionNone, actionStatus(t, svc, seq), "the action is released")
		})
	}
}

// wantAtomicPromote is set by the database builds; see
// realtime_promote_db_test.go.
var wantAtomicPromote bool

// TestPromote_MarkFailureRollsBack pins D1: on the database builds a mark
// that fails inside the transaction undoes the entity and its relation, and
// the error says nothing was written. On the other builds the promoter
// leaves the mark to the service and the entity stays.
func TestPromote_MarkFailureRollsBack(t *testing.T) {
	svc, _ := promoteServices(t)
	p, atomic, err := appbuild.PromoterFor(svc)
	require.NoError(t, err)
	require.Equal(t, wantAtomicPromote, atomic)

	key := channels.Key{Anchor: channels.Anchor{ID: promoteRoom.Anchor}, Type: "room"}
	res, err := p.Promote(promoteAs(t, svc, "alice"), realtime.PromoteRequest{
		Type: "minute", Props: map[string]any{"title": "orphan"}, Relate: "about",
		Anchor: key.Anchor, Origin: "channel:test",
		// No message 99 exists, so the mark fails.
		Mark: realtime.ActionMark{Key: key, Seq: 99, Name: "accept", Done: channels.Action{
			Status: channels.ActionDone, By: channels.Author{User: "alice"}, At: time.Now(), PendingAt: time.Now(),
		}},
	})
	if !atomic {
		require.NoError(t, err)
		require.False(t, res.Marked)
		require.Equal(t, 1, minutes(t, svc))
		return
	}
	require.ErrorIs(t, err, realtime.ErrNothingWritten)
	require.ErrorIs(t, err, channels.ErrNotFound)
	require.Zero(t, minutes(t, svc))
	n := 0
	for _, err := range svc.Store().ListRelations(context.Background(), store.RelationQuery{Type: "about"}) {
		require.NoError(t, err)
		n++
	}
	require.Zero(t, n, "the relation is rolled back with the entity")
}

// messageActions returns the _actions of message seq as user reads them.
func messageActions(t *testing.T, svc *appbuild.Services, user string, seq int64) []string {
	t.Helper()
	page, err := appbuild.ChannelService(svc).Messages(promoteAs(t, svc, user), promoteRoom, channels.Page{})
	require.NoError(t, err)
	for _, m := range page.Messages {
		if m.Seq == seq {
			return m.Actions
		}
	}
	t.Fatalf("message %d not found", seq)
	return nil
}

// TestPromote_AffordanceFollowsCreateACL pins that a promoting action is
// offered only to a caller the create ACL lets create its entity.
func TestPromote_AffordanceFollowsCreateACL(t *testing.T) {
	svc, _ := promoteServices(t)
	seq := postProposal(t, svc, map[string]any{"summary": "s"})
	require.Equal(t, []string{"accept", "draft"}, messageActions(t, svc, "alice", seq))
	require.Empty(t, messageActions(t, svc, "bob", seq), "bob may create nothing")
	require.Equal(t, []string{"accept"}, messageActions(t, svc, "carol", seq), "carol may create minutes, not notes")
}

// TestPromote_RefCheckedAsActor pins that a promote does not copy a ref the
// actor cannot read into the new entity, though the poster could read it.
// The refusal names the message property, never the id, and nothing is
// written.
func TestPromote_RefCheckedAsActor(t *testing.T) {
	svc, _ := promoteServices(t)
	sec := entity.New("", "secret")
	sec.SetString("title", "hidden")
	res, err := svc.EntityManager().CreateEntity(promoteAs(t, svc, "alice"), sec, entity.CreateOptions{})
	require.NoError(t, err)
	secretID := res.Entity.ID
	seq := postProposal(t, svc, map[string]any{"summary": "s", "source": secretID})

	_, err = appbuild.ChannelService(svc).RunAction(promoteAs(t, svc, "carol"), promoteRoom, seq, "accept")
	d, ok := errors.AsType[*realtime.DenialError](err)
	require.Truef(t, ok, "want a *DenialError, got %v", err)
	require.Equal(t, http.StatusUnprocessableEntity, d.Status)
	require.Equal(t, realtime.TypePromoteInvalid, d.Type)
	require.Contains(t, d.Title, "source")
	require.NotContains(t, d.Error(), secretID)
	require.Zero(t, minutes(t, svc))
	require.Equal(t, channels.ActionNone, actionStatus(t, svc, seq))

	// alice can read the secret, so her promote copies it.
	_, err = appbuild.ChannelService(svc).RunAction(promoteAs(t, svc, "alice"), promoteRoom, seq, "accept")
	require.NoError(t, err)
}

// TestPromote_FacedTarget pins that a promote_to face reaches the create,
// so the entity is written at that face.
func TestPromote_FacedTarget(t *testing.T) {
	svc, _ := promoteServices(t)
	seq := postProposal(t, svc, map[string]any{"summary": "Draft it"})
	res, err := appbuild.ChannelService(svc).RunAction(promoteAs(t, svc, "alice"), promoteRoom, seq, "draft")
	require.NoError(t, err)
	promoted := res.Message.Action.Promoted
	require.NotNil(t, promoted)
	got, err := svc.Store().GetEntity(context.Background(), entity.Ref{ID: promoted.ID, Face: "draft"})
	require.NoError(t, err)
	require.Equal(t, "Draft it", got.Properties["title"])
}

// TestPromote_FollowUpWarning pins that an automation failing after the
// create keeps the entity and the done action, and warns.
func TestPromote_FollowUpWarning(t *testing.T) {
	svc, _ := promoteServices(t)
	cs := appbuild.ChannelService(svc)
	first := postProposal(t, svc, map[string]any{"summary": "a", "code": "X1"})
	_, err := cs.RunAction(promoteAs(t, svc, "alice"), promoteRoom, first, "accept")
	require.NoError(t, err)

	// The dup-code automation sets code X1, which the first minute holds.
	seq := postProposal(t, svc, map[string]any{"summary": "dup"})
	res, err := cs.RunAction(promoteAs(t, svc, "alice"), promoteRoom, seq, "accept")
	require.NoError(t, err)
	require.Equal(t, channels.ActionDone, res.Message.Action.Status)
	require.Equal(t, []string{realtime.WarnPromoteFollowUpFailed}, warningCodes(res.Warnings))
	require.Equal(t, 2, minutes(t, svc))
}

// TestPromote_RelateFailure pins what a relation refused at write time
// does: the database builds fail the whole promote and release the action;
// the other builds keep the entity, mark the action done and warn.
func TestPromote_RelateFailure(t *testing.T) {
	// At most one minute may be about a doc.
	meta := strings.Replace(promoteMetamodel, "    to: [doc]\n", "    to: [doc]\n    max_incoming: 1\n", 1)
	require.NotEqual(t, promoteMetamodel, meta)
	svc, _ := promoteServicesWith(t, meta)
	cs := appbuild.ChannelService(svc)
	first := postProposal(t, svc, map[string]any{"summary": "a"})
	_, err := cs.RunAction(promoteAs(t, svc, "alice"), promoteRoom, first, "accept")
	require.NoError(t, err)

	seq := postProposal(t, svc, map[string]any{"summary": "b"})
	res, err := cs.RunAction(promoteAs(t, svc, "alice"), promoteRoom, seq, "accept")
	if wantAtomicPromote {
		d, ok := errors.AsType[*realtime.DenialError](err)
		require.Truef(t, ok, "want a *DenialError, got %v", err)
		require.Equal(t, realtime.TypePromoteInvalid, d.Type)
		require.Equal(t, 1, minutes(t, svc))
		require.Equal(t, channels.ActionNone, actionStatus(t, svc, seq))
		return
	}
	require.NoError(t, err)
	require.Equal(t, channels.ActionDone, res.Message.Action.Status)
	require.Equal(t, []string{realtime.WarnPromoteRelateFailed}, warningCodes(res.Warnings))
	require.Equal(t, 2, minutes(t, svc))
}

func warningCodes(ws []entity.Warning) []string {
	out := make([]string, 0, len(ws))
	for _, w := range ws {
		out = append(out, w.Code)
	}
	return out
}
