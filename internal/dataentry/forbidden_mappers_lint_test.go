package dataentry

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestForbiddenMappersHandleIdentityFaults keeps BUG-O51NS5 from coming back
// through a new error mapper. A file that unwraps an *acl.ForbiddenError
// must route it through writeIdentityFaultDeny (or failACLDenied, its
// comments-transport twin), so a write denied for a missing identity answers
// 401 and a mismatched one 500, never the handler's own 403.
//
// Exempt files name their reason. Add one only with an argued reason.
func TestForbiddenMappersHandleIdentityFaults(t *testing.T) {
	t.Parallel()
	// A declaration or type assertion that unwraps the error; prose that
	// merely names the type does not count.
	needle := regexp.MustCompile(`var \w+ \*acl\.ForbiddenError|AsType\[\*acl\.ForbiddenError\]`)
	handlers := []string{"writeIdentityFaultDeny(", "writeForbiddenIfACLDenied(", "failACLDenied("}
	exempt := map[string]string{
		"settings_handlers.go": "defines writeIdentityFaultDeny and writeForbiddenIfACLDenied",
		"caldav_write.go": "CalDAV answers WebDAV statuses on its own edge; a denied create is " +
			"the uniform not-found there, which a 401 would not improve",
		"attach_render.go": "turns a refusal into a Lua script's error message, not an HTTP " +
			"status; the script's own request answers the status",
	}

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		if _, ok := exempt[f]; ok {
			continue
		}
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		src := string(body)
		if !needle.MatchString(src) {
			continue
		}
		handled := false
		for _, h := range handlers {
			if strings.Contains(src, h) {
				handled = true
				break
			}
		}
		if !handled {
			t.Errorf("%s unwraps *acl.ForbiddenError without writeIdentityFaultDeny; a write denied "+
				"for a missing identity would answer 403 instead of 401 (BUG-O51NS5)", f)
		}
	}
}
