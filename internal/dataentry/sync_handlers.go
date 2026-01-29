package dataentry

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
)

// handleSyncStatus returns the current sync status as JSON.
// coverage-ignore: HTTP handler
func (a *App) handleSyncStatus(w http.ResponseWriter, _ *http.Request) {
	status := a.sync.Status()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Printf("JSON encode error: %v", err)
	}
}

// handleSyncStatusIndicator returns the sync indicator HTML fragment for HTMX polling.
// coverage-ignore: HTTP handler
func (a *App) handleSyncStatusIndicator(w http.ResponseWriter, _ *http.Request) {
	data := map[string]interface{}{
		"SyncStatus": a.sync.Status(),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.tmpl.ExecuteTemplate(w, "sync-indicator-content", data); err != nil {
		log.Printf("template error: %v", err)
	}
}

// handleSyncBranches returns the list of branches as JSON.
// coverage-ignore: HTTP handler
func (a *App) handleSyncBranches(w http.ResponseWriter, _ *http.Request) {
	branches, err := a.sync.Branches()
	if err != nil {
		http.Error(w, "Failed to list branches: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(branches); err != nil {
		log.Printf("JSON encode error: %v", err)
	}
}

// handleSyncBranch handles branch switching and creation.
// coverage-ignore: HTTP handler
func (a *App) handleSyncBranch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	action := r.FormValue("action")
	name := r.FormValue("name")

	if name == "" {
		http.Error(w, "Branch name is required", http.StatusBadRequest)
		return
	}

	switch action {
	case "switch":
		if err := a.sync.SwitchBranch(name); err != nil {
			http.Error(w, "Failed to switch branch: "+err.Error(), http.StatusInternalServerError)
			return
		}
	case "create":
		if err := a.sync.CreateBranch(name); err != nil {
			http.Error(w, "Failed to create branch: "+err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "Invalid action: must be 'switch' or 'create'", http.StatusBadRequest)
		return
	}

	// Rebuild graph from new branch's files
	if err := a.rebuildGraph(); err != nil {
		log.Printf("Warning: graph rebuild after branch switch failed: %v", err)
	}

	log.Printf("Branch %s: %s", action, name)

	// Redirect to root to reload the UI
	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

// handleSyncBranchesList returns the branch list as an HTML fragment for the branch dropdown.
// coverage-ignore: HTTP handler
func (a *App) handleSyncBranchesList(w http.ResponseWriter, _ *http.Request) {
	branches, err := a.sync.Branches()
	if err != nil {
		http.Error(w, "Failed to list branches: "+err.Error(), http.StatusInternalServerError)
		return
	}
	data := map[string]interface{}{
		"Branches": branches,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.tmpl.ExecuteTemplate(w, "branch-list-content", data); err != nil {
		log.Printf("template error: %v", err)
	}
}

// handleSyncPush triggers an immediate sync (fetch + squash + push).
// coverage-ignore: HTTP handler
func (a *App) handleSyncPush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := a.sync.Push(); err != nil {
		log.Printf("Push failed: %v", err)
		status := a.sync.Status()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(status); encErr != nil {
			log.Printf("JSON encode error: %v", encErr)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(a.sync.Status()); err != nil {
		log.Printf("JSON encode error: %v", err)
	}
}

// handleSyncPull triggers an immediate pull (fetch + fast-forward/rebase).
// coverage-ignore: HTTP handler
func (a *App) handleSyncPull(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := a.sync.Pull(); err != nil {
		log.Printf("Pull failed: %v", err)
		status := a.sync.Status()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(status); encErr != nil {
			log.Printf("JSON encode error: %v", encErr)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(a.sync.Status()); err != nil {
		log.Printf("JSON encode error: %v", err)
	}
}

// handleSyncMoveToBranch creates a new branch from current HEAD and pushes with tracking.
// This is for moving unpushed commits off a protected branch.
// coverage-ignore: HTTP handler
func (a *App) handleSyncMoveToBranch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "Branch name is required", http.StatusBadRequest)
		return
	}

	if err := a.sync.MoveToBranch(name); err != nil {
		http.Error(w, "Failed to move to branch: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Moved commits to branch: %s", name)

	// Redirect to root to reload the UI
	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

// handleSyncSSE streams sync status updates as Server-Sent Events.
// The client receives "sync-status" events with HTML fragments.
// coverage-ignore: HTTP handler
func (a *App) handleSyncSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	id, ch := a.sync.Subscribe()
	defer a.sync.Unsubscribe(id)

	// Send initial status immediately
	a.writeSSEIndicator(w, flusher)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ch:
			if !ok {
				return
			}
			a.writeSSEIndicator(w, flusher)
		}
	}
}

// writeSSEIndicator renders the sync indicator template and writes it as an SSE event.
func (a *App) writeSSEIndicator(w http.ResponseWriter, flusher http.Flusher) {
	data := map[string]interface{}{
		"SyncStatus": a.sync.Status(),
	}
	var buf strings.Builder
	if err := a.tmpl.ExecuteTemplate(&buf, "sync-indicator-content", data); err != nil {
		log.Printf("SSE template error: %v", err)
		return
	}
	// SSE format: multiline data fields
	for _, line := range strings.Split(buf.String(), "\n") {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprintf(w, "\n")
	flusher.Flush()
}

// rebuildGraph clears and re-syncs the in-memory graph from disk.
func (a *App) rebuildGraph() error {
	newGraph := graph.New()
	result, err := markdown.SyncFromFiles(a.projCtx, a.meta, newGraph)
	if err != nil {
		return err
	}
	a.g = newGraph
	log.Printf("Graph rebuilt: %d entities, %d relations", result.EntitiesLoaded, result.RelationsLoaded)
	return nil
}
