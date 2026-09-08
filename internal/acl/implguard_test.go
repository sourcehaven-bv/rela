package acl

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// knownACLImplementations is the closed set of production [ACL]
// implementations. Adding one is a deliberate act with consequences beyond
// this package, so the set is pinned here and this test fails until the new
// implementation is added to the list — at which point the reader is pointed
// at what else must change.
//
// It is an ALLOWLIST rather than a scan-for-problems, so a new implementation
// fails closed: it cannot be introduced without someone reading this comment.
var knownACLImplementations = map[string]string{
	"NopACL":      "allow-all; the no-policy default when acl.yaml is absent",
	"ReadOnlyACL": "deny-all writes; rela-server --read-only",
	"Declarative": "the only implementation compiled from an operator's acl.yaml",
	// Request satisfies the interface but is NOT a deployment-wide policy: it
	// is a per-principal scope obtained from Declarative.ForPrincipal and
	// threaded on the context, never assigned to entitymanager.Deps.ACL. It is
	// listed so the scan stays honest about what it found; the is-policy-active
	// sites are unaffected by it.
	"Request": "per-principal request scope, never wired as a deployment ACL",
}

// aclImplementation matches a method declaration satisfying the ACL interface.
// The receiver may be a pointer or a value, and — as the stand-ins here do —
// may be unnamed, so the name is optional in the pattern.
var aclImplementation = regexp.MustCompile(
	`func \((?:\w+ )?\*?(\w+)\) AuthorizeWrite\(`)

// TestACLImplementations_AreAClosedSet guards a cross-package invariant that
// no compiler check can express.
//
// # Why this matters outside this package
//
// Several call sites answer "is a real policy active?" by TYPE, because that is
// the only thing distinguishing a compiled acl.yaml from the allow-all and
// deny-all stand-ins:
//
//   - internal/entitymanager New (requireCopyGates) — refuses a nil copy read
//     gate when the ACL is *Declarative (#1437)
//   - internal/appbuild CompileTransitions — sets the fail-closed posture of
//     the transition guard and both copy gates
//   - internal/dataentry views_handler.go, commands.go,
//     standalone_document_handler.go — switch on NopACL/ReadOnlyACL
//
// Those two groups fail in OPPOSITE directions on a type they do not
// recognize: the dataentry switches default to fail-closed, while an
// is-Declarative assertion falls through to "no policy", i.e. open. A
// decorating implementation — auditedACL{inner: declarative}, a multi-tenant
// wrapper, a metrics shim — would therefore make dataentry hide a UI element
// while entitymanager waves an ungated copy read through. That is the exact
// forgotten-wiring-becomes-a-bypass shape (RR-X9NVHI) #1437 closed, one
// wrapper later.
//
// The set is closed today, which is what makes every one of those sites
// correct. This test is what makes "closed" a property rather than an
// observation.
//
// If you are here because you added an implementation: decide, per site above,
// whether it represents an active policy, and update each one. Prefer wrapping
// Declarative's behavior to introducing a fourth peer.
func TestACLImplementations_AreAClosedSet(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	found := map[string]string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, rerr := os.ReadFile(filepath.Clean(name))
		if rerr != nil {
			t.Fatalf("read %s: %v", name, rerr)
		}
		for _, m := range aclImplementation.FindAllStringSubmatch(string(src), -1) {
			found[m[1]] = name
		}
	}

	if len(found) == 0 {
		t.Fatal("the scan found no ACL implementations at all — the regexp has " +
			"drifted from the code and this guard is now vacuous")
	}

	for impl, file := range found {
		if _, known := knownACLImplementations[impl]; !known {
			t.Errorf("new ACL implementation %q in %s.\n\n"+
				"Several packages answer \"is a real policy active?\" by asserting on "+
				"*acl.Declarative, and they fail OPEN on a type they do not recognize "+
				"while internal/dataentry's switches fail closed. Read this test's doc "+
				"comment, update the sites it lists, then add %q to "+
				"knownACLImplementations with a one-line reason.",
				impl, file, impl)
		}
	}

	// The reverse direction: a removed implementation should not leave a stale
	// entry claiming the set is wider than it is.
	for impl := range knownACLImplementations {
		if _, ok := found[impl]; !ok {
			t.Errorf("knownACLImplementations lists %q but no such implementation "+
				"exists — remove the stale entry", impl)
		}
	}
}
