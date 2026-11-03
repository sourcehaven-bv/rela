package aclmap_test

import (
	"context"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclmap"
)

// TestVerbs_FollowsOpsExceptRename pins the report verb set: read, then every
// write Op except rename (TKT-4APGVF), so a new Op is reported without a change
// here.
func TestVerbs_FollowsOpsExceptRename(t *testing.T) {
	want := []acl.Verb{acl.VerbRead}
	for _, op := range acl.Ops() {
		if op != acl.OpRename {
			want = append(want, acl.Verb(op))
		}
	}
	if got := aclmap.Verbs(); !slices.Equal(got, want) {
		t.Errorf("Verbs() = %v, want %v", got, want)
	}
}

// TestReports_RejectRename: rename is a valid acl.Verb, but the reports refuse
// it until their grant helpers answer it as the runtime does.
func TestReports_RejectRename(t *testing.T) {
	w := groundingWorld(t)
	ctx := context.Background()
	if _, err := w.eng.Can(ctx, "PERS-ALICE", acl.VerbRename, "INC-042"); err == nil {
		t.Error("Can(rename) succeeded, want an unknown-verb error")
	}
	if _, err := w.eng.WhoCan(ctx, acl.VerbRename, "INC-042"); err == nil {
		t.Error("WhoCan(rename) succeeded, want an unknown-verb error")
	}
	if _, err := w.eng.MapAll(ctx, acl.VerbRename, "", groundingTypes); err == nil {
		t.Error("MapAll(rename) succeeded, want an unknown-verb error")
	}
}
