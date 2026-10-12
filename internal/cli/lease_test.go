package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lease"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/script"
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

// newLeaseProject writes a filesystem project with one doc and returns its
// root and the doc id, as ids[0].
func newLeaseProject(t *testing.T) (root string, ids []string) {
	t.Helper()
	root = t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".rela"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "metamodel.yaml"), []byte(leaseCLIMetamodel), 0o600))
	svc, err := appbuild.Discover(root, script.NewEngine())
	require.NoError(t, err)
	defer svc.Close()
	e := entity.New("", "doc")
	e.SetString("title", "a doc")
	created, err := svc.EntityManager().CreateEntity(leaseCtx(), e, entity.CreateOptions{})
	require.NoError(t, err)
	return root, []string{created.Entity.ID}
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
//
// It swaps the package globals out and leaseStdin for the length of the
// test, so a test that uses it must not call t.Parallel.
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
	root, ids := newLeaseProject(t)
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
	require.ErrorContains(t, err, lease.HeldPrefix)
	require.ErrorContains(t, err, `"label":"agent-1"`, "a held error names the holder's label")

	got, err = runCLI(t, svc, c.LeaseID, "lease", "release", "--lease-id-stdin")
	require.NoError(t, err)
	require.Equal(t, "released\n", got)

	_, err = runCLI(t, svc, "", "lease", "renew", c.LeaseID)
	require.EqualError(t, err, lease.LostText)
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
	root, ids := newLeaseProject(t)
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
		{"empty stdin", "", []string{"lease", "release", "--lease-id-stdin"}, "missing lease id"},
		{"blank first line", "  \n" + strings.Repeat("a", 32), []string{"lease", "release", "--lease-id-stdin"},
			"missing lease id"},
		{"stdin line too long", strings.Repeat("a", 2000), []string{"lease", "release", "--lease-id-stdin"},
			"--lease-id-stdin"},
		{"both lease id sources", "x", []string{"lease", "release", "abc", "--lease-id-stdin"}, "not both"},
		{"unknown lease", "", []string{"lease", "renew", strings.Repeat("a", 32)}, lease.LostText},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runCLI(t, svc, tc.stdin, tc.args...)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

// TestLeaseCLI_StdinTerminal refuses --lease-id-stdin when stdin is a
// character device, as an interactive terminal is. /dev/null is one too.
func TestLeaseCLI_StdinTerminal(t *testing.T) {
	f, err := os.Open(os.DevNull)
	require.NoError(t, err)
	defer f.Close()
	prev := leaseStdin
	leaseStdin = f
	t.Cleanup(func() { leaseStdin = prev })
	_, err = readLeaseID("", true)
	require.ErrorContains(t, err, "stdin is a terminal")
}

// TestReadLeaseID_FirstLine reads only the first line of stdin.
func TestReadLeaseID_FirstLine(t *testing.T) {
	prev := leaseStdin
	leaseStdin = strings.NewReader(" abc \nignored\n")
	t.Cleanup(func() { leaseStdin = prev })
	got, err := readLeaseID("", true)
	require.NoError(t, err)
	require.Equal(t, "abc", got)
}

func TestLeaseCLI_Disabled(t *testing.T) {
	for _, cmd := range []interface {
		Run(context.Context, *leaseServices) error
	}{&LeaseClaimCmd{ID: "X"}, &LeaseRenewCmd{}, &LeaseReleaseCmd{}, &LeaseListCmd{}} {
		err := cmd.Run(leaseCtx(), &leaseServices{})
		require.ErrorIs(t, err, lease.ErrNotEnabled)
		require.EqualError(t, err, lease.NotEnabledText)
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
