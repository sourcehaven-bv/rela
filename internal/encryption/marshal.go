package encryption

import "fmt"

// MarshalIdentity returns the "AGE-SECRET-KEY-PQ-1..." wire form of
// id, suitable for writing to a private-key file on disk. This is the
// one legitimate way to serialize a private identity; Identity.String
// and MarshalJSON deliberately return a redacted marker so that
// identity values flowing through logs, error messages, and JSON
// responses do not leak the secret.
//
// Callers MUST:
//   - chmod the destination file to 0o600 (owner read/write only)
//   - keep the file outside the repo (never commit)
//   - not log, print, or otherwise surface the returned string
//
// Returns an error on an unsupported Identity kind. v1 only supports
// *hybridIdentity (post-quantum hybrid age identities); a future kind
// that reaches this code without a serializer is a bug, not a silent
// empty-key write.
func MarshalIdentity(id Identity) (string, error) {
	if h, ok := id.(*hybridIdentity); ok {
		return h.i.String(), nil
	}
	return "", fmt.Errorf("encryption: MarshalIdentity: unsupported identity kind %T", id)
}
