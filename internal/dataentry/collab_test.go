package dataentry

import (
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gws "github.com/gorilla/websocket"
	"github.com/reearth/ygo/crdt"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// collabPolicy: alice reads and updates tickets, bob only reads them, and
// neither reads features.
func collabPolicy() *acl.Policy {
	return &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"author": {Read: []string{"ticket"}, Update: []string{"ticket"}},
			"viewer": {Read: []string{"ticket"}},
		},
		Assignments: map[string]string{"alice": "author", "bob": "viewer"},
	}
}

type collabFixture struct {
	app   *App
	relay *CollabRelay
	srv   *httptest.Server
}

// newCollabFixture serves handleCollab for app over a real listener. The
// X-Test-User header names the principal, bound like the middleware would.
func newCollabFixture(t *testing.T, build string) *collabFixture {
	t.Helper()
	app, d := newBatchACLApp(t, collabPolicy(),
		&entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "a"}},
		&entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "secret"}},
	)
	relay := NewCollabRelay(build)
	require.NoError(t, app.SetSecurityConfig(SecurityConfig{BindAddress: "127.0.0.1:8080"}))
	require.NoError(t, EnableCollab(app, relay))
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+CollabPathPrefix+"{type}/{id}", func(w http.ResponseWriter, r *http.Request) {
		pctx := principal.With(r.Context(), principal.Principal{User: r.Header.Get("X-Test-User"), Tool: principal.ToolDataEntry})
		ctx := gateCtxFor(pctx, t, d)
		if exp := r.Header.Get("X-Test-Expiry"); exp != "" {
			at, err := time.Parse(time.RFC3339Nano, exp)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			ctx = withCredentialExpiry(ctx, at)
		}
		sw := &statsResponseWriter{ResponseWriter: w}
		handleCollab(app.collab, sw, r.WithContext(ctx))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(func() {
		srv.Close()
		_ = relay.Shutdown(t.Context())
	})
	return &collabFixture{app: app, relay: relay, srv: srv}
}

func (f *collabFixture) dial(t *testing.T, user, path string) (*gws.Conn, *http.Response, error) {
	t.Helper()
	u := "ws" + strings.TrimPrefix(f.srv.URL, "http") + path
	h := http.Header{"X-Test-User": {user}}
	return gws.DefaultDialer.Dial(u, h)
}

func (f *collabFixture) join(t *testing.T, user, path string) *gws.Conn {
	t.Helper()
	c, resp, err := f.dial(t, user, path)
	require.NoError(t, err)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// sendText sends a y-websocket sync update that inserts s into the
// document's "body" text.
func sendText(t *testing.T, c *gws.Conn, s string) {
	t.Helper()
	doc := crdt.New()
	text := doc.GetText("body")
	doc.Transact(func(tx *crdt.Transaction) { text.Insert(tx, 0, s, nil) })
	update := crdt.EncodeStateAsUpdateV1(doc, nil)
	msg := binary.AppendUvarint([]byte{0, 2}, uint64(len(update)))
	msg = append(msg, update...)
	require.NoError(t, c.WriteMessage(gws.BinaryMessage, msg))
}

// roomText reads the relay's copy of the fixture entity's "body" text.
func (f *collabFixture) roomText() string {
	doc := f.relay.srv.GetDoc("ticket/TKT-001")
	if doc == nil {
		return ""
	}
	return doc.GetText("body").ToString()
}

// closeCode reads until the server closes c and returns the close code.
func closeCode(t *testing.T, c *gws.Conn) int {
	t.Helper()
	require.NoError(t, c.SetReadDeadline(time.Now().Add(5*time.Second)))
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			var ce *gws.CloseError
			require.ErrorAs(t, err, &ce)
			return ce.Code
		}
	}
}

func TestCollab_JoinNeedsRead(t *testing.T) {
	f := newCollabFixture(t, "")
	for _, path := range []string{"feature/FEAT-001", "feature/FEAT-404"} {
		t.Run(path, func(t *testing.T) {
			_, resp, err := f.dial(t, "alice", CollabPathPrefix+path)
			require.Error(t, err)
			require.NotNil(t, resp)
			_ = resp.Body.Close()
			require.Equal(t, http.StatusNotFound, resp.StatusCode)
		})
	}
}

func TestCollab_ReadOnlyPeerCannotWrite(t *testing.T) {
	f := newCollabFixture(t, "")
	path := CollabPathPrefix + "ticket/TKT-001"

	bob := f.join(t, "bob", path)
	sendText(t, bob, "from bob")
	alice := f.join(t, "alice", path)
	sendText(t, alice, "from alice")

	require.Eventually(t, func() bool { return f.roomText() == "from alice" },
		2*time.Second, 10*time.Millisecond, "room holds %q", f.roomText())
}

func TestCollab_StaleBuildIsClosed(t *testing.T) {
	f := newCollabFixture(t, "build-b")
	path := CollabPathPrefix + "ticket/TKT-001"

	stale := f.join(t, "alice", path+"?build=build-a")
	require.Equal(t, collabCloseStaleBuild, closeCode(t, stale))

	current := f.join(t, "alice", path+"?build=build-b")
	sendText(t, current, "x")
	require.Eventually(t, func() bool { return f.roomText() == "x" },
		2*time.Second, 10*time.Millisecond)
}

