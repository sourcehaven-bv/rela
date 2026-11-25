package dataentry

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/access"
	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/cmdexec"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// Protocol prefix for structured command output messages.
const commandOutputPrefix = "::rela::"

// cancelGrace is the time a SIGINTed command gets to clean up before SIGKILL.
// Shared between explicit /api/command-cancel and context-cancel (client disconnect).
const cancelGrace = 3 * time.Second

// ResolvedCommand is a command that has been matched to a specific page context.
//
// Field-for-field convertible to [v1.Command] (handleV1Commands does exactly
// that conversion), so the two must be changed together.
type ResolvedCommand struct {
	ID      string
	Label   string
	Confirm string
	Context string
}

// commandDenyReason explains why a command may not run. It is deliberately
// coarse: the wire 403 must not echo the required permission name or any
// policy data, mirroring [acl.Decision.Reason] ("never contains raw policy
// data so 403 bodies don't leak the full effective-role set").
const commandDenyReason = "not permitted to run this command"

// commandAuthorizer reports whether the principal on ctx may execute cmd.
//
// It is the SINGLE decision point (one instance held by commandHandler), called
// by both the exec handler and resolveCommands so the rendered button set and
// the enforced boundary cannot drift (DEC-EIHQSU). The 403 is the boundary; the
// resolve filter is a UX affordance.
//
// Which implementation a handler holds is decided ONCE at the wiring site from
// (ACL, bind, override) — see SelectCommandAuthorizer. This is the seam that
// distinguishes NopACL from ReadOnlyACL and loopback from network: the ctx read
// gate cannot, because readGateFromContext hands back the permissive nopReadGate
// under BOTH NopACL and ReadOnlyACL (RR-QWVG8Y / RR-CWWJGW). A guard written
// against the read gate alone therefore fails OPEN — so gatedAuthorizer is only
// ever constructed when a real *acl.Declarative is present.
type commandAuthorizer interface {
	Authorize(ctx context.Context, cmd CommandConfig) bool
}

// ungatedAuthorizer grants every command. It is the pre-ACL / NopACL behavior,
// selected only for a loopback bind (or the desktop/in-process server, which has
// no network listener) or when an operator explicitly opts in on a network bind
// via --allow-unauthenticated-commands. Named, not an anonymous `return true`,
// so every ungated command path is greppable.
type ungatedAuthorizer struct{}

func (ungatedAuthorizer) Authorize(context.Context, CommandConfig) bool { return true }

// denyAuthorizer refuses every command. It is the fail-closed choice:
// ReadOnlyACL, a network bind with no policy and no override, a gate that was
// required but could not be built, and the nil-authorizer wiring-bug fallback.
// The DenyReader analog for command execution.
type denyAuthorizer struct{}

func (denyAuthorizer) Authorize(context.Context, CommandConfig) bool { return false }

// commandsOffReason is the problem type the exec route answers when the
// server turned commands off as a whole, rather than for this caller.
const commandsOffReason = "commands_disabled_in_tenant_mode"

// tenantModeAuthorizer refuses every command because the server runs in
// tenant mode, where a shell command would run in the one project root that
// every tenant shares. It refuses like [denyAuthorizer], so the buttons are
// not offered, and the exec route names the reason (see [commandRefusal]).
type tenantModeAuthorizer struct{ denyAuthorizer }

// refusal names why every command is refused.
func (tenantModeAuthorizer) refusal() (errType, detail string) {
	return commandsOffReason, "Configured commands are turned off in tenant mode: " +
		"they would run in the project directory every tenant shares."
}

// commandRefusal is implemented by an authorizer that refuses every
// command for one server-wide reason. The exec route then answers with
// that reason instead of the per-caller [commandDenyReason]. Naming it is
// safe: it is a fact about the server's configuration, not about the
// caller's permissions.
type commandRefusal interface {
	refusal() (errType, detail string)
}

// TenantModeCommandAuthorizer returns the authorizer of a server in tenant
// mode: every command is refused, and the exec route answers 403 with the
// problem type commands_disabled_in_tenant_mode.
func TenantModeCommandAuthorizer() commandAuthorizer { return tenantModeAuthorizer{} }

