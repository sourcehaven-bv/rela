package lua

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCryptoTestRuntime(t *testing.T) (*Runtime, *strings.Builder) {
	t.Helper()
	var sb strings.Builder
	rt := NewReader(ReadDeps{}, &sb)
	return rt, &sb
}

// runCryptoString runs a script that outputs a single string and returns it.
func runCryptoString(t *testing.T, script string) string {
	t.Helper()
	rt, buf := newCryptoTestRuntime(t)
	defer rt.Close()
	require.NoError(t, rt.RunString(script))
	var out string
	require.NoError(t, json.Unmarshal([]byte(buf.String()), &out))
	return out
}

func TestSHA256Hex(t *testing.T) {
	t.Parallel()
	// Known vector: sha256("") = e3b0c442...b855.
	got := runCryptoString(t, `rela.output(crypto.sha256_hex(""))`)
	assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", got)

	// Cross-check an arbitrary input (with embedded newlines, built in Lua so the
	// Go-side source stays a single line) against Go's own hasher.
	const in = "GET\n/api/v1/orgs/org_1/members/usr_1\n1700000000\n"
	sum := sha256.Sum256([]byte(in))
	want := hex.EncodeToString(sum[:])
	got = runCryptoString(t,
		`rela.output(crypto.sha256_hex("GET" .. "\n" .. "/api/v1/orgs/org_1/members/usr_1" .. "\n" .. "1700000000" .. "\n"))`)
	assert.Equal(t, want, got)
}

func TestHMACSHA256Base64(t *testing.T) {
	t.Parallel()
	const key = "dev-operator-secret"
	const msg = "GET\n/api/v1/orgs/org_1/members/usr_1\n1700000000\nabc123"

	// Cross-check the Lua binding against Go's own HMAC — the whole point is that
	// a signature assembled in Lua is byte-identical to what an HMAC-verifying
	// server (e.g. Pratique's operator API) recomputes.
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(msg))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// Build msg in Lua via concatenation so the embedded newlines don't break the
	// Go-side source string.
	got := runCryptoString(t,
		`local msg = "GET" .. "\n" .. "/api/v1/orgs/org_1/members/usr_1" .. "\n" .. "1700000000" .. "\n" .. "abc123"
		 rela.output(crypto.hmac_sha256_base64("`+key+`", msg))`)
	assert.Equal(t, want, got)
}

// TestCryptoSignsLikePratiqueOperator proves the end-to-end intent: a Lua action
// can build the EXACT canonical string + signature Pratique's operator API
// verifies (METHOD\nPATH\nDATE\nhex(sha256(body)), HMAC-SHA256, base64), using
// only the generic crypto primitives. The canonical SHAPE lives in the script
// (provider-specific); the primitives live in Go (generic).
func TestCryptoSignsLikePratiqueOperator(t *testing.T) {
	t.Parallel()
	const (
		key    = "shared-hmac-key"
		method = "GET"
		path   = "/api/v1/orgs/org_acme/members/usr_founder"
		date   = "1700000000"
		body   = "" // GET, empty body
	)

	script := `
		local body_hash = crypto.sha256_hex("` + body + `")
		local canonical = "` + method + `" .. "\n" .. "` + path + `" .. "\n" .. "` + date + `" .. "\n" .. body_hash
		rela.output(crypto.hmac_sha256_base64("` + key + `", canonical))
	`
	got := runCryptoString(t, script)

	// Recompute the same way Pratique's signOperator does (docs/04-architecture.md).
	bodySum := sha256.Sum256([]byte(body))
	canonical := method + "\n" + path + "\n" + date + "\n" + hex.EncodeToString(bodySum[:])
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(canonical))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	assert.Equal(t, want, got, "Lua-assembled operator signature must match the Go signer")
}

func TestCryptoRaisesOnNonString(t *testing.T) {
	t.Parallel()
	rt, _ := newCryptoTestRuntime(t)
	defer rt.Close()
	// A table where a string is required is a programming error → raise. (A
	// number would be silently coerced by gopher-lua's CheckString, so use a
	// value that genuinely cannot convert.)
	err := rt.RunString(`crypto.sha256_hex({})`)
	require.Error(t, err)
}

// TestBase64EncodeKnownVectors covers AC6's encode half with the RFC 4648 §10
// test vectors — the padding cases are where a hand-rolled implementation goes
// wrong, so they are enumerated rather than sampled.
func TestBase64EncodeKnownVectors(t *testing.T) {
	t.Parallel()

	for input, want := range map[string]string{
		"":       "",
		"f":      "Zg==",
		"fo":     "Zm8=",
		"foo":    "Zm9v",
		"foob":   "Zm9vYg==",
		"fooba":  "Zm9vYmE=",
		"foobar": "Zm9vYmFy",
	} {
		got := runCryptoString(t, `rela.output(crypto.base64_encode("`+input+`"))`)
		assert.Equal(t, want, got, "base64_encode(%q)", input)
	}
}

