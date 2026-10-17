//go:build !postgres && !memorybackend && !sqlite

// fsstore-only for the reason fieldredaction_test.go gives: the fixture is
// markdown written to disk.

package appbuild_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lease"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// alice may read and update tickets; bob may only read them; neither may
// read secrets.
const leasePolicy = `roles:
  editor:
    read: [ticket]
    update: [ticket]
  viewer:
    read: [ticket]
  admin:
    read: ["*"]
    create: ["*"]
    update: ["*"]
    delete: ["*"]
    rename: ["*"]
assignments:
  alice: editor
  bob: viewer
  system:provisioner: admin
`

// leaseCtx stamps user and binds its Request through svc's edge binder, as
// every transport that calls the lease service does.
func leaseCtx(t *testing.T, svc *appbuild.Services, user string) context.Context {
	t.Helper()
	return bindAt(principal.With(context.Background(), principal.Principal{User: user, Tool: principal.ToolCLI}), t, svc)
}

// bindAt runs svc's request-edge step on ctx.
func bindAt(ctx context.Context, t *testing.T, svc *appbuild.Services) context.Context {
	t.Helper()
	bound, err := appbuild.MustIdentityBinder(svc)(ctx)
	require.NoError(t, err)
	return bound
}

// The lease service checks access through the real read gate and write ACL:
// a hidden entity is as absent as a missing one, read-only access is a
// permission error that names no holder, and a rename or delete through the
// entity manager moves or drops the lease.
func TestLeases_RealGate(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, `version: "1.0"
entities:
  ticket:
    label: Ticket
    plural: tickets
    id_type: manual
    properties:
      title:
        type: string
  secret:
    label: Secret
    plural: secrets
    id_type: manual
    properties:
      title:
        type: string
relations: {}
`)
	writePolicy(t, root, leasePolicy)
	writeFile(t, filepath.Join(root, "entities", "tickets", "T-1.md"), "---\nid: T-1\ntype: ticket\n---\n")
	writeFile(t, filepath.Join(root, "entities", "secrets", "S-1.md"), "---\nid: S-1\ntype: secret\n---\n")

	svc, err := appbuildOnDisk(t, root)
	require.NoError(t, err)
	defer svc.Close()
	leases := appbuild.Leases(svc)
	require.NotNil(t, leases)

	_, hidden := leases.Claim(leaseCtx(t, svc, "alice"), lease.ClaimRequest{Addr: "S-1"})
	_, missing := leases.Claim(leaseCtx(t, svc, "alice"), lease.ClaimRequest{Addr: "S-404"})
	require.ErrorIs(t, hidden, store.ErrNotFound)
	require.ErrorIs(t, missing, store.ErrNotFound)
	require.Equal(t, missing.Error(), hidden.Error(), "a hidden entity is indistinguishable from a missing one")

	claimed, err := leases.Claim(leaseCtx(t, svc, "alice"), lease.ClaimRequest{Addr: "T-1", Status: "triage"})
	require.NoError(t, err)

	_, err = leases.Claim(leaseCtx(t, svc, "bob"), lease.ClaimRequest{Addr: "T-1"})
	require.ErrorIs(t, err, acl.ErrForbidden, "read without update is a permission error")
	var held *lease.HeldError
	require.NotErrorAs(t, err, &held)
	require.NotContains(t, err.Error(), "alice")
	require.NotContains(t, err.Error(), "triage")

	wake, cancel := leases.Subscribe()
	defer cancel()
	admin := bindAt(principal.With(context.Background(),
		principal.Principal{User: principal.UserProvisioner, Tool: principal.ToolScheduler}), t, svc)
	_, err = svc.EntityManager().RenameEntity(admin, "T-1", "T-2", entity.RenameOptions{})
	require.NoError(t, err)
	requireLeaseWake(t, wake)
	got, err := leases.GetReadable(context.Background(), []string{"T-1", "T-2"})
	require.NoError(t, err)
	require.NotContains(t, got, "T-1")
	require.Equal(t, "alice", got["T-2"].Holder.User, "the lease follows the rename")

	_, err = svc.EntityManager().DeleteEntity(admin, "T-2", false)
	require.NoError(t, err)
	requireLeaseWake(t, wake)
	got, err = leases.GetReadable(context.Background(), []string{"T-2"})
	require.NoError(t, err)
	require.Empty(t, got, "deleting the entity drops its lease")
	_, err = leases.Renew(leaseCtx(t, svc, "alice"), claimed.LeaseID, 0, nil)
	require.ErrorIs(t, err, lease.ErrLost)
}