// gatedAuthorizer enforces a configured Declarative policy: a command needs its
// `permission:` set AND held by the principal (resolved per-request from the ctx
// read gate). `context: view` commands are denied outright — they have no
// fine-grained control yet (their payload is the whole traversal closure, not
// one entity; see TKT-MJ02AO), so a granted permission must not open the gate.
//
// LOAD-BEARING (RR-QWVG8Y): a gatedAuthorizer is only valid over a real
// Declarative policy. newGatedAuthorizer rejects a nil *acl.Declarative, and the
// wiring site never selects this impl for NopACL/ReadOnly/unknown — because on
// those paths readGateFromContext returns the permissive nopReadGate and the
// per-permission check would fail OPEN.
//
// The d field is not read by Authorize (the verdict comes from the per-request
// ctx read gate, which shares the same policy) — it exists so the type cannot be
// constructed without a non-nil policy. Do not drop it: an empty gatedAuthorizer{}
// selected on a NopACL path is exactly the fail-open this guard prevents.
type gatedAuthorizer struct {
	d *acl.Declarative
}

// newGatedAuthorizer builds a gatedAuthorizer, rejecting a nil policy so a
// wiring mistake can never produce a gate that silently grants via nopReadGate.
func newGatedAuthorizer(d *acl.Declarative) (gatedAuthorizer, error) {
	if d == nil {
		return gatedAuthorizer{}, errors.New("dataentry: newGatedAuthorizer: acl.Declarative is nil")
	}
	return gatedAuthorizer{d: d}, nil
}

// Authorize checks cmd's permission. View commands do not honor
// `permission:` yet, so a grant never opens them.
//
//archplan:guard=authz reason="checks the command's config-declared permission; fails closed"
func (gatedAuthorizer) Authorize(ctx context.Context, cmd CommandConfig) bool {
	if cmd.Context == "view" {
		return false
	}
	if cmd.Permission == "" {
		return false
	}
	return readGateFromContext(ctx).HoldsPermission(ctx, cmd.Permission)
}

// UngatedCommandAuthorizer returns the pass-through authorizer for an
// in-process or strictly-loopback server (rela-desktop's desktop-shell asset server,
// docscapture's local render harness) — hosts with no network listener where
// the server host IS the user's machine. Exported so those cmd entry points can
// wire it without the (ACL, bind) selection that only rela-server needs.
func UngatedCommandAuthorizer() commandAuthorizer { return ungatedAuthorizer{} }

// CommandAuthNotifier carries the startup-log hooks for the two command-auth
// outcomes an operator must not discover at runtime. Either field may be nil.
//
// Both are needed. Instrumenting only OnOverride (the path that grants) leaves
// the path that DENIES silent — and that is the one that changes behavior for
// an existing non-loopback deployment on upgrade.
type CommandAuthNotifier struct {
	// OnOverride fires when --allow-unauthenticated-commands actually takes
	// effect on a network bind, i.e. commands became ungated by operator choice.
	OnOverride func()
	// OnRefuse fires when a network bind with no policy refuses command
	// execution, i.e. configured commands stopped working and the operator
	// needs to know the remedy.
	OnRefuse func()
}

// SelectCommandAuthorizer chooses the command authorizer for rela-server. It is
// fail-closed; the only ungated paths are loopback or an explicit override:
// ReadOnlyACL denies exec on every bind (value or pointer form); a Declarative
// policy is gated and never falls back to the ctx read gate; NopACL is ungated
// on loopback or with override, and denied otherwise. loopback reports whether
// the bind host is loopback; override is --allow-unauthenticated-commands.
// notify logs both the override and the refusal at startup.
func SelectCommandAuthorizer(
	active acl.ACL, declarative *acl.Declarative, loopback, override bool, notify CommandAuthNotifier,
) (commandAuthorizer, error) {
	if access.ModeOf(active).RefusesWrites() {
		return denyAuthorizer{}, nil
	}

	if declarative != nil {
		return newGatedAuthorizer(declarative)
	}

	if loopback {
		return ungatedAuthorizer{}, nil
	}
	if override {
		if notify.OnOverride != nil {
			notify.OnOverride()
		}
		return ungatedAuthorizer{}, nil
	}
	if notify.OnRefuse != nil {
		notify.OnRefuse()
	}
	return denyAuthorizer{}, nil
}

