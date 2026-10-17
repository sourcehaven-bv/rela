package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/ingest"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// DefaultWebhookMaxBodyBytes caps an inbound declarative-webhook body. The
// parsed body becomes an in-memory map that every template interpolation reads,
// so this bounds real memory per concurrent delivery — the action endpoint has
// no such cap, which is precisely the gap this closes.
//
// 1 MiB is generous for the payload shapes this serves (a monitoring alert, a
// form post, an upstream event) while staying small enough that a burst of
// concurrent deliveries cannot exhaust the heap.
const DefaultWebhookMaxBodyBytes int64 = 1 << 20

// webhookMaxAttempts is the conflict-retry budget handed to ingest.Run, set to
// the concurrency webhookMaxInFlight admits (see ingest.Deps.MaxAttempts). A
// caller that does not retry is better served by a fast, honest 409
// than a slow success, and exceeding the budget is reported, never silently
// dropped.
const webhookMaxAttempts = webhookMaxInFlight

// webhookExecTimeout bounds one delivery end to end, so a pathological hook
// cannot pin a request goroutine indefinitely.
const webhookExecTimeout = 30 * time.Second

// errWebhookBusy reports that the in-flight bound was reached. Answered as 503
// with Retry-After rather than 429: the caller did nothing wrong and is not
// rate-limited per se — the server is momentarily saturated, which is what 503
// means. A monitoring producer treats both as retryable, but 503 is the honest
// one.
var errWebhookBusy = errors.New("dataentry: webhook busy")

// webhookResult is the outcome of one delivery, serialized to the caller.
type webhookResult struct {
	Hook     string `json:"hook"`
	Action   string `json:"action"` // created | updated | no_match
	EntityID string `json:"entity_id,omitempty"`
}

// webhookRouter owns the declarative-webhook pipeline. It is a focused type
// rather than a cluster of methods on App for the reason CLAUDE.md gives: App
// is at its plimsoll method load line, and seven more handler methods there
// would be seven more things reaching into one struct's fields. This holds the
// three collaborators the pipeline actually needs.
//
// The fields are live accessors, not snapshots, because data-entry.yaml is
// reloadable and the store/manager are swapped on rebind — capturing them at
// construction would pin a stale wiring after a reload.
type webhookRouter struct {
	// state reads the current config+metamodel snapshot.
	state func() *Schema
	// write is the write surface (manager, luaDeps).
	write *writeHandler
	// rawStore is the ungated store, used ONLY for the write-prep body re-read
	// before an append_section step (ingest.Deps.WritePrep). Reads that decide
	// what a delivery acts on go through write.luaDeps().VisibleReader instead.
	rawStore store.Store

	// admit bounds how many deliveries may be in the pipeline at once.
	//
	// This endpoint is unauthenticated BY DESIGN (the fronting proxy owns
	// producer auth), so anyone who can reach the port can reach the write
	// path. Each delivery runs a full type scan and a write, so without a bound
	// a flood turns into unbounded concurrent scans that starve the store for
	// every other caller — the SPA, actions, other nodes. There is no
	// authorization gate to move ahead of that work, so the bound IS the
	// mitigation.
	//
	// Buffered channel rather than a semaphore package: the non-blocking send
	// gives "reject immediately when full" for free, which is what a producer
	// wants — a fast 503 it can retry beats a slow success it has timed out on.
	//
	// Built ONCE per router and shared, like cmdexec's pool. A per-request
	// bound would bound nothing.
	admit chan struct{}
}

// webhookMaxInFlight caps concurrent deliveries across all hooks.
//
// Each delivery scans a whole entity type, so admitting many more than this
// buys contention, not throughput. Small enough that a flood is shed rather
// than absorbed, large enough that a legitimate burst from a monitoring fan-out
// (Icinga dispatches notifications concurrently with no cap of its own) is not
// rejected in normal operation.
const webhookMaxInFlight = 8

