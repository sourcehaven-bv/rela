package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/lease"
	"github.com/Sourcehaven-BV/rela/internal/lease/kvlease"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// cliSubprocessEnv, when set to 1, makes the test binary run the real CLI
// entry point on its arguments instead of the tests. The lease tests start
// it as a separate process to claim from two processes at once.
const cliSubprocessEnv = "RELA_CLI_TEST_SUBPROCESS"

func TestMain(m *testing.M) {
	if os.Getenv(cliSubprocessEnv) == "1" {
		os.Exit(runKong())
	}
	os.Exit(m.Run())
}

const leaseCLIMetamodel = `version: "1.0"
entities:
  doc:
    label: Doc
    plural: docs
    id_prefix: "DOC-"
    id_type: sequential
    properties:
      title: {type: string}
`

func leaseCtx() context.Context {
	return principal.With(context.Background(), principal.Principal{User: "ops", Tool: principal.ToolCLI})
}

// newLeaseProject writes a filesystem project with n docs and returns its
// root and the doc ids.
func newLeaseProject(t *testing.T, n int) (root string, ids []string) {
	t.Helper()
	root = t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".rela"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "metamodel.yaml"), []byte(leaseCLIMetamodel), 0o600))
	svc, err := appbuild.Discover(root, script.NewEngine())
	require.NoError(t, err)
	defer svc.Close()
	ids = make([]string, 0, n)
	for range n {
		e := entity.New("", "doc")
		e.SetString("title", "a doc")
		created, err := svc.EntityManager().CreateEntity(leaseCtx(), e, entity.CreateOptions{})
		require.NoError(t, err)
		ids = append(ids, created.Entity.ID)
	}
	return root, ids
}

// openLeaseServices opens root as one CLI process would.
func openLeaseServices(t *testing.T, root string) *appbuild.Services {
	t.Helper()
	svc, err := appbuild.Discover(root, script.NewEngine())
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

// runCLI parses args with the real CLI grammar and runs the command over
// the bundles of svc, as runKong does. It returns what the command printed.
func runCLI(t *testing.T, svc *appbuild.Services, stdin string, args ...string) (string, error) {
	t.Helper()
	var cli CLI
	parser, err := kong.New(&cli, kong.Name("rela"), kong.Vars{"version": Version})
	require.NoError(t, err)
	ktx, err := parser.Parse(args)
	if err != nil {
		return "", err
	}
	b, err := newCLIBundles(svc)
	require.NoError(t, err)
	buf := new(bytes.Buffer)
	prevOut, prevStdin := out, leaseStdin
	out = output.NewWithWriter(buf, output.Format(cli.Output))
	leaseStdin = strings.NewReader(stdin)
	t.Cleanup(func() { out, leaseStdin = prevOut, prevStdin })
	ktx.BindTo(leaseCtx(), (*context.Context)(nil))
	ktx.Bind(out, b.read, b.write, b.leases)
	err = ktx.Run()
	return buf.String(), err
}

// captureLogs sends every log line to the returned buffer until the test
// ends.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := new(bytes.Buffer)
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

func claimJSONOf(t *testing.T, got string) claimJSON {
	t.Helper()
	var c claimJSON
	require.NoError(t, json.Unmarshal([]byte(got), &c), got)
	require.Regexp(t, `^[0-9a-f]{32}$`, c.LeaseID)
	return c
}

func listLeases(t *testing.T, svc *appbuild.Services) (rows []ownLeaseJSON, raw string) {
	t.Helper()
	raw, err := runCLI(t, svc, "", "-o", "json", "lease", "list")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal([]byte(raw), &rows), raw)
	return rows, raw
}

func TestLeaseRequiresProject(t *testing.T) {
	require.True(t, requiresProject("lease claim <id>"))
}