// resolveCommands returns commands available for a given page context.
// pageType is "entity", "list", "view", or "dashboard".
// qualifier is the specific list ID or view ID, or for a dashboard its name
// ("" for the default dashboard).
// entityType is the entity type shown on the page (empty for dashboard).
//
// Commands the principal may not execute are omitted, so the SPA never renders
// a button that would 403 on click. This is presentation only — the same
// authorizer is re-consulted at exec time, which is the actual boundary.
func (h *commandHandler) resolveCommands(
	ctx context.Context, pageType, qualifier, entityType string,
) []ResolvedCommand {
	s := h.schema()
	if len(s.Cfg.Commands) == 0 {
		return nil
	}

	ids := make([]string, 0, len(s.Cfg.Commands))
	for id := range s.Cfg.Commands {
		ids = append(ids, id)
	}
	natsort.Strings(ids)

	var result []ResolvedCommand
	for _, id := range ids {
		cmd := s.Cfg.Commands[id]
		if !matchesPage(cmd, pageType, qualifier, entityType) {
			continue
		}
		if h.authorizer().Authorize(ctx, cmd) {
			result = append(result, ResolvedCommand{
				ID:      id,
				Label:   cmd.Label,
				Confirm: cmd.Confirm,
				Context: cmd.Context,
			})
		}
	}
	return result
}

// matchesPage checks if a command should appear on the given page.
func matchesPage(cmd CommandConfig, pageType, qualifier, entityType string) bool {
	scope := cmd.AvailableOn

	if scope == nil {
		return contextMatchesPage(cmd.Context, pageType)
	}

	switch pageType {
	case "view":
		if slices.Contains(scope.Views, qualifier) {
			return true
		}
		if slices.Contains(scope.EntityTypes, entityType) {
			return true
		}
	case "entity":
		if slices.Contains(scope.EntityTypes, entityType) {
			return true
		}
	case "list":
		if slices.Contains(scope.Lists, qualifier) {
			return true
		}
	case "dashboard":
		if qualifier == dataentryconfig.DefaultDashboardName {
			qualifier = ""
		}
		if scope.Dashboard.IsSet() && scope.Dashboard.Name() == qualifier {
			return true
		}
	}
	return false
}

// contextMatchesPage returns true when a command's context type is compatible
// with the page type. Entity and view commands both appear on entity/view pages.
func contextMatchesPage(cmdContext, pageType string) bool {
	switch cmdContext {
	case "entity":
		return pageType == "entity" || pageType == "view"
	case "view":
		return pageType == "view"
	case "list":
		return pageType == "list"
	case "global":
		return pageType == "dashboard"
	}
	return false
}

// --- Stdin JSON builders ---

// commandInput is the JSON structure passed to a command script on stdin.
type commandInput struct {
	Context     string                      `json:"context"`
	Entity      *entity.Entity              `json:"entity,omitempty"`
	Entities    []*entity.Entity            `json:"entities,omitempty"`
	Collections map[string][]*entity.Entity `json:"collections,omitempty"`
	Relations   []*entity.Relation          `json:"relations,omitempty"`
	ListID      string                      `json:"list_id,omitempty"`
	ViewID      string                      `json:"view_id,omitempty"`
	Project     commandProjectInfo          `json:"project"`
}

type commandProjectInfo struct {
	Root      string `json:"root"`
	Metamodel string `json:"metamodel"`
}

// redactEntity applies field-level `visible:` redaction to an entity bound for
// a command's stdin.
//
// An unwired redactor FAILS CLOSED: the payload leaves the process, so a
// missing collaborator must withhold values rather than ship them raw. Both
// construction sites supply one (app.go and the test helper), so reaching the
// nil arm is a wiring bug, reported rather than silently tolerated.
//
// Nil: e nil returns nil.
func (h *commandHandler) redactEntity(ctx context.Context, e *entity.Entity) *entity.Entity {
	if e == nil {
		return nil
	}
	if h.redactor == nil {
		slog.Error("dataentry: command handler has no field redactor; "+
			"withholding every property from the command payload",
			"type", e.Type, "id", e.ID)
		out := *e
		out.Properties = nil
		out.Content = ""
		return &out
	}
	return visibility.Redact(ctx, h.redactor, e)
}

// buildEntityInput assembles the stdin JSON for an entity-context command.
// A failed relation read or gate is returned: the command must not run on a
// payload that claims e has no edges when the read never finished.
func (h *commandHandler) buildEntityInput(
	ctx context.Context, meta *metamodel.Metamodel, e *entity.Entity,
) (*commandInput, error) {
	rels, err := h.relationsForEntity(ctx, meta, e)
	if err != nil {
		return nil, err
	}
	return &commandInput{
		Context:   "entity",
		Entity:    e,
		Relations: rels,
		Project:   h.projectInfo(),
	}, nil
}

