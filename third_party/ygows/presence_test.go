package websocket

// rela fork: tests for presence.go and the awareness ownership rule.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gws "github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/reearth/ygo/encoding"
)

// stampUser sets user.name to name and keeps nothing else the client sent
// under user.
func stampUser(name string) func(map[string]any) map[string]any {
	return func(state map[string]any) map[string]any {
		state["user"] = map[string]any{"name": name}
		return state
	}
}

// decodeAwFrame returns the states of an awareness frame keyed by client id.
func decodeAwFrame(t *testing.T, frame []byte) map[uint64]string {
	t.Helper()
	dec := encoding.NewDecoder(frame)
	n, err := dec.ReadVarUint()
	require.NoError(t, err)
	out := map[uint64]string{}
	for i := uint64(0); i < n; i++ {
		id, err := dec.ReadVarUint()
		require.NoError(t, err)
		_, err = dec.ReadVarUint()
		require.NoError(t, err)
		raw, err := dec.ReadVarBytes()
		require.NoError(t, err)
		out[id] = string(raw)
	}
	return out
}

func TestStampAwareness(t *testing.T) {
	tests := []struct {
		name    string
		frame   []byte
		want    map[uint64]string
		wantErr bool
	}{
		{
			name:  "does not escape HTML characters",
			frame: awFrame(awEntry{7, 1, `{"s":"<&>"}`}),
			want:  map[uint64]string{7: `{"s":"<&>","user":{"name":"alice"}}`},
		},
		{
			name:  "rewrites a live state",
			frame: awFrame(awEntry{7, 1, `{"user":{"name":"bob","color":"#123456"},"cursor":null}`}),
			want:  map[uint64]string{7: `{"cursor":null,"user":{"name":"alice"}}`},
		},
		{
			name:  "passes a removal through",
			frame: awFrame(nullEntry(7, 2)),
			want:  map[uint64]string{7: "null"},
		},
		{
			name:  "keeps large integers exact",
			frame: awFrame(awEntry{7, 1, `{"pos":9007199254740993}`}),
			want:  map[uint64]string{7: `{"pos":9007199254740993,"user":{"name":"alice"}}`},
		},
		{
			name:    "refuses a state over the size limit",
			frame:   awFrame(awEntry{7, 1, `{"x":"` + strings.Repeat("a", maxStampedStateBytes) + `"}`}),
			wantErr: true,
		},
		{
			name:    "refuses a state nested too deep",
			frame:   awFrame(awEntry{7, 1, strings.Repeat(`{"a":`, maxStampedDepth+1) + "1" + strings.Repeat("}", maxStampedDepth+1)}),
			wantErr: true,
		},
		{
			name:  "ignores brackets inside strings",
			frame: awFrame(awEntry{7, 1, `{"s":"` + strings.Repeat("[", 50) + `\"x"}`}),
			want:  map[uint64]string{7: `{"s":"` + strings.Repeat("[", 50) + `\"x","user":{"name":"alice"}}`},
		},
		{name: "refuses a state that is not an object", frame: awFrame(awEntry{7, 1, `[1]`}), wantErr: true},
		{name: "refuses trailing data after the object", frame: awFrame(awEntry{7, 1, `{} {}`}), wantErr: true},
		{name: "refuses a truncated frame", frame: []byte{1, 7}, wantErr: true},
		{
			name: "refuses too many entries",
			frame: encoding.EncodeBytes(func(enc *encoding.Encoder) {
				enc.WriteVarUint(maxStampedEntries + 1)
			}),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := stampAwareness(tc.frame, stampUser("alice"))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, decodeAwFrame(t, out))
		})
	}
}

// newStampServer returns a running server whose connections are stamped
// with the name in the "as" query parameter, and owned by it when withOwner.
func newStampServer(t *testing.T, withOwner bool) (*Server, *httptest.Server) {
	t.Helper()
	s := NewServer()
	s.MaxAwarenessClientsPerConn = 2
	s.Authorize = func(r *http.Request) (ConnectionConfig, bool) {
		as := r.URL.Query().Get("as")
		cfg := ConnectionConfig{StampAwareness: stampUser(as)}
		if withOwner {
			cfg.Owner = as
		}
		return cfg, true
	}
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	return s, ts
}

func dialAs(t *testing.T, ts *httptest.Server, name string) *gws.Conn {
	t.Helper()
	u := "ws" + strings.TrimPrefix(ts.URL, "http") + "/room?as=" + name
	conn, resp, err := gws.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)
	_ = resp.Body.Close()
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// userName returns the stamped user name of a client in the room, or "" when
// the client has no state.
func userName(t *testing.T, s *Server, id uint64) string {
	t.Helper()
	rm, _, err := s.getOrCreateRoom(context.Background(), "room")
	require.NoError(t, err)
	defer s.releaseInflight(rm)
	st, ok := rm.awareness.GetStates()[id]
	if !ok || st.State == nil {
		return ""
	}
	b, err := json.Marshal(st.State["user"])
	require.NoError(t, err)
	var user struct{ Name string }
	require.NoError(t, json.Unmarshal(b, &user))
	return user.Name
}

