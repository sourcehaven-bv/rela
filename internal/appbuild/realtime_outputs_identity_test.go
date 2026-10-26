package appbuild_test

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// identityServices is outputsServices with a user entity type and
// `unmatched_principal: reject`. carol is the person CAROL (sub carol)
// and a dpo; alice matches no person but keeps her dpo assignment.
func identityServices(t *testing.T) *appbuild.Services {
	t.Helper()
	root := t.TempDir()
	meta := strings.Replace(outputsMetamodel, "types:\n", `  person:
    label: Person
    plural: people
    id_type: manual
    properties:
      sub: {type: string, unique: true}
types:
`, 1)
	writeMetamodelBody(t, root, meta)
	writePolicy(t, root, "user_entity_type: person\nprincipal_property: sub\nunmatched_principal: reject\n"+
		strings.Replace(outputsPolicy, "  root: admin\n", "  root: admin\n  CAROL: dpo\n", 1))
	require.NoError(t, os.WriteFile(filepath.Join(root, "realtime.yaml"),
		[]byte(outputsRealtime(`                - set: { properties: { status: assessed } }`)), 0o600))
	svc, err := appbuild.Discover(root, script.NewEngine(), backendtest.Options(t)...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	rootCtx := promoteAs(t, svc, "root")
	inc := entity.New("", "incident")
	inc.SetString("title", "Leak")
	_, err = svc.EntityManager().CreateEntity(rootCtx, inc, entity.CreateOptions{})
	require.NoError(t, err)
	carol := entity.New("CAROL", "person")
	carol.SetString("sub", "carol")
	require.NoError(t, svc.Store().CreateEntity(context.Background(), carol))
	return svc
}

// verifiedAs binds a verified assertion for sub, as the data-entry edge
// does for a JWT.
func verifiedAs(t *testing.T, svc *appbuild.Services, sub string) context.Context {
	t.Helper()
	return acltest.BindVerified(principal.With(context.Background(),
		principal.Verified(sub, principal.ToolDataEntry, "", "", nil)), t, svc.ACLDeclarative())
}

func incidentSet(retry *realtime.OutputIdentity) realtime.OutputRequest {
	return realtime.OutputRequest{
		Target: realtime.OutputTarget{Kind: realtimecfg.OutputSet, AnchorID: "INC-001", AnchorType: "incident"},
		Props:  map[string]any{"status": "written"}, Origin: "session:test", Retry: retry,
	}
}

func requireForbidden(t *testing.T, err error) {
	t.Helper()
	f, ok := errors.AsType[*realtime.OutputError](err)
	require.Truef(t, ok, "want an *OutputError, got %v", err)
	require.Equal(t, realtime.OutputCodeForbidden, f.Code)
	require.True(t, f.WroteNothing)
}

// TestOutputWriter_UnmatchedRejected pins that a verified poster who
// matches no user entity, under `unmatched_principal: reject`, has no
// outputs written: the post is refused at the pre-check, an inline write
// is refused, and a retry from a mark that records the poster as
// unmatched is refused too.
func TestOutputWriter_UnmatchedRejected(t *testing.T) {
	svc := identityServices(t)
	alice := verifiedAs(t, svc, "alice")

	w, err := appbuild.OutputsFor(svc, true)
	require.NoError(t, err)
	poster, err := w.Poster(alice)
	require.NoError(t, err)
	require.Equal(t, realtime.OutputPoster{Unmatched: true}, poster)

	_, err = appbuild.ChannelService(svc).Post(alice, incidentRoom,
		realtime.PostInput{Type: "assess", Props: map[string]any{"risk": "high"}})
	d, ok := errors.AsType[*realtime.DenialError](err)
	require.Truef(t, ok, "want a *DenialError, got %v", err)
	require.Equal(t, http.StatusForbidden, d.Status)
	require.Equal(t, realtime.TypeSessionOutputDenied, d.Type)

	_, err = w.Write(alice, incidentSet(nil))
	requireForbidden(t, err)

	system := principal.With(context.Background(),
		principal.Principal{User: principal.UserRealtime, Tool: principal.ToolRealtime})
	_, err = w.Write(system, incidentSet(&realtime.OutputIdentity{
		Writer: realtimeAuthor("alice"), OutputPoster: poster,
	}))
	requireForbidden(t, err)
	require.Empty(t, incident(t, svc).Properties["status"], "no output was written")
}

// TestOutputWriter_RetryRemovedUser pins that a retry as a poster whose
// user entity was removed since the post is refused, not written under a
// name that no longer resolves.
func TestOutputWriter_RetryRemovedUser(t *testing.T) {
	svc := identityServices(t)
	carol := verifiedAs(t, svc, "carol")
	w, err := appbuild.OutputsFor(svc, true)
	require.NoError(t, err)
	poster, err := w.Poster(carol)
	require.NoError(t, err)
	require.Equal(t, realtime.OutputPoster{UserEntity: true}, poster)

	system := principal.With(context.Background(),
		principal.Principal{User: principal.UserRealtime, Tool: principal.ToolRealtime})
	retry := &realtime.OutputIdentity{Writer: realtimeAuthor("CAROL"), OutputPoster: poster}
	_, err = w.Write(system, incidentSet(retry))
	require.NoError(t, err, "the poster still resolves")
	require.Equal(t, "written", incident(t, svc).Properties["status"])

	_, err = svc.Store().DeleteFamily(context.Background(), "CAROL", true)
	require.NoError(t, err)
	_, err = w.Write(system, incidentSet(retry))
	requireForbidden(t, err)
}
