package dataentry

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// DefaultActionMaxBodyBytes caps an action request body (TKT-EFMRQM).
//
// The action endpoint had NO cap before this: it read the body straight into
// json.Decode, so a large POST was bounded only by whatever the fronting proxy
// happened to enforce. With a request-scoped action the parsed body also
// becomes a Lua table the script holds for its whole run, so the cap now bounds
// script-visible memory as well as decode memory.
//
// Same 1 MiB as DefaultWebhookMaxBodyBytes, and for the same reasons — the
// payload shapes are the same shapes. Sharing the number rather than the
// constant keeps the two independently tunable; sharing the CEILING
// (maxWebhookBodyCap, enforced at config load) is what stops either being
// raised without limit.
const DefaultActionMaxBodyBytes int64 = DefaultWebhookMaxBodyBytes

// errActionBodyTooLarge reports a body over the action's cap.
var errActionBodyTooLarge = errors.New("request body too large")

// actionPayload is one action invocation's decoded request: the entity_id the
// SPA sends, and the optional request-scoped projection.
type actionPayload struct {
	// EntityID is the caller-supplied entity id. It is read from the SAME
	// parsed body whether or not the action is request-scoped, so a request:
	// block is never a second route to an entity — the caller still gets it
	// resolved through visibility.ScriptReader in handleV1Action.
	EntityID string

	// Request is the lua.Request the script sees, or nil for an action with no
	// request: block.
	Request *lua.Request
}

// readActionPayload reads the request body under the action's cap and projects
// exactly what the action's `request:` block opted into.
//
// The cap is read at limit+1 bytes so an oversized body is DETECTED rather than
// truncated (the readWebhookPayload reasoning: truncated JSON usually fails to
// parse, but truncated form data parses fine and would have the script act on
// quietly-wrong values).
//
// A body that does not parse is NOT an error for a request-scoped action: the
// script may legitimately expect a text or vendor payload, and only it knows.
// It gets Raw with Body left nil. The pre-existing NON-request-scoped path
// keeps its 400 on malformed JSON, because there the body's only purpose is to
// carry entity_id and a caller who sent garbage got nothing they asked for.
func readActionPayload(r *http.Request, action dataentryconfig.Action) (actionPayload, error) {
	limit := DefaultActionMaxBodyBytes
	if action.Request != nil && action.Request.MaxBodyBytes > 0 {
		limit = action.Request.MaxBodyBytes
	}

	var raw []byte
	if r.Body != nil {
		var err error
		raw, err = io.ReadAll(io.LimitReader(r.Body, limit+1))
		if err != nil {
			return actionPayload{}, err
		}
		if int64(len(raw)) > limit {
			return actionPayload{}, errActionBodyTooLarge
		}
	}

	var body map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &body); err != nil {
			body = nil
			if action.Request == nil {
				// Legacy shape: the body exists only to carry entity_id.
				return actionPayload{}, err
			}
		}
	}

	payload := actionPayload{EntityID: stringField(body, "entity_id")}
	if action.Request == nil {
		return payload, nil
	}

	req := &lua.Request{
		Method:      r.Method,
		Path:        r.URL.Path,
		ContentType: r.Header.Get("Content-Type"),
		Headers:     extractAllowedHeaders(r, action.Request.Headers),
	}
	if action.Request.Body {
		req.Raw = string(raw)
		req.Body = body
	}
	if action.Request.Query {
		req.Query = r.URL.Query()
	}
	payload.Request = req
	return payload, nil
}

// stringField reads a string field out of a decoded JSON object, tolerating a
// nil map and a non-string value.
//
// `entity_type` is the field this deliberately does NOT read. The SPA still
// sends it, so it stays accepted on the wire, but the server must never
// authorize against it: a caller-supplied type is forgeable, and gating on it
// is a cross-type escalation (claim a type you may read, name an id of a type
// you may not — BUG-ZWTDH9). The stored type is the only one that means
// anything, and visibility.ScriptReader.GetEntity reads it from the row itself.
// TestAction_EntityTypeIsIgnored pins that no claim can change the outcome.
// Do not "notice it's missing" and wire it back in.
func stringField(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

// writeActionResponse serves a script's return value.
//
// Two shapes, discriminated by script.ActionResponse.IsRich:
//
//   - the SPA envelope (redirect/message/message_type), unchanged, still the
//     default so every existing action keeps working byte for byte;
//   - the rich shape, served with the script's status, body and content type.
//
// The rich body is HARDENED like an attachment or an export download, because
// it is exactly the same kind of output: attacker-influenceable bytes served
// same-origin with the SPA. nosniff pins the (allowlisted) declared type so a
// browser cannot re-interpret a text/plain body as HTML; the sandbox CSP
// neutralizes active content if it ever does; no-store keeps a response that
// may name entity ids out of shared caches.
//
// There is deliberately NO Content-Disposition: unlike an export this is an API
// response a producer reads programmatically, and an attachment disposition
// would break every non-browser consumer. The CSP and nosniff are what carry
// the safety, and the content-type allowlist (script.validActionContentTypes)
// is what keeps an active type off this surface in the first place.
func writeActionResponse(w http.ResponseWriter, resp *script.ActionResponse) {
	if resp == nil || (!resp.IsRich() && resp.Redirect == "" && resp.Message == "") {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if !resp.IsRich() {
		writeV1JSON(w, http.StatusOK, actionEnvelope(resp))
		return
	}

	hdr := w.Header()
	hdr.Set("Content-Type", resp.ContentType)
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	hdr.Set("Cache-Control", "no-store")
	w.WriteHeader(resp.Status)
	if _, err := w.Write([]byte(resp.Body)); err != nil {
		slog.Warn("dataentry: writing action response failed", "err", err)
	}
}

// actionEnvelope projects the SPA vocabulary onto the wire type. Isolated so
// the rich path in writeActionResponse cannot accidentally reach it.
func actionEnvelope(resp *script.ActionResponse) v1.ActionResponse {
	return v1.ActionResponse{
		Redirect:    resp.Redirect,
		Message:     resp.Message,
		MessageType: resp.MessageType,
	}
}
