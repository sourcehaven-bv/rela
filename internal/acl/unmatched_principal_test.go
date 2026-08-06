package acl_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// unmatched_principal policy key (TKT-0C3II2). Covers Validate (AC5/AC6) and
// the AuthorizeWrite reject/provision branch (AC1/AC2 unit level; the
// multi-write-path integration lives in internal/dataentry).

func TestUnmatchedPrincipal_ValidatesEnum(t *testing.T) {
	t.Parallel()
	// A valid mode needs the principal_property lookup configured, so include
	// it for the accepted cases.
	const lookup = "user_entity_type: person\nprincipal_property: sub\n"

	for _, tc := range []struct {
		name    string
		yaml    string
		wantErr string // "" ⇒ must load clean
	}{
		{"absent", "roles: {}", ""},
		{"anonymous", lookup + "unmatched_principal: anonymous", ""},
		{"reject", lookup + "unmatched_principal: reject", ""},
		{"provision reserved", lookup + "unmatched_principal: provision", ""},
		{"unknown value", lookup + "unmatched_principal: banish", "not a valid mode"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := acl.LoadPolicyBytes([]byte(tc.yaml))
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("want clean load, got %v", err)
			case tc.wantErr != "" && err == nil:
				t.Fatalf("want error containing %q, got nil", tc.wantErr)
			case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
				t.Errorf("error = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestUnmatchedPrincipal_RejectRequiresLookup(t *testing.T) {
	t.Parallel()
	// AC5: reject (and provision) without principal_property + user_entity_type
	// is a LOAD error — otherwise the lookup is disabled and every principal
	// looks unmatched, so it would reject everyone.
	for _, tc := range []struct {
		name, yaml string
		wantErr    bool
	}{
		{"reject, no lookup at all", "unmatched_principal: reject", true},
		{"reject, only user_entity_type", "user_entity_type: person\nunmatched_principal: reject", true},
		{"reject, only principal_property", "principal_property: sub\nunmatched_principal: reject", true},
		{"reject, both set", "user_entity_type: person\nprincipal_property: sub\nunmatched_principal: reject", false},
		{"provision, no lookup", "unmatched_principal: provision", true},
		{"anonymous, no lookup (fine)", "unmatched_principal: anonymous", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := acl.LoadPolicyBytes([]byte(tc.yaml))
			if tc.wantErr && err == nil {
				t.Fatal("want a load error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("want clean load, got %v", err)
			}
			if tc.wantErr && err != nil && !strings.Contains(err.Error(), "requires") {
				t.Errorf("error = %v, want it to explain the requirement", err)
			}
		})
	}
}

// rejectWorld builds a Declarative over an empty store with a policy that has
// the lookup configured and the given unmatched_principal mode.
func rejectWorld(t *testing.T, mode string) *acl.Declarative {
	t.Helper()
	yaml := "user_entity_type: person\nprincipal_property: sub\n" +
		"roles:\n  editor: {read: [ticket], create: [ticket], update: [ticket]}\n" +
		"unmatched_principal: " + mode + "\n"
	p, err := acl.LoadPolicyBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}
	st := memstore.New()
	d, err := acl.NewDeclarative(p, acl.NewStoreGraph(st), st,
		acl.WithPrincipalLookup(acl.NewStorePrincipalLookup(st)))
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	return d
}

func writeReq() acl.WriteRequest {
	return acl.WriteRequest{
		Op:      acl.OpUpdate,
		Subject: acl.EntitySubject{Type: "ticket", ID: "TKT-1"},
	}
}

func TestUnmatchedPrincipal_RejectDeniesFlaggedWrite(t *testing.T) {
	t.Parallel()
	d := rejectWorld(t, acl.UnmatchedReject)

	// A verified principal marked unmatched (the data-entry middleware sets
	// this) is denied its write.
	ctx := acl.WithUnmatchedVerified(
		principal.With(context.Background(),
			principal.Verified("usr_nobody", principal.ToolDataEntry, "", "", nil)))

	dec := d.AuthorizeWrite(ctx, writeReq())
	if dec.Allow {
		t.Fatal("reject: an unmatched verified write was allowed")
	}
	if dec.RuleKind != "unmatched-principal" {
		t.Errorf("RuleKind = %q, want unmatched-principal", dec.RuleKind)
	}
}

func TestUnmatchedPrincipal_RejectIgnoresUnflagged(t *testing.T) {
	t.Parallel()
	// The flag is the whole gate: a principal NOT marked unmatched-verified
	// (header, CLI, scheduler, or a JWT principal that DID resolve) is judged
	// on roles alone, exactly as without the feature. AC4/AC8.
	d := rejectWorld(t, acl.UnmatchedReject)

	// No WithUnmatchedVerified. This principal has no role grant either, so it
	// is denied — but with a ROLE-GRANT denial, not the unmatched gate.
	ctx := principal.With(context.Background(),
		principal.Principal{User: "system:scheduler", Tool: principal.ToolScheduler})

	dec := d.AuthorizeWrite(ctx, writeReq())
	if dec.RuleKind == "unmatched-principal" {
		t.Error("an unflagged principal was rejected by the unmatched gate — " +
			"the flag is not gating correctly (would hit scheduler/header/CLI)")
	}
}

func TestUnmatchedPrincipal_AnonymousDoesNotReject(t *testing.T) {
	t.Parallel()
	// AC1: under anonymous (and no key), a flagged unmatched principal is NOT
	// rejected by the gate — it falls through to normal role evaluation.
	for _, mode := range []string{acl.UnmatchedAnonymous, acl.UnmatchedProvision} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			d := rejectWorld(t, mode)
			ctx := acl.WithUnmatchedVerified(
				principal.With(context.Background(),
					principal.Verified("usr_nobody", principal.ToolDataEntry, "", "", nil)))

			dec := d.AuthorizeWrite(ctx, writeReq())
			if dec.RuleKind == "unmatched-principal" {
				t.Errorf("mode %q rejected via the unmatched gate; only reject should", mode)
			}
		})
	}
}
