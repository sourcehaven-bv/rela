package appbuild_test

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

const outputsMetamodel = `version: "1.0"
entities:
  incident:
    label: Incident
    plural: incidents
    id_prefix: "INC-"
    id_type: sequential
    properties:
      title: {type: string}
      status: {type: string}
      risk: {type: string}
      code: {type: string, unique: true}
      owner: {type: string}
      secret: {type: string}
      phase: {type: incident-phase}
  tl:
    label: Timeline
    plural: timelines
    id_prefix: "TL-"
    id_type: sequential
    properties:
      title: {type: string, required: true}
types:
  incident-phase:
    values: [open, review, closed]
    initial: open
    transitions:
      - from: open
        to: review
      - from: review
        to: closed
relations:
  logs:
    label: Logs
    from: [tl]
    to: [incident]
`

// alice (dpo) may update incidents but not write their owner, and cannot
// see their secret; bob reads everything and writes nothing; root writes
// everything.
const outputsPolicy = `roles:
  dpo:
    read: ["*"]
    create: [incident, tl]
    update: [incident]
    fields:
      incident:
        - field: title
        - field: status
        - field: risk
        - field: code
        - field: phase
    visible:
      incident:
        - field: title
        - field: status
        - field: risk
        - field: code
        - field: owner
        - field: phase
  reader:
    read: ["*"]
  admin:
    read: ["*"]
    create: [incident, tl]
    update: [incident]
assignments:
  alice: dpo
  bob: reader
  root: admin
`

// outputsRealtime declares a response session whose assess transition
// writes outputs.
func outputsRealtime(outputs string) string {
	return `version: 1
channel_types:
  response:
    attach_to: [incident]
    cardinality: one
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      assess:
        properties:
          risk:  { type: string, required: true }
          code:  { type: string }
          owner: { type: string }
          phase: { type: incident-phase }
        post: [member]
    session:
      initial: new
      states:
        new:
          on:
            assess:
              to: assessed
              outputs:
` + outputs + `
        assessed: { final: true }
`
}

const setAndCreate = `                - set: { properties: { risk: "{{message.risk}}", status: assessed } }
                - create: { type: tl, properties: { title: "Assessed {{message.risk}}" }, relate: { type: logs } }`

var incidentRoom = realtime.RawAddress{EntityType: "incident", Anchor: "INC-001", Type: "response"}

