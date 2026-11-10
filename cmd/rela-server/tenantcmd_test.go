package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// fakeLifecycle records the lifecycle calls and returns err from each.
type fakeLifecycle struct {
	calls []string
	err   error
}

func (f *fakeLifecycle) Create(_ context.Context, org string, adopt bool) (tenant.Tenant, error) {
	f.calls = append(f.calls, "create "+org+map[bool]string{true: " adopt"}[adopt])
	return tenant.Tenant{OrgID: org, Schema: "org_x"}, f.err
}

func (f *fakeLifecycle) Suspend(_ context.Context, org string) error {
	f.calls = append(f.calls, "suspend "+org)
	return f.err
}

func (f *fakeLifecycle) Resume(_ context.Context, org string) error {
	f.calls = append(f.calls, "resume "+org)
	return f.err
}

func (f *fakeLifecycle) Erase(_ context.Context, org string) (tenant.Tenant, tenant.Erasure, error) {
	f.calls = append(f.calls, "erase "+org)
	return tenant.Tenant{OrgID: org, Schema: "org_x"}, tenant.Erasure{SchemaDropped: f.err == nil}, f.err
}

// fakeReader answers Lookup with rec or err.
type fakeReader struct {
	rec tenant.Record
	err error
}

func (r fakeReader) Lookup(context.Context, string) (tenant.Record, error) { return r.rec, r.err }
func (fakeReader) Close() error                                            { return nil }

// projectDir returns a minimal rela project directory.
func projectDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "schema.yaml"), []byte("version: \"1.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

const fakeDSN = "postgres://rela:hunter2@db.example/rela"

// runTenantFake runs a tenant command against fakes and returns its exit
// code and output.
func runTenantFake(t *testing.T, lc *fakeLifecycle, rd fakeReader, stdin string, args ...string) (
	code int, stdout, stderr string,
) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = runTenant(t.Context(), args, tenantIO{
		in: strings.NewReader(stdin), out: &out, err: &errOut,
		getenv: func(k string) string {
			if k == "RELA_DATABASE_URL" {
				return fakeDSN
			}
			return ""
		},
		user:        func() string { return "tester" },
		provisioner: func(string, string) (lifecycle, error) { return lc, nil },
		reader:      func(context.Context, string) (recordReader, error) { return rd, nil },
	})
	return code, out.String(), errOut.String()
}

// TestTenantCommand_Usage pins the usage errors: each exits 2 and calls
// nothing.
func TestTenantCommand_Usage(t *testing.T) {
	dir := projectDir(t)
	for _, args := range [][]string{
		nil,
		{"drop", "org-a"},
		{"erase", "-project", dir},
		{"erase", "-project", dir, "org-a", "org-b"},
		{"suspend", "-adopt", "org-a"},
		{"create", "-force", "org-a"},
	} {
		lc := &fakeLifecycle{}
		code, _, stderr := runTenantFake(t, lc, fakeReader{}, "", args...)
		if code != 2 || len(lc.calls) != 0 {
			t.Errorf("%v: exit %d, calls %v, stderr %q; want 2 and no call", args, code, lc.calls, stderr)
		}
	}
	code, _, stderr := runTenantFake(t, &fakeLifecycle{}, fakeReader{}, "", "erase", "-h")
	if code != 0 || !strings.Contains(stderr, "usage") {
		t.Errorf("help: exit %d, stderr %q; want 0 and the usage", code, stderr)
	}
}

// TestTenantCommand_Dispatch pins that each command calls its lifecycle
// method and reports the outcome without the DSN.
func TestTenantCommand_Dispatch(t *testing.T) {
	dir := projectDir(t)
	tests := []struct {
		args     []string
		stdin    string
		lcErr    error
		wantCode int
		wantCall string
		wantOut  string
	}{
		{args: []string{"create", "-project", dir, "org-a"}, wantCall: "create org-a", wantOut: "active"},
		{args: []string{"create", "-project", dir, "-adopt", "org-a"}, wantCall: "create org-a adopt"},
		{args: []string{"create", "-project", dir, "org-a"}, lcErr: tenant.ErrSchemaExists, wantCode: 1,
			wantCall: "create org-a", wantOut: "-adopt"},
		{args: []string{"suspend", "-project", dir, "org-a"}, wantCall: "suspend org-a", wantOut: "suspended"},
		{args: []string{"resume", "-project", dir, "org-a"}, wantCall: "resume org-a", wantOut: "resumed"},
		{args: []string{"resume", "-project", dir, "org-a"}, lcErr: tenant.ErrWrongStatus, wantCode: 1,
			wantCall: "resume org-a", wantOut: "not allowed"},
		{args: []string{"erase", "-project", dir, "org-a"}, stdin: "org-a\n", wantCall: "erase org-a",
			wantOut: "dropped"},
		{args: []string{"erase", "-project", dir, "-force", "org-a"}, lcErr: tenant.ErrTenantInUse, wantCode: 1,
			wantCall: "erase org-a", wantOut: "retry"},
		{args: []string{"erase", "-project", dir, "-force", "org-z"}, lcErr: tenant.ErrUnknownTenant, wantCode: 1,
			wantCall: "erase org-z", wantOut: "nothing was erased"},
	}
	for _, tc := range tests {
		t.Run(strings.Join(tc.args[:1], " ")+" "+tc.wantOut, func(t *testing.T) {
			lc := &fakeLifecycle{err: tc.lcErr}
			code, stdout, stderr := runTenantFake(t, lc, fakeReader{}, tc.stdin, tc.args...)
			if code != tc.wantCode {
				t.Errorf("exit %d, want %d (stderr %q)", code, tc.wantCode, stderr)
			}
			if len(lc.calls) != 1 || lc.calls[0] != tc.wantCall {
				t.Errorf("calls %v, want [%s]", lc.calls, tc.wantCall)
			}
			if !strings.Contains(stdout+stderr, tc.wantOut) {
				t.Errorf("output %q, want it to contain %q", stdout+stderr, tc.wantOut)
			}
			if strings.Contains(stdout+stderr, "hunter2") {
				t.Error("the output carries the DSN password")
			}
		})
	}
}

