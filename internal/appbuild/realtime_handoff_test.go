package appbuild_test

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// handoffTestdata is the realtimecfg testdata: the schema, acl.yaml and
// valid.yaml, whose breach-response block is the guide's example.
const handoffTestdata = "../realtimecfg/testdata"

// handoffServices assembles a project from the realtimecfg testdata. The
// test policy adds to acl.yaml: handler may also create timeline entries
// (the contained output) and read the user entities a handoff names; clerk
// reads datalek and may not update it; steward may update datalek, but its
// fields: block grants only risk, so it may not write owner. USR-1, USR-2,
// USR-4 and USR-5 are handlers, USR-3 is a clerk, USR-6 a steward. DL-1 is
// owned by USR-1.
func handoffServices(t *testing.T) (svc *appbuild.Services, root string) {
	t.Helper()
	root = t.TempDir()
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(handoffTestdata, name))
		require.NoError(t, err)
		return string(b)
	}
	// The testdata schema predates id_type; a project on disk needs it.
	writeMetamodelBody(t, root, idPrefix.ReplaceAllString(read("schema.yaml"), "$0\n    id_type: sequential"))
	policy := read("acl.yaml")
	policy = strings.Replace(policy, `  handler:
    read: [datalek]
    update: [datalek]
`, `  handler:
    read: [datalek, risico, timeline-entry]
    update: [datalek]
    create: [timeline-entry]
  clerk:
    read: [datalek, risico, timeline-entry]
  steward:
    read: [datalek, risico, timeline-entry]
    update: [datalek]
    fields:
      datalek:
        - field: risk
`, 1)
	policy = strings.Replace(policy, "  USR-1: handler\n",
		"  USR-1: handler\n  USR-2: handler\n  USR-3: clerk\n  USR-4: handler\n  USR-5: handler\n  USR-6: steward\n", 1)
	require.Contains(t, policy, "clerk:")
	writePolicy(t, root, policy)
	require.NoError(t, os.WriteFile(filepath.Join(root, "realtime.yaml"), []byte(read("valid.yaml")), 0o600))
	svc, err := appbuild.Discover(root, script.NewEngine(), backendtest.Options(t)...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	ctx := context.Background()
	for _, id := range []string{"USR-1", "USR-2", "USR-3", "USR-4", "USR-5", "USR-6"} {
		u := entity.New(id, "risico")
		u.SetString("title", id)
		require.NoError(t, svc.Store().CreateEntity(ctx, u))
	}
	dl := entity.New("DL-1", "datalek")
	dl.SetString("title", "Leak")
	dl.SetString("owner", "USR-1")
	require.NoError(t, svc.Store().CreateEntity(ctx, dl))
	return svc, root
}

var idPrefix = regexp.MustCompile(`(?m)^ {4}id_prefix: .*$`)

var breach = realtime.RawAddress{EntityType: "datalek", Anchor: "DL-1", Type: "breach-response"}

func handoffPost(
	ctx context.Context, t *testing.T, svc *appbuild.Services, typ string, props map[string]any,
) realtime.PostResult {
	t.Helper()
	res, err := appbuild.ChannelService(svc).Post(ctx, breach, realtime.PostInput{Type: typ, Props: props})
	require.NoError(t, err)
	return res
}

func dlOwner(t *testing.T, svc *appbuild.Services) any {
	t.Helper()
	got, err := svc.Store().GetEntity(context.Background(), entity.Ref{ID: "DL-1"})
	require.NoError(t, err)
	return got.Properties["owner"]
}

// toHandedOver moves DL-1's session to contained as USR-1, then hands the
// lead to USR-2.
func toHandedOver(t *testing.T, svc *appbuild.Services) realtime.PostResult {
	t.Helper()
	lead := promoteAs(t, svc, "USR-1")
	res := handoffPost(lead, t, svc, "contained", map[string]any{"note": "door locked"})
	require.Equal(t, "contained", res.Session.State)
	require.True(t, res.Session.WaitingOnMe)
	return handoffPost(lead, t, svc, "handoff", map[string]any{"to": "USR-2"})
}