// TestBase64DecodeKnownVectors covers AC6's decode half against the same
// vectors, read in the other direction.
func TestBase64DecodeKnownVectors(t *testing.T) {
	t.Parallel()

	for encoded, want := range map[string]string{
		"":         "",
		"Zg==":     "f",
		"Zm8=":     "fo",
		"Zm9v":     "foo",
		"Zm9vYg==": "foob",
		"Zm9vYmE=": "fooba",
		"Zm9vYmFy": "foobar",
	} {
		got := runCryptoString(t, `
local out, err = crypto.base64_decode("`+encoded+`")
if err then error(err.message) end
rela.output(out)`)
		assert.Equal(t, want, got, "base64_decode(%q)", encoded)
	}
}

// TestBase64RoundTripBinary covers AC6's round-trip clause on BINARY data.
//
// Binary specifically: Lua strings are byte strings, and a NUL or a high byte
// is exactly what a naive implementation truncates or mangles. Text-only
// vectors would not catch it.
func TestBase64RoundTripBinary(t *testing.T) {
	t.Parallel()

	got := runCryptoString(t, `
local raw = ""
for i = 0, 255 do
  raw = raw .. string.char(i)
end
local encoded = crypto.base64_encode(raw)
local decoded, err = crypto.base64_decode(encoded)
if err then error(err.message) end
if decoded ~= raw then error("round-trip changed the bytes") end
rela.output(encoded)`)

	all := make([]byte, 256)
	for i := range all {
		all[i] = byte(i)
	}
	assert.Equal(t, base64.StdEncoding.EncodeToString(all), got)
}

// TestBase64DecodeVariants pins that decode accepts the unpadded and URL-safe
// alphabets. A script decoding someone else's output does not get to choose
// which variant it receives.
func TestBase64DecodeVariants(t *testing.T) {
	t.Parallel()

	// ">>>???" encodes with '+' and '/' in the standard alphabet and with '-'
	// and '_' in the URL-safe one, so it distinguishes them.
	raw := ">>>???"
	for name, encoded := range map[string]string{
		"std":    base64.StdEncoding.EncodeToString([]byte(raw)),
		"rawStd": base64.RawStdEncoding.EncodeToString([]byte(raw)),
		"url":    base64.URLEncoding.EncodeToString([]byte(raw)),
		"rawURL": base64.RawURLEncoding.EncodeToString([]byte(raw)),
	} {
		got := runCryptoString(t, `
local out, err = crypto.base64_decode("`+encoded+`")
if err then error(err.message) end
rela.output(out)`)
		assert.Equal(t, raw, got, "variant %s", name)
	}
}

// TestBase64DecodeInvalid pins the error CONVENTION: malformed input from a
// remote is an expected runtime condition, so it returns (nil, err_table)
// rather than raising. A raise would make a script lose its whole run because
// an upstream sent a bad header.
func TestBase64DecodeInvalid(t *testing.T) {
	t.Parallel()

	rt, buf := newCryptoTestRuntime(t)
	defer rt.Close()

	require.NoError(t, rt.RunString(`
local out, err = crypto.base64_decode("!!!not base64!!!")
if out ~= nil then error("expected nil value on failure") end
rela.output(err.kind .. "|" .. err.message)`))

	var got string
	require.NoError(t, json.Unmarshal([]byte(buf.String()), &got))
	assert.True(t, strings.HasPrefix(got, "bad_input|"), "got %q", got)
	// The message must not echo the input: decode is reached with credentials
	// often enough that quoting the offending string into a log is a leak.
	assert.NotContains(t, got, "!!!not base64!!!")
}

// TestBase64EncodeRaisesOnNonString pins the other half of the convention:
// a wrong-type argument is a programming error and raises.
func TestBase64EncodeRaisesOnNonString(t *testing.T) {
	t.Parallel()

	rt, _ := newCryptoTestRuntime(t)
	defer rt.Close()
	require.Error(t, rt.RunString(`crypto.base64_encode({})`))
}

// TestCryptoAlwaysRegistered pins that crypto.* needs no capability grant —
// hashing and encoding reach nothing outside the process, so gating them would
// cost usability and buy no containment.
func TestCryptoAlwaysRegistered(t *testing.T) {
	t.Parallel()

	rt := NewReader(ReadDeps{}, &strings.Builder{})
	defer rt.Close()
	require.NoError(t, rt.RunString(`
assert(crypto.base64_encode ~= nil)
assert(crypto.base64_decode ~= nil)
assert(crypto.sha256_hex ~= nil)
assert(crypto.hmac_sha256_base64 ~= nil)`))
}
