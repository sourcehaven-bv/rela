package dataentry

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Protocol prefix for structured command output messages.
const commandOutputPrefix = "::rela::"

// ResolvedCommand is a command that has been matched to a specific page context.
type ResolvedCommand struct {
	ID       string
	Label    string
	Confirm  string
	Context  string
	AutoOpen *bool
}

// resolveCommands returns commands available for a given page context.
// pageType is "entity", "list", "view", or "dashboard".
// qualifier is the specific list ID or view ID.
// entityType is the entity type shown on the page (empty for dashboard).
func (a *App) resolveCommands(pageType, qualifier, entityType string) []ResolvedCommand {
	s := a.State()
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
		if matchesPage(cmd, pageType, qualifier, entityType) {
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
func contextMatchesPage(context, pageType string) bool {
	switch context {
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
	for _, s := range slice {
		if s == val {
			return true
		}
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

func (a *App) buildEntityInput(e *entity.Entity) *commandInput {
	return &commandInput{
		Context:   "entity",
		Entity:    e,
		Relations: relationsForEntity(a.Services(), e.ID),
		Project:   a.projectInfo(),
	}
}

// relationsForEntity loads every relation where id is either endpoint
// and returns them as []*entity.Relation for the command-input payload.
func relationsForEntity(svc Services, id string) []*entity.Relation {
	rels := make([]*entity.Relation, 0)
	q := store.RelationQuery{EntityID: id, Direction: store.DirectionBoth}
	for r, err := range svc.Store.ListRelations(context.Background(), q) {
		if err != nil {
			return rels
		}
		rels = append(rels, r)
	}
	return rels
}

func (a *App) buildListInput(listID string, entities []*entity.Entity) *commandInput {
	return &commandInput{
		Context:  "list",
		ListID:   listID,
		Entities: entities,
		Project:  a.projectInfo(),
	}
}

func (a *App) buildViewInput(viewID string, vr *viewResult) *commandInput {
	// Collect all entity IDs in the result set.
	idSet := map[string]bool{vr.Entry.ID: true}
	for _, entities := range vr.Collections {
		for _, e := range entities {
			idSet[e.ID] = true
		}
	}

	// Gather relations between entities in the result set.
	svc := a.Services()
	var rels []*entity.Relation
	for id := range idSet {
		q := store.RelationQuery{EntityID: id, Direction: store.DirectionOutgoing}
		for r, err := range svc.Store.ListRelations(context.Background(), q) {
			if err != nil {
				break
			}
			if idSet[r.To] {
				rels = append(rels, r)
			}
		}
	}

	collections := make(map[string][]*entity.Entity, len(vr.Collections))
	for k, es := range vr.Collections {
		collections[k] = es
	}

	return &commandInput{
		Context:     "view",
		ViewID:      viewID,
		Entity:      vr.Entry,
		Collections: collections,
		Relations:   rels,
		Project:     a.projectInfo(),
	}
}

func (a *App) buildGlobalInput() *commandInput {
	return &commandInput{
		Context: "global",
		Project: a.projectInfo(),
	}
}

func (a *App) projectInfo() commandProjectInfo {
	return commandProjectInfo{
		Root:      a.ProjectRoot(),
		Metamodel: "metamodel.yaml",
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
	if strings.HasPrefix(line, commandOutputPrefix) {
		payload := strings.TrimPrefix(line, commandOutputPrefix)
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
func (a *App) handleCommandExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	commandID := strings.TrimPrefix(r.URL.Path, "/api/command/")
	s := a.State()
	cmd, ok := s.Cfg.Commands[commandID]
	if !ok {
		http.Error(w, "Unknown command: "+commandID, http.StatusNotFound)
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
		svc := a.Services()
		entityDomain, err := svc.Store.GetEntity(context.Background(), entityID)
		if err != nil {
			http.Error(w, "Entity not found: "+entityID, http.StatusNotFound)
			return
		}
		input = a.buildEntityInput(entityDomain)
	case "list":
		listID := r.URL.Query().Get("list_id")
		listCfg, found := s.Cfg.Lists[listID]
		if !found {
			http.Error(w, "List not found: "+listID, http.StatusNotFound)
			return
		}
		entities := listFromStoreByTypes(a.Services(), []string{listCfg.EntityType})
		entities = applyFilters(entities, listCfg.Filters)
		input = a.buildListInput(listID, entities)
	case "view":
		viewID := r.URL.Query().Get("view_id")
		entityID := r.URL.Query().Get("entity_id")
		viewCfg, found := s.Cfg.Views[viewID]
		if !found {
			http.Error(w, "View not found: "+viewID, http.StatusNotFound)
			return
		}
		vr, err := a.executeView(viewCfg, entityID)
		if err != nil {
			http.Error(w, "View error: "+err.Error(), http.StatusBadRequest)
			return
		}
		input = a.buildViewInput(viewID, vr)
	case "global":
		input = a.buildGlobalInput()
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

	// Start the script.
	proc := exec.Command("sh", "-c", cmd.Script)
	proc.Dir = a.ProjectRoot()
	proc.Env = a.buildCommandEnv(cmd, input)
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

	// Register for cancellation.
	runningCommands.Store(execID, &runningCommand{cmd: proc})
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
func (a *App) handleCommandCancel(w http.ResponseWriter, r *http.Request) {
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

	// Send SIGINT for graceful shutdown.
	if rc.cmd.Process != nil {
		_ = rc.cmd.Process.Signal(syscall.SIGINT)
	}

	// Wait briefly, then force kill.
	go func() {
		time.Sleep(3 * time.Second)
		if rc.cmd.Process != nil {
			_ = rc.cmd.Process.Kill()
		}
	}()

	w.WriteHeader(http.StatusOK)
}

// handleOpenFile handles POST /api/open-file to open or reveal files.
func (a *App) handleOpenFile(w http.ResponseWriter, r *http.Request) { // coverage-ignore: requires OS interaction
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

	resolved, err := containedProjectPath(a.ProjectRoot(), filePath)
	switch {
	case errors.Is(err, errPathNotFound):
		http.Error(w, "file not found", http.StatusNotFound)
		return
	case err != nil:
		http.Error(w, "path outside project", http.StatusForbidden)
		return
	}
	filePath = resolved

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		if action == "reveal" {
			cmd = exec.Command("open", "-R", filePath)
		} else {
			cmd = exec.Command("open", filePath)
		}
	case "linux":
		if action == "reveal" {
			cmd = exec.Command("xdg-open", filepath.Dir(filePath))
		} else {
			cmd = exec.Command("xdg-open", filePath)
		}
	case "windows":
		if action == "reveal" {
			cmd = exec.Command("explorer", "/select,", filePath)
		} else {
			cmd = exec.Command("cmd", "/c", "start", "", filePath)
		}
	default:
		http.Error(w, "Unsupported platform", http.StatusInternalServerError)
		return
	}

	if err := cmd.Start(); err != nil {
		http.Error(w, "Failed to open file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleOpenURL handles POST /api/open-url to open URLs in the default browser.
func (a *App) handleOpenURL(w http.ResponseWriter, r *http.Request) { // coverage-ignore: requires OS interaction
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

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "linux":
		cmd = exec.Command("xdg-open", rawURL)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", rawURL)
	default:
		http.Error(w, "Unsupported platform", http.StatusInternalServerError)
		return
	}

	if err := cmd.Start(); err != nil {
		http.Error(w, "Failed to open URL: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
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

func (a *App) buildCommandEnv(cmd CommandConfig, input *commandInput) []string {
	env := os.Environ()
	env = append(env,
		"RELA_PROJECT_ROOT="+a.ProjectRoot(),
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
