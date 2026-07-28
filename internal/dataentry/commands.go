package dataentry

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Protocol prefix for structured command output messages.
const commandOutputPrefix = "::rela::"

// cancelGrace is the time a SIGINTed command gets to clean up before SIGKILL.
// Shared between explicit /api/command-cancel and context-cancel (client disconnect).
const cancelGrace = 3 * time.Second

// ResolvedCommand is a command that has been matched to a specific page context.
type ResolvedCommand struct {
	ID       string
	Label    string
	Confirm  string
	Context  string
	AutoOpen *bool
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

func (gatedAuthorizer) Authorize(ctx context.Context, cmd CommandConfig) bool {
	// View commands have no fine-grained control yet: `permission:` is not
	// honored for them, so a granted permission must NOT open the gate.
	// Deferred deliberately, not overlooked.
	if cmd.Context == "view" {
		return false
	}
	if cmd.Permission == "" {
		return false // fail closed: a policy is configured, this command is ungoverned
	}
	return readGateFromContext(ctx).HoldsPermission(ctx, cmd.Permission)
}

// UngatedCommandAuthorizer returns the pass-through authorizer for an
// in-process or strictly-loopback server (rela-desktop's Wails asset server,
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

// SelectCommandAuthorizer chooses the command authorizer for rela-server from
// the active ACL, the concrete Declarative policy (nil when none), the bind
// host, and the operator's --allow-unauthenticated-commands override. This is
// the seam: the bind address lives here in the cmd layer, not in App, so the
// loopback-vs-network decision cannot be made inside dataentry alone.
//
// Decision matrix (fail-closed by construction — the only paths to ungated are
// loopback or an explicit override):
//
//   - ReadOnlyACL                       → deny  (read-only means no exec, ever)
//   - Declarative policy present        → gated (permission must be set AND held)
//   - NopACL + loopback bind            → ungated (pre-ACL local behavior)
//   - NopACL + network bind + override  → ungated (operator opted in; log loudly)
//   - NopACL + network bind, no override→ deny  (THE FIX: was ungated)
//   - anything else / gate unbuildable  → deny
//
// loopback reports whether the bind host is loopback (the caller passes
// isLoopbackHost(bind); an in-process server with no bind passes true).
// override is the --allow-unauthenticated-commands flag.
//
// notify carries the two startup-log hooks. BOTH branches that change what an
// operator gets are instrumented, deliberately symmetric: taking the override
// is loud because it opens a network shell, and REFUSING is loud because it
// silently breaks a previously-working deployment (buttons vanish, exec 403s)
// with nothing on screen to say why.
func SelectCommandAuthorizer(
	active acl.ACL, declarative *acl.Declarative, loopback, override bool, notify CommandAuthNotifier,
) (commandAuthorizer, error) {
	// ReadOnly is checked before anything else: it is a stronger guarantee than
	// "no policy" and must deny exec on every bind. Match both value and pointer
	// forms (AuthorizeWrite has a value receiver, so &ReadOnlyACL{} also
	// satisfies acl.ACL) — the same &-reachable bypass the old switch guarded.
	switch active.(type) {
	case acl.ReadOnlyACL, *acl.ReadOnlyACL:
		return denyAuthorizer{}, nil
	}

	// A configured Declarative policy governs commands on any bind. Build the
	// gated authorizer from the concrete policy — never fall back to the ctx
	// read gate for NopACL/ReadOnly, which answers permissive there (RR-QWVG8Y).
	if declarative != nil {
		return newGatedAuthorizer(declarative)
	}

	// No policy (NopACL / off-metamodel). Loopback stays ungated for local
	// desktop/dev use; a network bind refuses unless the operator opts in.
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
// qualifier is the specific list ID or view ID.
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

	// Sort command IDs for deterministic order.
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
				ID:       id,
				Label:    cmd.Label,
				Confirm:  cmd.Confirm,
				Context:  cmd.Context,
				AutoOpen: cmd.AutoOpen,
			})
		}
	}
	return result
}

