package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/access"
	"github.com/Sourcehaven-BV/rela/internal/acl"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/remotestatus"
)

// remotesPath is the root of the remote-sync routes.
const remotesPath = "/api/v1/_remotes"

// remoteBodyLimit caps a POST body on the remote routes: a local id, a
// field name and an address.
const remoteBodyLimit = 16 << 10

// The errors a [RemoteService] wraps, so the routes can answer them.
var (
	// ErrRemoteUnknown: remotes.yaml declares no remote of that name.
	ErrRemoteUnknown = errors.New("dataentry: no such remote")
	// ErrRemoteNoEntry: the conflict list holds no entry the request names.
	ErrRemoteNoEntry = errors.New("dataentry: no such conflict entry")
	// ErrRemoteBusy: a sync or another change holds the remote now.
	ErrRemoteBusy = errors.New("dataentry: the remote is busy")
	// ErrRemoteRefused: the entry cannot take that action, such as a link
	// to an address that is not a row of the mapped remote type.
	ErrRemoteRefused = errors.New("dataentry: the remote entry refuses the action")
)

// RemoteService is the remote-sync capability the remote routes use
// (TKT-0FMYPEZ). The wiring site adapts the scheduler's remote functions to
// it. It reads and changes operator state outside the graph: the routes
// gate every entry and count by the caller's read access before serving it.
type RemoteService interface {
	// Remotes returns every remote in remotes.yaml with its state, sorted by
	// name.
	Remotes(ctx context.Context) ([]RemoteStatus, error)
	// Resolve resolves the one entry key names to side take, under the
	// remote's run lock, without a sync, and records it in the audit log.
	Resolve(ctx context.Context, name string, key RemoteEntryKey, take string) (int, error)
	// Dismiss removes the one superseded entry key names, as Resolve does.
	Dismiss(ctx context.Context, name string, key RemoteEntryKey) (int, error)
	// Link links the uncertain create key names to the remote row at
	// address, as Resolve does.
	Link(ctx context.Context, name string, key RemoteEntryKey, address string) error
}

// RemoteEntryKey names one conflict list entry: a local row and one of its
// fields, or the row itself when Field is empty. Fields is the set of local
// fields the handler checked on a row-level entry; the service refuses the
// write when the entry it finds under the lock names another set.
type RemoteEntryKey struct {
	Local, Field string
	Fields       []string
}

// RemoteStatus is one remote as [RemoteService.Remotes] returns it.
type RemoteStatus struct {
	// Remote's settings carry the credential's kind, never its source or
	// value.
	Remote     remotestatus.Remote
	LinkedRows []RemoteRow
	Entries    []RemoteEntry
}

// RemoteRow is a local row a remote's ledger links: an entity of Type with
// id Local, or a relation when Kind is [remoteKindRelation].
type RemoteRow struct {
	Kind, Type, Local string
	// Relation is the relation's key when Kind is [remoteKindRelation].
	Relation entityPkg.RelationKey
}

// RemoteEntry is one conflict list entry. Detail text from the remote is
// not part of it: Cause and Status say why a refused or uncertain entry
// is held.
type RemoteEntry struct {
	RemoteRow
	Address, Field, RemoteField string
	// Fields are the local fields a row-level entry tried to send.
	Fields              []string
	Reason, Cause, Rule string
	Fallback            string
	Status              int
	Since, Seen         time.Time
	RemoteValue         json.RawMessage
	RemoteValueOmitted  bool
	Superseded          *remotestatus.Superseded
}

// The entry kinds, reasons and sides the routes act on. They match the
// values remotesync stores.
const (
	remoteKindRelation    = "relation"
	remoteReasonBoth      = "both_changed"
	remoteReasonUncertain = "uncertain"
	remoteReasonSupersede = "superseded"
	remoteTakeLocal       = "local"
	remoteTakeRemote      = "remote"
)

// The per-entry `_actions` keys.
const (
	remoteActionResolveLocal  = "resolve_local"
	remoteActionResolveRemote = "resolve_remote"
	remoteActionDismiss       = "dismiss"
	remoteActionLink          = "link"
)