// outputsServices assembles the outputs project with the given outputs
// and creates INC-001 as root, with a secret alice cannot see.
func outputsServices(t *testing.T, outputs string) (svc *appbuild.Services, root string) {
	t.Helper()
	root = t.TempDir()
	writeMetamodelBody(t, root, outputsMetamodel)
	writePolicy(t, root, outputsPolicy)
	require.NoError(t, os.WriteFile(filepath.Join(root, "realtime.yaml"), []byte(outputsRealtime(outputs)), 0o600))
	svc, err := appbuild.Discover(root, script.NewEngine(), backendtest.Options(t)...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	for _, code := range []string{"A1", "B2"} {
		e := entity.New("", "incident")
		e.SetString("title", "Leak "+code)
		e.SetString("code", code)
		e.SetString("secret", "s3cret")
		_, err = svc.EntityManager().CreateEntity(promoteAs(t, svc, "root"), e, entity.CreateOptions{})
		require.NoError(t, err)
	}
	return svc, root
}

func postAssess(t *testing.T, svc *appbuild.Services, user string, props map[string]any) (realtime.PostResult, error) {
	t.Helper()
	return appbuild.ChannelService(svc).Post(promoteAs(t, svc, user), incidentRoom,
		realtime.PostInput{Type: "assess", Props: props})
}

func incident(t *testing.T, svc *appbuild.Services) *entity.Entity {
	t.Helper()
	got, err := svc.Store().GetEntity(context.Background(), entity.Ref{ID: "INC-001"})
	require.NoError(t, err)
	return got
}

// TestOutputWriter_SetAndCreate pins a set and a create through the
// assembled services: the set writes as the poster and keeps the property
// the poster cannot see, the create relates the new entity to the anchor,
// and the graph audit records carry the poster, tool realtime and the
// session origin.
func TestOutputWriter_SetAndCreate(t *testing.T) {
	svc, root := outputsServices(t, setAndCreate)
	res, err := postAssess(t, svc, "alice", map[string]any{"risk": "high"})
	require.NoError(t, err)
	require.NotNil(t, res.Outputs)
	require.Equal(t, realtime.OutputsDone, res.Outputs.Status, "%+v", res.Outputs.Results)
	require.Len(t, res.Outputs.Results, 2)
	created := res.Outputs.Results[1].Entity
	require.NotNil(t, created)
	require.Equal(t, "Assessed high", created.Title)

	inc := incident(t, svc)
	require.Equal(t, "high", inc.Properties["risk"])
	require.Equal(t, "assessed", inc.Properties["status"])
	require.Equal(t, "s3cret", inc.Properties["secret"], "a property alice cannot see is preserved")
	_, err = svc.Store().GetRelation(context.Background(),
		entity.RelationKey{From: created.ID, Type: "logs", To: "INC-001"})
	require.NoError(t, err)

	origin := "session:INC-001/response/new->assessed#1/"
	var ops []string
	for _, rec := range auditRecords(t, root) {
		if strings.HasPrefix(rec.TriggeredBy, origin) {
			require.Equal(t, "alice", rec.Principal.User)
			require.Equal(t, principal.ToolRealtime, rec.Principal.Tool)
			ops = append(ops, rec.Op+" "+strings.TrimPrefix(rec.TriggeredBy, origin))
		}
	}
	require.ElementsMatch(t, []string{
		audit.OpUpdateEntity + " 0", audit.OpCreateEntity + " 1", audit.OpCreateRelation + " 1",
	}, ops)
}

// TestOutputWriter_FailedCodes pins the code of each refusal the
// pre-check cannot see. The move stays, and no result names an id.
func TestOutputWriter_FailedCodes(t *testing.T) {
	cases := []struct {
		name    string
		outputs string
		props   map[string]any
		code    string
	}{
		{"field gate", `                - set: { properties: { owner: "{{message.owner}}" } }`,
			map[string]any{"owner": "mallory"}, realtime.OutputCodeFieldDenied},
		{"field gate on a literal", `                - set: { properties: { owner: mallory } }`,
			nil, realtime.OutputCodeFieldDenied},
		{"unique", `                - set: { properties: { code: "{{message.code}}" } }`,
			map[string]any{"code": "B2"}, realtime.OutputCodeConflict},
		{"illegal transition", `                - set: { properties: { phase: "{{message.phase}}" } }`,
			map[string]any{"phase": "closed"}, realtime.OutputCodeInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := outputsServices(t, tc.outputs)
			props := map[string]any{"risk": "low"}
			maps.Copy(props, tc.props)
			res, err := postAssess(t, svc, "alice", props)
			require.NoError(t, err)
			require.Equal(t, realtime.OutputsFailed, res.Outputs.Status)
			require.Equal(t, tc.code, res.Outputs.Results[0].Code)
			require.Equal(t, "assessed", res.Session.State, "the move stays")
			require.NotContains(t, res.Outputs.Results[0].Code+strings.Join(res.Outputs.Results[0].Props, ","), "INC-")
		})
	}
}

// TestOutputWriter_PreCheckDenied pins that a poster who may not update
// the anchor is refused before anything is stored.
func TestOutputWriter_PreCheckDenied(t *testing.T) {
	svc, _ := outputsServices(t, setAndCreate)
	_, err := postAssess(t, svc, "bob", map[string]any{"risk": "high"})
	d, ok := errors.AsType[*realtime.DenialError](err)
	require.Truef(t, ok, "want a *DenialError, got %v", err)
	require.Equal(t, http.StatusForbidden, d.Status)
	require.Equal(t, realtime.TypeSessionOutputDenied, d.Type)
	require.Contains(t, d.Title, "outputs[0]")
	page, err := appbuild.ChannelService(svc).Read(promoteAs(t, svc, "bob"), incidentRoom, realtime.ReadRequest{})
	require.NoError(t, err)
	require.Empty(t, page.Messages, "nothing was stored")
}

