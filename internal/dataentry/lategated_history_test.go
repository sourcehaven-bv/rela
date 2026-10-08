package dataentry

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestLateGatedReader_ServesHistory: the reader validation rules run on
// forwards history to the gated reader, so a rule on a versioned backend is
// not told the backend keeps none (TKT-EC7F65).
func TestLateGatedReader_ServesHistory(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "now"}})
	rd := lateGatedReader{app: app}
	ctx := context.Background()

	app.versions = nil
	if _, err := rd.EntityVersions(ctx, "TKT-001"); !errors.Is(err, store.ErrHistoryUnsupported) {
		t.Fatalf("without versions: err = %v, want ErrHistoryUnsupported", err)
	}

	app.versions = historyStore{versions: map[string][]store.VersionSnapshot{
		"TKT-001": {snapshot("ticket", "then", map[string]any{"title": "then"})},
	}}
	metas, err := rd.EntityVersions(ctx, "TKT-001")
	if err != nil || len(metas) != 1 {
		t.Fatalf("EntityVersions = %v, %v; want one version", metas, err)
	}
	e, _, err := rd.EntityVersion(ctx, "TKT-001", 1)
	if err != nil || e.GetString("title") != "then" {
		t.Fatalf("EntityVersion = %v, %v; want title then", e, err)
	}
}
