package fsstore

import "bytes"

// gitCryptMagic is the 9-byte header git-crypt prepends to every
// encrypted blob: a NUL byte, the literal "GITCRYPT", and another NUL.
// See https://github.com/AGWA/git-crypt — version 0.6+ uses this header.
var gitCryptMagic = []byte{0x00, 'G', 'I', 'T', 'C', 'R', 'Y', 'P', 'T', 0x00}

// isGitCryptEncrypted reports whether b starts with the git-crypt magic
// header. The check is byte-exact and tolerates trailing data of any
// length (including none). Files shorter than the header cannot be
// encrypted and return false.
func isGitCryptEncrypted(b []byte) bool {
	return len(b) >= len(gitCryptMagic) && bytes.Equal(b[:len(gitCryptMagic)], gitCryptMagic)
}
