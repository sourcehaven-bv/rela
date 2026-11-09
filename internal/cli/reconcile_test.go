package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/reconwire"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	relaerrors "github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/reconcile"
	"github.com/Sourcehaven-BV/rela/internal/reconclient"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

const reconcileSchema = `
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    properties:
      title: {type: string}
  feature:
    label: Feature
    id_prefix: FEAT
    properties:
      title: {type: string}
  note:
    label: Note
    id_prefix: NOTE
    properties:
      title: {type: string}
relations:
  implements:
    from: [ticket]
    to: [feature]
  about:
    from: [note, ticket]
    to: [note]
`

func reconcileMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	m, err := metamodel.Parse([]byte(reconcileSchema))
	require.NoError(t, err)
	return m
}

// emptyLocal is a local side with no elements.
type emptyLocal struct{}

func (emptyLocal) Sum(context.Context, visibility.ElementTypes, store.ElementRange) (reconclient.Sum, error) {
	return reconclient.Sum{Fingerprint: strings.Repeat("0", 32)}, nil
}

func (emptyLocal) List(
	context.Context, visibility.ElementTypes, store.ElementRange, int,
) (reconclient.Page, error) {
	return reconclient.Page{}, nil
}

// oneTicketRemote serves a remote that holds the one ticket TKT-1 and
// checks the credential it was given.
func oneTicketRemote(t *testing.T, wantAuth string) *httptest.Server {
	t.Helper()
	el := reconcile.Element{
		Kind: reconcile.KindEntity, Address: "ticket/TKT-1",
		Hash: canonical.Digest(sha256.Sum256([]byte("TKT-1"))),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != wantAuth {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var req reconwire.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var resp reconwire.Response
		for _, rr := range req.Ranges {
			has := rr.Kind == "entity" && slices.Contains(rr.Entities, "ticket") && rr.Lo <= el.Address &&
				(rr.Hi == nil || el.Address < *rr.Hi) && (rr.After == nil || *rr.After < el.Address)
			if rr.Op == "fingerprint" {
				var sum reconcile.Range
				if has {
					sum = sum.Add(el)
				}
				n := sum.Count
				resp.Ranges = append(resp.Ranges, reconwire.Result{
					Status: reconwire.StatusOK, Count: &n, Fingerprint: sum.Sum.String(),
				})
				continue
			}
			res := reconwire.Result{Status: reconwire.StatusOK}
			if has {
				res.Elements = []reconwire.Element{{Address: el.Address, Hash: el.Hash.String()}}
				res.Last = el.Address
			}
			resp.Ranges = append(resp.Ranges, res)
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// runReconcile parses args as a rela command line and runs it against s.
func runReconcile(t *testing.T, s *reconcileServices, args ...string) (string, error) {
	t.Helper()
	var cli CLI
	parser, err := kong.New(&cli, kong.Name("rela"), kong.Vars{"version": Version})
	require.NoError(t, err)
	ktx, err := parser.Parse(args)
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	prev := out
	out = output.NewWithWriter(buf, output.Format(cli.Output))
	t.Cleanup(func() { out = prev })
	ktx.BindTo(context.Background(), (*context.Context)(nil))
	ktx.Bind(out, s)
	err = ktx.Run()
	return buf.String(), err
}

func localServices(t *testing.T) *reconcileServices {
	t.Helper()
	return &reconcileServices{
		Meta:  reconcileMeta(t),
		Local: func() (reconclient.Local, error) { return emptyLocal{}, nil },
	}
}

func TestReconcileDiff_Output(t *testing.T) {
	srv := oneTicketRemote(t, "Bearer s3cret")
	t.Setenv("RELA_RECONCILE_TOKEN", "s3cret")
	run := func(args ...string) (string, error) {
		return runReconcile(t, localServices(t),
			append([]string{"reconcile", "diff", "--token-env", "RELA_RECONCILE_TOKEN"}, args...)...)
	}

	got, err := run("--remote", srv.URL)
	require.NoError(t, err)
	require.Contains(t, got, "Only on the remote (1)\n  - ticket/TKT-1\n")
	require.NotContains(t, got, "s3cret")

	got, err = runReconcile(t, localServices(t), "-o", "json", "reconcile", "diff", "--remote", srv.URL,
		"--types", "ticket", "--token-env", "RELA_RECONCILE_TOKEN")
	require.NoError(t, err)
	var doc reconcileJSON
	require.NoError(t, json.Unmarshal([]byte(got), &doc))
	require.False(t, doc.Same)
	require.Len(t, doc.OnlyRemote, 1)
	require.Equal(t, "ticket/TKT-1", doc.OnlyRemote[0].Address)
	require.Equal(t, srv.URL, doc.Remote)
	require.NotNil(t, doc.OnlyLocal)

	// With --exit-code a difference exits 1 and prints nothing to stderr.
	_, err = run("--remote", srv.URL, "--exit-code")
	var exit *relaerrors.ExitError
	require.ErrorAs(t, err, &exit)
	require.Equal(t, 1, exit.Code)

	// No difference in a scope without tickets.
	got, err = run("--remote", srv.URL, "--types", "feature", "--exit-code")
	require.NoError(t, err)
	require.Contains(t, got, "No differences.")
}

func TestReconcileDiff_Credentials(t *testing.T) {
	srv := oneTicketRemote(t, "Bearer from-file")
	// A default variable is never read: the credential source is named.
	t.Setenv("RELA_RECONCILE_TOKEN", "from-file")

	// Without a credential the remote refuses, and the error says so.
	_, err := runReconcile(t, localServices(t), "reconcile", "diff", "--remote", srv.URL)
	require.ErrorContains(t, err, "authentication required")

	// --exit-code turns an error into exit 2.
	_, err = runReconcile(t, localServices(t), "reconcile", "diff", "--remote", srv.URL, "--exit-code")
	var exit *relaerrors.ExitError
	require.ErrorAs(t, err, &exit)
	require.Equal(t, 2, exit.Code)

	file := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(file, []byte("from-file\n"), 0o600))
	_, err = runReconcile(t, localServices(t), "reconcile", "diff", "--remote", srv.URL, "--token-file", file)
	require.NoError(t, err)

	t.Setenv("PEER_TOKEN", "from-file")
	_, err = runReconcile(t, localServices(t), "reconcile", "diff", "--remote", srv.URL, "--token-env", "PEER_TOKEN")
	require.NoError(t, err)

	empty := filepath.Join(t.TempDir(), "empty")
	require.NoError(t, os.WriteFile(empty, nil, 0o600))
	_, err = runReconcile(t, localServices(t), "reconcile", "diff", "--remote", srv.URL, "--token-file", empty)
	require.ErrorContains(t, err, "--token-file: no credential")

	_, err = runReconcile(t, localServices(t), "reconcile", "diff", "--remote", srv.URL, "--token-env", "UNSET_X")
	require.ErrorContains(t, err, "$UNSET_X: no credential")

	big := filepath.Join(t.TempDir(), "big")
	require.NoError(t, os.WriteFile(big, bytes.Repeat([]byte("a"), maxTokenFile+1), 0o600))
	_, err = runReconcile(t, localServices(t), "reconcile", "diff", "--remote", srv.URL, "--token-file", big)
	require.ErrorContains(t, err, "larger than")

	t.Setenv("PEER_TOKEN", "bad\x01value")
	_, err = runReconcile(t, localServices(t), "reconcile", "diff", "--remote", srv.URL, "--token-env", "PEER_TOKEN")
	require.ErrorContains(t, err, "$PEER_TOKEN")
	require.NotContains(t, err.Error(), "bad\x01value")

	// Plain http to a host that is not loopback is refused.
	_, err = runReconcile(t, localServices(t), "reconcile", "diff", "--remote", "http://rela.example")
	require.ErrorContains(t, err, "refusing plain http")

	_, err = runReconcile(t, localServices(t), "reconcile", "diff", "--remote", srv.URL, "--max-round-trips", "0")
	require.ErrorContains(t, err, "--max-round-trips")
	_, err = runReconcile(t, localServices(t), "reconcile", "diff", "--remote", srv.URL, "--timeout", "-1s")
	require.ErrorContains(t, err, "--timeout")
}

// TestReconcileDiff_TokenSecret pins --token-secret: it reads only a
// remote_credential. name, through the global secrets of a real project
// directory, and never puts a value in an error (TKT-TB2B8N5).
func TestReconcileDiff_TokenSecret(t *testing.T) {
	srv := oneTicketRemote(t, "Bearer from-secret")
	relaDir := filepath.Join(t.TempDir(), ".rela")
	require.NoError(t, os.MkdirAll(relaDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(relaDir, "secrets.yaml"), []byte(
		"remote_credential.up: \"from-secret\\n\"\n"+
			"remote_credential.empty: \"\"\n"+
			"api_key: from-secret\n"+
			"token_key: from-secret\n"+
			"overrides:\n  x.lua:\n    remote_credential.other: leaked\n"), 0o600))
	withSecrets := func() *reconcileServices {
		s := localServices(t)
		s.Secret = func(name string) (string, error) { return globalSecret(relaDir, name) }
		return s
	}
	diff := func(s *reconcileServices, extra ...string) error {
		_, err := runReconcile(t, s, append([]string{"reconcile", "diff", "--remote", srv.URL}, extra...)...)
		return err
	}

	require.NoError(t, diff(withSecrets(), "--token-secret", "remote_credential.up"))

	for name, want := range map[string]string{
		"token_key":                 "must start with",
		"api_key":                   "must start with",
		"remote_credential.":        "must start with",
		"remote_credential.missing": `no secret "remote_credential.missing"`,
		"remote_credential.other":   `no secret "remote_credential.other"`,
		"remote_credential.empty":   "no credential",
	} {
		err := diff(withSecrets(), "--token-secret", name)
		require.ErrorContains(t, err, want, name)
		require.NotContains(t, err.Error(), "from-secret", name)
		require.NotContains(t, err.Error(), "leaked", name)
	}

	// A project with no secrets source names the flag in the error.
	noSource := localServices(t)
	noSource.Secret = func(name string) (string, error) { return globalSecret(t.TempDir(), name) }
	err := diff(noSource, "--token-secret", "remote_credential.up")
	require.ErrorContains(t, err, "--token-secret remote_credential.up: ")
	require.ErrorIs(t, err, secrets.ErrNotFound)

	// A project without secrets refuses the flag.
	require.ErrorContains(t, diff(localServices(t), "--token-secret", "remote_credential.up"),
		"this project has no secrets")

	// The credential sources exclude each other.
	t.Setenv("RELA_RECONCILE_TOKEN", "from-secret")
	require.ErrorContains(t, diff(withSecrets(), "--token-secret", "remote_credential.up",
		"--token-env", "RELA_RECONCILE_TOKEN"), "can't be used together")
}

func TestReconcileDiff_TokenFilePermissionsWarn(t *testing.T) {
	logs := captureLogs(t)
	file := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(file, []byte("tok"), 0o600))
	require.NoError(t, os.Chmod(file, 0o644))
	v, err := readTokenFile(file)
	require.NoError(t, err)
	require.Equal(t, "tok", v)
	require.Contains(t, logs.String(), "can be read by other users")
	require.NotContains(t, logs.String(), "tok\"")
}

func TestReconcileScope(t *testing.T) {
	meta := reconcileMeta(t)
	cases := []struct {
		name    string
		types   []string
		want    reconclient.Scope
		wantErr string
	}{
		{name: "every type", want: reconclient.Scope{
			Entities: []string{"feature", "note", "ticket"}, Relations: []string{"about", "implements"},
		}},
		{name: "entities only", types: []string{"ticket", " feature", "ticket"},
			want: reconclient.Scope{Entities: []string{"feature", "ticket"}}},
		{name: "relation with its ends", types: []string{"ticket", "feature", "implements"},
			want: reconclient.Scope{Entities: []string{"feature", "ticket"}, Relations: []string{"implements"}}},
		{name: "relation without an end", types: []string{"ticket", "implements"}, wantErr: "at its to end"},
		{name: "relation with one of its from types", types: []string{"note", "about"},
			want: reconclient.Scope{Entities: []string{"note"}, Relations: []string{"about"}}},
		{name: "unknown type", types: []string{"nope"}, wantErr: `"nope" is not an entity or relation type`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := reconcileScope(meta, tc.types)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
	_, err := reconcileScope(nil, nil)
	require.Error(t, err)
}

func TestReconcileDiff_LocalUnavailable(t *testing.T) {
	s := &reconcileServices{Meta: reconcileMeta(t), Local: func() (reconclient.Local, error) {
		return nil, errors.New("no ranges here")
	}}
	_, err := runReconcile(t, s, "reconcile", "diff", "--remote", "https://rela.example")
	require.ErrorContains(t, err, "no ranges here")
	_, err = runReconcile(t, &reconcileServices{Meta: reconcileMeta(t)}, "reconcile", "diff", "--remote",
		"https://rela.example")
	require.Error(t, err)
}

func TestReconcileText_QuotesUnsafeAddresses(t *testing.T) {
	var b bytes.Buffer
	res := reconclient.Result{
		OnlyLocal: []reconclient.Element{{Kind: reconclient.KindEntity, Address: "ticket/\x1b[2JX"}},
		Skipped: []reconclient.Skipped{{
			Kind: reconclient.KindRelation, Type: "implements", Remote: true, Reason: "gated\x1b",
		}},
	}
	require.NoError(t, writeReconcileText(&b, "https://rela.example", "", res))
	require.NotContains(t, b.String(), "\x1b")
	require.Contains(t, b.String(), `"ticket/\x1b[2JX"`)
	require.Contains(t, b.String(), "(no identity)")
	require.Contains(t, b.String(), "Not compared (1)")
}
