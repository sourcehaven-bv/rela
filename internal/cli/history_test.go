package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// historyTestServices builds filesystem-backed CLI bundles (whose store is
// NOT a store.HistoryReader), so the history/restore commands exercise their
// graceful "backend does not support history" degradation path. The
// pgstore-backed supported path is covered by the DB-gated pgstore tests.
func historyTestServices(t *testing.T) *cliBundles {
	t.Helper()
	meta, err := metamodel.Parse([]byte(testutil.SimpleMetamodelYAML()))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	b, err := newCLIBundles(appbuildtest.New(meta))
	if err != nil {
		t.Fatalf("newCLIBundles: %v", err)
	}
	return b
}

// captureOut redirects the package-level output writer to a buffer for the
// duration of the test and restores it after.
func captureOut(t *testing.T) *bytes.Buffer {
	t.Helper()
	prev := out
	buf := &bytes.Buffer{}
	out = output.NewWithWriter(buf, output.FormatTable)
	t.Cleanup(func() { out = prev })
	return buf
}

func TestHistoryCmd_UnsupportedBackend(t *testing.T) {
	buf := captureOut(t)
	svc := historyTestServices(t)

	cmd := &HistoryCmd{ID: "REQ-1"}
	if err := cmd.Run(context.Background(), svc.read); err != nil {
		t.Fatalf("HistoryCmd.Run: %v", err)
	}
	if !strings.Contains(buf.String(), "does not support version history") {
		t.Errorf("expected an unsupported-backend message, got: %q", buf.String())
	}
}

func TestRestoreCmd_UnsupportedBackend(t *testing.T) {
	buf := captureOut(t)
	svc := historyTestServices(t)

	cmd := &RestoreCmd{ID: "REQ-1", Version: 1}
	if err := cmd.Run(context.Background(), svc.write); err != nil {
		t.Fatalf("RestoreCmd.Run: %v", err)
	}
	if !strings.Contains(buf.String(), "does not support version history") {
		t.Errorf("expected an unsupported-backend message, got: %q", buf.String())
	}
}