var remoteActions = []string{
	remoteActionResolveLocal, remoteActionResolveRemote, remoteActionDismiss, remoteActionLink,
}

// remotesHandler owns the remote-sync routes (TKT-0FMYPEZ):
//
//	GET  /api/v1/_remotes                               every remote, its run health and gated counts
//	GET  /api/v1/_remotes/{name}/conflicts              the gated conflict list, paged, with `_actions`
//	POST /api/v1/_remotes/{name}/conflicts/_resolve     {local, field, take}
//	POST /api/v1/_remotes/{name}/conflicts/_dismiss     {local, field}
//	POST /api/v1/_remotes/{name}/conflicts/_link        {local, address}
//
// Every route needs the remote:sync permission, checked like git:sync; a
// caller without it gets a 403 that names it. Each entry is then gated by
// the caller's read access to its local row and field. A hidden or missing
// entry is the uniform 404 on every write route. A nil *remotesHandler is
// "no remote service": every route answers 404. [SetRemotes] installs it.
type remotesHandler struct {
	app *App
	svc RemoteService
}

// SetRemotes installs the remote-sync service. Call it before
// [App.NewRouter]. It is a package function because App is at its plimsoll
// method cap.
//
// Nil: rejected for a and svc; a nil is a wiring bug.
func SetRemotes(a *App, svc RemoteService) error {
	if a == nil || svc == nil {
		return errors.New("dataentry: SetRemotes requires an app and a remote service")
	}
	a.remotes = &remotesHandler{app: a, svc: svc}
	return nil
}

// permitsRemoteSync reports whether the caller holds remote:sync, by the
// same predicate as [permitsGitSync].
func permitsRemoteSync(impl acl.ACL, r *http.Request) bool {
	ctx := r.Context()
	return access.ModeOf(impl).PermitsProjectWrite(ctx, readGateFromContext(ctx), acl.PermRemoteSync)
}

// handleV1Remotes serves every route under /api/v1/_remotes.
//
//archplan:auth=authz reason="requires remote:sync, then row- and field-gates each entry; writes re-check the gate"
func (h *remotesHandler) handleV1Remotes(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Not found", "")
		return
	}
	if !permitsRemoteSync(h.app.acl, r) {
		writeV1Error(w, r, http.StatusForbidden, "permission_required", "Forbidden",
			"Remote sync needs the "+acl.PermRemoteSync+" permission.")
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, remotesPath), "/"), "/")
	switch {
	case len(parts) == 1 && parts[0] == "":
		if allowMethod(w, r, http.MethodGet) {
			h.list(w, r)
		}
	case len(parts) == 2 && parts[1] == "conflicts" && principal.ValidIntegrationName(parts[0]):
		if allowMethod(w, r, http.MethodGet) {
			h.conflicts(w, r, parts[0])
		}
	case len(parts) == 3 && parts[1] == "conflicts" && principal.ValidIntegrationName(parts[0]):
		if allowMethod(w, r, http.MethodPost) {
			h.act(w, r, parts[0], parts[2])
		}
	default:
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Not found", "")
	}
}

// remoteSummary is one remote on GET /_remotes.
type remoteSummary struct {
	Name string `json:"name"`
	remotestatus.Settings
	// CredentialKind is the kind of source, never its name or value.
	CredentialKind             string                    `json:"credential_kind"`
	MaxClockSkewSeconds        int64                     `json:"max_clock_skew_seconds"`
	SupersededRetentionSeconds int64                     `json:"superseded_retention_seconds"`
	Types                      []remotestatus.MappedType `json:"types"`
	remotestatus.Run
	// Linked and Conflicts count only the rows the caller may read.
	Linked    int          `json:"linked"`
	Conflicts remoteCounts `json:"conflicts"`
}

// remoteCounts counts a remote's entries the caller may see, by reason.
type remoteCounts struct {
	Total    int            `json:"total"`
	ByReason map[string]int `json:"by_reason"`
}

