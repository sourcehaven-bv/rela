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
