package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// stubOSUser makes the uid lookup behind asserted_by return name, or fail
// when name is empty.
func stubOSUser(t *testing.T, name string) {
	t.Helper()
	prev := lookupOSUser
	t.Cleanup(func() { lookupOSUser = prev })
	lookupOSUser = func() (string, error) {
		if name == "" {
			return "", errors.New("unknown userid")
		}
		return name, nil
	}
}

// TestNewLocalIdentity pins the identity a local command acts as: the OS
// user by default, a job identity with --run-as, and a refusal for any
// other name before a service is called. asserted_by comes from the uid,
// so $USER (bob here) cannot choose it.
func TestNewLocalIdentity(t *testing.T) {
	t.Setenv("USER", "alice")
	stubOSUser(t, "alice")
	tests := []struct {
		name     string
		tool     string
		runAs    string
		want     principal.Principal
		asserted string
		wantErr  string
	}{
		{name: "default cli", tool: principal.ToolCLI, want: principal.Principal{User: "alice", Tool: principal.ToolCLI}},
		{name: "default mcp", tool: principal.ToolMCP, want: principal.Principal{User: "alice", Tool: principal.ToolMCP}},
		{
			name: "system job", tool: principal.ToolCLI, runAs: "system:ci",
			want: principal.Principal{User: "system:ci", Tool: principal.ToolCLI}, asserted: "alice",
		},
		{
			name: "integration", tool: principal.ToolMCP, runAs: "integration:triage-bot",
			want: principal.Principal{User: "integration:triage-bot", Tool: principal.ToolMCP}, asserted: "alice",
		},
		{name: "person", tool: principal.ToolCLI, runAs: "bob", wantErr: "not a job identity"},
		{name: "unknown", tool: principal.ToolCLI, runAs: principal.Unknown, wantErr: "not a job identity"},
		{name: "realtime", tool: principal.ToolCLI, runAs: principal.UserRealtime, wantErr: "rela's own"},
		{name: "scheduler", tool: principal.ToolCLI, runAs: principal.UserScheduler, wantErr: "rela's own"},
		{name: "automation", tool: principal.ToolCLI, runAs: principal.UserAutomation, wantErr: "rela's own"},
		{name: "provisioner", tool: principal.ToolCLI, runAs: principal.UserProvisioner, wantErr: "rela's own"},
		{name: "webhook", tool: principal.ToolCLI, runAs: "system:webhook:github", wantErr: "rela's own"},
		{name: "mail", tool: principal.ToolCLI, runAs: principal.UserMail, wantErr: "rela's own"},
		{name: "fs-import", tool: principal.ToolCLI, runAs: principal.UserFSImport, wantErr: "rela's own"},
		{name: "gc-sweep", tool: principal.ToolCLI, runAs: principal.UserGCSweep, wantErr: "rela's own"},
		{name: "perf-seed", tool: principal.ToolCLI, runAs: principal.UserPerfSeed, wantErr: "rela's own"},
		{name: "subscription", tool: principal.ToolCLI, runAs: "system:subscription:news", wantErr: "rela's own"},
		{name: "job namespace", tool: principal.ToolCLI, runAs: "system:job:nightly",
			want: principal.Principal{User: "system:job:nightly", Tool: principal.ToolCLI}, asserted: "alice"},
		{name: "bare prefix", tool: principal.ToolCLI, runAs: "system:", wantErr: "lowercase letters"},
		{name: "zero width", tool: principal.ToolCLI, runAs: "system:realtime\u200b", wantErr: "lowercase letters"},
		{name: "word joiner", tool: principal.ToolCLI, runAs: "system:realtime\u2060", wantErr: "lowercase letters"},
		{name: "bidi override", tool: principal.ToolCLI, runAs: "system:\u202erealtime", wantErr: "lowercase letters"},
		{name: "cyrillic e", tool: principal.ToolCLI, runAs: "system:r\u0435altime", wantErr: "lowercase letters"},
		{name: "upper case", tool: principal.ToolCLI, runAs: "system:Realtime", wantErr: "lowercase letters"},
		{name: "empty segment", tool: principal.ToolCLI, runAs: "system:webhook:", wantErr: "lowercase letters"},
		{name: "bad integration", tool: principal.ToolCLI, runAs: "integration:Bad Name", wantErr: "integration name"},
		{name: "space", tool: principal.ToolCLI, runAs: " system:ci", wantErr: "not a job identity"},
		{name: "control", tool: principal.ToolCLI, runAs: "system:c\x1bi", wantErr: "lowercase letters"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := newLocalIdentity(tc.tool, tc.runAs)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				assert.Contains(t, err.Error(), runAsFlag)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.Principal, "no claims: no principal type, no roles")
			assert.Equal(t, tc.asserted, got.AssertedBy)
		})
	}
}