// TestTenantCommand_EraseConfirmation pins the typed confirmation: anything
// but the exact org id erases nothing.
func TestTenantCommand_EraseConfirmation(t *testing.T) {
	dir := projectDir(t)
	for _, stdin := range []string{"org-b\n", "org-\n", "", " org-a\n"} {
		lc := &fakeLifecycle{}
		code, stdout, stderr := runTenantFake(t, lc, fakeReader{}, stdin, "erase", "-project", dir, "org-a")
		if code != 1 || len(lc.calls) != 0 || !strings.Contains(stderr, "does not match") {
			t.Errorf("stdin %q: exit %d, calls %v, stderr %q; want a refusal", stdin, code, lc.calls, stderr)
		}
		if !strings.Contains(stdout, "PROCEDURE-u020") || !strings.Contains(stdout, "cannot be undone") {
			t.Errorf("prompt %q lacks the procedure check or the one-way warning", stdout)
		}
	}
}

// TestTenantCommand_EraseRecord pins that every erase attempt is recorded
// on the host, failed ones included.
func TestTenantCommand_EraseRecord(t *testing.T) {
	dir := projectDir(t)
	runTenantFake(t, &fakeLifecycle{err: errors.New("boom")}, fakeReader{}, "", "erase", "-project", dir,
		"-force", "org-a")
	runTenantFake(t, &fakeLifecycle{}, fakeReader{}, "", "erase", "-project", dir, "-force", "org-a")
	rec, err := os.ReadFile(filepath.Join(dir, ".rela", "audit", erasureLog))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"ok":false`, `"ok":true`, `"operator":"tester"`, `"org_id":"org-a"`} {
		if !strings.Contains(string(rec), want) {
			t.Errorf("erasure record %q lacks %s", rec, want)
		}
	}
}

// TestTenantCommand_Show pins the show output for a tenant and for an org
// without one.
func TestTenantCommand_Show(t *testing.T) {
	dir := projectDir(t)
	code, out, _ := runTenantFake(t, &fakeLifecycle{}, fakeReader{
		rec: tenant.Record{OrgID: "org-a", Schema: "org_x", Status: tenant.StatusSuspended, SchemaExists: true},
	}, "", "show", "-project", dir, "org-a")
	if code != 0 || !strings.Contains(out, "suspended") {
		t.Errorf("show: exit %d, out %q", code, out)
	}
	code, out, _ = runTenantFake(t, &fakeLifecycle{}, fakeReader{err: tenant.ErrUnknownTenant}, "",
		"show", "-project", dir, "org-z")
	want, _ := tenant.SchemaFor("org-z")
	if code != 0 || !strings.Contains(out, "no tenant") || !strings.Contains(out, want) {
		t.Errorf("show unknown: exit %d, out %q; want the derived schema %s", code, out, want)
	}
}

// TestTenantCommand_NeedsDSN pins that the commands refuse without a base
// DSN.
func TestTenantCommand_NeedsDSN(t *testing.T) {
	var errOut bytes.Buffer
	code := runTenant(t.Context(), []string{"create", "-project", projectDir(t), "org-a"}, tenantIO{
		in: strings.NewReader(""), out: &bytes.Buffer{}, err: &errOut, getenv: func(string) string { return "" },
	})
	if code != 1 || !strings.Contains(errOut.String(), "RELA_DATABASE_URL") {
		t.Errorf("exit %d, stderr %q", code, errOut.String())
	}
}

// TestRefuseExtraArgs pins that the server refuses arguments after its
// flags, and points a misplaced tenant command at the right form.
func TestRefuseExtraArgs(t *testing.T) {
	if err := refuseExtraArgs(nil); err != nil {
		t.Errorf("no args: %v", err)
	}
	if err := refuseExtraArgs([]string{"tenant", "erase", "org-a"}); err == nil ||
		!strings.Contains(err.Error(), "must come first") {

		t.Errorf("misplaced tenant command: %v", err)
	}
	if err := refuseExtraArgs([]string{"serve"}); err == nil {
		t.Error("a stray argument was accepted")
	}
}

// TestRefuseTenantDeployment_LegacyMap pins that a server without tenant
// mode refuses a project that still holds the old tenant map.
func TestRefuseTenantDeployment_LegacyMap(t *testing.T) {
	dir := projectDir(t)
	none := func(string) string { return "" }
	if err := refuseTenantDeployment(t.Context(), dir, none); err != nil {
		t.Fatalf("a plain project was refused: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tenants.yaml"), []byte("tenants: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := refuseTenantDeployment(t.Context(), dir, none); !errors.Is(err, tenant.ErrLegacyTenantMap) {
		t.Fatalf("error = %v, want ErrLegacyTenantMap", err)
	}
}
