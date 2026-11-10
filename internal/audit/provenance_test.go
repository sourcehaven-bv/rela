package audit

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

func TestRecordSource(t *testing.T) {
	t.Parallel()
	cli := principal.Principal{User: "alice", Tool: principal.ToolCLI}
	tests := []struct {
		name string
		rec  Record
		want string
	}{
		{"direct write uses the tool", Record{Principal: cli}, principal.ToolCLI},
		{"flow step wins", Record{Principal: cli, Flow: "review", TriggeredBy: "automation:x"}, SourceFlow},
		{"named automation", Record{Principal: cli, TriggeredBy: "automation:on-create"}, SourceAutomation},
		{"unnamed automation", Record{Principal: cli, TriggeredBy: "automation"}, SourceAutomation},
		{"automation job", Record{Principal: cli, TriggeredBy: "automation-job:notify;by=bob"}, SourceAutomation},
		{"schedule", Record{Principal: cli, TriggeredBy: "schedule:nightly"}, SourceSchedule},
		{"entity cascade", Record{Principal: cli, TriggeredBy: "cascade:delete-entity:REQ-1"}, SourceCascade},
		{"owner cascade", Record{Principal: cli, TriggeredBy: "cascade:owner-delete:REQ-1"}, SourceCascade},
		{"restore", Record{Principal: cli, TriggeredBy: "restore-entity:REQ-1"}, SourceRestore},
		{"renumber", Record{Principal: cli, TriggeredBy: "renumber:rank"}, SourceRenumber},
		{"channel promote", Record{Principal: cli, TriggeredBy: "channel:k#1/promote"}, SourceChannel},
		{"channel output", Record{Principal: cli, TriggeredBy: "session:k/a->b#1/0"}, SourceChannel},
		{"channel reminder", Record{Principal: cli, TriggeredBy: "reminder:r1"}, SourceChannel},
		{"channel timer", Record{Principal: cli, TriggeredBy: "timer:t/open/nudge"}, SourceChannel},
		{"channel trigger rule", Record{Principal: cli, TriggeredBy: "trigger:escalate"}, SourceChannel},
		{"channel auto operation", Record{Principal: cli, TriggeredBy: "auto:t/close"}, SourceChannel},
		{"rename record", Record{Principal: cli, TriggeredBy: "rename:A->B"}, SourceRename},
		{"rename cascade", Record{Principal: cli, TriggeredBy: "rename-entity:A->B"}, SourceRename},
		{"conversion", Record{Principal: cli, TriggeredBy: "convert-entity:A->B"}, SourceConvert},
		{"unknown kind falls back to the tool", Record{Principal: cli, TriggeredBy: "something:else"}, principal.ToolCLI},
		{"no tool, no trigger", Record{}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := SourceOf(tc.rec); got != tc.want {
				t.Errorf("SourceOf = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStamp(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if got := Stamp(ctx, Record{Op: OpCreateEntity}); got.Flow != "" || got.Run != "" || got.Step != "" {
		t.Errorf("Stamp without a flow step = %+v, want no flow fields", got)
	}
	ctx = WithFlowStep(ctx, FlowStep{Flow: "review", Run: "RUN-1", Step: "approve", By: "alice"})
	got := Stamp(ctx, Record{Op: OpCreateEntity})
	if got.Flow != "review" || got.Run != "RUN-1" || got.Step != "approve" || got.By != "alice" {
		t.Errorf("Stamp = %+v, want review/RUN-1/approve by alice", got)
	}

	m := NewMemory()
	m.Record(got)
	m.Record(Record{Principal: principal.Principal{Tool: principal.ToolMCP}})
	recs := m.Records()
	if recs[0].Source != SourceFlow || recs[1].Source != principal.ToolMCP {
		t.Errorf("sources = %q, %q; want flow, mcp", recs[0].Source, recs[1].Source)
	}
}
