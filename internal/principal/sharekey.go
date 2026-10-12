package principal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// ShareKey is a key that is equal for two principals exactly when [Principal.Equal]
// holds. Code that shares work computed for one principal with another (live
// views share a replay per key) uses it, so sharing can never cross a
// difference Equal sees. It is its own encoding, not the JSON wire format,
// so a change to what the audit log records cannot merge two keys.
func (p Principal) ShareKey() string {
	// nil and empty slices are equal under Equal, so both encode as [].
	nonNil := func(s []string) []string {
		if s == nil {
			return []string{}
		}
		return s
	}
	b, err := json.Marshal([]any{
		p.User, p.Tool, p.RawUser, p.orgID, p.orgSlug, p.orgName,
		p.principalType, p.email, nonNil(p.roles), nonNil(p.scopes),
	})
	if err != nil {
		// coverage-ignore: strings and string slices always marshal.
		panic("principal: marshal share key: " + err.Error())
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// SameIdentity reports whether p and q are the same caller: the same user,
// tool and organization. Unlike [Principal.Equal] it ignores the claims a
// token refresh may change (roles, scopes, email, display names).
func (p Principal) SameIdentity(q Principal) bool {
	return p.User == q.User && p.Tool == q.Tool && p.RawUser == q.RawUser && p.orgID == q.orgID
}
