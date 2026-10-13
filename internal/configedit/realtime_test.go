package configedit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSave_RefusesRealtime pins that a Configure save cannot write
// realtime.yaml. A draft carries only schema.yaml and data-entry.yaml, and
// the handler refuses a draft with any other file, so a save that tries to
// change realtime.yaml is a bad request and writes nothing: its roles and
// triggers decide who reads and posts into channels, which is not a browser
// edit.
func TestSave_RefusesRealtime(t *testing.T) {
	h := newHarness(t, counts{}, nil)
	const realtime = "version: 1\n"
	h.files["realtime.yaml"] = []byte(realtime)
	snap, err := h.svc.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{
		"base_version": snap.Version,
		"realtime":     map[string]any{"version": 1, "channel_types": map[string]any{"room": map[string]any{}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.svc.Handler("/api/configure").ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/configure/save", strings.NewReader(string(body))))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want 400 for a draft naming realtime.yaml (body %s)", rec.Code, rec.Body)
	}
	if string(h.files["realtime.yaml"]) != realtime || string(h.files["schema.yaml"]) != testSchema ||
		len(h.act.activated) != 0 || h.act.paused != 0 {

		t.Fatal("a refused save changed something")
	}
}
