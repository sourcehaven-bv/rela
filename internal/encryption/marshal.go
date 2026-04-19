package encryption

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
// Returns the empty string on an unsupported Identity kind. v1 only
// supports *hybridIdentity (post-quantum hybrid age identities).
func MarshalIdentity(id Identity) string {
	if h, ok := id.(*hybridIdentity); ok {
		return h.i.String()
	}
	return ""
}