// relationsForEntity returns the relations incident to e that may travel with
// it to a command's stdin. An outgoing content-scoped edge travels only with the
// face that owns it ([ownedByFace]). Both endpoints must be readable, as
// [visibleReader.readableRelations] decides, so a hidden peer's id never reaches
// the script. It costs one relation query and one header batch.
func (h *commandHandler) relationsForEntity(
	ctx context.Context, meta *metamodel.Metamodel, e *entity.Entity,
) ([]*entity.Relation, error) {
	var candidates []*entity.Relation
	q := store.RelationQuery{EntityID: e.ID, Direction: store.DirectionBoth}
	for r, err := range h.services().Store.ListRelations(ctx, q) {
		if err != nil {
			return nil, fmt.Errorf("list relations of %s: %w", e.ID, err)
		}
		if r.From == e.ID && !ownedByFace(meta, r, e.Face) {
			continue
		}
		candidates = append(candidates, r)
	}
	return h.visible.readableRelations(ctx, candidates)
}

func (h *commandHandler) buildListInput(listID string, entities []*entity.Entity) *commandInput {
	return &commandInput{
		Context:  "list",
		ListID:   listID,
		Entities: entities,
		Project:  h.projectInfo(),
	}
}

// visibleListRows reads the rows of a list-context command: every row of
// typeName the principal may read, with its body, redacted. It runs the gated
// header scan over the whole type, reads bodies of admitted rows only, then
// primes field redaction for the set. The list's `filters:` run afterwards on
// redacted rows, so a filter on a hidden property cannot keep a row.
func (h *commandHandler) visibleListRows(ctx context.Context, typeName string) ([]*entity.Entity, error) {
	rqr := readGateFromContext(ctx).ReadQuery(ctx, typeName)
	svc := h.services()
	req := scopeRequest{Type: typeName, Faces: rqr.Faces}
	q, errClass, withheld, err := scopedQuery(ctx, rqr, req)
	if withheld || err != nil {
		return nil, err
	}
	headers, err := scanHeaders(ctx, svc, q, errClass, req)
	if err != nil {
		return nil, err
	}
	rows := headerEntities(headers)
	if err := loadRowContent(ctx, svc.Store, rows); err != nil {
		return nil, fmt.Errorf("%w: %w", errListLoad, err)
	}
	if h.redactor != nil {
		ctx = visibility.PrimeTraversals(ctx, h.redactor, rows)
	}
	for i, e := range rows {
		rows[i] = h.redactEntity(ctx, e)
	}
	return rows, nil
}

// buildViewInput assembles the stdin JSON for a view-context command. The
// viewResult it receives is already row-gated + field-redacted (executeView,
// DEC-ZBI39P): a command script sees the same visibility the HTTP view does, so
// a property hidden from the invoking principal is absent from the entity JSON
// rather than raw. (Behavior change since BUG-9QL9XV: previously raw.) A failed
// relation read is returned, as in [commandHandler.buildEntityInput].
func (h *commandHandler) buildViewInput(
	ctx context.Context, meta *metamodel.Metamodel, viewID string, vr *viewResult,
) (*commandInput, error) {
	rels, err := h.viewRelations(ctx, meta, vr)
	if err != nil {
		return nil, err
	}

	collections := make(map[string][]*entity.Entity, len(vr.Collections))
	maps.Copy(collections, vr.Collections)

	return &commandInput{
		Context:     "view",
		ViewID:      viewID,
		Entity:      vr.Entry,
		Collections: collections,
		Relations:   rels,
		Project:     h.projectInfo(),
	}, nil
}