func TestCollab_LifetimeAndRetireClose(t *testing.T) {
	t.Run("lifetime", func(t *testing.T) {
		f := newCollabFixture(t, "")
		f.app.collab.lifetime = 50 * time.Millisecond
		c := f.join(t, "alice", CollabPathPrefix+"ticket/TKT-001")
		require.Equal(t, collabCloseRejoin, closeCode(t, c))
	})
	t.Run("retire", func(t *testing.T) {
		f := newCollabFixture(t, "")
		c := f.join(t, "alice", CollabPathPrefix+"ticket/TKT-001")
		Retire(f.app)
		require.Equal(t, collabCloseRejoin, closeCode(t, c))
	})
}

// A connection never outlives the credential it was admitted under.
func TestCollab_CredentialExpiry(t *testing.T) {
	path := CollabPathPrefix + "ticket/TKT-001"
	dial := func(f *collabFixture, expiry time.Time) (*gws.Conn, *http.Response, error) {
		u := "ws" + strings.TrimPrefix(f.srv.URL, "http") + path
		h := http.Header{"X-Test-User": {"alice"}, "X-Test-Expiry": {expiry.Format(time.RFC3339Nano)}}
		return gws.DefaultDialer.Dial(u, h)
	}
	t.Run("expired credential is refused", func(t *testing.T) {
		f := newCollabFixture(t, "")
		_, resp, err := dial(f, time.Now().Add(-time.Second))
		require.Error(t, err)
		require.NotNil(t, resp)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
	t.Run("connection closes when the credential expires", func(t *testing.T) {
		f := newCollabFixture(t, "")
		c, resp, err := dial(f, time.Now().Add(100*time.Millisecond))
		require.NoError(t, err)
		_ = resp.Body.Close()
		t.Cleanup(func() { _ = c.Close() })
		require.Equal(t, collabCloseRejoin, closeCode(t, c))
	})
}

func TestCollab_RefusedWithoutUpgradeOrRelay(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, CollabPathPrefix+"ticket/TKT-001", http.NoBody)
	w := httptest.NewRecorder()
	handleCollab(nil, w, r)
	require.Equal(t, http.StatusNotFound, w.Code)

	f := newCollabFixture(t, "")
	w = httptest.NewRecorder()
	handleCollab(f.app.collab, w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCollab_ConfigReportsRelay(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		app := newTestAppV1(t)
		relay := NewCollabRelay("b1")
		require.NoError(t, app.SetSecurityConfig(SecurityConfig{BindAddress: "127.0.0.1:8080"}))
		if enabled {
			require.NoError(t, EnableCollab(app, relay))
		}
		r := httptest.NewRequest(http.MethodGet, "/api/v1/_config", http.NoBody)
		r.Host = "127.0.0.1:8080"
		r.Header.Set("Origin", "http://127.0.0.1:8080")
		w := httptest.NewRecorder()
		app.NewRouter().ServeHTTP(w, r)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var cfg v1.Config
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &cfg))
		require.Equal(t, enabled, cfg.App.CollabEnabled)
		if enabled {
			require.Equal(t, relay.Epoch(), cfg.App.CollabEpoch)
		} else {
			require.Empty(t, cfg.App.CollabEpoch)
		}
	}
}

func TestCollab_EnableRejectsMissingDeps(t *testing.T) {
	require.Error(t, EnableCollab(newTestAppV1(t), nil), "nil relay")
	require.Error(t, EnableCollab(newTestAppV1(t), NewCollabRelay("")), "no security config")
}

// A client served by an earlier process must not refill a room.
func TestCollab_EarlierEpochIsClosed(t *testing.T) {
	f := newCollabFixture(t, "")
	path := CollabPathPrefix + "ticket/TKT-001"
	old := f.join(t, "alice", path+"?epoch=earlier")
	require.Equal(t, collabCloseGone, closeCode(t, old))

	current := f.join(t, "alice", path+"?epoch="+f.relay.Epoch())
	sendText(t, current, "x")
	require.Eventually(t, func() bool { return f.roomText() == "x" },
		2*time.Second, 10*time.Millisecond)
}

func TestCollab_ConnectionsPerPrincipalAreCapped(t *testing.T) {
	f := newCollabFixture(t, "")
	path := CollabPathPrefix + "ticket/TKT-001"
	for range collabMaxConnsPerPrincipal {
		f.join(t, "bob", path)
	}
	_, resp, err := f.dial(t, "bob", path)
	require.Error(t, err)
	require.NotNil(t, resp)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)

	// Another principal is not affected.
	f.join(t, "alice", path)
}

// An open connection ends once a recheck no longer finds the entity.
func TestCollab_RecheckClosesLostAccess(t *testing.T) {
	f := newCollabFixture(t, "")
	f.app.collab.recheck = 20 * time.Millisecond
	c := f.join(t, "bob", CollabPathPrefix+"ticket/TKT-001")
	_, err := f.app.store.DeleteFamily(t.Context(), "TKT-001", true)
	require.NoError(t, err)
	require.Equal(t, collabCloseRejoin, closeCode(t, c))
}

func TestCollab_RoomSizeIsCapped(t *testing.T) {
	f := newCollabFixture(t, "")
	f.relay.roomBytes["ticket/TKT-001"] = collabMaxRoomUpdateBytes
	c := f.join(t, "alice", CollabPathPrefix+"ticket/TKT-001")
	sendText(t, c, "x")
	require.Equal(t, collabCloseTooLarge, closeCode(t, c))
	require.Empty(t, f.roomText())
}
