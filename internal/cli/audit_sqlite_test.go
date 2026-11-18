//go:build sqlite

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/audit/sqliteaudit"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/sqlexec/sqliteexec"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// TestAuditCmds_SQLite runs import, verify and prune against a sqlite log.
func TestAuditCmds_SQLite(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: filepath.Join(t.TempDir(), "rela.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	log, err := sqliteaudit.New(db.DB())
	require.NoError(t, err)
	svc := &writeServices{AuditLog: log}

	old := time.Now().UTC().AddDate(-4, 0, 0)
	dir := t.TempDir()
	var body []byte
	for _, op := range []string{audit.OpCreateEntity, audit.OpUpdateEntity} {
		line, mErr := json.Marshal(audit.Record{Time: old, Op: op})
		require.NoError(t, mErr)
		body = append(append(body, line...), '\n')
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "old.jsonl"), body, 0o600))

	captureOut(t)
	require.NoError(t, (&AuditImportCmd{Paths: []string{dir}}).Run(ctx, svc))

	// Imported rows are appended at their own time, so prune takes them.
	require.NoError(t, (&AuditPruneCmd{KeepDays: 1095}).Run(ctx, svc))

	buf := captureOut(t)
	prev := out
	out = output.NewWithWriter(buf, output.FormatJSON)
	t.Cleanup(func() { out = prev })
	require.NoError(t, (&AuditVerifyCmd{}).Run(ctx, svc))
	raw, err := sqliteexec.New(db.DB())
	require.NoError(t, err)
	var got auditVerifyJSON
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.True(t, got.OK)
	require.Equal(t, int64(2), got.Anchor)
	require.Equal(t, int64(2), got.Rows, "the import record and the prune record")
	require.Equal(t, int64(4), got.HeadSeq)

	head := fmt.Sprintf("%d:%s", got.HeadSeq, got.HeadHash)
	require.NoError(t, (&AuditVerifyCmd{ExpectHead: head}).Run(ctx, svc))
	require.ErrorContains(t, (&AuditVerifyCmd{ExpectHead: "4:00"}).Run(ctx, svc), "chain was rewritten")
	require.ErrorContains(t, (&AuditVerifyCmd{ExpectHead: "9:00"}).Run(ctx, svc), "newer rows were deleted")
	require.ErrorContains(t, (&AuditVerifyCmd{ExpectHead: "1:00"}).Run(ctx, svc), "was pruned")

	_, err = raw.Exec(ctx, `UPDATE audit_log SET doc = '{}' WHERE seq = 3`)
	require.NoError(t, err)
	require.ErrorContains(t, (&AuditVerifyCmd{}).Run(ctx, svc), "broken at seq 3")
}