// TestLeaseCLI_Loop walks the agent claim loop: claim, list, renew (keeping,
// replacing and clearing the status), a refused second claim, release, and
// the lost renew after it (AC4). The lease id appears only in the claim
// output, never in list output or the logs (AC5).
func TestLeaseCLI_Loop(t *testing.T) {
	root, ids := newLeaseProject(t, 1)
	id := ids[0]
	svc := openLeaseServices(t, root)
	logs := captureLogs(t)

	got, err := runCLI(t, svc, "", "-o", "json", "lease", "claim", id, "--label", "agent-1", "--status", "planning")
	require.NoError(t, err)
	c := claimJSONOf(t, got)
	require.NotZero(t, c.Token)

	rows, listOut := listLeases(t, svc)
	require.Equal(t, []ownLeaseJSON{{
		ID: id, Since: rows[0].Since, ExpiresAt: c.ExpiresAt, Status: "planning", Label: "agent-1",
	}}, rows)
	table, err := runCLI(t, svc, "", "lease", "list")
	require.NoError(t, err)
	require.Contains(t, table, id)
	require.Contains(t, table, "agent-1")

	statusOf := func() string {
		t.Helper()
		rows, _ := listLeases(t, svc)
		require.Len(t, rows, 1)
		return rows[0].Status
	}
	steps := []struct {
		name   string
		stdin  string
		args   []string
		status string
	}{
		{"absent status keeps it", "", []string{"lease", "renew", c.LeaseID}, "planning"},
		{"status replaces it", "", []string{"lease", "renew", c.LeaseID, "--status", "coding"}, "coding"},
		{"lease id from stdin", c.LeaseID + "\n", []string{"lease", "renew", "--lease-id-stdin", "--ttl", "20m"}, "coding"},
		{"empty status clears it", "", []string{"lease", "renew", c.LeaseID, "--status", ""}, ""},
	}
	for _, st := range steps {
		t.Run(st.name, func(t *testing.T) {
			renewed, runErr := runCLI(t, svc, st.stdin, st.args...)
			require.NoError(t, runErr)
			require.Contains(t, renewed, "expires_at: ")
			require.Equal(t, st.status, statusOf())
		})
	}

	_, err = runCLI(t, svc, "", "lease", "claim", id, "--label", "agent-2")
	require.ErrorContains(t, err, leaseHeldPrefix)
	require.ErrorContains(t, err, `"label":"agent-1"`, "a held error names the holder's label")

	got, err = runCLI(t, svc, c.LeaseID, "lease", "release", "--lease-id-stdin")
	require.NoError(t, err)
	require.Equal(t, "released\n", got)

	_, err = runCLI(t, svc, "", "lease", "renew", c.LeaseID)
	require.EqualError(t, err, leaseLostText)
	got, err = runCLI(t, svc, "", "-o", "json", "lease", "release", c.LeaseID)
	require.NoError(t, err, "release succeeds whether or not the lease still exists")
	require.JSONEq(t, `{"released":true}`, got)

	empty, err := runCLI(t, svc, "", "lease", "list")
	require.NoError(t, err)
	require.Equal(t, "You hold no leases.\n", empty)

	require.NotContains(t, listOut, c.LeaseID)
	require.NotContains(t, table, c.LeaseID)
	require.NotContains(t, logs.String(), c.LeaseID)
	require.Contains(t, logs.String(), "lease: claimed", "the logs were captured")
}

