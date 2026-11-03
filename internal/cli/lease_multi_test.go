//go:build !sqlite

package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/lease"
)

// The tests in this file open two instances on one project. The sqlite
// build refuses a second instance by design (DEC-LFSYNY), so they do not
// run there.

// TestLeaseCLI_TwoProcesses claims one entity from two rela processes at
// once (AC1): exactly one wins, and the loser's error names the winner's
// label. The winner's stderr carries its log lines but not the lease id.
func TestLeaseCLI_TwoProcesses(t *testing.T) {
	root, ids := newLeaseProject(t)
	labels := []string{"agent-a", "agent-b"}
	type result struct {
		stdout, stderr string
		err            error
	}
	results := make([]result, len(labels))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var wg sync.WaitGroup
	for i, label := range labels {
		wg.Go(func() {
			cmd := exec.CommandContext(ctx, os.Args[0],
				"--project", root, "-o", "json", "lease", "claim", ids[0], "--label", label)
			cmd.Env = append(os.Environ(), cliSubprocessEnv+"=1", "USER=lease-tester")
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			results[i] = result{stdout.String(), stderr.String(), err}
		})
	}
	wg.Wait()

	var winners, losers []int
	for i, r := range results {
		if r.err == nil {
			winners = append(winners, i)
			continue
		}
		var exitErr *exec.ExitError
		require.ErrorAs(t, r.err, &exitErr, "process %d: %s", i, r.stderr)
		require.NotZero(t, exitErr.ExitCode())
		losers = append(losers, i)
	}
	require.Len(t, winners, 1, "exactly one claim wins: %+v", results)
	require.Len(t, losers, 1)
	win, lose := results[winners[0]], results[losers[0]]
	require.Contains(t, lose.stderr, lease.HeldPrefix)
	require.Contains(t, lose.stderr, `"label":"`+labels[winners[0]]+`"`)
	c := claimJSONOf(t, win.stdout)
	require.NotContains(t, win.stderr, c.LeaseID)
	require.NotContains(t, lose.stderr, c.LeaseID)
}

// TestLeaseCLI_VisibleToAnotherInstance claims through the CLI over one
// Services and reads the lease through a second instance whose lease watcher
// runs (AC3): the claim is readable there within 2 s, and gone within 2 s of
// `rela lease release`. The HTTP `_lease` attach that serves this to a
// browser is covered in internal/dataentry.
func TestLeaseCLI_VisibleToAnotherInstance(t *testing.T) {
	const within = 2 * time.Second
	root, ids := newLeaseProject(t)
	a := openLeaseServices(t, root)
	b := openLeaseServices(t, root)
	other := appbuild.Leases(b)
	require.NotNil(t, other)
	ctx, stop := context.WithCancel(context.Background())
	done := appbuild.WatchLeases(ctx, b)
	defer func() { stop(); <-done }()

	readable := func() (lease.Lease, bool) {
		got, err := other.GetReadable(leaseCtx(), []string{ids[0]})
		require.NoError(t, err)
		l, ok := got[ids[0]]
		return l, ok
	}

	got, err := runCLI(t, a, "", "-o", "json", "lease", "claim", ids[0], "--label", "agent-a", "--status", "on it")
	require.NoError(t, err)
	c := claimJSONOf(t, got)
	require.Eventually(t, func() bool { _, ok := readable(); return ok }, within, 50*time.Millisecond,
		"the claim through A is readable through B")
	l, _ := readable()
	require.Equal(t, "agent-a", l.Holder.Label)
	require.Equal(t, "on it", l.Status)

	_, err = runCLI(t, a, c.LeaseID, "lease", "release", "--lease-id-stdin")
	require.NoError(t, err)
	require.Eventually(t, func() bool { _, ok := readable(); return !ok }, within, 50*time.Millisecond,
		"the release through A is seen through B")
}