// viewRelations returns the edges between entities in the view result, in
// ONE relation query for the whole set (TKT-1U8XYN), not one per entity.
//
// Every entity in vr is already row-gated and face-gated by executeView, and
// an edge is kept only when both ends are in vr, so the peers need no second
// gate. What executeView does not decide is which face owns an edge: a
// content-scoped edge is kept only when its tail face is a face vr serves its
// source at ([ownedByFace], BUG-BZQQDP). One id may be served at several
// faces, so each id keeps the set of faces it is served at.
func (h *commandHandler) viewRelations(
	ctx context.Context, meta *metamodel.Metamodel, vr *viewResult,
) ([]*entity.Relation, error) {
	served := make(map[string][]entity.Face)
	add := func(e *entity.Entity) {
		if e != nil && !slices.Contains(served[e.ID], e.Face) {
			served[e.ID] = append(served[e.ID], e.Face)
		}
	}
	add(vr.Entry)
	for _, entities := range vr.Collections {
		for _, e := range entities {
			add(e)
		}
	}
	if len(served) == 0 {
		return nil, nil
	}

	var rels []*entity.Relation
	q := store.RelationQuery{EntityIDs: slices.Sorted(maps.Keys(served)), Direction: store.DirectionOutgoing}
	for r, err := range h.services().Store.ListRelations(ctx, q) {
		if err != nil {
			return nil, fmt.Errorf("list view relations: %w", err)
		}
		if _, ok := served[r.To]; !ok {
			continue
		}
		if slices.ContainsFunc(served[r.From], func(f entity.Face) bool { return ownedByFace(meta, r, f) }) {
			rels = append(rels, r)
		}
	}
	return rels, nil
}

func (h *commandHandler) buildGlobalInput() *commandInput {
	return &commandInput{
		Context: "global",
		Project: h.projectInfo(),
	}
}

func (h *commandHandler) projectInfo() commandProjectInfo {
	schema := project.SchemaFile
	if h.schemaFile != nil {
		schema = h.schemaFile()
	}
	return commandProjectInfo{
		Root:      h.projectRoot(),
		Metamodel: schema,
	}
}

// --- Protocol parser ---

// CommandMessage is a structured message parsed from a command's stdout, and
// also the shape written back out as an SSE event.
//
// The two directions are not identical for `file` messages: a script supplies
// Path, the server replaces it with Token, and the outbound payload carries no
// Path at all (see [commandHandler.mintFileToken]). Path keeps `omitempty` so
// an emitted message simply omits it — the raw server path must not reach the
// browser, which is what the token exists to prevent.
type CommandMessage struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	Level string `json:"level,omitempty"`
	// Path is INBOUND ONLY: the location a script reports for a `file`
	// message. Never serialized back to the client.
	Path string `json:"path,omitempty"`
	// Token is OUTBOUND ONLY: the opaque handle the browser presents to
	// /api/command-file/ to download the file Path named.
	Token      string `json:"token,omitempty"`
	Label      string `json:"label,omitempty"`
	Action     string `json:"action,omitempty"`
	ID         string `json:"id,omitempty"`
	EntityType string `json:"entity_type,omitempty"`
	URL        string `json:"url,omitempty"`
}

// parseCommandOutput parses a single line of command stdout.
// If the line has the ::rela:: prefix, it returns the parsed message.
// Otherwise it returns a log-type message with the raw text.
func parseCommandOutput(line string) CommandMessage {
	if after, ok := strings.CutPrefix(line, commandOutputPrefix); ok {
		payload := after
		var msg CommandMessage
		if err := json.Unmarshal([]byte(payload), &msg); err == nil {
			return msg
		}
	}
	return CommandMessage{Type: "log", Text: line}
}

// --- Process management ---

type runningCommand struct {
	cmd *exec.Cmd

	// owner is the principal that started this command. runningCommands is a
	// package-level map keyed only by execID, and execID is client-supplied
	// (see handleCommandExec), so without an owner recorded here any caller
	// who guesses or reuses an id could cancel someone else's run — a
	// cross-principal kill that the exec-side permission check does not cover
	// (RR-YZV7SY). handleCommandCancel compares against this.
	owner principal.Principal
}

var (
	runningCommands sync.Map
)

// --- HTTP Handlers ---

