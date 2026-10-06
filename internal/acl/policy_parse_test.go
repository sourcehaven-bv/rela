package acl_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// ParsePolicy names its source in errors, so a policy read from a database
// rather than a path is still diagnosable.
func TestParsePolicy(t *testing.T) {
	if _, err := acl.ParsePolicy([]byte("roles: {}\n"), "acl.yaml"); err != nil {
		t.Fatalf("valid policy: %v", err)
	}
	_, err := acl.ParsePolicy([]byte("roles: [broken\n"), "baked/acl.yaml")
	if err == nil || !strings.Contains(err.Error(), "baked/acl.yaml") {
		t.Fatalf("err = %v, want a parse error naming the source", err)
	}
}