// TestPost_HandoffAnswersNewHolder pins AC 19: the post that hands the
// lead over answers with the new holder from a fresh anchor read.
func TestPost_HandoffAnswersNewHolder(t *testing.T) {
	svc, _ := handoffServices(t)
	res := toHandedOver(t, svc)
	require.Equal(t, realtime.OutputsDone, res.Outputs.Status, "%+v", res.Outputs.Results)
	s := res.Session
	require.Equal(t, "handed-over", s.State)
	require.Equal(t, "input_required", s.Status)
	require.Equal(t, []realtime.SessionWaitingView{
		{Role: "lead", Label: "the incident lead", Holders: []string{"USR-2"}},
	}, s.WaitingFor)
	require.False(t, s.WaitingOnMe, "USR-1 handed the lead to USR-2")
}

// TestSession_HandoffAndClaim pins AC 20 end to end.
func TestSession_HandoffAndClaim(t *testing.T) {
	svc, root := handoffServices(t)
	cs := appbuild.ChannelService(svc)
	toHandedOver(t, svc)
	require.Equal(t, "USR-2", dlOwner(t, svc))
	require.Eventually(t, func() bool {
		for _, rec := range auditRecords(t, root) {
			if rec.Op == audit.OpUpdateEntity && rec.Subject != nil && rec.Subject.ID == "DL-1" &&
				strings.HasPrefix(rec.TriggeredBy, "session:DL-1/breach-response/contained->handed-over") {

				return rec.Principal.User == "USR-1" && rec.Principal.Tool == principal.ToolRealtime
			}
		}
		return false
	}, 5*time.Second, 10*time.Millisecond, "the owner write is audited as USR-1")

	usr2 := promoteAs(t, svc, "USR-2")
	got, err := cs.Session(usr2, breach)
	require.NoError(t, err)
	require.True(t, got.Session.WaitingOnMe)
	require.Contains(t, got.Transitions, "accept-handoff")
	res := handoffPost(usr2, t, svc, "accept-handoff", nil)
	require.Equal(t, "contained", res.Session.State)
	require.True(t, res.Session.WaitingOnMe)

	// Hand over again, then claim.
	handoffPost(usr2, t, svc, "handoff", map[string]any{"to": "USR-1"})
	// A member who may not update the row, and one who may update it but
	// not write owner, are both refused before anything is stored.
	for _, who := range []string{"USR-3", "USR-6"} {
		before, rerr := cs.Read(usr2, breach, realtime.ReadRequest{})
		require.NoError(t, rerr)
		_, perr := cs.Post(promoteAs(t, svc, who), breach, realtime.PostInput{Type: "claim"})
		d, ok := errors.AsType[*realtime.DenialError](perr)
		require.Truef(t, ok, "%s: want a *DenialError, got %v", who, perr)
		require.Equal(t, http.StatusForbidden, d.Status, who)
		require.Equal(t, realtime.TypeSessionOutputDenied, d.Type, who)
		after, rerr := cs.Read(usr2, breach, realtime.ReadRequest{})
		require.NoError(t, rerr)
		require.Equal(t, before.Channel.LastSeq, after.Channel.LastSeq, "%s: nothing was stored", who)
		require.Equal(t, "handed-over", after.Channel.Session.State, who)
		require.Equal(t, "USR-1", dlOwner(t, svc), who)
	}

	res = handoffPost(promoteAs(t, svc, "USR-4"), t, svc, "claim", nil)
	require.Equal(t, "contained", res.Session.State)
	require.Equal(t, "USR-4", dlOwner(t, svc))
	require.True(t, res.Session.WaitingOnMe)

	// An MCP-stamped claim: the raw identifier becomes the owner.
	usr4 := promoteAs(t, svc, "USR-4")
	handoffPost(usr4, t, svc, "handoff", map[string]any{"to": "USR-1"})
	mcp := boundAs(t, svc, principal.Principal{User: "USR-5", Tool: principal.ToolMCP})
	handoffPost(mcp, t, svc, "claim", nil)
	require.Equal(t, "USR-5", dlOwner(t, svc))
	got, err = cs.Session(mcp, breach)
	require.NoError(t, err)
	require.True(t, got.Session.WaitingOnMe)
}
