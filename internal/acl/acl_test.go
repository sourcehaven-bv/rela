package acl_test

import (
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// AC1.1: ForbiddenError satisfies errors.Is(err, ErrForbidden).
func TestForbiddenError_IsErrForbidden(t *testing.T) {
	err := &acl.ForbiddenError{Decision: acl.Decision{
		Allow:    false,
		RuleKind: "role-grant",
		RuleID:   "viewer",
		Reason:   "no role grants write on type 'ticket'",
	}}

	if !errors.Is(err, acl.ErrForbidden) {
		t.Fatalf("errors.Is(err, ErrForbidden) = false, want true")
	}
}

// ForbiddenError.Error includes rule context so logs are debuggable.
func TestForbiddenError_ErrorString(t *testing.T) {
	err := &acl.ForbiddenError{Decision: acl.Decision{
		RuleKind: "role-grant",
		RuleID:   "viewer",
		Reason:   "no role grants write on type 'ticket'",
	}}

	got := err.Error()
	wantContains := []string{"forbidden", "no role grants", "role-grant", "viewer"}
	for _, sub := range wantContains {
		if !contains(got, sub) {
			t.Errorf("error string %q missing %q", got, sub)
		}
	}
}

// errors.As unwraps to *ForbiddenError so HTTP handlers can read the
// Decision for structured 403 bodies.
func TestForbiddenError_AsExposesDecision(t *testing.T) {
	original := &acl.ForbiddenError{Decision: acl.Decision{
		RuleKind: "read-only",
		RuleID:   "read-only-acl",
		Reason:   "this rela instance is configured read-only",
	}}

	var got *acl.ForbiddenError
	if !errors.As(error(original), &got) {
		t.Fatalf("errors.As did not unwrap to *ForbiddenError")
	}
	if got.Decision.RuleKind != "read-only" {
		t.Errorf("Decision.RuleKind = %q, want %q", got.Decision.RuleKind, "read-only")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