// buildCommandInput builds the stdin JSON for cmd from the request. When it
// cannot, it has already written the error response and returns false; the
// command must not run.
//
// The entity id is an address resolved through the row gate, the face gate and
// redaction, on its stored type.
func (h *commandHandler) buildCommandInput(
	w http.ResponseWriter, r *http.Request, s *Schema, cmd CommandConfig, commandID string,
) (*commandInput, bool) {
	var input *commandInput
	switch cmd.Context {
	case "entity":
		entityID := r.URL.Query().Get("entity_id")
		entityDomain, found, gerr := h.visible.untypedAddress(r.Context(), entityID)
		if gerr != nil || !found {
			if gerr != nil {
				slog.Warn("dataentry: command entity read gate failed; answering not-found",
					"command", commandID, "entity_id", entityID, "err", gerr)
			}
			http.Error(w, "Entity not found: "+entityID, http.StatusNotFound)
			return nil, false
		}
		var berr error
		input, berr = h.buildEntityInput(r.Context(), s.Meta, h.redactEntity(r.Context(), entityDomain))
		if berr != nil {
			slog.Error("dataentry: command payload read failed; not running the command",
				"command", commandID, "err", berr)
			http.Error(w, "Failed to build input", http.StatusInternalServerError)
			return nil, false
		}
	case "list":
		listID := r.URL.Query().Get("list_id")
		listCfg, found := s.Cfg.Lists[listID]
		if !found {
			http.Error(w, "List not found: "+listID, http.StatusNotFound)
			return nil, false
		}
		entities, lerr := h.visibleListRows(r.Context(), listCfg.EntityType)
		if lerr != nil {
			slog.Error("dataentry: command payload read failed; not running the command",
				"command", commandID, "err", lerr)
			http.Error(w, "Failed to build input", http.StatusInternalServerError)
			return nil, false
		}
		entities = applyFilters(entities, listCfg.Filters, filterUser(r.Context()))
		input = h.buildListInput(listID, entities)
	case "view":
		viewID := r.URL.Query().Get("view_id")
		entityID := r.URL.Query().Get("entity_id")
		viewCfg, found := s.Cfg.Views[viewID]
		if !found {
			http.Error(w, "View not found: "+viewID, http.StatusNotFound)
			return nil, false
		}
		vr, err := h.executeView(r.Context(), viewCfg, entityID, defaultViewWorld())
		if err != nil {
			http.Error(w, "View error: "+err.Error(), http.StatusBadRequest)
			return nil, false
		}
		var berr error
		input, berr = h.buildViewInput(r.Context(), s.Meta, viewID, vr)
		if berr != nil {
			slog.Error("dataentry: command payload read failed; not running the command",
				"command", commandID, "err", berr)
			http.Error(w, "Failed to build input", http.StatusInternalServerError)
			return nil, false
		}
	case "global":
		input = h.buildGlobalInput()
	default:
		http.Error(w, "Invalid command context: "+cmd.Context, http.StatusBadRequest)
		return nil, false
	}
	return input, true
}

