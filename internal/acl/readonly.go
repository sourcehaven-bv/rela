package acl

import "context"

// ReadOnlyACL denies every write with a fixed [Decision]. Useful for:
//
//   - Operating rela-server in observe-only mode for demos or
//     maintenance windows.
//   - Exercising the full deny path (HTTP 403, audit denied-write,
//     UI affordances) without writing an acl.yaml.
//   - A fail-safe an operator can wire when they want absolute
//     confidence no writes happen (e.g. post-incident forensic
//     mode).
//
// Wired via `rela-server --read-only` or the env var
// `RELA_READ_ONLY=1`. ReadOnlyACL is intentionally process-wide:
// once set at startup, it cannot be toggled at runtime.
type ReadOnlyACL struct{}

// AuthorizeWrite always returns the same deny [Decision].
func (ReadOnlyACL) AuthorizeWrite(_ context.Context, _ WriteRequest) Decision {
	return Decision{
		Allow:    false,
		RuleKind: "read-only",
		RuleID:   "read-only-acl",
		Reason:   "this rela instance is configured read-only",
	}
}
