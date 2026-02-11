package dataentry

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
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
	if len(a.Cfg.Commands) == 0 {
		return nil
	}

	// Sort command IDs for deterministic order.
	ids := make([]string, 0, len(a.Cfg.Commands))
	for id := range a.Cfg.Commands {
		ids = append(ids, id)
	}
	natsort.Strings(ids)

	var result []ResolvedCommand
	for _, id := range ids {
		cmd := a.Cfg.Commands[id]
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
	Context     string                     `json:"context"`
	Entity      *model.Entity              `json:"entity,omitempty"`
	Entities    []*model.Entity            `json:"entities,omitempty"`
	Collections map[string][]*model.Entity `json:"collections,omitempty"`
	Relations   []*model.Relation          `json:"relations,omitempty"`
	ListID      string                     `json:"list_id,omitempty"`
	ViewID      string                     `json:"view_id,omitempty"`
	Project     commandProjectInfo         `json:"project"`
}

type commandProjectInfo struct {
	Root      string `json:"root"`
	Metamodel string `json:"metamodel"`
}

func (a *App) buildEntityInput(entity *model.Entity) *commandInput {
	// Collect all relations involving this entity.
	outgoing := a.g.OutgoingEdges(entity.ID)
	incoming := a.g.IncomingEdges(entity.ID)
	rels := make([]*model.Relation, 0, len(outgoing)+len(incoming))
	rels = append(rels, outgoing...)
	rels = append(rels, incoming...)
	return &commandInput{
		Context:   "entity",
		Entity:    entity,
		Relations: rels,
		Project:   a.projectInfo(),
	}
}

func (a *App) buildListInput(listID string, entities []*model.Entity) *commandInput {
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
	var rels []*model.Relation
	for id := range idSet {
		for _, e := range a.g.OutgoingEdges(id) {
			if idSet[e.To] {
				rels = append(rels, e)
			}
		}
	}

	return &commandInput{
		Context:     "view",
		ViewID:      viewID,
		Entity:      vr.Entry,
		Collections: vr.Collections,
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
func (a *App) handleCommandExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	commandID := strings.TrimPrefix(r.URL.Path, "/api/command/")
	cmd, ok := a.Cfg.Commands[commandID]
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
		entity, found := a.g.GetNode(entityID)
		if !found {
			http.Error(w, "Entity not found: "+entityID, http.StatusNotFound)
			return
		}
		input = a.buildEntityInput(entity)
	case "list":
		listID := r.URL.Query().Get("list_id")
		listCfg, found := a.Cfg.Lists[listID]
		if !found {
			http.Error(w, "List not found: "+listID, http.StatusNotFound)
			return
		}
		entities := a.g.NodesByType(listCfg.EntityType)
		entities = applyFilters(entities, listCfg.Filters)
		input = a.buildListInput(listID, entities)
	case "view":
		viewID := r.URL.Query().Get("view_id")
		entityID := r.URL.Query().Get("entity_id")
		viewCfg, found := a.Cfg.Views[viewID]
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

	// Resolve relative paths against project root.
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(a.ProjectRoot(), filePath)
	}

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
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
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
