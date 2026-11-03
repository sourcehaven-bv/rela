package lua_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// taggedReader serves fixed tagged rows over an EntityReader and records the
// arguments it was asked with.
type taggedReader struct {
	lua.EntityReader
	rows  []store.TaggedEntity
	err   error
	asked *[]string
}

func (r taggedReader) TaggedEntities(
	_ context.Context, name store.VersionTagName, entityType string,
) ([]store.TaggedEntity, error) {
	*r.asked = append(*r.asked, name.String()+"|"+entityType)
	return r.rows, r.err
}

func TestTaggedEntities_Binding(t *testing.T) {
	rows := []store.TaggedEntity{
		{Ref: entity.Ref{ID: "M-1"}, Type: "maatregel", Version: 3},
		{Ref: entity.Ref{ID: "M-2", Face: "draft"}, Type: "maatregel", Version: 1, Deleted: true},
	}
	tests := []struct {
		name      string
		src       string
		err       error
		wantOut   []string
		wantAsked string
		wantErr   string
	}{
		{
			name: "rows",
			src: `for _, r in ipairs(rela.tagged_entities("baseline/audit-2026")) do
  rela.output(r.id .. "|" .. r.face .. "|" .. r.addr .. "|" .. r.type .. "|" .. r.version .. "|" .. tostring(r.deleted))
end`,
			wantOut:   []string{"M-1||M-1|maatregel|3|false", "M-2|draft|M-2@draft|maatregel|1|true"},
			wantAsked: "baseline/audit-2026|",
		},
		{
			name:      "type filter",
			src:       `rela.output("n=" .. #rela.tagged_entities("baseline/audit-2026", {type = "maatregel"}))`,
			wantOut:   []string{"n=2"},
			wantAsked: "baseline/audit-2026|maatregel",
		},
		{
			name:      "nil opts",
			src:       `rela.output("n=" .. #rela.tagged_entities("baseline/audit-2026", nil))`,
			wantOut:   []string{"n=2"},
			wantAsked: "baseline/audit-2026|",
		},
		{name: "bad name", src: `rela.tagged_entities("Bad Name!")`, wantErr: "rela.tagged_entities"},
		{name: "missing name", src: `rela.tagged_entities()`, wantErr: "string expected"},
		{name: "unknown option", src: `rela.tagged_entities("x", {face = "draft"})`, wantErr: "unknown option face"},
		{name: "empty type", src: `rela.tagged_entities("x", {type = ""})`, wantErr: "type must be a non-empty string"},
		{name: "non-string type", src: `rela.tagged_entities("x", {type = 3})`, wantErr: "type must be a non-empty string"},
		{
			name: "unsupported", src: `rela.tagged_entities("x")`,
			err: store.ErrHistoryUnsupported, wantErr: store.ErrHistoryUnsupported.Error(), wantAsked: "x|",
		},
		{
			name: "other errors are not echoed", src: `rela.tagged_entities("x")`,
			err: errors.New("pq: secret dsn detail"), wantErr: "rela.tagged_entities", wantAsked: "x|",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, deps, decl := newACLWorld(t)
			var asked []string
			deps.VisibleReader = taggedReader{EntityReader: deps.VisibleReader, rows: rows, err: tc.err, asked: &asked}
			if tc.wantErr != "" {
				err := runAsAliceErr(t, decl, deps, tc.src)
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
				}
				if strings.Contains(err.Error(), "secret dsn") {
					t.Errorf("err %q echoes the store error", err)
				}
			} else {
				out := runAsAlice(t, decl, deps, tc.src)
				for _, want := range tc.wantOut {
					if !strings.Contains(out, want) {
						t.Errorf("output %q lacks %q", out, want)
					}
				}
			}
			if got := strings.Join(asked, ","); got != tc.wantAsked {
				t.Errorf("asked %q, want %q", got, tc.wantAsked)
			}
		})
	}
}

// TestTaggedEntities_ReaderWithoutListingRaises: a VisibleReader that cannot
// list tags raises like a backend without history.
func TestTaggedEntities_ReaderWithoutListingRaises(t *testing.T) {
	_, deps, decl := newACLWorld(t)
	deps.VisibleReader = readerOnly{deps.VisibleReader}
	err := runAsAliceErr(t, decl, deps, `rela.tagged_entities("baseline/x")`)
	if err == nil || !strings.Contains(err.Error(), store.ErrHistoryUnsupported.Error()) {
		t.Errorf("err = %v", err)
	}
}