// remoteList answers GET /_remotes.
type remoteList struct {
	Data []remoteSummary `json:"data"`
}

func (h *remotesHandler) list(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	remotes, err := h.svc.Remotes(ctx)
	if err != nil {
		writeRemoteError(w, r, err)
		return
	}
	var entries []RemoteEntry
	var linked []RemoteRow
	for _, rs := range remotes {
		entries = append(entries, rs.Entries...)
		linked = append(linked, rs.LinkedRows...)
	}
	ctx, g, err := h.gate(ctx, entries, linked)
	if err != nil {
		writeInternalError(w, r, "remote_gate_failed", "Failed to check access to the remote rows", err)
		return
	}
	out := remoteList{Data: make([]remoteSummary, 0, len(remotes))}
	for _, rs := range remotes {
		s := summaryOf(rs)
		for _, row := range rs.LinkedRows {
			if g.rowVisible(row) {
				s.Linked++
			}
		}
		for _, e := range rs.Entries {
			if g.entryVisible(ctx, e) {
				s.Conflicts.Total++
				s.Conflicts.ByReason[e.Reason]++
			}
		}
		out.Data = append(out.Data, s)
	}
	writeV1JSON(w, http.StatusOK, out)
}

// summaryOf is rs's settings and run health, with zero counts.
func summaryOf(rs RemoteStatus) remoteSummary {
	r := rs.Remote
	return remoteSummary{
		Name: r.Name, Settings: r.Settings, CredentialKind: r.CredentialKind,
		MaxClockSkewSeconds:        int64(r.MaxClockSkew / time.Second),
		SupersededRetentionSeconds: int64(r.SupersededRetention / time.Second),
		Types:                      orEmpty(r.Types),
		Run:                        r.Run,
		Conflicts:                  remoteCounts{ByReason: map[string]int{}},
	}
}

// remoteEntryWire is one entry on GET /_remotes/{name}/conflicts.
type remoteEntryWire struct {
	Kind               string                `json:"kind,omitempty"`
	Local              string                `json:"local"`
	Type               string                `json:"type"`
	Address            string                `json:"address,omitempty"`
	Field              string                `json:"field,omitempty"`
	RemoteField        string                `json:"remote_field,omitempty"`
	Fields             []string              `json:"fields,omitempty"`
	Reason             string                `json:"reason"`
	Cause              string                `json:"cause,omitempty"`
	Status             int                   `json:"status,omitempty"`
	Rule               string                `json:"rule"`
	Fallback           string                `json:"fallback,omitempty"`
	Since              time.Time             `json:"since"`
	Seen               time.Time             `json:"seen"`
	RemoteValue        json.RawMessage       `json:"remote_value,omitempty"`
	RemoteValueOmitted bool                  `json:"remote_value_omitted,omitempty"`
	Superseded         *remoteSupersededWire `json:"superseded,omitempty"`
	Actions            map[string]bool       `json:"_actions"`
}

// remoteSupersededWire is [remotestatus.Superseded] on the wire.
// LostValueKept says whether the list kept the overwritten value: under
// superseded_retention 0 it keeps none, and a value too large to keep is
// marked omitted instead.
type remoteSupersededWire struct {
	remotestatus.Superseded
	LostValueKept bool `json:"lost_value_kept"`
}

// remoteConflictPage answers GET /_remotes/{name}/conflicts. Total and
// Counts count the entries the caller may see.
type remoteConflictPage struct {
	Data    []remoteEntryWire `json:"data"`
	Total   int               `json:"total"`
	Page    int               `json:"page"`
	PerPage int               `json:"per_page"`
	Counts  map[string]int    `json:"counts"`
}

