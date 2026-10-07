package entitymanager

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// A failure on an owned entity must not echo text naming that entity or its
// neighbors, which the caller may not be able to read.
func TestOwnedErrors_WithholdDetail(t *testing.T) {
	inner := errors.New("resolve source of CHILD-1 --blocks--> HIDDEN-2")
	for name, err := range map[string]error{
		"delete":  ownedDeleteError("OWNER-1", inner),
		"restore": ownedRestoreError("OWNER-1", inner),
	} {
		t.Run(name, func(t *testing.T) {
			msg := err.Error()
			if !strings.Contains(msg, "OWNER-1") || strings.Contains(msg, "CHILD-1") || strings.Contains(msg, "HIDDEN-2") {
				t.Fatalf("error = %q, want the owner id and no other id", msg)
			}
		})
	}
	if err := ownedDeleteError("OWNER-1", context.Canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want it to wrap context.Canceled", err)
	}
}

// TestMarkedTogether pins which marks a restore of the owner treats as taken
// with the owner's: the same deleter, within coMarkWindow either way. A small
// negative delta happens on fsstore, which keeps wall-clock time only.
func TestMarkedTogether(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	owner := store.MarkedEntity{ID: "OWNER-1", DeletedBy: "alice", DeletedAt: at}
	tests := []struct {
		name string
		by   string
		at   time.Time
		want bool
	}{
		{name: "same deleter, same instant", by: "alice", at: at, want: true},
		{name: "same deleter, just after", by: "alice", at: at.Add(time.Millisecond), want: true},
		{name: "child marked slightly before owner", by: "alice", at: at.Add(-50 * time.Millisecond), want: true},
		{name: "different deleter not restored", by: "bob", at: at, want: false},
		{name: "marked again long after", by: "alice", at: at.Add(coMarkWindow + time.Second), want: false},
		{name: "marked long before", by: "alice", at: at.Add(-coMarkWindow - time.Second), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			child := store.MarkedEntity{ID: "CHILD-1", DeletedBy: tt.by, DeletedAt: tt.at}
			if got := markedTogether(owner, child); got != tt.want {
				t.Fatalf("markedTogether = %v, want %v", got, tt.want)
			}
		})
	}
}