// matchesPage checks if a command should appear on the given page.
func matchesPage(cmd CommandConfig, pageType, qualifier, entityType string) bool {
	scope := cmd.AvailableOn

	// No scope restriction: show on any page that matches the command's context.
	if scope == nil {
		return contextMatchesPage(cmd.Context, pageType)
	}

	// Check explicit scope matches.
	switch pageType {
	case "view":
		if contains(scope.Views, qualifier) {
			return true
		}
		if contains(scope.EntityTypes, entityType) {
			return true
		}
	case "entity":
		if contains(scope.EntityTypes, entityType) {
			return true
		}
	case "list":
		if contains(scope.Lists, qualifier) {
			return true
		}
	case "dashboard":
		if scope.Dashboard {
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

// contains checks if a string slice contains a value.
func contains(slice []string, val string) bool {
	return slices.Contains(slice, val)
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

func (h *commandHandler) buildEntityInput(ctx context.Context, e *entity.Entity) *commandInput {
	return &commandInput{
		Context:   "entity",
		Entity:    e,
		Relations: relationsForEntity(ctx, h.services(), e.ID),
		Project:   h.projectInfo(),
	}
}

// relationsForEntity loads every relation where id is either endpoint
// and returns them as []*entity.Relation for the command-input payload.
func relationsForEntity(ctx context.Context, svc Services, id string) []*entity.Relation {
	rels := make([]*entity.Relation, 0)
	q := store.RelationQuery{EntityID: id, Direction: store.DirectionBoth}
	for r, err := range svc.Store.ListRelations(ctx, q) {
		if err != nil {
			return rels
		}
		rels = append(rels, r)
	}
	return rels
}

func (h *commandHandler) buildListInput(listID string, entities []*entity.Entity) *commandInput {
	return &commandInput{
		Context:  "list",
		ListID:   listID,
		Entities: entities,
		Project:  h.projectInfo(),
	}
}

// buildViewInput assembles the stdin JSON for a view-context command. The
// viewResult it receives is already row-gated + field-redacted (executeView,
// DEC-ZBI39P): a command script sees the same visibility the HTTP view does, so
// a property hidden from the invoking principal is absent from the entity JSON
// rather than raw. (Behavior change since BUG-9QL9XV: previously raw.)
func (h *commandHandler) buildViewInput(ctx context.Context, viewID string, vr *viewResult) *commandInput {
	// Collect all entity IDs in the result set.
	idSet := map[string]bool{vr.Entry.ID: true}
	for _, entities := range vr.Collections {
		for _, e := range entities {
			idSet[e.ID] = true
		}
	}

	// Gather relations between entities in the result set.
	svc := h.services()
	var rels []*entity.Relation
	for id := range idSet {
		q := store.RelationQuery{EntityID: id, Direction: store.DirectionOutgoing}
		for r, err := range svc.Store.ListRelations(ctx, q) {
			if err != nil {
				break
			}
			if idSet[r.To] {
				rels = append(rels, r)
			}
		}
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
	}
}

func (h *commandHandler) buildGlobalInput() *commandInput {
	return &commandInput{
		Context: "global",
		Project: h.projectInfo(),
	}
}

func (h *commandHandler) projectInfo() commandProjectInfo {
	// Report the schema file this project actually has — the value is handed
	// to external commands, which would fail opening a name that isn't there.
	// Read from the path resolved at discovery rather than re-statting: that
	// keeps this off the disk on a request path and off the OS filesystem,
	// which an injected (e.g. in-memory) FS would not have been.
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

// CommandMessage is a structured message parsed from a command's stdout.
type CommandMessage struct {
	Type       string `json:"type"`
	Text       string `json:"text,omitempty"`
	Level      string `json:"level,omitempty"`
	Path       string `json:"path,omitempty"`
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

// handleCommandExec handles POST /api/command/{commandID} and streams results as SSE.
//
// Restricted to POST: this endpoint runs configured shell commands and a GET
// would let `<img src=/api/command/X>` invoke them cross-origin from any
// browser tab, bypassing same-origin policy entirely.
//
//nolint:funlen // dispatches over every command context and argument shape inline; each branch is one command variant, and extracting them would fragment the request lifecycle.
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

	// Authorization boundary (TKT-MJ02AO). resolveCommands already hides
	// unauthorized commands from the UI, but that is presentation: this is the
	// check that actually holds. 403 rather than 404 — the command's existence
	// is already public via config, so there is no oracle to protect.
	if !h.authorizer().Authorize(r.Context(), cmd) {
		http.Error(w, commandDenyReason, http.StatusForbidden)
		return
	}

	execID := r.URL.Query().Get("exec_id")
	if execID == "" {
		execID = fmt.Sprintf("cmd-%d", time.Now().UnixNano())
	}

	// Build stdin JSON based on context.
	var input *commandInput
	switch cmd.Context {
	case "entity":
		entityID := r.URL.Query().Get("entity_id")
		svc := h.services()
		// An ADDRESS, as everywhere an id is accepted: `ID` or `ID@face`.
		// Parsed rather than handed to the store whole — on fsstore the raw
		// string happens to hit a faced row's index key, on pgstore it never
		// does, and a command that runs on one backend and 404s on the
		// other is the split this grammar exists to remove.
		id, face, perr := entity.ParseStateRef(entityID)
		if perr != nil {
			http.Error(w, "Entity not found: "+entityID, http.StatusNotFound)
			return
		}
		entityDomain, err := svc.Store.GetEntityState(r.Context(), id, face)
		if err != nil {
			http.Error(w, "Entity not found: "+entityID, http.StatusNotFound)
			return
		}
		input = h.buildEntityInput(r.Context(), entityDomain)
	case "list":
		listID := r.URL.Query().Get("list_id")
		listCfg, found := s.Cfg.Lists[listID]
		if !found {
			http.Error(w, "List not found: "+listID, http.StatusNotFound)
			return
		}
		entities := listFromStoreByTypes(r.Context(), h.services(), []string{listCfg.EntityType})
		entities = applyFilters(entities, listCfg.Filters)
		input = h.buildListInput(listID, entities)
	case "view":
		viewID := r.URL.Query().Get("view_id")
		entityID := r.URL.Query().Get("entity_id")
		viewCfg, found := s.Cfg.Views[viewID]
		if !found {
			http.Error(w, "View not found: "+viewID, http.StatusNotFound)
			return
		}
		// DEFAULT WORLD, named explicitly — and this is the call site the
		// explicit-parameter design exists for. The viewResult below is
		// marshaled to JSON and piped to an operator shell script's stdin, so
		// a world applied here changes what an EXTERNAL PROCESS receives, past
		// any layer that could observe it. Scoping the command surface for
		// worlds is its own ticket, with its own thinking about what a
		// world-bound command even means.
		vr, err := h.executeView(r.Context(), viewCfg, entityID, defaultViewWorld())
		if err != nil {
			http.Error(w, "View error: "+err.Error(), http.StatusBadRequest)
			return
		}
		input = h.buildViewInput(r.Context(), viewID, vr)
	case "global":
		input = h.buildGlobalInput()
	default:
		http.Error(w, "Invalid command context: "+cmd.Context, http.StatusBadRequest)
		return
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		http.Error(w, "Failed to build input: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Set up SSE response. Flusher is optional — Wails' asset server on
	// macOS/Linux delivers each Write() immediately without needing Flush().
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Start the script. Use r.Context() so a client disconnect triggers cancel,
	// but mirror handleCommandCancel's SIGINT+3s-grace contract so scripts that
	// catch SIGINT (e.g., to flush output or commit a transaction) still get
	// the chance rather than being SIGKILLed outright.
	// #nosec G702 -- cmd.Script is operator-authored: it comes from the
	// `commands:` map in the project's data-entry.yaml (dataentryconfig),
	// selected by name via commandID. The request may only choose a
	// registered command key; it never supplies a command, flag, or path.
	// Request-derived values reach the script only as stdin JSON and
	// RELA_* env vars, never as shell text. A shell is intentional here —
	// `script:` is documented as a shell snippet, and executing it is the
	// feature. The trust boundary is the config file (operator/repo write
	// access), enforced upstream by authorizeCommand.
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

	// Register for cancellation, bound to the starting principal so only they
	// can cancel it (RR-YZV7SY).
	runningCommands.Store(execID, &runningCommand{
		cmd:   proc,
		owner: principal.From(r.Context()),
	})
	defer runningCommands.Delete(execID)

	// Capture stderr in background.
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

	// Stream stdout as SSE events.
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		msg := parseCommandOutput(line)
		data, _ := json.Marshal(msg)
		writeSSEEvent(w, flusher, msg.Type, string(data))
	}

	// Wait for stderr goroutine and process to finish.
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

	// Only the principal that started the command may cancel it (RR-YZV7SY).
	// execID is client-supplied and the registry is process-global, so without
	// this a caller who guessed an id could kill another user's run — including
	// a caller whose own exec attempts are being 403'd. Answer 404, identical
	// to an unknown id, so cancel cannot be used to probe which commands are
	// currently running under other principals.
	if !rc.owner.Equal(principal.From(r.Context())) {
		http.Error(w, "No running command: "+execID, http.StatusNotFound)
		return
	}

	// Send SIGINT for graceful shutdown.
	if rc.cmd.Process != nil {
		_ = rc.cmd.Process.Signal(syscall.SIGINT)
	}

	// Wait briefly, then force kill.
	go func() {
		time.Sleep(cancelGrace)
		if rc.cmd.Process != nil {
			_ = rc.cmd.Process.Kill()
		}
	}()

	w.WriteHeader(http.StatusOK)
}

// handleOpenFile handles POST /api/open-file to open or reveal files.
//
// coverage-ignore-func: requires OS interaction
func (h *commandHandler) handleOpenFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filePath := r.URL.Query().Get("path")
	action := r.URL.Query().Get("action")
	if filePath == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}

	resolved, err := containedProjectPath(h.projectRoot(), filePath)
	switch {
	case errors.Is(err, errPathNotFound):
		http.Error(w, "file not found", http.StatusNotFound)
		return
	case err != nil:
		http.Error(w, "path outside project", http.StatusForbidden)
		return
	}
	filePath = resolved

	// Fire-and-forget launcher: MUST outlive the HTTP handler. If we used
	// r.Context() here, xdg-open on Linux would be killed before it could
	// dispatch to the real handler (gedit, nautilus, etc. don't daemonize
	// and die with their parent).
	cmd := openFileCommand(runtime.GOOS, action, filePath)
	// coverage-ignore-start: external-tool: launches the OS file opener (xdg-open/open); unsupported-platform is
	// unreachable on the test GOOS and
	// Start/Wait spawn a real process (project marks it coverage-ignore)
	if cmd == nil {
		http.Error(w, "Unsupported platform", http.StatusInternalServerError)
		return
	}

	if err := cmd.Start(); err != nil {
		http.Error(w, "Failed to open file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	go func() { _ = cmd.Wait() }() // reap zombie
	w.WriteHeader(http.StatusOK)
	// coverage-ignore-end
}

// openFileCommand builds the OS-specific launcher for handleOpenFile.
// Returned command has no context binding — the launcher process must
// survive the HTTP handler's return (see handleOpenFile for rationale).
//
// Security: the program name is a compile-time constant on every branch and
// filePath is passed as a distinct argv element, so no shell parses it — the
// only injection shape left is argument injection (a path read as a flag),
// which the `--` separator below closes. filePath has already been through
// containedProjectPath in the caller: it is absolute, symlink-resolved, and
// proven to be inside the project root, so it always starts with `/` (or a
// drive letter on Windows) and can never be NUL-bearing.
func openFileCommand(goos, action, filePath string) *exec.Cmd {
	switch goos {
	case "darwin":
		if action == "reveal" {
			// #nosec G702 -- argv array, no shell; constant program `open`.
			// filePath is containedProjectPath-validated and `--` stops flag parsing.
			return exec.Command("open", "-R", "--", filePath) //nolint:noctx // fire-and-forget launcher
		}
		// #nosec G702 -- argv array, no shell; see function doc.
		return exec.Command("open", "--", filePath) //nolint:noctx // fire-and-forget launcher
	case "linux":
		if action == "reveal" {
			// #nosec G702 -- argv array, no shell; see function doc.
			return exec.Command("xdg-open", filepath.Dir(filePath)) //nolint:noctx // fire-and-forget launcher
		}
		// #nosec G702 -- argv array, no shell; see function doc.
		return exec.Command("xdg-open", filePath) //nolint:noctx // fire-and-forget launcher
	case "windows":
		if action == "reveal" {
			// #nosec G702 -- argv array, no shell; constant program `explorer`.
			return exec.Command("explorer", "/select,", filePath) //nolint:noctx // fire-and-forget launcher
		}
		// #nosec G702 -- `cmd /c start` with an explicit empty title argument, so
		// filePath lands in the path slot rather than being read as the title.
		// filePath is containedProjectPath-validated (absolute, inside the project
		// root, no NUL). Windows is not a supported data-entry server platform;
		// this branch exists for the desktop build.
		return exec.Command("cmd", "/c", "start", "", filePath) //nolint:noctx // fire-and-forget launcher
	default:
		return nil
	}
}

// handleOpenURL handles POST /api/open-url to open URLs in the default browser.
//
// coverage-ignore-func: requires OS interaction
func (h *commandHandler) handleOpenURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rawURL := r.URL.Query().Get("url")
	if rawURL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}
	if err := validateOpenURL(rawURL); err != nil {
		http.Error(w, "Invalid URL scheme", http.StatusBadRequest)
		return
	}

	// Fire-and-forget launcher: see handleOpenFile for why we can't bind to r.Context().
	cmd := openURLCommand(runtime.GOOS, rawURL)
	// coverage-ignore-start: external-tool: launches the OS URL opener (xdg-open/open); unsupported-platform is
	// unreachable on the test GOOS and
	// Start/Wait spawn a real process (project marks it coverage-ignore)
	if cmd == nil {
		http.Error(w, "Unsupported platform", http.StatusInternalServerError)
		return
	}

	if err := cmd.Start(); err != nil {
		http.Error(w, "Failed to open URL: "+err.Error(), http.StatusInternalServerError)
		return
	}
	go func() { _ = cmd.Wait() }() // reap zombie
	w.WriteHeader(http.StatusOK)
	// coverage-ignore-end
}

// openURLCommand builds the OS-specific URL launcher. Returned command is
// deliberately unbound from any HTTP request context — see handleOpenURL.
//
// Security: the program name is a compile-time constant on every branch and
// rawURL is a distinct argv element, so no shell parses it. rawURL has already
// passed validateOpenURL in the caller, which parses it with net/url and
// admits only the http, https, and mailto schemes — so it always begins with a
// scheme name, never a `-`, closing the argument-injection shape. The `--`
// separator below makes that structural rather than a consequence of the
// scheme allow-list.
func openURLCommand(goos, rawURL string) *exec.Cmd {
	switch goos {
	case "darwin":
		// #nosec G702 -- argv array, no shell; validateOpenURL-restricted scheme
		// and `--` stops flag parsing. See function doc.
		return exec.Command("open", "--", rawURL) //nolint:noctx // fire-and-forget launcher
	case "linux":
		// #nosec G702 -- argv array, no shell; see function doc.
		return exec.Command("xdg-open", rawURL) //nolint:noctx // fire-and-forget launcher
	case "windows":
		// #nosec G702 -- `cmd /c start` with an explicit empty title argument, so
		// rawURL lands in the target slot rather than being read as the title.
		// Scheme is restricted to http/https/mailto by validateOpenURL.
		return exec.Command("cmd", "/c", "start", "", rawURL) //nolint:noctx // fire-and-forget launcher
	default:
		return nil
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
// inside projectRoot. The returned path has absolute, symlink-resolved form
// suitable for passing to OS commands.
//
// A small TOCTOU window remains: between this check and the synchronous
// invocation of the OS open command, an attacker with local FS write
// privileges could swap a contained path for a symlink. The local
// filesystem is the trust boundary; we accept this residual risk because
// portable mitigation (file descriptor passing through `open`/`xdg-open`/
// `explorer`) does not exist.
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
		// Path does not exist (or contains a broken symlink). Distinguish
		// "not found inside project" from "outside project" so the handler
		// can return 404 vs 403. Verify the unresolved abs path is at
		// least structurally inside the project root before reporting it
		// as a 404; otherwise it's a traversal attempt against a
		// non-existent file.
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

// validateOpenURL allows only safe URL schemes for /api/open-url. Without
// this, an attacker could pass file:// (file disclosure) or javascript:
// (XSS in some default handlers).
func validateOpenURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "mailto":
		return nil
	}
	return errors.New("disallowed url scheme")
}

func (h *commandHandler) buildCommandEnv(cmd CommandConfig, input *commandInput) []string {
	env := os.Environ()
	env = append(env,
		"RELA_PROJECT_ROOT="+h.projectRoot(),
		"RELA_CONTEXT="+cmd.Context,
	)
	if input.Entity != nil {
		env = append(env,
			"RELA_ENTITY_ID="+input.Entity.ID,
			"RELA_ENTITY_TYPE="+input.Entity.Type,
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
