package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

func TestHistoryTagCmd_UnsupportedBackend(t *testing.T) {
	buf := captureOut(t)
	svc := historyTestServices(t)
	cmd := &HistoryTagCmd{ID: "REQ-1", Name: "reviewed"}
	if err := cmd.Run(context.Background(), svc.write); err != nil {
		t.Fatalf("HistoryTagCmd.Run: %v", err)
	}
	if !strings.Contains(buf.String(), "does not support version tags") {
		t.Errorf("expected an unsupported-backend message, got: %q", buf.String())
	}
}

func TestHistoryTagCmd_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     HistoryTagCmd
		wantErr string
	}{
		{name: "current", cmd: HistoryTagCmd{}},
		{name: "version", cmd: HistoryTagCmd{Version: 2}},
		{name: "expect", cmd: HistoryTagCmd{Expect: "tok"}},
		{name: "delete", cmd: HistoryTagCmd{Delete: true}},
		{name: "negative version", cmd: HistoryTagCmd{Version: -1}, wantErr: "positive"},
		{name: "version and expect", cmd: HistoryTagCmd{Version: 1, Expect: "tok"}, wantErr: "not both"},
		{name: "delete with version", cmd: HistoryTagCmd{Delete: true, Version: 1}, wantErr: "--delete"},
		{name: "delete with expect", cmd: HistoryTagCmd{Delete: true, Expect: "tok"}, wantErr: "--delete"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cmd.validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("validate: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("validate err = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestCommitRefused(t *testing.T) {
	target := []store.PurgeTarget{{Vseq: 1}}
	tests := []struct {
		name    string
		res     store.PurgeResult
		wantErr string
	}{
		{name: "ran", res: store.PurgeResult{Targets: target, Purged: 1}},
		{name: "nothing matched", res: store.PurgeResult{}},
		{
			name:    "refused by the store",
			res:     store.PurgeResult{Targets: target, Refusal: store.PurgeRefusedTagged},
			wantErr: string(store.PurgeRefusedTagged),
		},
		{name: "targets but no rows deleted", res: store.PurgeResult{Targets: target}, wantErr: "deleted none"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := commitRefused(&tc.res, "DOC-1")
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("commitRefused = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("commitRefused = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}
