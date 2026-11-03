//go:build !postgres && !memorybackend && !sqlite

package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/configedit"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
)

const mcpRealtime = `version: 1
channel_types:
  room:
    attach_to: [task]
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: string } }
        post: [member]
`

// testIDP is a throwaway ES256 issuer with a JWKS server on loopback.
type testIDP struct {
	url  string
	mint func(sub string) string
}

func newTestIDP(t *testing.T) testIDP {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	pub, err := key.PublicKey.ECDH()
	require.NoError(t, err)
	pt := pub.Bytes() // 0x04 || X(32) || Y(32)
	const kid = "configure-mcp"
	doc := map[string]any{"keys": []map[string]any{{
		"kty": "EC", "crv": "P-256", "use": "sig", "alg": "ES256", "kid": kid,
		"x": base64.RawURLEncoding.EncodeToString(pt[1:33]),
		"y": base64.RawURLEncoding.EncodeToString(pt[33:65]),
	}}}
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	}))
	t.Cleanup(jwks.Close)
	return testIDP{url: jwks.URL, mint: func(sub string) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
			"iss": "https://idp.test", "aud": "rela", "sub": sub,
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		tok.Header["kid"] = kid
		s, err := tok.SignedString(key)
		require.NoError(t, err)
		return s
	}}
}

// mcpConfigServer starts a server with --config-editing, -mcp and JWT
// identity over a project with realtime.yaml.
func mcpConfigServer(t *testing.T) (*server, string, testIDP) {
	t.Helper()
	t.Setenv(dataentry.EnvDataEntryUserVar, "")
	idp := newTestIDP(t)
	dir := newConfigProject(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "realtime.yaml"), []byte(mcpRealtime), 0o644))
	f := &serverFlags{
		projectDir: dir, bind: "127.0.0.1", port: "8080", configEditing: true, remoteMCP: true,
		jwtIssuer: "https://idp.test", jwtAudience: "rela", jwtJWKSURL: idp.url, jwtHeader: "X-Auth-Assertion",
	}
	svc, err := discoverProject(f)
	require.NoError(t, err)
	srv, err := newServer(f, svc, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		g := srv.cur.Load()
		waitRetired(t, srv)
		g.haltScheduler()
		g.app.StopWatching()
		_ = g.svc.Close()
	})
	return srv, dir, idp
}

// handlerTransport serves a request on srv in process, as user.
type handlerTransport struct {
	srv   *server
	token string
}

func (h handlerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("X-Auth-Assertion", h.token)
	rec := httptest.NewRecorder()
	h.srv.ServeHTTP(rec, req)
	return rec.Result(), nil
}

// readRoom calls read_channel on TSK-001's room over the MCP endpoint as
// user and returns the answer text and whether it is an error.
func readRoom(t *testing.T, srv *server, idp testIDP, user string) (string, bool) {
	t.Helper()
	ctx := context.Background()
	client := mcpgo.NewClient(&mcpgo.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(ctx, &mcpgo.StreamableClientTransport{
		Endpoint:             "http://" + srv.addr + dataentry.MCPPath,
		HTTPClient:           &http.Client{Transport: handlerTransport{srv: srv, token: idp.mint(user)}},
		DisableStandaloneSSE: true,
	}, nil)
	require.NoError(t, err)
	defer func() { _ = cs.Close() }()
	res, err := cs.CallTool(ctx, &mcpgo.CallToolParams{
		Name: "read_channel", Arguments: map[string]any{"id": "TSK-001", "channel_type": "room"},
	})
	require.NoError(t, err)
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcpgo.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String(), res.IsError
}

// jwtCall serves one Configure request as user.
func jwtCall(t *testing.T, srv *server, idp testIDP, user, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Host = srv.addr
	req.Header.Set("Origin", "http://"+srv.addr)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Assertion", idp.mint(user))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

// TestConfigure_RemoteMCPChannelsFollowSave pins that a Configure save gives
// the remote MCP channel tools the new generation's channel service. rela-
// server does not watch acl.yaml, so an edit to it applies only when a save
// builds a new generation: before the save carol still reads nothing, and
// after it the MCP endpoint applies the new acl.yaml.
func TestConfigure_RemoteMCPChannelsFollowSave(t *testing.T) {
	srv, dir, idp := mcpConfigServer(t)

	text, isErr := readRoom(t, srv, idp, "bob")
	require.False(t, isErr, "bob reads tasks and so the room: %s", text)
	_, isErr = readRoom(t, srv, idp, "carol")
	require.True(t, isErr, "carol has no role")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "acl.yaml"), []byte(cfgACL+"  carol: viewer\n"), 0o644))
	_, isErr = readRoom(t, srv, idp, "carol")
	require.True(t, isErr, "an acl.yaml edit applied before any save")

	rec := jwtCall(t, srv, idp, "ada", http.MethodGet, dataentry.ConfigurePath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var snap struct {
		Version string          `json:"version"`
		Schema  json.RawMessage `json:"schema"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &snap))
	tree, err := configedit.UnmarshalTree(snap.Schema)
	require.NoError(t, err)
	schema := tree.(*configedit.Map)
	taskProperties(t, schema).Set("due", mapOf("type", "date"))
	raw, err := json.Marshal(schema)
	require.NoError(t, err)

	first := srv.cur.Load()
	rec = jwtCall(t, srv, idp, "ada", http.MethodPost, dataentry.ConfigurePath+"/save",
		configedit.Draft{BaseVersion: snap.Version, Schema: raw})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotSame(t, first, srv.cur.Load(), "the save switched generations")

	text, isErr = readRoom(t, srv, idp, "carol")
	require.False(t, isErr, "the MCP channel tools did not follow the save: %s", text)
}