// registerDeclarativeWebhookRoutes mounts POST /hooks/{id} for every configured
// webhook.
//
// # Reachability
//
// Mounted on the OUTER mux, exactly like registerWebhookRoutes. `inner` is only
// reachable under the `/api/` prefix, so registering a non-/api/ path there
// leaves it unreachable and every request falls through to the SPA catch-all —
// a 200 of HTML with the handler never running (BUG-F3ADZO). The route table in
// router_walk_test.go is what keeps that from recurring.
//
// # Route shape
//
// One pattern per configured hook, resolved at registration from the config
// snapshot. A single `/hooks/{id}` pattern with a runtime lookup would be
// reloadable, but ServeMux gives no way to unregister, so the two would drift
// after a config reload; registering per hook keeps the mounted set and the
// validated set identical at startup. A hook added to data-entry.yaml therefore
// needs a restart, matching how routes behave elsewhere in this package.
// A free function taking the router explicitly rather than an App method: it is
// route wiring, not application behavior, and App is at its plimsoll method load
// line (the same reasoning as dispatchWebhookAction in webhook.go).
func registerDeclarativeWebhookRoutes(mux *http.ServeMux, hooks *webhookRouter) {
	cfg := hooks.state().Cfg
	if len(cfg.Webhooks) == 0 {
		return
	}
	// Deterministic order so registration is reproducible and a duplicate
	// pattern (impossible today — map keys are unique) would fail predictably.
	ids := make([]string, 0, len(cfg.Webhooks))
	for id := range cfg.Webhooks {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		hookID := id
		mux.HandleFunc("POST /hooks/"+hookID, func(w http.ResponseWriter, r *http.Request) {
			hooks.handle(w, r, hookID)
		})
	}
}

