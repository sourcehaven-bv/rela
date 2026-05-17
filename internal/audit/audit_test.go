package audit_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
)

func TestRecord_JSONRoundtrip(t *testing.T) {
	tests := []struct {
		name    string
		rec     audit.Record
		wantKey string
	}{
		{
			name: "entity record",
			rec: audit.Record{
				Time:      time.Date(2026, 5, 17, 8, 0, 0, 0, time.UTC),
				Op:        audit.OpCreateEntity,
				Subject:   audit.Subject{Kind: "entity", Type: "ticket", ID: "TKT-1"},
				Principal: audit.Principal{User: "alice", Tool: audit.ToolCLI},
				Summary:   "created",
			},
			wantKey: `"subject":{"kind":"entity"`,
		},
		{
			name: "relation record",
			rec: audit.Record{
				Time:      time.Date(2026, 5, 17, 8, 0, 0, 0, time.UTC),
				Op:        audit.OpCreateRelation,
				Subject:   audit.Subject{Kind: "relation", RelationType: "requires", FromID: "F-1", ToID: "C-2"},
				Principal: audit.Principal{User: "bob", Tool: audit.ToolMCP},
			},
			wantKey: `"from_id":"F-1"`,
		},
		{
			name: "rename record",
			rec: audit.Record{
				Time:      time.Date(2026, 5, 17, 8, 0, 0, 0, time.UTC),
				Op:        audit.OpRenameEntity,
				Before:    audit.Subject{Kind: "entity", Type: "ticket", ID: "TKT-OLD"},
				After:     audit.Subject{Kind: "entity", Type: "ticket", ID: "TKT-NEW"},
				Principal: audit.Principal{User: "carol", Tool: audit.ToolCLI},
			},
			wantKey: `"after":{"kind":"entity"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.rec)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if !strings.Contains(string(data), tt.wantKey) {
				t.Errorf("expected JSON to contain %q, got: %s", tt.wantKey, data)
			}
			var back audit.Record
			if err := json.Unmarshal(data, &back); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if back.Op != tt.rec.Op {
				t.Errorf("Op round-trip: got %q want %q", back.Op, tt.rec.Op)
			}
			if back.Principal != tt.rec.Principal {
				t.Errorf("Principal round-trip: got %+v want %+v", back.Principal, tt.rec.Principal)
			}
		})
	}
}

func TestRecord_OmitemptyOnOptionalFields(t *testing.T) {
	rec := audit.Record{
		Time:      time.Date(2026, 5, 17, 8, 0, 0, 0, time.UTC),
		Op:        audit.OpCreateEntity,
		Subject:   audit.Subject{Kind: "entity", Type: "ticket", ID: "TKT-1"},
		Principal: audit.Principal{User: "alice", Tool: audit.ToolCLI},
		// TriggeredBy and Summary left empty
	}
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "triggered_by") {
		t.Errorf("expected triggered_by to be omitted, got: %s", data)
	}
	if strings.Contains(string(data), "summary") {
		t.Errorf("expected summary to be omitted, got: %s", data)
	}
}