// conflicts serves one remote's conflict list. It gates the whole list
// first and pages the visible entries, so the total and a page's length
// say nothing about hidden ones.
func (h *remotesHandler) conflicts(w http.ResponseWriter, r *http.Request, name string) {
	rs, ok := h.remote(w, r, name)
	if !ok {
		return
	}
	ctx, g, err := h.gate(r.Context(), rs.Entries, nil)
	if err != nil {
		writeInternalError(w, r, "remote_gate_failed", "Failed to check access to the remote rows", err)
		return
	}
	var visible []RemoteEntry
	counts := map[string]int{}
	for _, e := range rs.Entries {
		if g.entryVisible(ctx, e) {
			visible = append(visible, e)
			counts[e.Reason]++
		}
	}
	page, perPage := parseV1Pagination(r.URL.Query())
	out := remoteConflictPage{Data: []remoteEntryWire{}, Total: len(visible), Page: page, PerPage: perPage,
		Counts: counts}
	// A page past the end is empty. The comparison is made in pages, so a
	// huge page number cannot overflow the offset.
	if page-1 < (len(visible)+perPage-1)/perPage {
		start := (page - 1) * perPage
		for _, e := range visible[start:min(start+perPage, len(visible))] {
			out.Data = append(out.Data, h.entryWire(ctx, g, e))
		}
	}
	writeV1JSON(w, http.StatusOK, out)
}

// remote returns the status of remote name, answering the error itself
// when there is none.
func (h *remotesHandler) remote(w http.ResponseWriter, r *http.Request, name string) (RemoteStatus, bool) {
	remotes, err := h.svc.Remotes(r.Context())
	if err != nil {
		writeRemoteError(w, r, err)
		return RemoteStatus{}, false
	}
	for _, rs := range remotes {
		if rs.Remote.Name == name {
			return rs, true
		}
	}
	writeV1Error(w, r, http.StatusNotFound, "not_found", "Not found", "")
	return RemoteStatus{}, false
}

// entryWire is e as the caller sees it, with its `_actions`.
func (h *remotesHandler) entryWire(ctx context.Context, g remoteGate, e RemoteEntry) remoteEntryWire {
	out := remoteEntryWire{
		Kind: e.Kind, Local: e.Local, Type: e.Type, Address: e.Address, Field: e.Field,
		RemoteField: e.RemoteField, Fields: e.Fields, Reason: e.Reason, Cause: e.Cause, Status: e.Status,
		Rule: e.Rule, Fallback: e.Fallback, Since: e.Since, Seen: e.Seen, RemoteValue: e.RemoteValue,
		RemoteValueOmitted: e.RemoteValueOmitted, Actions: make(map[string]bool, len(remoteActions)),
	}
	if s := e.Superseded; s != nil {
		lost, omitted := s.LocalValue, s.LocalValueOmitted
		if s.Winner == remoteTakeLocal {
			lost, omitted = e.RemoteValue, e.RemoteValueOmitted
		}
		out.Superseded = &remoteSupersededWire{Superseded: *s, LostValueKept: len(lost) > 0 || omitted}
	}
	for _, a := range remoteActions {
		out.Actions[a] = h.verdict(ctx, g, e, a) == remoteAllowed
	}
	return out
}

// remoteActRequest is the body of the POST routes. Field is empty for the
// row-level entry; Take is for _resolve, Address for _link.
type remoteActRequest struct {
	Local   string `json:"local"`
	Field   string `json:"field"`
	Take    string `json:"take,omitempty"`
	Address string `json:"address,omitempty"`
}