func TestStampAwareness_RoomStoresTheServerName(t *testing.T) {
	s, ts := newStampServer(t, true)
	alice := dialAs(t, ts, "alice")
	sendAwFrame(t, alice, awFrame(awEntry{7, 1, `{"user":{"name":"bob"}}`}))
	require.Eventually(t, func() bool { return userName(t, s, 7) == "alice" },
		2*time.Second, 10*time.Millisecond)
}

func TestAwarenessOwnership_PeerCannotTouchAnotherPeersClient(t *testing.T) {
	for _, withOwner := range []bool{false, true} {
		t.Run(map[bool]string{false: "without owner", true: "with owner"}[withOwner], func(t *testing.T) {
			peerCannotTouchAnotherPeersClient(t, withOwner)
		})
	}
}

func peerCannotTouchAnotherPeersClient(t *testing.T, withOwner bool) {
	s, ts := newStampServer(t, withOwner)
	alice := dialAs(t, ts, "alice")
	sendAwFrame(t, alice, awFrame(awEntry{7, 1, `{"cursor":1}`}))
	require.Eventually(t, func() bool { return userName(t, s, 7) == "alice" },
		2*time.Second, 10*time.Millisecond)

	mallory := dialAs(t, ts, "mallory")
	sendAwFrame(t, mallory, awFrame(awEntry{7, 2, `{"cursor":2}`}))
	sendAwFrame(t, mallory, awFrame(nullEntry(7, 3)))
	// A client of mallory's own is applied, so once it is in the room both
	// earlier frames have been handled.
	sendAwFrame(t, mallory, awFrame(awEntry{8, 1, `{}`}))
	require.Eventually(t, func() bool { return userName(t, s, 8) == "mallory" },
		2*time.Second, 10*time.Millisecond)
	assert.Equal(t, "alice", userName(t, s, 7))
}

// waitName waits until client id holds a state stamped with name, or none
// when name is "".
func waitName(t *testing.T, s *Server, id uint64, name string) {
	t.Helper()
	require.Eventually(t, func() bool { return userName(t, s, id) == name },
		2*time.Second, 10*time.Millisecond, "client %d: want %q, have %q", id, name, userName(t, s, id))
}

// Peers receive the stamped state, not the frame the client sent.
func TestStampAwareness_PeersReceiveTheServerName(t *testing.T) {
	_, ts := newStampServer(t, true)
	bob := dialAs(t, ts, "bob")
	alice := dialAs(t, ts, "alice")
	sendAwFrame(t, alice, awFrame(awEntry{7, 1, `{"user":{"name":"bob"}}`}))

	require.NoError(t, bob.SetReadDeadline(time.Now().Add(2*time.Second)))
	for {
		_, msg, err := bob.ReadMessage()
		require.NoError(t, err)
		dec := encoding.NewDecoder(msg)
		typ, err := dec.ReadVarUint()
		require.NoError(t, err)
		if typ != 1 {
			continue
		}
		payload, err := dec.ReadVarBytes()
		require.NoError(t, err)
		if st, ok := decodeAwFrame(t, payload)[7]; ok {
			assert.JSONEq(t, `{"user":{"name":"alice"}}`, st)
			return
		}
	}
}

// The owner's own connection may update and remove its client.
func TestAwarenessOwnership_OwnerUpdatesAndRemoves(t *testing.T) {
	s, ts := newStampServer(t, true)
	alice := dialAs(t, ts, "alice")
	sendAwFrame(t, alice, awFrame(awEntry{7, 1, `{"cursor":1}`}))
	waitName(t, s, 7, "alice")
	sendAwFrame(t, alice, awFrame(nullEntry(7, 2)))
	waitName(t, s, 7, "")
}

// A client id stays bound to its owner after the owner leaves, and a new
// connection of the same owner may take it over while the old one is live.
func TestAwarenessOwnership_BindingOutlivesTheConnection(t *testing.T) {
	s, ts := newStampServer(t, true)
	_ = dialAs(t, ts, "bob") // keeps the room, and its bindings, loaded
	alice := dialAs(t, ts, "alice")
	sendAwFrame(t, alice, awFrame(awEntry{7, 1, `{}`}))
	waitName(t, s, 7, "alice")

	// Same owner, second connection, old one still live: accepted.
	alice2 := dialAs(t, ts, "alice")
	sendAwFrame(t, alice2, awFrame(awEntry{7, 2, `{"tab":2}`}))
	require.Eventually(t, func() bool {
		rm, _, err := s.getOrCreateRoom(context.Background(), "room")
		require.NoError(t, err)
		defer s.releaseInflight(rm)
		return rm.awareness.GetStates()[7].Clock == 2
	}, 2*time.Second, 10*time.Millisecond)

	// Both alice connections leave; the binding remains.
	require.NoError(t, alice.Close())
	require.NoError(t, alice2.Close())
	waitName(t, s, 7, "")

	mallory := dialAs(t, ts, "mallory")
	sendAwFrame(t, mallory, awFrame(awEntry{7, 9, `{}`}))
	sendAwFrame(t, mallory, awFrame(awEntry{8, 1, `{}`}))
	waitName(t, s, 8, "mallory")
	assert.Empty(t, userName(t, s, 7))
}

func TestStampAwareness_NilStampIsRefused(t *testing.T) {
	_, err := stampAwareness(awFrame(awEntry{7, 1, `{}`}),
		func(map[string]any) map[string]any { return nil })
	require.ErrorIs(t, err, errStampNil)
}
