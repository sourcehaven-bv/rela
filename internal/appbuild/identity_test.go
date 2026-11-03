package appbuild_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
)

func TestIdentityBinder_RejectsNil(t *testing.T) {
	t.Parallel()
	if bind, err := appbuild.IdentityBinder(nil); err == nil || bind != nil {
		t.Errorf("IdentityBinder(nil) = %v, %v; want an error", bind != nil, err)
	}
}
