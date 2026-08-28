package lua

import (
	"bytes"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// formCapture is what a form-posting stub recorded.
type formCapture struct {
	contentType string
	fields      map[string][]string
	user        string
	pass        string
	basicOK     bool
	authHeader  string
	rawBody     string
}

// newFormStub returns a server that parses multipart bodies and records Basic
// auth, plus an accessor for what it saw.
func newFormStub(t *testing.T) (srv *httptest.Server, received func() formCapture) {
	t.Helper()

	var mu sync.Mutex
	var got formCapture

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw bytes.Buffer
		_, _ = raw.ReadFrom(r.Body)

		capture := formCapture{
			contentType: r.Header.Get("Content-Type"),
			fields:      map[string][]string{},
			authHeader:  r.Header.Get("Authorization"),
			rawBody:     raw.String(),
		}
		capture.user, capture.pass, capture.basicOK = r.BasicAuth()

		mediaType, params, err := mime.ParseMediaType(capture.contentType)
		if err == nil && strings.HasPrefix(mediaType, "multipart/") {
			mr := multipart.NewReader(bytes.NewReader(raw.Bytes()), params["boundary"])
			for {
				part, perr := mr.NextPart()
				if perr != nil {
					break
				}
				var buf bytes.Buffer
				_, _ = buf.ReadFrom(part)
				capture.fields[part.FormName()] = append(capture.fields[part.FormName()], buf.String())
			}
		}

		mu.Lock()
		got = capture
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	received = func() formCapture {
		mu.Lock()
		defer mu.Unlock()
		return got
	}
	return srv, received
}

// TestHTTPForm_MultipartWellFormed covers AC7's form half: the body is a
// well-formed multipart/form-data document a standard parser reads back.
//
// Parsed with mime/multipart rather than string-matched, deliberately. A
// substring assertion would pass on a body with a mismatched boundary or a
// missing terminator — exactly the malformations a real server rejects.
func TestHTTPForm_MultipartWellFormed(t *testing.T) {
	t.Parallel()

	srv, received := newFormStub(t)
	rt := newHTTPRuntime(t)

	require.NoError(t, rt.RunString(`
local resp, err = http.request({
  url = "`+srv.URL+`/f",
  method = "POST",
  form = { subject = "Hello", text = "body text" },
})
if err then error(err.message) end
assert(resp.status_code == 200)`))

	got := received()
	assert.True(t, strings.HasPrefix(got.contentType, "multipart/form-data; boundary="),
		"Content-Type must declare the generated boundary, got %q", got.contentType)
	assert.Equal(t, []string{"Hello"}, got.fields["subject"])
	assert.Equal(t, []string{"body text"}, got.fields["text"])
}

// TestHTTPForm_RepeatedFieldNames pins the positional shape. Form encoding
// permits a repeated name and several providers rely on it (Mailgun takes
// multiple `to` fields); a Lua table keyed by name cannot express that at all.
func TestHTTPForm_RepeatedFieldNames(t *testing.T) {
	t.Parallel()

	srv, received := newFormStub(t)
	rt := newHTTPRuntime(t)

	require.NoError(t, rt.RunString(`
local resp, err = http.request({
  url = "`+srv.URL+`/f",
  method = "POST",
  form = {
    {"to", "a@example.com"},
    {"to", "b@example.com"},
    {"subject", "Hi"},
  },
})
if err then error(err.message) end
assert(resp.status_code == 200)`))

	got := received()
	assert.Equal(t, []string{"a@example.com", "b@example.com"}, got.fields["to"])
	assert.Equal(t, []string{"Hi"}, got.fields["subject"])
}

// TestHTTPForm_MapShapeIsSorted pins that the map shape produces a
// DETERMINISTIC body. Lua map iteration order is undefined, so without the
// sort every test asserting on a produced body would be intermittently wrong.
func TestHTTPForm_MapShapeIsSorted(t *testing.T) {
	t.Parallel()

	srv, received := newFormStub(t)
	rt := newHTTPRuntime(t)

	require.NoError(t, rt.RunString(`
for i = 1, 5 do
  local _, err = http.request({
    url = "`+srv.URL+`/f",
    method = "POST",
    form = { zulu = "z", alpha = "a", mike = "m" },
  })
  if err then error(err.message) end
end`))

	got := received()
	iAlpha := strings.Index(got.rawBody, `name="alpha"`)
	iMike := strings.Index(got.rawBody, `name="mike"`)
	iZulu := strings.Index(got.rawBody, `name="zulu"`)
	require.NotEqual(t, -1, iAlpha)
	assert.Less(t, iAlpha, iMike, "map-shaped form parts must be sorted by name")
	assert.Less(t, iMike, iZulu)
}

// TestHTTPForm_BasicAuthHeader covers AC7's basic_auth half.
func TestHTTPForm_BasicAuthHeader(t *testing.T) {
	t.Parallel()

	srv, received := newFormStub(t)
	rt := newHTTPRuntime(t)

	require.NoError(t, rt.RunString(`
local resp, err = http.request({
  url = "`+srv.URL+`/f",
  method = "POST",
  form = { x = "1" },
  basic_auth = { user = "api", pass = "key-abc123" },
})
if err then error(err.message) end
assert(resp.status_code == 200)`))

	got := received()
	assert.True(t, got.basicOK, "server must see a parseable Basic credential")
	assert.Equal(t, "api", got.user)
	assert.Equal(t, "key-abc123", got.pass)
	// The header the server must have received; base64 of the api:key pair.
	assert.Equal(t, "Basic YXBpOmtleS1hYmMxMjM=", got.authHeader)
}

// TestHTTPForm_BasicAuthPasswordWithColon pins why basic_auth is a two-field
// table rather than a "user:pass" string: a colon in the password is legal and
// a string form would split it in the wrong place.
func TestHTTPForm_BasicAuthPasswordWithColon(t *testing.T) {
	t.Parallel()

	srv, received := newFormStub(t)
	rt := newHTTPRuntime(t)

	require.NoError(t, rt.RunString(`
local _, err = http.request({
  url = "`+srv.URL+`/f",
  method = "POST",
  basic_auth = { user = "u", pass = "pa:ss:word" },
})
if err then error(err.message) end`))

	got := received()
	assert.True(t, got.basicOK)
	assert.Equal(t, "u", got.user)
	assert.Equal(t, "pa:ss:word", got.pass)
}

// TestHTTPForm_BasicAuthOnConvenienceMethods pins that the convenience methods
// accept the same options as http.request. They previously took a narrower
// option set, and a caller who moved a working call from http.request to
// http.post would have silently dropped the credential.
func TestHTTPForm_BasicAuthOnConvenienceMethods(t *testing.T) {
	t.Parallel()

	srv, received := newFormStub(t)
	rt := newHTTPRuntime(t)

	require.NoError(t, rt.RunString(`
local _, err = http.post("`+srv.URL+`/f", nil, {
  form = { a = "1" },
  basic_auth = { user = "u", pass = "p" },
})
if err then error(err.message) end`))

	got := received()
	assert.True(t, got.basicOK)
	assert.Equal(t, "u", got.user)
	assert.Equal(t, []string{"1"}, got.fields["a"])
}

// TestHTTPForm_ContentTypeNotClobbered pins that a caller-set Content-Type
// cannot override the generated multipart boundary. A declared boundary that
// does not match the body is unparseable at the far end, and the failure reads
// as "the server rejected my request".
func TestHTTPForm_ContentTypeNotClobbered(t *testing.T) {
	t.Parallel()

	srv, received := newFormStub(t)
	rt := newHTTPRuntime(t)

	require.NoError(t, rt.RunString(`
local _, err = http.request({
  url = "`+srv.URL+`/f",
  method = "POST",
  headers = { ["Content-Type"] = "multipart/form-data" },
  form = { a = "1" },
})
if err then error(err.message) end`))

	got := received()
	assert.Contains(t, got.contentType, "boundary=",
		"the generated boundary must survive a caller-set Content-Type")
	assert.Equal(t, []string{"1"}, got.fields["a"])
}

// TestHTTPForm_BodyAndFormAreExclusive pins that setting both RAISES rather
// than silently preferring one. A quiet precedence rule would send a request
// missing half of what its author intended.
func TestHTTPForm_BodyAndFormAreExclusive(t *testing.T) {
	t.Parallel()

	rt := newHTTPRuntime(t)
	err := rt.RunString(`
http.request({
  url = "https://example.com/x",
  method = "POST",
  body = "raw",
  form = { a = "1" },
})`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

// TestHTTPForm_InvalidShapesRaise table-tests the argument validation. These
// are programming errors, so they raise rather than returning an err_table.
func TestHTTPForm_InvalidShapesRaise(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct{ script, want string }{
		"form not a table": {
			script: `http.request({url="https://e.com/x", form = "nope"})`,
			want:   "form must be a table",
		},
		"form numeric value": {
			script: `http.request({url="https://e.com/x", form = {a = 1}})`,
			want:   "use tostring",
		},
		"form bad pair": {
			script: `http.request({url="https://e.com/x", form = {{"only"}}})`,
			want:   "{name, value} pair",
		},
		"form empty name": {
			script: `http.request({url="https://e.com/x", form = {{"", "v"}}})`,
			want:   "empty field name",
		},
		"basic_auth not a table": {
			script: `http.request({url="https://e.com/x", basic_auth = "u:p"})`,
			want:   "basic_auth must be a table",
		},
		"basic_auth no user": {
			script: `http.request({url="https://e.com/x", basic_auth = {pass = "p"}})`,
			want:   "basic_auth.user must be a non-empty string",
		},
		"basic_auth empty user": {
			script: `http.request({url="https://e.com/x", basic_auth = {user = "", pass = "p"}})`,
			want:   "basic_auth.user must be a non-empty string",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rt := newHTTPRuntime(t)
			err := rt.RunString(tc.script)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestHTTPForm_EmptyPassAllowed pins that a token-as-username credential works
// — the mirror image of Mailgun's api:KEY, and common enough to support.
func TestHTTPForm_EmptyPassAllowed(t *testing.T) {
	t.Parallel()

	srv, received := newFormStub(t)
	rt := newHTTPRuntime(t)

	require.NoError(t, rt.RunString(`
local _, err = http.request({
  url = "`+srv.URL+`/f",
  method = "POST",
  basic_auth = { user = "token-as-username" },
})
if err then error(err.message) end`))

	got := received()
	assert.True(t, got.basicOK)
	assert.Equal(t, "token-as-username", got.user)
	assert.Empty(t, got.pass)
}

// TestHTTPForm_RequiresHTTPCapability pins that the new options widen nothing:
// without the http capability the global is still absent, so form and
// basic_auth are unreachable however a script is written.
func TestHTTPForm_RequiresHTTPCapability(t *testing.T) {
	t.Parallel()

	rt := NewReader(ReadDeps{}, &bytes.Buffer{})
	t.Cleanup(rt.Close)
	require.NoError(t, rt.RunString(`assert(http == nil, "http must be absent without the capability")`))
}
