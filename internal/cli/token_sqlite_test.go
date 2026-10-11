//go:build sqlite

package cli

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

const tokenCLIConnections = `connections:
  crm:
    token_url: https://crm.example.com/oauth/token
    client_id_secret: crm_id
    client_secret_secret: crm_secret
    style: rfc6749
`

func tokenKeyYAML(b byte) string {
	return "token_key: " + base64.StdEncoding.EncodeToString([]byte(strings.Repeat(string(b), 32))) + "\n"
}

// openTokenProject opens root as a sqlite project and returns its bundles.
// The caller closes the returned Services before reopening the project.
func openTokenProject(t *testing.T, root string) (*appbuild.Services, *cliBundles) {
	t.Helper()
	svc, err := appbuild.Discover(root, script.NewEngine())
	require.NoError(t, err)
	b, err := newCLIBundles(svc)
	require.NoError(t, err)
	return svc, b
}

func runTokenStatus(t *testing.T, b *cliBundles) []tokenStatusJSON {
	t.Helper()
	buf := withOutput(t, output.FormatJSON)
	require.NoError(t, (&TokenStatusCmd{}).Run(tagCLICtx(), b.write))
	var rows []tokenStatusJSON
	require.NoError(t, json.Unmarshal(buf.Bytes(), &rows), buf.String())
	return rows
}

// TestTokenCmd_SQLite drives set, status and delete, and checks the
// values never reach the output or the audit log.
func TestTokenCmd_SQLite(t *testing.T) {
	root := t.TempDir()
	rela := filepath.Join(root, ".rela")
	require.NoError(t, os.MkdirAll(rela, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "metamodel.yaml"), []byte(tagCLIMetamodel), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "connections.yaml"), []byte(tokenCLIConnections), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(rela, "secrets.yaml"), []byte(tokenKeyYAML('a')), 0o600))

	svc, b := openTokenProject(t, root)
	sink := audit.NewMemory()
	b.write.Audit = sink

	rows := runTokenStatus(t, b)
	require.Len(t, rows, 1)
	require.Equal(t, "missing", rows[0].State)

	prev := tokenStdin
	t.Cleanup(func() { tokenStdin = prev })
	tokenStdin = strings.NewReader(`{"refresh_token":"R-secret","access_token":"A-secret","expires_in":3600}`)
	buf := captureOut(t)
	require.NoError(t, (&TokenSetCmd{Name: "crm"}).Run(tagCLICtx(), b.write))
	require.NotContains(t, buf.String(), "secret")

	rows = runTokenStatus(t, b)
	require.Equal(t, "ok", rows[0].State)
	require.NotEmpty(t, rows[0].ExpiresAt)
	raw, _ := json.Marshal(rows)
	require.NotContains(t, string(raw), "secret")

	tokenStdin = strings.NewReader("R2")
	require.Error(t, (&TokenSetCmd{Name: "undeclared"}).Run(tagCLICtx(), b.write),
		"set must refuse a connection connections.yaml does not declare")

	recs := sink.Records()
	require.Len(t, recs, 1)
	require.Equal(t, audit.OpTokenSet, recs[0].Op)
	require.Equal(t, "connection=crm", recs[0].Summary)
	require.NoError(t, svc.Close())

	// A different key cannot open the token; status says so.
	require.NoError(t, os.WriteFile(filepath.Join(rela, "secrets.yaml"), []byte(tokenKeyYAML('b')), 0o600))
	svc, b = openTokenProject(t, root)
	rows = runTokenStatus(t, b)
	require.Equal(t, "unreadable", rows[0].State)
	require.Contains(t, rows[0].Error, "different token_key")

	b.write.Audit = sink
	captureOut(t)
	require.NoError(t, (&TokenDeleteCmd{Name: "crm"}).Run(tagCLICtx(), b.write))
	require.Equal(t, "missing", runTokenStatus(t, b)[0].State)
	require.Equal(t, audit.OpTokenDelete, sink.Records()[1].Op)
	require.NoError(t, svc.Close())
}

// Without a token_key the project has no token store, and the command
// says how to make one.
func TestTokenCmd_SQLiteNoKey(t *testing.T) {
	t.Setenv("RELA_TOKEN_KEY", "")
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".rela"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "metamodel.yaml"), []byte(tagCLIMetamodel), 0o600))
	svc, b := openTokenProject(t, root)
	defer svc.Close()
	err := (&TokenStatusCmd{}).Run(tagCLICtx(), b.write)
	require.ErrorContains(t, err, "token_key")
}

func TestTokenLockHint(t *testing.T) {
	inUse := fmt.Errorf("open: %w", sqlitedb.ErrInUse)
	require.Contains(t, tokenLockHint("token set <name>", inUse).Error(), "Stop it")
	require.Equal(t, inUse, tokenLockHint("history <id>", inUse))
	other := errors.New("boom")
	require.Equal(t, other, tokenLockHint("token status", other))
}