// handleCommandExec handles POST /api/command/{commandID} and streams results as SSE.
//
// Restricted to POST: this endpoint runs configured shell commands and a GET
// would let `<img src=/api/command/X>` invoke them cross-origin from any
// browser tab, bypassing same-origin policy entirely.
//
// It is the authorization boundary: an unauthorized command gets 403, since
// command names are public config.
//
//nolint:funlen // dispatches over every command context and argument shape inline; each branch is one command variant, and extracting them would fragment the request lifecycle.
//archplan:auth=authz
func (h *commandHandler) handleCommandExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	commandID := strings.TrimPrefix(r.URL.Path, "/api/command/")
	s := h.schema()
	cmd, ok := s.Cfg.Commands[commandID]
	if !ok {
		http.Error(w, "Unknown command: "+commandID, http.StatusNotFound)
		return
	}

	if authz := h.authorizer(); !authz.Authorize(r.Context(), cmd) {
		if off, isOff := authz.(commandRefusal); isOff {
			errType, detail := off.refusal()
			writeV1Error(w, r, http.StatusForbidden, errType, "Commands are turned off", detail)
			return
		}
		http.Error(w, commandDenyReason, http.StatusForbidden)
		return
	}

	execID := r.URL.Query().Get("exec_id")
	if execID == "" {
		execID = fmt.Sprintf("cmd-%d", time.Now().UnixNano())
	}

	runKey := newRunKey()

	input, ok := h.buildCommandInput(w, r, s, cmd, commandID)
	if !ok {
		return
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		slog.ErrorContext(r.Context(), "dataentry: command input failed", "err", err, "path", r.URL.Path)
		http.Error(w, "Failed to build input", http.StatusInternalServerError)
		return
	}

	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// #nosec G702 -- cmd.Script is operator-authored: it comes from the
	// `commands:` map in the project's data-entry.yaml (dataentryconfig),
	// selected by name via commandID. The request may only choose a
	// registered command key; it never supplies a command, flag, or path.
	// Request-derived values reach the script only as stdin JSON and
	// RELA_* env vars, never as shell text. A shell is intentional here —
	// `script:` is documented as a shell snippet, and executing it is the
	// feature. The trust boundary is the config file (operator/repo write
	// access), enforced upstream by authorizeCommand. The same reason
	// covers the semgrep suppression below.
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	proc := exec.CommandContext(r.Context(), "sh", "-c", cmd.Script)
	proc.Cancel = func() error { return proc.Process.Signal(syscall.SIGINT) }
	proc.WaitDelay = cancelGrace
	proc.Dir = h.projectRoot()
	proc.Env = h.buildCommandEnv(cmd, input)
	proc.Stdin = strings.NewReader(string(inputJSON))

	stdout, err := proc.StdoutPipe()
	if err != nil {
		writeSSEEvent(w, flusher, "error", `{"text":"Failed to create stdout pipe"}`)
		writeSSEDone(w, flusher, false)
		return
	}
	stderr, err := proc.StderrPipe()
	if err != nil {
		writeSSEEvent(w, flusher, "error", `{"text":"Failed to create stderr pipe"}`)
		writeSSEDone(w, flusher, false)
		return
	}

	if startErr := proc.Start(); startErr != nil {
		msg, _ := json.Marshal(map[string]string{"text": "Failed to start: " + startErr.Error()})
		writeSSEEvent(w, flusher, "error", string(msg))
		writeSSEDone(w, flusher, false)
		return
	}

	runningCommands.Store(execID, &runningCommand{
		cmd:   proc,
		owner: principal.From(r.Context()),
	})
	defer runningCommands.Delete(execID)
	defer h.files.release(runKey)

	var stderrBuf strings.Builder
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			stderrBuf.WriteString(scanner.Text())
			stderrBuf.WriteString("\n")
		}
	}()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		msg := parseCommandOutput(line)
		if msg.Type == "file" {
			msg = h.mintFileToken(runKey, commandID, cmd, msg)
		}
		data, _ := json.Marshal(msg)
		writeSSEEvent(w, flusher, msg.Type, string(data))
	}

	<-stderrDone
	waitErr := proc.Wait()

	if waitErr != nil {
		errText := "Command failed"
		if stderrBuf.Len() > 0 {
			errText = strings.TrimSpace(stderrBuf.String())
		}
		msg, _ := json.Marshal(map[string]string{"text": errText})
		writeSSEEvent(w, flusher, "error", string(msg))
		writeSSEDone(w, flusher, false)
		return
	}

	writeSSEDone(w, flusher, true)
}

// handleCommandCancel handles POST /api/command-cancel/{execID}.
//
//archplan:auth=required reason="owner check on the run, not an ACL decision"
func (h *commandHandler) handleCommandCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	execID := strings.TrimPrefix(r.URL.Path, "/api/command-cancel/")
	val, ok := runningCommands.Load(execID)
	if !ok {
		http.Error(w, "No running command: "+execID, http.StatusNotFound)
		return
	}
	rc, castOK := val.(*runningCommand)
	if !castOK {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	if !rc.owner.Equal(principal.From(r.Context())) {
		http.Error(w, "No running command: "+execID, http.StatusNotFound)
		return
	}

	if rc.cmd.Process != nil {
		_ = rc.cmd.Process.Signal(syscall.SIGINT)
	}

	go func() {
		time.Sleep(cancelGrace)
		if rc.cmd.Process != nil {
			_ = rc.cmd.Process.Kill()
		}
	}()

	w.WriteHeader(http.StatusOK)
}

// mintFileToken replaces a script `file` message's server path with an opaque
// download token. The path is containment-checked once, here, and the token is
// bound to cmd so the download re-authorizes as exec did. A path that fails
// containment or does not exist yields a message with no token, not an error.
// The server path is always dropped from the payload.
func (h *commandHandler) mintFileToken(
	runKey, commandID string, cmd CommandConfig, msg CommandMessage,
) CommandMessage {
	label := msg.Label
	if label == "" {
		label = filepath.Base(msg.Path)
	}

	resolved, err := containProjectPath(h.projectRoot(), msg.Path)
	if err != nil {
		slog.Warn("dataentry: command file not downloadable",
			"err", err, "command", commandID, "run", runKey)
		return CommandMessage{Type: msg.Type, Label: label}
	}

	info, err := os.Stat(resolved.abs)
	if err != nil || !info.Mode().IsRegular() {
		slog.Warn("dataentry: command file is not a regular file",
			"err", err, "command", commandID, "run", runKey)
		return CommandMessage{Type: msg.Type, Label: label}
	}

	token, err := h.files.mint(runKey, resolved, label, cmd)
	if err != nil {
		slog.Error("dataentry: minting command file token failed",
			"err", err, "command", commandID, "run", runKey)
		return CommandMessage{Type: msg.Type, Label: label}
	}
	return CommandMessage{Type: msg.Type, Label: label, Token: token}
}

