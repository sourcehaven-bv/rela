// Lua bindings for the top-level crypto.* module.
//
// Generic, provider-neutral hashing primitives so a Lua action can assemble a
// signed request for an upstream that authenticates with an HMAC signature (an
// identity proxy's operator API, a webhook callback, ...). The PRIMITIVES live
// here in Go; the provider-specific SHAPE — which canonical string to hash, which
// header names to send — lives in the Lua action. That split is deliberate: it
// lets rela sign requests for any HMAC-authenticated IdP without compiling in
// support for any particular one (point the action at a different proxy and only
// the script changes, not this package).
//
// Crypto is Go, not Lua, for the same reason http.get is: the primitive belongs
// in the host where it can be reviewed and bounded. But note the ASYMMETRY with
// verification — rela VERIFIES signed assertions entirely in Go (see
// internal/jwtauth), because a verifier is a root of trust. These functions only
// SIGN outbound requests with a key the operator already placed in secrets; they
// are a hashing tool for the glue layer, not a trust boundary.
//
// Convention (matches json.encode): a hashing primitive cannot fail on a valid
// string input, so these RAISE on a wrong-type argument (a programming error) and
// otherwise always return a value — no (value, err) pair.
//
//	crypto.sha256_hex(data)             -> string  (lowercase hex of SHA-256)
//	crypto.hmac_sha256_base64(key, msg) -> string  (std base64 of HMAC-SHA256)
package lua

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"

	lua "github.com/yuin/gopher-lua"
)

// registerCryptoModule installs the top-level `crypto` global. Always
// registered; no configuration needed (matches http.*). A free function rather
// than a *Runtime method to keep the Runtime type's method count flat (the type
// is at its plimsoll load line; see the directive on Runtime).
func registerCryptoModule(r *Runtime) {
	tbl := r.L.NewTable()
	r.L.SetField(tbl, "sha256_hex", r.L.NewFunction(luaSHA256Hex))
	r.L.SetField(tbl, "hmac_sha256_base64", r.L.NewFunction(luaHMACSHA256Base64))
	r.L.SetGlobal("crypto", tbl)
}

// luaSHA256Hex implements crypto.sha256_hex(data) -> lowercase-hex string.
// Used to compute a body digest for a canonical signing string. Raises on a
// non-string argument.
func luaSHA256Hex(ls *lua.LState) int {
	data := ls.CheckString(1)
	sum := sha256.Sum256([]byte(data))
	ls.Push(lua.LString(hex.EncodeToString(sum[:])))
	return 1
}

// luaHMACSHA256Base64 implements crypto.hmac_sha256_base64(key, message) ->
// std-base64 string. key and message are raw bytes carried as Lua strings (Lua
// strings are byte strings, so a binary key round-trips intact). Raises on a
// non-string argument. An empty key is accepted — HMAC is defined for it — and is
// the caller's responsibility to avoid; a wrong/empty key simply produces a
// signature the server rejects.
func luaHMACSHA256Base64(ls *lua.LState) int {
	key := ls.CheckString(1)
	msg := ls.CheckString(2)
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(msg))
	ls.Push(lua.LString(base64.StdEncoding.EncodeToString(mac.Sum(nil))))
	return 1
}