// A re-assembly builds a fresh lease service over its predecessor's backend
// and hub, so a subscriber of the old service hears the new one's changes.
func TestLeases_ReassemblySharesBackendAndHub(t *testing.T) {
	base := newSharedBase(t)
	st := memstore.New()
	origin, err := base.Assemble(st, search.New(st, search.NewLinearSearch()), nil, nil)
	require.NoError(t, err)
	defer origin.Close()
	successor, err := base.ForReassembly(origin).Assemble(
		origin.Store(), origin.Searcher(), origin.VisibleSearcher(), nil)
	require.NoError(t, err)
	defer successor.CloseAssembly()

	require.NotSame(t, appbuild.Leases(origin), appbuild.Leases(successor), "the service is rebuilt")
	wake, cancel := appbuild.Leases(origin).Subscribe()
	defer cancel()
	require.NoError(t, appbuild.Leases(successor).EntityDeleted(context.Background(), "T-1"))
	select {
	case <-wake:
	default:
		t.Fatal("the predecessor's subscriber was not woken")
	}
}

// requireLeaseWake waits for a wake on ch, failing the test after a deadline
// instead of hanging it.
func requireLeaseWake(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatal("lease subscriber was not woken")
	}
}

// Mine gates through the real read gate. alice's agent token (principal type
// "app") shares her lease identity, but its ceiling hides secrets; a lease
// she claimed on a secret in her browser session is absent from the agent's
// Mine.
func TestLeases_MineFollowsTheCallersCeiling(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, `version: "1.0"
entities:
  ticket:
    label: Ticket
    plural: tickets
    id_type: manual
    properties:
      title:
        type: string
  secret:
    label: Secret
    plural: secrets
    id_type: manual
    properties:
      title:
        type: string
relations: {}
`)
	writePolicy(t, root, `roles:
  editor:
    read: [ticket, secret]
    update: [ticket, secret]
assignments:
  alice: editor
client_baselines:
  agents:
    applies_to: [app]
    deny_read: [secret]
`)
	writeFile(t, filepath.Join(root, "entities", "tickets", "T-1.md"), "---\nid: T-1\ntype: ticket\n---\n")
	writeFile(t, filepath.Join(root, "entities", "secrets", "S-1.md"), "---\nid: S-1\ntype: secret\n---\n")

	svc, err := appbuildOnDisk(t, root)
	require.NoError(t, err)
	defer svc.Close()
	leases := appbuild.Leases(svc)

	browser := bindAt(principal.With(context.Background(),
		principal.Principal{User: "alice", Tool: principal.ToolDataEntry}), t, svc)
	agent := bindAt(principal.With(context.Background(), principal.VerifiedFrom(
		"alice", principal.ToolDataEntry, principal.Claims{PrincipalType: "app"})), t, svc)
	require.Equal(t, lease.Identity(principal.From(browser)), lease.Identity(principal.From(agent)),
		"the session and the token share one lease identity")

	for _, addr := range []string{"T-1", "S-1"} {
		_, err = leases.Claim(browser, lease.ClaimRequest{Addr: addr})
		require.NoError(t, err)
	}
	ids := func(ls []lease.Lease) []string {
		out := make([]string, len(ls))
		for i, l := range ls {
			out[i] = l.ID
		}
		return out
	}
	mine, err := leases.Mine(browser)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"T-1", "S-1"}, ids(mine))
	mine, err = leases.Mine(agent)
	require.NoError(t, err)
	require.Equal(t, []string{"T-1"}, ids(mine), "the agent's ceiling hides the secret's lease")
}
