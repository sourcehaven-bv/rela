package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/audit"
)

func TestAuditCmds_NeedDatabaseLog(t *testing.T) {
	ctx := context.Background()
	svc := &writeServices{Audit: audit.Nop{}}
	require.ErrorIs(t, (&AuditVerifyCmd{}).Run(ctx, svc), errNoSQLAudit)
	require.ErrorIs(t, (&AuditPruneCmd{KeepDays: 1100}).Run(ctx, svc), errNoSQLAudit)
	require.ErrorIs(t, (&AuditImportCmd{Paths: []string{t.TempDir()}}).Run(ctx, svc), errNoSQLAudit)
}

func TestAuditPrune_RetentionFloor(t *testing.T) {
	ctx := context.Background()
	svc := &writeServices{Audit: audit.Nop{}}
	tests := []struct {
		name string
		cmd  AuditPruneCmd
		want string
	}{
		{"zero", AuditPruneCmd{KeepDays: 0}, "at least 1"},
		{"below the floor", AuditPruneCmd{KeepDays: 30}, "below the 1095-day"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorContains(t, tc.cmd.Run(ctx, svc), tc.want)
		})
	}
	short := AuditPruneCmd{KeepDays: 30, AllowShortRetention: true}
	require.ErrorIs(t, short.Run(ctx, svc), errNoSQLAudit, "the flag lifts the floor")
}

func TestJSONLFilesAndRecords(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
		return p
	}
	write("2026-11-02.jsonl", `{"op":"update-entity"}`+"\n")
	write("2026-11-01.jsonl", `{"op":"create-entity"}`+"\n\n"+`{"op":"delete-entity"}`+"\n")
	write("notes.txt", "ignored")
	single := write("extra.log", `{"op":"rename-entity"}`+"\n")

	files, err := jsonlFiles([]string{dir, single})
	require.NoError(t, err)
	require.Len(t, files, 3)
	require.True(t, strings.HasSuffix(files[0], "2026-11-01.jsonl"), "directories expand in name order")

	var ops []string
	for rec, err := range jsonlRecords(files) {
		require.NoError(t, err)
		ops = append(ops, rec.Op)
	}
	require.Equal(t, []string{"create-entity", "delete-entity", "update-entity", "rename-entity"}, ops)

	_, err = jsonlFiles([]string{filepath.Join(dir, "missing")})
	require.Error(t, err)

	bad := write("bad.jsonl", "{not json}\n")
	var gotErr error
	for _, err := range jsonlRecords([]string{bad}) {
		gotErr = err
	}
	require.ErrorContains(t, gotErr, "bad.jsonl: line 1")
}

func TestParseExpectHead(t *testing.T) {
	tests := []struct {
		in      string
		seq     int64
		hash    string
		wantErr bool
	}{
		{in: ""},
		{in: "12:ab", seq: 12, hash: "ab"},
		{in: "12", wantErr: true},
		{in: "0:ab", wantErr: true},
		{in: "x:ab", wantErr: true},
		{in: "12:", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			seq, hash, err := parseExpectHead(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.seq, seq)
			require.Equal(t, tc.hash, hash)
		})
	}
}