// act serves the POSTs on one entry. It finds the entry the body names,
// gates it as the list does, and asks [remotesHandler.verdict] the same
// question `_actions` answered, so the button and the write agree.
func (h *remotesHandler) act(w http.ResponseWriter, r *http.Request, name, op string) {
	var action string
	switch op {
	case "_resolve":
	case "_dismiss":
		action = remoteActionDismiss
	case "_link":
		action = remoteActionLink
	default:
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Not found", "")
		return
	}
	var req remoteActRequest
	if !decodeStrictBody(w, r, remoteBodyLimit, "16 KiB", &req) {
		return
	}
	if msg := checkRemoteAct(op, req); msg != "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", msg, "")
		return
	}
	if op == "_resolve" {
		action = remoteActionResolveLocal
		if req.Take == remoteTakeRemote {
			action = remoteActionResolveRemote
		}
	}
	rs, ok := h.remote(w, r, name)
	if !ok {
		return
	}
	key := RemoteEntryKey{Local: req.Local, Field: req.Field}
	ctx := r.Context()
	e, found := findRemoteEntry(rs.Entries, key)
	var g remoteGate
	if found {
		var err error
		if ctx, g, err = h.gate(ctx, []RemoteEntry{e}, nil); err != nil {
			writeInternalError(w, r, "remote_gate_failed", "Failed to check access to the remote rows", err)
			return
		}
	}
	if !found || !g.entryVisible(ctx, e) {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}
	switch h.verdict(ctx, g, e, action) {
	case remoteWrongState:
		writeV1Error(w, r, http.StatusConflict, "invalid_state", "The entry cannot take that action",
			"the entry's reason is "+e.Reason)
		return
	case remoteForbidden:
		writeV1Error(w, r, http.StatusForbidden, "forbidden", "Permission denied",
			"taking the remote value or linking needs edit rights on the local fields")
		return
	case remoteAllowed:
	}
	if e.Field == "" {
		// The checks above covered these fields; the write covers no other.
		key.Fields = append([]string{}, e.Fields...)
	}
	h.apply(w, r, name, op, key, req)
}

// apply runs the action on the service. The service's ctx is the request's
// own, not the primed one: the prime is a read snapshot.
func (h *remotesHandler) apply(
	w http.ResponseWriter, r *http.Request, name, op string, key RemoteEntryKey, req remoteActRequest,
) {
	ctx := r.Context()
	var n int
	var err error
	switch op {
	case "_resolve":
		n, err = h.svc.Resolve(ctx, name, key, req.Take)
	case "_dismiss":
		n, err = h.svc.Dismiss(ctx, name, key)
	default:
		err = h.svc.Link(ctx, name, key, req.Address)
		n = 1
	}
	if err != nil {
		writeRemoteError(w, r, err)
		return
	}
	writeV1JSON(w, http.StatusOK, struct {
		Changed int `json:"changed"`
	}{n})
}

// checkRemoteAct checks the body of a POST op, returning the problem title
// or "".
func checkRemoteAct(op string, req remoteActRequest) string {
	switch {
	case req.Local == "":
		return "local is required"
	case op == "_resolve" && req.Take != remoteTakeLocal && req.Take != remoteTakeRemote:
		return "take must be local or remote"
	case op != "_resolve" && req.Take != "":
		return "take is only for _resolve"
	case op == "_link" && (req.Address == "" || req.Field != ""):
		return "a link names a local row and an address, and no field"
	case op != "_link" && req.Address != "":
		return "address is only for _link"
	}
	return ""
}

// findRemoteEntry returns the entry key names exactly.
func findRemoteEntry(list []RemoteEntry, key RemoteEntryKey) (RemoteEntry, bool) {
	for _, e := range list {
		if e.Local == key.Local && e.Field == key.Field {
			return e, true
		}
	}
	return RemoteEntry{}, false
}

// writeRemoteError answers an error from the [RemoteService]. A store
// fault is a 500 with no detail; the error text stays in the log.
func writeRemoteError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case requestCanceled(r, err):
	case writeNoIdentity(w, r, err):
	case errors.Is(err, ErrRemoteUnknown):
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Not found", "")
	case errors.Is(err, ErrRemoteNoEntry):
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
	case errors.Is(err, ErrRemoteBusy):
		writeV1Error(w, r, http.StatusConflict, "remote_busy", "The remote is busy",
			"a sync or another change holds the remote; try again shortly")
	case errors.Is(err, ErrRemoteRefused):
		writeV1Error(w, r, http.StatusConflict, "invalid_state", "The entry cannot take that action",
			"check the address and the entry")
	default:
		writeInternalError(w, r, "remote_failed", "Remote request failed", err)
	}
}
