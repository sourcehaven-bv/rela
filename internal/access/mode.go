package access

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// Mode is the kind of ACL a deployment runs under. Some decisions depend on
// the mode rather than on a principal: whether a `permission:` can be asked at
// all, whether a history reveal is worth an audit row, whether writes are
// accepted. [ModeOf] is the one place that inspects the ACL implementation, so
// these decisions cannot drift apart in separate type switches.
//
// The archguard ACL-mode guard covers type checks on acl.NopACL and
// acl.ReadOnlyACL only. Assertions on *acl.Declarative that ask "is a policy
// configured?" remain in dataentry, entitymanager and the watcher; they move
// behind Mode in a later step of TKT-WFS778, and the guard widens then.
//
// The modes are unexported on purpose: a caller asks a Mode method, and
// cannot rebuild a switch on the mode outside this package.
//
// The decisions per mode:
//
//	mode           gated UI    elevated read  reveal audited  policy configured  refuses writes  project write
//	unknown        hide        deny           yes             no                 no              deny
//	unwired (nil)  hide        deny           no              no                 no              deny
//	no policy      show        deny           no              no                 no              allow
//	read-only      show        deny           no              no                 yes             deny
//	policy         permission  permission     yes             yes                no              permission
//	broken policy  hide        deny           yes             yes                no              deny
//
// The zero value is the unknown mode. An unknown implementation is reachable
// only from tests (the App constructor accepts the four production shapes).
// It denies permission-gated UI and elevation, but it does not count as a
// configured policy, so the comment routes skip their permission checks for
// it, as they did before this type existed.
type Mode uint8

const (
	modeUnknown      Mode = iota // an implementation ModeOf does not recognize
	modeUnwired                  // a nil acl.ACL
	modeNoPolicy                 // acl.NopACL, value or pointer
	modeReadOnly                 // acl.ReadOnlyACL, value or pointer
	modePolicy                   // a non-nil *acl.Declarative
	modeBrokenPolicy             // a typed-nil *acl.Declarative: a wiring bug that must not fail open
)

// ModeOf classifies a. The value and pointer forms of acl.NopACL and
// acl.ReadOnlyACL are both recognized, a nil pointer included, because their
// AuthorizeWrite has a value receiver: matching only the value form would
// send `&acl.ReadOnlyACL{}` to the unknown mode.
func ModeOf(a acl.ACL) Mode {
	switch impl := a.(type) {
	case nil:
		return modeUnwired
	case acl.NopACL, *acl.NopACL:
		return modeNoPolicy
	case acl.ReadOnlyACL, *acl.ReadOnlyACL:
		return modeReadOnly
	case *acl.Declarative:
		if impl == nil {
			return modeBrokenPolicy
		}
		return modePolicy
	default:
		return modeUnknown
	}
}

// String names the mode for logs and test failures.
func (m Mode) String() string {
	switch m {
	case modeUnwired:
		return "unwired"
	case modeNoPolicy:
		return "no policy"
	case modeReadOnly:
		return "read-only"
	case modePolicy:
		return "policy"
	case modeBrokenPolicy:
		return "broken policy"
	default:
		return "unknown"
	}
}

// PermissionHolder answers whether the request's principal holds a global
// named permission. The request's read gate implements it.
type PermissionHolder interface {
	HoldsPermission(ctx context.Context, permission string) bool
}

// PermitsGatedUIElement reports whether a UI element (a navigation entry, a
// space, a dashboard card) that declares `permission:` is shown. An element
// without a permission is always shown. This is presentation, not access
// control: the rows behind an element are gated by the read path.
//
// No policy and read-only both show gated elements. Neither can answer
// whether a principal holds a permission, and hiding an entry under
// ReadOnlyACL would make `permission:` mean "hide from all" because of a
// process-wide flag about writes. An operator in post-incident forensic mode
// (a documented ReadOnlyACL use case) would lose the audit-log entry they came
// for. The answer is reached in an explicit arm rather than by asking holder:
// under these modes no read gate is attached, and the fallback gate answers
// yes to every permission, so asking it would look like a check that never
// happened (RR-CWWJGW).
//
// A configured policy asks holder. Every other mode hides the element.
func (m Mode) PermitsGatedUIElement(ctx context.Context, holder PermissionHolder, permission string) bool {
	if permission == "" {
		return true
	}
	switch m {
	case modeNoPolicy, modeReadOnly:
		return true
	case modePolicy:
		return holder.HoldsPermission(ctx, permission)
	default:
		return false
	}
}

// PermitsElevatedRead reports whether a document configured with
// `allow_acl_bypass` may render for this request. An elevated render reads
// without the read gate, so only a configured policy can grant it, and only
// to a principal holding the document's permission.
//
// No policy and read-only deny: a policy-less deployment that declares
// elevation has nobody to check, and refusing to serve is the honest answer.
// An empty permission denies too. Config validation rejects it, but a guard
// that depends on validation having run fails open on the next construction
// path that skips it.
func (m Mode) PermitsElevatedRead(ctx context.Context, holder PermissionHolder, permission string) bool {
	switch m {
	case modePolicy:
		if permission == "" {
			return false
		}
		return holder.HoldsPermission(ctx, permission)
	default:
		return false
	}
}

// RevealIsPrivileged reports whether a history read that overrides redaction
// is worth an audit row.
//
// Without a policy (unwired, no policy, read-only) nothing is redacted, so a
// reveal reveals nothing. The fallback read gate also grants every
// permission, so every history read would take the reveal arm; recording
// them would bury real reveals in noise. Every other mode records: for a log,
// a spurious row is recoverable and a missing one is not.
func (m Mode) RevealIsPrivileged() bool {
	switch m {
	case modeUnwired, modeNoPolicy, modeReadOnly:
		return false
	default:
		return true
	}
}

// PolicyConfigured reports whether a declarative policy is wired, including a
// broken (typed-nil) one, so that a wiring bug takes the gated path and fails
// there rather than skipping the gate.
func (m Mode) PolicyConfigured() bool {
	return m == modePolicy || m == modeBrokenPolicy
}

// RefusesWrites reports whether the instance is read-only. Read-only is a
// process-wide refusal, not a permission a principal could hold, so no grant
// in any acl.yaml makes it writable: comment writes and command execution
// check this before any per-request permission.
func (m Mode) RefusesWrites() bool {
	return m == modeReadOnly
}

// PermitsProjectWrite reports whether the request may run a write that acts
// on the whole project rather than on one entity, such as a git sync, which
// commits and pushes every pending change.
//
// No policy allows it: every principal may already write every entity, and
// the operation adds nothing they could not do. Read-only refuses it before
// asking holder, because the fallback read gate grants every permission
// there. A configured policy asks holder for permission. Every other mode
// refuses, so a wiring bug fails closed.
func (m Mode) PermitsProjectWrite(ctx context.Context, holder PermissionHolder, permission string) bool {
	switch m {
	case modeNoPolicy:
		return true
	case modePolicy:
		return permission != "" && holder.HoldsPermission(ctx, permission)
	default:
		return false
	}
}
