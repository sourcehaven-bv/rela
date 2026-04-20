package userstate

import (
	"errors"
	"regexp"
	"strings"
)

// ErrInvalidRepoID is returned when a candidate repo-id fails
// canonical-format validation. The repo-id is used as a path
// segment under the per-repo state directory; loose validation
// opens a path-traversal surface, so we enforce strict format.
var ErrInvalidRepoID = errors.New("userstate: invalid repo-id (expected 32 lowercase hex chars)")

// repoIDPattern matches encryption.NewRepoID's output: 32
// lowercase hex characters (128 bits random, hex-encoded). This
// is not a canonical UUIDv4 — it has no version/variant bits —
// but it's the format already present on disk in encrypted repos,
// so .rela/repo-id and Keyring.RepoID remain byte-comparable.
//
// The strict format also defends the path segment: no separator,
// no dot, no case ambiguity on case-insensitive filesystems.
var repoIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// validateRepoID rejects any string that is not a 32-char
// lowercase-hex repo-id. Callers trim whitespace upstream; this
// function does not tolerate surrounding newlines or tabs.
func validateRepoID(id string) error {
	if id == "" {
		return ErrInvalidRepoID
	}
	if !repoIDPattern.MatchString(id) {
		return ErrInvalidRepoID
	}
	return nil
}

// validateKey mirrors state.FSKV's key validation to keep the Get/
// Put contract identical. Duplicated deliberately: state is a peer
// package we consume, not own, and coupling our validation to
// theirs through an exported helper would force state to publish an
// API it otherwise doesn't need.
func validateKey(name string) error {
	if name == "" {
		return errors.New("userstate: key must not be empty")
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return errors.New("userstate: control character (including NUL) not allowed")
		}
	}
	if strings.ContainsRune(name, '\\') {
		return errors.New("userstate: backslash not allowed (use forward slash)")
	}
	if strings.HasPrefix(name, "/") {
		return errors.New("userstate: key must be relative")
	}
	for _, seg := range strings.Split(name, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return errors.New("userstate: traversal or empty segment not allowed")
		}
	}
	if len(name) >= 2 && name[1] == ':' {
		return errors.New("userstate: drive letter not allowed")
	}
	return nil
}
