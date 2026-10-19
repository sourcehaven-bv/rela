package appbuild_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
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
  minute:
    label: Minute
    plural: minutes
    id_prefix: "MIN-"
    id_type: sequential
    properties:
      title: {type: string, required: true}
      status: {type: string}
      code: {type: string, unique: true}
relations:
  about:
    label: About
    from: [minute]
    to: [doc]
`

// alice may create minutes but write only their title and code; bob may
// read everything and create nothing.
const promotePolicy = `roles:
  writer:
    read: ["*"]
    create: [doc, minute]
    fields:
      minute:
        - field: title
        - field: code
  reader:
    read: ["*"]
assignments:
  alice: writer
  bob: reader
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
        post: [member]
        actions:
          accept:
            roles: [member]
            promote_to: { type: minute, map: { title: summary, status: status, code: code }, relate: { type: about } }
`

// promoteServices assembles the promote project on the build's backend
// and creates the anchor doc as alice.
func promoteServices(t *testing.T) (*appbuild.Services, string) {
	t.Helper()
	root := t.TempDir()
	writeMetamodelBody(t, root, promoteMetamodel)
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
