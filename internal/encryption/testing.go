package encryption

// IdentitySecretForTest returns the AGE-SECRET-KEY-1... encoding of
// id. It exists so tests in other packages (fsstore, workspace) can
// write the identity to an on-disk key file and load it through the
// standard LoadKeyring path.
//
// This function deliberately bypasses the Identity.String() /
// MarshalJSON redaction that protects against accidental leakage in
// logs and production error messages. It MUST NOT be called outside
// tests.
//
// The leak guard (TestSecretTypes_NoStringMethods etc.) remains
// intact: String()/MarshalJSON still redact; this function is a
// deliberate, clearly-named escape hatch.
func IdentitySecretForTest(id Identity) string {
	if x, ok := id.(*x25519Identity); ok {
		return x.i.String()
	}
	return ""
}
