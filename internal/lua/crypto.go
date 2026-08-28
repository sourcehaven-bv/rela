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
//	crypto.base64_encode(data)          -> string  (std base64, padded)
//	crypto.base64_decode(s)             -> (string, nil) | (nil, err_table)
//
// base64_decode is the one function here that takes UNTRUSTED input — an API
// response, a file, anything the script did not itself encode — so it is the
// one that returns an error pair instead of raising. Malformed base64 from a
// remote is an expected runtime condition, not a programming error, and a
// script must be able to branch on it.
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
	r.L.SetField(tbl, "base64_encode", r.L.NewFunction(luaBase64Encode))
	r.L.SetField(tbl, "base64_decode", r.L.NewFunction(luaBase64Decode))
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

// luaBase64Encode implements crypto.base64_encode(data) -> std-base64 string.
//
// Standard encoding with padding (RFC 4648 §4), which is what every API that
// says "base64" means unless it says otherwise. A script needing the URL-safe
// alphabet can translate the two differing characters itself; shipping four
// variants here would be four names to get wrong at the call site for a
// two-character substitution.
//
// Lua strings are byte strings, so binary input (an image, a signature)
// round-trips intact. Raises on a non-string argument.
func luaBase64Encode(ls *lua.LState) int {
	data := ls.CheckString(1)
	ls.Push(lua.LString(base64.StdEncoding.EncodeToString([]byte(data))))
	return 1
}

// luaBase64Decode implements crypto.base64_decode(s) -> (string, nil) |
// (nil, err_table).
//
// Unlike its siblings this returns a PAIR: the input is typically untrusted
// (an API response body, a header, a file), so malformed base64 is an expected
// runtime condition a script must be able to handle, not the programming error
// a raise would signal. The error table carries the ai.*/http.* shape so a
// script sees one error vocabulary across every fallible binding.
//
// Both the padded standard alphabet and the unpadded/URL-safe variants are
// accepted, because a script decoding someone else's output does not get to
// choose which one it receives, and "decode this base64" is one intent rather
// than four. Encoding stays single-variant (see luaBase64Encode) — being
// liberal in what you accept does not oblige you to be liberal in what you
// emit.
func luaBase64Decode(ls *lua.LState) int {
	s := ls.CheckString(1)
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		if out, err := enc.DecodeString(s); err == nil {
			ls.Push(lua.LString(string(out)))
			ls.Push(lua.LNil)
			return 2
		}
	}
	ls.Push(lua.LNil)
	tbl := ls.NewTable()
	tbl.RawSetString("kind", lua.LString("bad_input"))
	// The message deliberately does NOT echo the input. Decoding is reached
	// with credentials (a Basic-auth pair, an API token) often enough that
	// quoting the offending string into an error bound for a log is a leak
	// waiting for the one call site that does.
	tbl.RawSetString("message", lua.LString("crypto.base64_decode: input is not valid base64"))
	tbl.RawSetString("retry_after", lua.LNumber(0))
	tbl.RawSetString("details", lua.LString(""))
	ls.Push(tbl)
	return 2
}