// TestLocalIdentity_Stamp pins that the asserting OS user goes on ctx for
// the graph audit only when --run-as is in use.
func TestLocalIdentity_Stamp(t *testing.T) {
	t.Setenv("USER", "alice")
	stubOSUser(t, "alice")
	plain, err := newLocalIdentity(principal.ToolCLI, "")
	require.NoError(t, err)
	ctx := plain.stamp(context.Background())
	assert.Equal(t, "alice", principal.From(ctx).User)
	assert.Empty(t, audit.AssertedByFrom(ctx))

	asserted, err := newLocalIdentity(principal.ToolCLI, "system:ci")
	require.NoError(t, err)
	ctx = asserted.stamp(context.Background())
	assert.Equal(t, principal.Principal{User: "system:ci", Tool: principal.ToolCLI}, principal.From(ctx))
	assert.Equal(t, "alice", audit.AssertedByFrom(ctx))
}

// TestAssertingUser pins the OS user behind --run-as: the account of the
// uid whatever $USER says, and the numeric uid when it has no account.
func TestAssertingUser(t *testing.T) {
	t.Setenv("USER", "system:realtime")
	t.Setenv("LOGNAME", "bob")
	stubOSUser(t, "carol")
	assert.Equal(t, "carol", assertingUser())
	stubOSUser(t, "")
	assert.Equal(t, "uid:"+strconv.Itoa(os.Getuid()), assertingUser())
}

// TestNewLocalIdentity_ReservedUserRefused pins that a job identity can
// enter only through --run-as: a $USER in the system: or integration:
// namespace is refused, so it cannot skip the refusals and asserted_by.
func TestNewLocalIdentity_ReservedUserRefused(t *testing.T) {
	for _, u := range []string{principal.UserRealtime, principal.UserScheduler, "system:ci", "integration:bot", "\x01system:ci"} {
		t.Run(u, func(t *testing.T) {
			t.Setenv("USER", u)
			_, err := newLocalIdentity(principal.ToolCLI, "")
			require.Error(t, err)
			assert.Contains(t, err.Error(), runAsFlag)
		})
	}
}

// TestCommandIdentity pins where --run-as applies: validated only for a
// command that acts as an identity, refused by the scheduler, and the mcp
// tool for `rela mcp`.
func TestCommandIdentity(t *testing.T) {
	t.Setenv("USER", "alice")
	tests := []struct {
		name    string
		cmd     string
		runAs   string
		flag    bool
		want    principal.Principal
		wantErr string
	}{
		{name: "version ignores a bad value", cmd: "version", runAs: "bob",
			want: principal.Principal{User: "alice", Tool: principal.ToolCLI}},
		{name: "version refuses the flag", cmd: "version", runAs: "system:ci", flag: true, wantErr: "records no identity"},
		{name: "db migrate refuses the flag", cmd: "db migrate", runAs: "system:ci", flag: true,
			wantErr: "rela db migrate"},
		{name: "db load uses it", cmd: "db load", runAs: "system:ci", flag: true,
			want: principal.Principal{User: "system:ci", Tool: principal.ToolCLI}},
		{name: "db import-fs uses it", cmd: "db import-fs <source> <target>", runAs: "system:ci",
			want: principal.Principal{User: "system:ci", Tool: principal.ToolCLI}},
		{name: "init ignores it", cmd: "init", runAs: "system:ci",
			want: principal.Principal{User: "alice", Tool: principal.ToolCLI}},
		{name: "show validates", cmd: "show <id>", runAs: "bob", wantErr: "not a job identity"},
		{name: "channel post uses it", cmd: "channel post <id> <type> <message-type>", runAs: "system:ci",
			want: principal.Principal{User: "system:ci", Tool: principal.ToolCLI}},
		{name: "mcp uses it", cmd: "mcp", runAs: "integration:agent",
			want: principal.Principal{User: "integration:agent", Tool: principal.ToolMCP}},
		{name: "flow uses it", cmd: "flow <script>", runAs: "system:ci",
			want: principal.Principal{User: "system:ci", Tool: principal.ToolCLI}},
		{name: "scheduler refuses it", cmd: "scheduler run", runAs: "system:ci", wantErr: "schedules.yaml"},
		{name: "scheduler without it", cmd: "scheduler run",
			want: principal.Principal{User: "alice", Tool: principal.ToolCLI}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := commandIdentity(tc.cmd, tc.runAs, tc.flag)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.Principal)
		})
	}
}