// TestLeaseCLI_Errors pins the refusals and their texts.
func TestLeaseCLI_Errors(t *testing.T) {
	root, ids := newLeaseProject(t, 1)
	svc := openLeaseServices(t, root)
	_, showErr := runCLI(t, svc, "", "show", "DOC-999")
	require.Error(t, showErr)

	tests := []struct {
		name  string
		stdin string
		args  []string
		want  string
	}{
		{"missing entity reads like show", "", []string{"lease", "claim", "DOC-999"}, showErr.Error()},
		{"zero ttl", "", []string{"lease", "claim", ids[0], "--ttl", "0"}, errLeaseTTL.Error()},
		{"ttl below range", "", []string{"lease", "claim", ids[0], "--ttl", "29"}, errLeaseTTL.Error()},
		{"ttl above range", "", []string{"lease", "claim", ids[0], "--ttl", "2h"}, errLeaseTTL.Error()},
		{"ttl not a duration", "", []string{"lease", "claim", ids[0], "--ttl", "soon"}, errLeaseTTL.Error()},
		{"bad label", "", []string{"lease", "claim", ids[0], "--label", "a b"}, "label must be at most 32"},
		{"malformed lease id", "", []string{"lease", "renew", "nope"}, "malformed lease id"},
		{"no lease id", "", []string{"lease", "release"}, "missing lease id"},
		{"both lease id sources", "x", []string{"lease", "release", "abc", "--lease-id-stdin"}, "not both"},
		{"unknown lease", "", []string{"lease", "renew", strings.Repeat("a", 32)}, leaseLostText},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runCLI(t, svc, tc.stdin, tc.args...)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestLeaseCLI_Disabled(t *testing.T) {
	for _, cmd := range []interface {
		Run(context.Context, *leaseServices) error
	}{&LeaseClaimCmd{ID: "X"}, &LeaseRenewCmd{}, &LeaseReleaseCmd{}, &LeaseListCmd{}} {
		require.ErrorIs(t, cmd.Run(leaseCtx(), &leaseServices{}), errLeasesDisabled)
	}
}

func TestParseLeaseTTL(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
		err  bool
	}{
		{"", 0, false},
		{"900", 15 * time.Minute, false},
		{"30", 30 * time.Second, false},
		{"3600", time.Hour, false},
		{"15m", 15 * time.Minute, false},
		{"1h", time.Hour, false},
		{"0", 0, true},
		{"0s", 0, true},
		{"3601", 0, true},
		{"-60", 0, true},
		{"99999999999999999999", 0, true},
		{"10s", 0, true},
		{"abc", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := parseLeaseTTL(tc.in)
			if tc.err {
				require.ErrorIs(t, err, errLeaseTTL)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestLeaseCLI_TwoProcesses claims one entity from two rela processes at
// once (AC1): exactly one wins, and the loser's error names the winner's
// label. The winner's stderr carries its log lines but not the lease id.
func TestLeaseCLI_TwoProcesses(t *testing.T) {
	root, ids := newLeaseProject(t, 1)
	labels := []string{"agent-a", "agent-b"}
	type result struct {
		stdout, stderr string
		err            error
	}
	results := make([]result, len(labels))
	var wg sync.WaitGroup
	for i, label := range labels {
		wg.Go(func() {
			cmd := exec.Command(os.Args[0], "--project", root, "-o", "json", "lease", "claim", ids[0], "--label", label)
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
	require.Contains(t, lose.stderr, leaseHeldPrefix)
	require.Contains(t, lose.stderr, `"label":"`+labels[winners[0]]+`"`)
	c := claimJSONOf(t, win.stdout)
	require.NotContains(t, win.stderr, c.LeaseID)
	require.NotContains(t, lose.stderr, c.LeaseID)
}

// TestLeaseCLI_AbandonedLeaseExpires claims through the lease files the way
// an instance that then died would, with a lease that has already run out,
// and claims again through the CLI (AC2). The abandoned instance writes
// with its own clock, so no test has to wait out a TTL.
func TestLeaseCLI_AbandonedLeaseExpires(t *testing.T) {
	root, ids := newLeaseProject(t, 1)
	cache := filepath.Join(root, ".rela")
	rfs, err := storage.NewRootedFS(storage.NewSafeFS(storage.NewOsFS()), cache)
	require.NoError(t, err)
	locker, err := filelock.New(filepath.Join(cache, "leases.lock"))
	require.NoError(t, err)
	abandoned, err := kvlease.New(state.NewFSKV(rfs), locker)
	require.NoError(t, err)
	sum := sha256.Sum256([]byte("abandoned"))
	then := time.Now().Add(-time.Hour)
	_, err = abandoned.Claim(context.Background(), ids[0],
		lease.Holder{Identity: "ghost-identity", User: "ghost", Tool: "mcp", Label: "ghost"},
		hex.EncodeToString(sum[:]), then, then.Add(lease.MinTTL), "gone")
	require.NoError(t, err)

	svc := openLeaseServices(t, root)
	got, err := runCLI(t, svc, "", "-o", "json", "lease", "claim", ids[0], "--label", "next")
	require.NoError(t, err, "an expired lease does not hold the entity")
	claimJSONOf(t, got)

	// The same claim with a lease still running is refused.
	now := time.Now()
	_, err = abandoned.Claim(context.Background(), ids[0],
		lease.Holder{Identity: "ghost-identity", User: "ghost", Tool: "mcp", Label: "ghost"},
		hex.EncodeToString(sum[:]), now, now.Add(lease.MinTTL), "")
	var held *lease.HeldError
	require.ErrorAs(t, err, &held, "the live CLI lease holds the entity for the other instance too")
	require.Equal(t, "next", held.Current.Holder.Label)
}

// TestLeaseCLI_VisibleToAnotherInstance claims through one Services and
// reads the lease through another instance's lease service once its watcher
// has seen the change (AC3).
func TestLeaseCLI_VisibleToAnotherInstance(t *testing.T) {
	root, ids := newLeaseProject(t, 2)
	a := openLeaseServices(t, root)
	b := openLeaseServices(t, root)
	other := appbuild.Leases(b)
	require.NotNil(t, other)
	wake, cancel := other.Subscribe()
	defer cancel()
	ctx, stop := context.WithCancel(context.Background())
	done := appbuild.WatchLeases(ctx, b)
	defer func() { stop(); <-done }()

	// Prime: B's watcher takes its baseline version when it starts. Change
	// the leases through A until B wakes, so the claim below is a change
	// the baseline does not cover.
	primed := false
	for deadline := time.Now().Add(15 * time.Second); !primed && time.Now().Before(deadline); {
		got, err := runCLI(t, a, "", "-o", "json", "lease", "claim", ids[1])
		require.NoError(t, err)
		_, err = runCLI(t, a, "", "lease", "release", claimJSONOf(t, got).LeaseID)
		require.NoError(t, err)
		select {
		case <-wake:
			primed = true
		case <-time.After(2 * lease.DefaultPollInterval):
		}
	}
	require.True(t, primed, "B's watcher never saw a change made through A")

	got, err := runCLI(t, a, "", "-o", "json", "lease", "claim", ids[0], "--label", "agent-a", "--status", "on it")
	require.NoError(t, err)
	claimJSONOf(t, got)
	select {
	case <-wake:
	case <-time.After(10 * time.Second):
		t.Fatal("B's watcher did not wake after the claim through A")
	}
	leases, err := other.GetReadable(leaseCtx(), []string{ids[0]})
	require.NoError(t, err)
	require.Contains(t, leases, ids[0])
	require.Equal(t, "agent-a", leases[ids[0]].Holder.Label)
	require.Equal(t, "on it", leases[ids[0]].Status)
}
