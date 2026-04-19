package encryption

// MarshalIdentity returns the "AGE-SECRET-KEY-1..." wire form of id,
// suitable for writing to a private-key file on disk. This is the
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
// supports *x25519Identity; future PQ plugins would extend this.
func MarshalIdentity(id Identity) string {
	if x, ok := id.(*x25519Identity); ok {
		return x.i.String()
	}
	return ""
}