// TestRunAsNotice pins the stderr notice: printed by write commands under
// --run-as only, and made printable.
func TestRunAsNotice(t *testing.T) {
	stubOSUser(t, "al\x1bice")
	for _, tc := range []struct {
		cmd   string
		write bool
	}{
		{"create <type>", true},
		{"channel post <id> <type> <message-type>", true},
		{"lease claim <id>", true},
		{"show <id>", false},
		{"channel read <id> <type>", false},
		{"channel watch <id> <type>", false},
		{"lease list", false},
		{"db load", true},
		{"db migrate", false},
		{"version", false},
	} {
		assert.Equal(t, tc.write, writesAsIdentity(tc.cmd), tc.cmd)
	}

	id, err := newLocalIdentity(principal.ToolCLI, "system:ci")
	require.NoError(t, err)
	var buf bytes.Buffer
	id.notice(&buf)
	assert.Equal(t, "acting as system:ci (asserted by al�ice)\n", buf.String())

	plain, err := newLocalIdentity(principal.ToolCLI, "")
	require.NoError(t, err)
	buf.Reset()
	plain.notice(&buf)
	assert.Empty(t, buf.String())
}

// runCLIProcess runs the real CLI entry point in a subprocess with env
// added, and returns its stdout, stderr and exit code.
func runCLIProcess(t *testing.T, env []string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(append(os.Environ(), cliSubprocessEnv+"=1"), env...)
	var o, e bytes.Buffer
	cmd.Stdout, cmd.Stderr = &o, &e
	err := cmd.Run()
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return o.String(), e.String(), exitErr.ExitCode()
	}
	require.NoError(t, err)
	return o.String(), e.String(), 0
}

// TestRunAs_AssertedRecorded runs `rela create` under RELA_RUN_AS: the
// write is attributed to the job identity, the graph audit names the OS
// user who asserted it, and stderr says so. $USER is spoofed and must not
// reach asserted_by.
func TestRunAs_AssertedRecorded(t *testing.T) {
	setBackendEnv(t)
	root, _ := newLeaseProject(t)
	osUser := assertingUser()
	require.NotEqual(t, "alice-spoofed", osUser)
	_, stderr, code := runCLIProcess(t, []string{"USER=alice-spoofed", principal.RunAsEnv + "=system:ci"},
		"--project", root, "create", "doc", "-P", "title=by a job")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stderr, "acting as system:ci (asserted by "+osUser+")\n")

	files, err := filepath.Glob(filepath.Join(root, ".rela", "audit", "*.jsonl"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
	var found bool
	for _, f := range files {
		data, err := os.ReadFile(f)
		require.NoError(t, err)
		for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
			var rec audit.Record
			require.NoError(t, json.Unmarshal([]byte(line), &rec), line)
			if rec.Op != audit.OpCreateEntity {
				continue
			}
			if rec.Principal.User == "system:ci" {
				found = true
				assert.Equal(t, osUser, rec.AssertedBy)
				assert.Equal(t, principal.ToolCLI, rec.Principal.Tool)
			}
		}
	}
	assert.True(t, found, "no create record by system:ci")
}

// TestRunAs_InvalidRefusedBeforeServices pins that a bad RELA_RUN_AS fails
// a command that acts as an identity before the project is opened, and is
// ignored by one that acts as no one.
func TestRunAs_InvalidRefusedBeforeServices(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-project")
	_, stderr, code := runCLIProcess(t, []string{principal.RunAsEnv + "=bob"},
		"--project", missing, "show", "DOC-1")
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "not a job identity")
	assert.NotContains(t, stderr, "no project")

	stdout, stderr, code := runCLIProcess(t, []string{principal.RunAsEnv + "=bob"}, "version")
	assert.Equal(t, 0, code, stderr)
	assert.NotEmpty(t, stdout)

	_, stderr, code = runCLIProcess(t, []string{"USER=system:realtime"}, "--project", missing, "show", "DOC-1")
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "names a job identity")

	_, stderr, code = runCLIProcess(t, nil, "--run-as", "system:ci", "version")
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "records no identity")
}

// TestIdentityWrites_ClassifiesEveryCommand walks the kong tree: every
// command that acts as an identity must say in identityWrites whether it
// writes, so a new one does not print the --run-as notice by accident or
// skip it.
func TestIdentityWrites_ClassifiesEveryCommand(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli, kong.Name("rela"), kong.Vars{"version": "test"})
	require.NoError(t, err)
	var walk func(n *kong.Node, path []string)
	walk = func(n *kong.Node, path []string) {
		leaf := true
		for _, ch := range n.Children {
			if ch.Type == kong.CommandNode {
				leaf = false
				walk(ch, append(slices.Clone(path), ch.Name))
			}
		}
		cmd := strings.Join(path, " ")
		if !leaf || !actsAsIdentity(cmd) {
			return
		}
		_, ok := identityWrites[cmd]
		assert.True(t, ok, "rela %s acts as an identity but is not classified in identityWrites", cmd)
	}
	walk(parser.Model.Node, nil)
	for cmd := range identityWrites {
		assert.True(t, actsAsIdentity(cmd), "identityWrites lists %q, which acts as no one", cmd)
	}
}
