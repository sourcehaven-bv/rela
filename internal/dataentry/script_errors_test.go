package dataentry

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// writeV1ScriptError must use the caller-supplied correlation id. This
// matters because singleflight may hand the same *lua.ScriptError to
// multiple in-flight requests; if we read se.CorrelationID directly,
// the first request's id would surface in the second request's response.
func TestWriteV1ScriptError_CorrelationIDOverride(t *testing.T) {
	se := &lua.ScriptError{
		Surface:       lua.SurfaceDocument,
		Path:          "scripts/x.lua",
		LuaMessage:    "boom",
		CorrelationID: "engine-id",
	}

	rec := httptest.NewRecorder()

	writeV1ScriptError(rec, se, false, "handler-id")

	var env ScriptErrorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if env.CorrelationID != "handler-id" {
		t.Errorf("CorrelationID=%q, want handler-id (caller wins)", env.CorrelationID)
	}

	// Second request: empty handler id falls back to engine id.
	rec = httptest.NewRecorder()
	writeV1ScriptError(rec, se, false, "")
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if env.CorrelationID != "engine-id" {
		t.Errorf("CorrelationID=%q, want engine-id (fallback)", env.CorrelationID)
	}
}