// TestOutputWriter_RetryAsPoster pins that a retry, which has no request,
// writes as the poster rebuilt from the mark, with tool realtime.
func TestOutputWriter_RetryAsPoster(t *testing.T) {
	svc, root := outputsServices(t, setAndCreate)
	w, err := appbuild.OutputsFor(svc, true)
	require.NoError(t, err)
	ctx := principal.With(context.Background(),
		principal.Principal{User: principal.UserRealtime, Tool: principal.ToolRealtime})
	retry := &realtime.OutputIdentity{Writer: realtimeAuthor("alice")}
	_, err = w.Write(ctx, realtime.OutputRequest{
		Target: realtime.OutputTarget{Kind: realtimecfg.OutputSet, AnchorID: "INC-001", AnchorType: "incident"},
		Props:  map[string]any{"status": "retried"}, Origin: "session:retry", Retry: retry,
	})
	require.NoError(t, err)
	require.Equal(t, "retried", incident(t, svc).Properties["status"])
	found := false
	for _, rec := range auditRecords(t, root) {
		if rec.TriggeredBy == "session:retry" {
			found = true
			require.Equal(t, "alice", rec.Principal.User)
			require.Equal(t, principal.ToolRealtime, rec.Principal.Tool)
		}
	}
	require.True(t, found)

	// bob may not update: the retry is refused as him, not as the system.
	_, err = w.Write(ctx, realtime.OutputRequest{
		Target: realtime.OutputTarget{Kind: realtimecfg.OutputSet, AnchorID: "INC-001", AnchorType: "incident"},
		Props:  map[string]any{"status": "bob"}, Origin: "session:retry", Retry: &realtime.OutputIdentity{
			Writer: realtimeAuthor("bob"),
		},
	})
	f, ok := errors.AsType[*realtime.OutputError](err)
	require.Truef(t, ok, "want an *OutputError, got %v", err)
	require.Equal(t, realtime.OutputCodeForbidden, f.Code)
	require.False(t, f.Retryable)
}

// TestOutputs_NoJoinUsesUnconfirmed pins that a writer whose channel store
// does not join the entity store records the start before the create,
// and that a failed start creates nothing.
func TestOutputs_NoJoinUsesUnconfirmed(t *testing.T) {
	svc, _ := outputsServices(t, setAndCreate)
	w, err := appbuild.OutputsFor(svc, false)
	require.NoError(t, err)
	ctx := promoteAs(t, svc, "alice")
	target := realtime.OutputTarget{Kind: realtimecfg.OutputCreate, AnchorID: "INC-001", Type: "tl", Relate: "logs"}
	started := 0
	_, err = w.Write(ctx, realtime.OutputRequest{
		Target: target, Props: map[string]any{"title": "x"}, Origin: "session:t",
		Started: func(context.Context) error { started++; return errors.New("disk on fire") },
	})
	f, ok := errors.AsType[*realtime.OutputError](err)
	require.Truef(t, ok, "want an *OutputError, got %v", err)
	require.True(t, f.WroteNothing)
	require.Equal(t, 1, started)
	require.Zero(t, timelines(t, svc))

	got, err := w.Write(ctx, realtime.OutputRequest{
		Target: target, Props: map[string]any{"title": "x"}, Origin: "session:t",
		Started: func(context.Context) error { started++; return nil },
	})
	require.NoError(t, err)
	require.False(t, got.Marked, "the progress is not written with the create")
	require.Equal(t, 2, started)
	require.Equal(t, 1, timelines(t, svc))
}

// timelines counts the stored tl entities.
func timelines(t *testing.T, svc *appbuild.Services) int {
	t.Helper()
	n := 0
	for _, err := range svc.Store().ListEntities(context.Background(), store.EntityQuery{Type: "tl", Faces: store.AllFaces()}) {
		require.NoError(t, err)
		n++
	}
	return n
}

// realtimeAuthor is a data-entry poster as a mark stores it.
func realtimeAuthor(name string) channels.Author {
	return channels.Author{User: name, Tool: principal.ToolDataEntry}
}