// handleDeclarativeWebhook runs one delivery through the configured pipeline.
func (h *webhookRouter) handle(w http.ResponseWriter, r *http.Request, hookID string) {
	// Re-read config per request so a reload that changes a hook's BEHAVIOR
	// takes effect without a restart (only the mounted route set is fixed).
	hook, ok := h.state().Cfg.Webhooks[hookID]
	if !ok {
		// The route exists but the hook no longer does — a reload removed it.
		// A hook id is config, not data, so naming it is not a disclosure (see
		// "The configuration is not a secret" in CLAUDE.md).
		http.Error(w, "unknown webhook: "+hookID, http.StatusNotFound)
		return
	}

	payload, err := readWebhookPayload(r, hook)
	if err != nil {
		var tooLarge *webhookBodyTooLargeError
		if errors.As(err, &tooLarge) {
			http.Error(w, tooLarge.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), webhookExecTimeout)
	defer cancel()

	// Attribute the write to the hook rather than to the data-entry default, so
	// the audit log names which endpoint produced the entity.
	//
	// The name is under principal.ReservedPrefix deliberately. A hook is granted
	// its write rights in acl.yaml — that is the only way it gets any — so this
	// name is BOTH grantable and privileged, exactly the shape the reserved
	// namespace exists to protect. Without the prefix, a deployment using
	// -principal-header would let a caller assert `webhook:icinga-alert` on the
	// ordinary /api/ surface and inherit the hook's grants, because the ACL
	// resolves a principal by matching the raw string and cannot tell where it
	// came from. Reusing the existing prefix inherits principal.IsReserved's
	// refusal at every request-path entry point rather than adding a second
	// boundary that must be remembered.
	//
	// The principal also carries principal_type `webhook`, so an acl.yaml
	// baseline with `applies_to: [webhook]` caps every hook below its role and
	// below `everyone`. The edge binds one Request for this principal, so the
	// find, the write and any foreground automation the write fires all see
	// the same ceiling.
	ctx = principal.With(ctx, principal.Webhook(hookID))
	// This route is outside /api/, so attachACLRequest did not run: resolve
	// the hook's identity here, once, as every request edge does, and attach
	// its read gate (a request under a policy with no gate is denied every
	// read). Binding replaces any Request already on ctx.
	ctx, err = acl.BindRequest(ctx, h.write.acl())
	if err != nil {
		writeWebhookError(w, hookID, err)
		return
	}
	ctx, err = attachBoundReadGate(ctx, h.write.acl())
	if err != nil {
		writeWebhookError(w, hookID, err)
		return
	}

	result, err := h.runPipeline(ctx, hookID, hook, payload)
	if err != nil {
		writeWebhookError(w, hookID, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	// The response names the entity it wrote, and entity ids are the one
	// identifier rela treats as secret (hence the uniform 404 on reads). This
	// route sits outside /api/, so noCacheMiddleware never runs for it — set the
	// header here rather than relying on a middleware that does not apply.
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(hook.Respond.StatusOrDefault())
	_ = json.NewEncoder(w).Encode(result)
}

// writeWebhookError maps a pipeline failure onto a status the producer can act
// on. The distinction matters because senders treat the classes differently: a
// 409 says "your delivery was fine, contention lost" and is worth retrying,
// while a 422 says "this payload cannot produce a valid entity" and is not.
func writeWebhookError(w http.ResponseWriter, hookID string, err error) {
	switch {
	case errors.Is(err, errWebhookBusy):
		// Retry-After is what makes this actionable: a producer that honors it
		// backs off instead of adding to the saturation it just hit.
		w.Header().Set("Retry-After", "2")
		slog.Warn("webhook shed load", "hook", hookID, "max_in_flight", webhookMaxInFlight)
		http.Error(w, "server busy, retry shortly", http.StatusServiceUnavailable)
	case errors.Is(err, ingest.ErrConflictExhausted):
		slog.Warn("webhook conflict retries exhausted", "hook", hookID)
		http.Error(w, "conflict: concurrent writes to the same entity", http.StatusConflict)
	case errors.Is(err, context.DeadlineExceeded):
		slog.Warn("webhook timed out", "hook", hookID)
		http.Error(w, "webhook processing timed out", http.StatusGatewayTimeout)
	default:
		// Do not echo err: it can carry stored property values from the write
		// path, and the producer is not necessarily entitled to them.
		slog.Error("webhook failed", "hook", hookID, "error", err)
		http.Error(w, "webhook processing failed", http.StatusInternalServerError)
	}
}

// runPipeline sheds load, then runs the hook's mapping through ingest.Run.
//
// The deps are built per delivery from the router's live accessors: the reader
// and manager are swapped on a config reload.
func (h *webhookRouter) runPipeline(
	ctx context.Context, hookID string, hook dataentryconfig.Webhook, payload webhookPayload,
) (webhookResult, error) {
	// Shed load BEFORE doing any work. Queueing here instead would mean an
	// unauthenticated flood parks goroutines that each go on to scan the
	// store; a fast 503 the producer can retry is the better answer. A nil
	// channel means unbounded, which only happens in tests that construct the
	// router directly.
	if h.admit != nil {
		select {
		case h.admit <- struct{}{}:
			defer func() { <-h.admit }()
		default:
			return webhookResult{}, errWebhookBusy
		}
	}

	deps := h.write.luaDeps()
	res, err := ingest.Run(ctx, ingest.Deps{
		Reader:      deps.VisibleReader,
		World:       deps.World,
		Writer:      h.write.manager,
		WritePrep:   h.rawStore,
		MaxAttempts: webhookMaxAttempts,
		Label:       "webhook:" + hookID,
	}, hook.Mapping(), payload)
	if err != nil {
		return webhookResult{}, err
	}
	return webhookResult{Hook: hookID, Action: res.Action, EntityID: res.EntityID}, nil
}

// --- payload ---------------------------------------------------------------

// webhookBodyTooLargeError reports a body over the hook's cap.
type webhookBodyTooLargeError struct{ limit int64 }

func (e *webhookBodyTooLargeError) Error() string {
	return "request body exceeds " + strconv.FormatInt(e.limit, 10) + " bytes"
}

// webhookPayload is the interpolation scope for one delivery: the parsed body,
// the query string, and the ALLOWLISTED headers.
type webhookPayload struct {
	body    map[string]any
	query   url.Values
	headers map[string]string
	now     time.Time
}

// readWebhookPayload reads and parses the request within the hook's body cap.
//
// The cap is enforced with an io.LimitReader at limit+1 bytes so an oversized
// body is DETECTED rather than silently truncated — a truncated JSON body would
// usually fail to parse, but a truncated form body parses fine and would write
// a quietly-wrong entity.
func readWebhookPayload(r *http.Request, hook dataentryconfig.Webhook) (webhookPayload, error) {
	limit := hook.MaxBodyBytes
	if limit <= 0 {
		limit = DefaultWebhookMaxBodyBytes
	}

	raw, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil {
		return webhookPayload{}, fmt.Errorf("read body: %w", err)
	}
	if int64(len(raw)) > limit {
		return webhookPayload{}, &webhookBodyTooLargeError{limit: limit}
	}

	p := webhookPayload{
		body:    map[string]any{},
		query:   r.URL.Query(),
		headers: extractAllowedHeaders(r, hook.Headers),
		now:     time.Now().UTC(),
	}

	contentType := r.Header.Get("Content-Type")
	mediaType, _, _ := strings.Cut(contentType, ";")
	switch strings.TrimSpace(strings.ToLower(mediaType)) {
	case "application/x-www-form-urlencoded":
		values, parseErr := url.ParseQuery(string(raw))
		if parseErr != nil {
			return webhookPayload{}, fmt.Errorf("parse form body: %w", parseErr)
		}
		for k := range values {
			p.body[k] = values.Get(k)
		}
	default:
		// JSON is the default: it is what every webhook producer this targets
		// sends, and an empty body is legitimate for a hook driven entirely by
		// query parameters.
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &p.body); err != nil {
				return webhookPayload{}, fmt.Errorf("parse json body: %w", err)
			}
		}
	}
	return p, nil
}

// extractAllowedHeaders projects ONLY the allowlisted headers.
//
// An allowlist, never pass-through: request headers carry session cookies,
// bearer tokens and proxy-injected identity assertions, and a template that
// could reach any header would let a hook persist one into entity content —
// where it is then served back on every read. The config validator additionally
// refuses the always-wrong names outright.
func extractAllowedHeaders(r *http.Request, allowed []string) map[string]string {
	if len(allowed) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(allowed))
	for _, name := range allowed {
		if v := r.Header.Get(name); v != "" {
			out[strings.ToLower(name)] = v
		}
	}
	return out
}

