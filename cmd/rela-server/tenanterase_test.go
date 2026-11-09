package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const eraseTenantsYAML = `tenants:
  - org_id: org-a
    schema: tenant_a
    dsn: "host=127.0.0.1 dbname=nowhere search_path=tenant_a,public"
    erase: true
  - org_id: org-b
    schema: tenant_b
    dsn: "host=127.0.0.1 dbname=nowhere search_path=tenant_b,public"
`

// runErase runs the erase command with stdin and returns its exit
// code and output.
func runErase(t *testing.T, stdin string, args ...string) (code int, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = eraseTenant(t.Context(), args, eraseIO{
		in: strings.NewReader(stdin), out: &out, err: &errOut,
		getenv: func(string) string { return "" },
	})
	return code, errOut.String()
}

// TestEraseTenant_Refusals pins every refusal that comes before the database:
// each exits non-zero and erases nothing.
func TestEraseTenant_Refusals(t *testing.T) {
	withMap := t.TempDir()
	if err := os.WriteFile(filepath.Join(withMap, tenantsFile), []byte(eraseTenantsYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	auditDir := filepath.Join(withMap, ".rela", "audit", "tenants", "tenant_a")
	if err := os.MkdirAll(auditDir, 0o750); err != nil {
		t.Fatal(err)
	}
	empty := t.TempDir()

	tests := []struct {
		name     string
		args     []string
		stdin    string
		wantCode int
		wantErr  string
	}{
		{name: "no subcommand", args: nil, wantCode: 2, wantErr: "usage"},
		{name: "unknown subcommand", args: []string{"drop", "org-a"}, wantCode: 2, wantErr: "usage"},
		{name: "no org", args: []string{"erase", "-project", withMap}, wantCode: 2, wantErr: "usage"},
		{name: "two orgs", args: []string{"erase", "-project", withMap, "org-a", "org-b"}, wantCode: 2},
		{name: "no tenant map", args: []string{"erase", "-project", empty, "-force", "org-a"},
			wantCode: 1, wantErr: "no tenants.yaml"},
		{name: "unknown org", args: []string{"erase", "-project", withMap, "-force", "org-c"},
			wantCode: 1, wantErr: "unknown organization"},
		{name: "org not marked", args: []string{"erase", "-project", withMap, "-force", "org-b"},
			wantCode: 1, wantErr: "not marked for erasure"},
		{name: "confirmation mismatch", args: []string{"erase", "-project", withMap, "org-a"},
			stdin: "org-b\n", wantCode: 1, wantErr: "does not match"},
		{name: "confirmation prefix", args: []string{"erase", "-project", withMap, "org-a"},
			stdin: "org-\n", wantCode: 1, wantErr: "does not match"},
		{name: "no confirmation", args: []string{"erase", "-project", withMap, "org-a"},
			stdin: "", wantCode: 1, wantErr: "does not match"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, stderr := runErase(t, tc.stdin, tc.args...)
			if code != tc.wantCode {
				t.Errorf("exit code = %d, want %d (stderr %q)", code, tc.wantCode, stderr)
			}
			if !strings.Contains(stderr, tc.wantErr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, tc.wantErr)
			}
			if _, err := os.Stat(auditDir); err != nil {
				t.Errorf("a refused erase removed the audit directory: %v", err)
			}
		})
	}
}

// TestEraseTenant_HelpExitsZero pins that asking for help is not an error.
func TestEraseTenant_HelpExitsZero(t *testing.T) {
	code, stderr := runErase(t, "", "erase", "-h")
	if code != 0 || !strings.Contains(stderr, "usage") {
		t.Errorf("exit %d, stderr %q; want 0 and the usage", code, stderr)
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
