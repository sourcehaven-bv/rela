package encryption

import (
	"fmt"
	"runtime"
)

// mustStdlibContract returns v when err is nil and panics otherwise.
// It wraps stdlib calls whose error branch is unreachable by the
// documented contract — e.g. X25519 NewPrivateKey when we pass a
// length-validated 32-byte scalar, or aes.NewCipher when we pass a
// 32-byte AES-256 key.
//
// If it ever panics, a stdlib upgrade has broken an invariant we
// relied on. Investigate the caller in the stack trace, don't silence
// the panic.
func mustStdlibContract[T any](v T, err error) T {
	if err != nil {
		_, file, line, _ := runtime.Caller(1)
		panic(fmt.Sprintf("encryption: stdlib contract broken at %s:%d: %v", file, line, err))
	}
	return v
}

// mustLen panics when got != want. It asserts size invariants that
// stdlib guarantees — e.g. that X25519 PublicKey.Bytes returns exactly
// 32 bytes.
func mustLen(what string, got, want int) {
	if got != want {
		_, file, line, _ := runtime.Caller(1)
		panic(fmt.Sprintf("encryption: stdlib contract broken at %s:%d: %s len=%d, want %d", file, line, what, got, want))
	}
}