// handleCommandFile handles GET /api/command-file/{token}, streaming a file a
// command run produced. The caller presents a token, never a path. The wired
// authorizer re-runs on every download, so a token stops working once its grant
// goes away. Unknown, expired and unauthorized all return 404, so a probe cannot
// confirm that a token is real.
//
//archplan:auth=authz
func (h *commandHandler) handleCommandFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := strings.TrimPrefix(r.URL.Path, "/api/command-file/")
	entry, ok := h.files.lookup(token)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if !h.authorizer().Authorize(r.Context(), entry.cmd) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	f, err := os.Open(entry.path.abs)
	if err != nil {
		slog.Warn("dataentry: opening command file failed", "err", err)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer f.Close()

	setHardenedDownloadHeaders(w.Header(), contentTypeForFilename(entry.label), entry.label)
	w.Header().Set("Cache-Control", "no-store")

	if _, err := io.Copy(w, f); err != nil {
		slog.Warn("dataentry: streaming command file failed", "err", err)
	}
}

// --- Helpers ---

// errPathOutsideProject is returned by containedProjectPath when the input
// resolves to a location outside the project root.
var errPathOutsideProject = errors.New("path outside project")

// errPathNotFound is returned by containedProjectPath when the path is
// inside the project root structurally but does not exist on disk.
var errPathNotFound = errors.New("path not found")

// containedProjectPath cleans, resolves, and validates that filePath lives
// inside projectRoot. The returned path is absolute and symlink-resolved.
// A TOCTOU window remains between token mint and download. It is accepted: the
// local filesystem is the trust boundary, and whoever can write the project root
// can also edit the `commands:` config.
func containedProjectPath(projectRoot, filePath string) (string, error) {
	if strings.ContainsRune(filePath, 0) {
		return "", errPathOutsideProject
	}

	clean := filepath.Clean(filePath)
	if !filepath.IsAbs(clean) {
		clean = filepath.Join(projectRoot, clean)
	}
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", errPathOutsideProject
	}

	rootAbs, err := filepath.Abs(filepath.Clean(projectRoot))
	if err != nil {
		return "", errPathOutsideProject
	}
	rootResolved, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		rootResolved = rootAbs
	}

	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		insideProject := abs == rootResolved ||
			strings.HasPrefix(abs, rootResolved+string(os.PathSeparator)) ||
			abs == rootAbs ||
			strings.HasPrefix(abs, rootAbs+string(os.PathSeparator))
		if insideProject {
			return "", errPathNotFound
		}
		return "", errPathOutsideProject
	}

	if resolved == rootResolved {
		return resolved, nil
	}
	if strings.HasPrefix(resolved, rootResolved+string(os.PathSeparator)) {
		return resolved, nil
	}
	return "", errPathOutsideProject
}

// It never passes the token key, the database DSN or a remote credential.
func (h *commandHandler) buildCommandEnv(cmd CommandConfig, input *commandInput) []string {
	env := cmdexec.Environ()
	env = append(env,
		"RELA_PROJECT_ROOT="+h.projectRoot(),
		"RELA_CONTEXT="+cmd.Context,
	)
	if input.Entity != nil {
		env = append(env,
			"RELA_ENTITY_ID="+input.Entity.ID,
			"RELA_ENTITY_TYPE="+input.Entity.Type,
			"RELA_ENTITY_FACE="+string(input.Entity.Face),
			"RELA_ENTITY_REF="+entity.FormatStateRef(input.Entity.ID, input.Entity.Face),
		)
	}
	if input.ListID != "" {
		env = append(env, "RELA_LIST_ID="+input.ListID)
	}
	if input.ViewID != "" {
		env = append(env, "RELA_VIEW_ID="+input.ViewID)
	}
	for k, v := range cmd.Env {
		env = append(env, k+"="+v)
	}
	return env
}

func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, event, data string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	if flusher != nil {
		flusher.Flush()
	}
}

func writeSSEDone(w http.ResponseWriter, flusher http.Flusher, success bool) {
	data, _ := json.Marshal(map[string]bool{"success": success})
	writeSSEEvent(w, flusher, "done", string(data))
}
