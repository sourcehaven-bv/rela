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

// authorizeCommand reports whether the principal on ctx may execute cmd.
//
// It is the SINGLE decision point, called by both the exec handler and
// resolveCommands so the rendered button set and the enforced boundary cannot
// drift. The 403 is the boundary; the resolve filter is a UX affordance.
//
// Policy (DEC-EIHQSU), keyed on the configured ACL implementation:
//
//   - [acl.ReadOnlyACL] → deny everything, every context. Checked FIRST and
//     independently of the read gate: command exec builds no acl.WriteRequest,
//     so ReadOnlyACL.AuthorizeWrite is never consulted, and readGateFromContext
//     hands back nopReadGate (HoldsPermission ⇒ true) under read-only exactly
//     as it does under NopACL. A guard written against the read gate alone
//     therefore fails OPEN here — that was the live bug (RR-CWWJGW) this
//     function exists to close. TestCommandExecReadOnlyDenied is its canary.
//   - [*acl.Declarative] → fail closed. `context: view` is denied outright
//     (its payload is the whole traversal closure, not one entity — see
//     TKT-MJ02AO); otherwise Permission must be set AND held.
//   - [acl.NopACL] → fail open, preserving pre-ACL behavior. This is the ONLY
//     arm that grants by default.
//   - anything else → DENY.
//
// The switch is closed by construction (RR-CAUBAZ): the default arm denies, so
// an ACL implementation nobody taught this function about cannot silently grant
// shell execution. Both value and pointer forms of the nop/read-only types are
// matched explicitly, because their AuthorizeWrite has a VALUE receiver — a
// `&acl.ReadOnlyACL{}` therefore satisfies acl.ACL, and matching only the value
// form would drop it into the default arm. When that arm granted, that was a
// silent `--read-only` bypass reachable by one `&`.
//
// Adding a new acl.ACL implementation? It denies commands until you add an arm.
// That is deliberate: the failure mode of forgetting is a denied command, not
// an ungoverned shell.
func authorizeCommand(ctx context.Context, aclImpl acl.ACL, cmd CommandConfig) bool {
	// A nil ACL means the handler was wired without one. Deny: an
	// authorization guard must fail closed on a wiring bug, never grant
	// because a field was left unset. (Catches an untyped nil; a typed-nil
	// interface falls to the arms below, which also deny.)
	if aclImpl == nil {
		return false
	}

	switch a := aclImpl.(type) {
	case acl.NopACL, *acl.NopACL:
		// No policy configured ⇒ commands behave exactly as they did before
		// this gating existed.
		return true

	case acl.ReadOnlyACL, *acl.ReadOnlyACL:
		return false

	case *acl.Declarative:
		if a == nil {
			return false // misconfigured policy must not fail open
		}
		// View commands have no fine-grained control yet: `permission:` is
		// not honored for them, so a granted permission must NOT open the
		// gate. Deferred deliberately, not overlooked.
		if cmd.Context == "view" {
			return false
		}
		if cmd.Permission == "" {
			return false // fail closed: a policy is configured, this command is ungoverned
		}
		return readGateFromContext(ctx).HoldsPermission(ctx, cmd.Permission)

	default:
		return false
	}
}

// resolveCommands returns commands available for a given page context.
// pageType is "entity", "list", "view", or "dashboard".
// qualifier is the specific list ID or view ID.
// entityType is the entity type shown on the page (empty for dashboard).
//
// Commands the principal may not execute are omitted, so the SPA never renders
// a button that would 403 on click. This is presentation only — authorizeCommand
// is re-consulted at exec time, which is the actual boundary.
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

	aclImpl := h.currentACL()
	var result []ResolvedCommand
	for _, id := range ids {
		cmd := s.Cfg.Commands[id]
		if !matchesPage(cmd, pageType, qualifier, entityType) {
			continue
		}
		if authorizeCommand(ctx, aclImpl, cmd) {
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
	if !authorizeCommand(r.Context(), h.currentACL(), cmd) {
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
		entityDomain, err := svc.Store.GetEntity(r.Context(), entityID)
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
		vr, err := h.executeView(r.Context(), viewCfg, entityID)
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
