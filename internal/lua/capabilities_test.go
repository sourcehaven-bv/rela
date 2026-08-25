package lua

import (
	"bytes"
	"strings"
	"testing"
)

// capProbe reports what a runtime exposes so a test can assert on
// presence/absence. It uses rela.output rather than print(): print writes to
// the process stdout, while rela.output goes to the runtime's own writer,
// which is what the helper captures.
const capProbe = `
rela.output("http=" .. type(http) ..
  " ai=" .. type(ai) ..
  " write_file=" .. type(rela.write_file) ..
  " dsn=" .. tostring(rela.secrets.db_dsn) ..
  " slack=" .. tostring(rela.secrets.slack_token))
`

func runCapProbe(t *testing.T, writer bool, opts ...Option) string {
	t.Helper()
	var buf bytes.Buffer
	secrets := map[string]string{
		"db_dsn":      "postgres://user:pw@host/db",
		"slack_token": "xoxb-token",
	}
	all := append([]Option{WithSecrets(secrets)}, opts...)

	var rt *Runtime
	if writer {
		ws := newMockWorkspace(t)
		rt = NewWriter(ws.services(t.TempDir()), &buf, all...)
	} else {
		rt = NewReader(ReadDeps{}, &buf, all...)
	}
	defer rt.Close()

	if err := rt.RunString(capProbe); err != nil {
		t.Fatalf("probe failed: %v", err)
	}
	return buf.String()
}

// TestCapabilities_ZeroValueDeniesEverything is the core guarantee of
// TKT-YH52OM: a runtime built WITHOUT an explicit grant reaches nothing.
//
// Before this ticket every runtime — reader and writer alike — held `http`,
// `ai` and the whole of .rela/secrets.yaml unconditionally, which is a
// two-call exfiltration path (read a secret, POST it out). If this test ever
// fails, that path is back.
func TestCapabilities_ZeroValueDeniesEverything(t *testing.T) {
	t.Parallel()
	for _, writer := range []bool{false, true} {
		name := "reader"
		if writer {
			name = "writer"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out := runCapProbe(t, writer)
			for _, want := range []string{
				"http=nil", "ai=nil", "write_file=nil", "dsn=nil", "slack=nil",
			} {
				if !strings.Contains(out, want) {
					t.Errorf("ungranted runtime: want %q in output, got:\n%s", want, out)
				}
			}
		})
	}
}

// TestCapabilities_SecretsAreNamedNotAllOrNothing pins the ticket's specific
// refinement: granting one secret must NOT hand over the rest of the file.
// This is why Secrets is a []string and not a bool — an action that needs a
// Slack webhook has no business holding the database DSN.
func TestCapabilities_SecretsAreNamedNotAllOrNothing(t *testing.T) {
	t.Parallel()
	out := runCapProbe(t, false,
		WithCapabilities(Capabilities{Secrets: []string{"slack_token"}}))

	if !strings.Contains(out, "slack=xoxb-token") {
		t.Errorf("granted secret missing, got:\n%s", out)
	}
	if !strings.Contains(out, "dsn=nil") {
		t.Errorf("UNGRANTED secret leaked — a named grant must not expose the "+
			"whole secrets file, got:\n%s", out)
	}
	// An ungranted key must be ABSENT, not present-and-empty: a typo should
	// surface as a nil index at the use site, not as a silently-empty
	// credential that reaches an upstream as an empty Authorization header.
	if strings.Contains(out, "dsn=") && !strings.Contains(out, "dsn=nil") {
		t.Errorf("ungranted secret should be nil, not empty string:\n%s", out)
	}
}

// TestCapabilities_GrantsAreIndependent guards against a gate that accidentally
// keys every capability off one flag.
func TestCapabilities_GrantsAreIndependent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		caps   Capabilities
		want   string
		absent []string
	}{
		{"http only", Capabilities{HTTP: true}, "http=table", []string{"ai=nil", "write_file=nil"}},
		{"ai only", Capabilities{AI: true}, "ai=table", []string{"http=nil", "write_file=nil"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := runCapProbe(t, true, WithCapabilities(tc.caps))
			if !strings.Contains(out, tc.want) {
				t.Errorf("want %q, got:\n%s", tc.want, out)
			}
			for _, a := range tc.absent {
				if !strings.Contains(out, a) {
					t.Errorf("want %q (other capabilities must stay denied), got:\n%s", a, out)
				}
			}
		})
	}
}

// TestCapabilities_WriteFileIsWriterOnly documents that WriteFile is gated by
// BOTH the runtime kind and the capability: granting it on a reader must not
// conjure a write binding onto a runtime that has no mutation surface.
func TestCapabilities_WriteFileIsWriterOnly(t *testing.T) {
	t.Parallel()
	out := runCapProbe(t, false, WithCapabilities(Capabilities{WriteFile: true}))
	if !strings.Contains(out, "write_file=nil") {
		t.Errorf("a READER granted WriteFile must still have no write_file binding, got:\n%s", out)
	}

	got := runCapProbe(t, true, WithCapabilities(Capabilities{WriteFile: true}))
	if !strings.Contains(got, "write_file=function") {
		t.Errorf("a WRITER granted WriteFile should have the binding, got:\n%s", got)
	}
}

// TestTrustedCapabilities_GrantsAll pins the operator-shell escape hatch. If
// this ever narrows silently, `rela script` / the docs build break in CI.
func TestTrustedCapabilities_GrantsAll(t *testing.T) {
	t.Parallel()
	out := runCapProbe(t, true, WithCapabilities(TrustedCapabilities()))
	for _, want := range []string{
		"http=table", "ai=table", "write_file=function",
		"dsn=postgres://user:pw@host/db", "slack=xoxb-token",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("TrustedCapabilities should grant everything: want %q, got:\n%s", want, out)
		}
	}
}

// TestCapabilities_AllSecretsIsNotConfigReachable pins that the broad secrets
// grant is a Go-side decision only. AllSecrets is a separate field rather than
// a "*" sentinel inside Secrets precisely so an operator cannot write it in
// YAML; if someone ever adds "*" handling to AllowsSecret, this fails.
func TestCapabilities_AllSecretsIsNotConfigReachable(t *testing.T) {
	t.Parallel()
	c := Capabilities{Secrets: []string{"*"}}
	if c.AllowsSecret("db_dsn") {
		t.Error(`"*" in Secrets must NOT mean "all" — the broad grant is ` +
			`AllSecrets, which config cannot set`)
	}
	if !c.AllowsSecret("*") {
		t.Error(`"*" should be treated as an ordinary (odd) key name`)
	}
}

func TestCapabilities_FilterSecrets(t *testing.T) {
	t.Parallel()
	all := map[string]string{"a": "1", "b": "2", "c": "3"}

	got := Capabilities{Secrets: []string{"a", "c", "missing"}}.filterSecrets(all)
	if len(got) != 2 || got["a"] != "1" || got["c"] != "3" {
		t.Errorf("filterSecrets: got %v, want {a:1 c:3}", got)
	}

	if n := len(Capabilities{}.filterSecrets(all)); n != 0 {
		t.Errorf("zero value must grant no secrets, got %d", n)
	}
	if n := len(Capabilities{AllSecrets: true}.filterSecrets(all)); n != 3 {
		t.Errorf("AllSecrets must pass everything through, got %d", n)
	}
}