// DefaultMatchRef implements ingest.Source: a find.match property with no
// find.values entry reads the body field of the same name.
func (p webhookPayload) DefaultMatchRef(prop string) string {
	return "body." + prop
}

// Kind implements ingest.Source. Every webhook value is raw: the producer is
// authenticated by the fronting proxy and may send markdown on purpose.
func (webhookPayload) Kind(string) ingest.ValueKind { return ingest.ValueRaw }

// Resolve implements ingest.Source: it looks up one {{...}} reference,
// returning "" when absent. Vocabulary, extending the automation template
// namespace:
//
//	{{body.<path>}}   a field of the parsed body (dotted path into nested objects)
//	{{query.<name>}}  a query-string parameter
//	{{header.<name>}} an ALLOWLISTED request header (case-insensitive)
//	{{now}}           delivery timestamp, RFC 3339 UTC
//	{{today}}         delivery date, YYYY-MM-DD
//
// It returns the RAW producer-supplied value, which is not safe to write into a
// document as-is. Only ingest.Interpolate may call it: that is where each value
// is flattened, the one guard that stops a payload forging markdown structure.
func (p webhookPayload) Resolve(ref string) string {
	switch ref {
	case "now":
		return p.now.Format(time.RFC3339)
	case "today":
		return p.now.Format("2006-01-02")
	}
	namespace, path, ok := strings.Cut(ref, ".")
	if !ok {
		return ""
	}
	switch namespace {
	case "body":
		return stringifyWebhookValue(lookupPath(p.body, path))
	case "query":
		return p.query.Get(path)
	case "header":
		return p.headers[strings.ToLower(path)]
	default:
		return ""
	}
}

// lookupPath walks a dotted path into a decoded JSON object.
func lookupPath(root map[string]any, path string) any {
	var current any = root
	for segment := range strings.SplitSeq(path, ".") {
		obj, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = obj[segment]
		if !ok {
			return nil
		}
	}
	return current
}

// stringifyWebhookValue renders a decoded JSON value for substitution.
//
// Numbers are formatted without a trailing ".0" because encoding/json decodes
// every number as float64: a plain %v would turn an alert's `attempt: 3` into
// "3" but `port: 8080` into "8080" and `1e+06` for a large one, so the integral
// case is normalized explicitly.
func stringifyWebhookValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		// Objects and arrays: JSON, so a nested payload is at least faithfully
		// representable rather than Go-syntax noise.
		encoded, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return string(encoded)
	}
}
