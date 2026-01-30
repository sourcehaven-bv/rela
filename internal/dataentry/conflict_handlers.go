package dataentry

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// getConflicts returns the current conflict set, preferring SyncManager's conflicts
// but falling back to the App-level conflicts (used for test data).
func (a *App) getConflicts() *ConflictSet {
	if cs := a.sync.Conflicts(); cs != nil {
		return cs
	}
	return a.conflicts
}

// handleConflicts renders the conflict list page showing all current conflicts.
// coverage-ignore: HTTP handler
func (a *App) handleConflicts(w http.ResponseWriter, r *http.Request) {
	conflicts := a.getConflicts()
	if conflicts == nil || len(conflicts.Files) == 0 {
		data := map[string]interface{}{
			"App":        a.Cfg.App,
			"Navigation": a.navItems(),
			"SyncStatus": a.sync.Status(),
			"ActiveList": "",
			"Conflicts":  nil,
			"Count":      0,
		}
		if r.Header.Get("HX-Request") != "" {
			if err := a.tmpl.ExecuteTemplate(w, "conflicts-content", data); err != nil {
				log.Printf("template error: %v", err)
			}
		} else {
			if err := a.tmpl.ExecuteTemplate(w, "conflicts-page", data); err != nil {
				log.Printf("template error: %v", err)
			}
		}
		return
	}

	// Build summary info for each conflict
	type ConflictSummary struct {
		ID              string
		Title           string
		EntityType      string
		FilePath        string
		ConflictFields  int
		HasBodyConflict bool
		Resolved        bool
	}

	summaries := make([]ConflictSummary, 0, len(conflicts.Files))
	for _, cf := range conflicts.Files {
		summaries = append(summaries, ConflictSummary{
			ID:              cf.ID,
			Title:           cf.Title,
			EntityType:      cf.EntityType,
			FilePath:        cf.FilePath,
			ConflictFields:  cf.CountConflictingFields(),
			HasBodyConflict: cf.BodyConflict != nil && !cf.BodyConflict.CanAutoMerge,
			Resolved:        cf.Resolved,
		})
	}

	data := map[string]interface{}{
		"App":        a.Cfg.App,
		"Navigation": a.navItems(),
		"SyncStatus": a.sync.Status(),
		"ActiveList": "",
		"Conflicts":  summaries,
		"Count":      conflicts.ConflictCount(),
	}

	if r.Header.Get("HX-Request") != "" {
		if err := a.tmpl.ExecuteTemplate(w, "conflicts-content", data); err != nil {
			log.Printf("template error: %v", err)
		}
	} else {
		if err := a.tmpl.ExecuteTemplate(w, "conflicts-page", data); err != nil {
			log.Printf("template error: %v", err)
		}
	}
}

// handleConflictResolve renders the per-entity conflict resolution page.
// coverage-ignore: HTTP handler
func (a *App) handleConflictResolve(w http.ResponseWriter, r *http.Request) {
	// Parse conflict ID from URL: /conflicts/resolve/{id}
	path := strings.TrimPrefix(r.URL.Path, "/conflicts/resolve/")
	conflictID := strings.TrimSuffix(path, "/")

	conflicts := a.getConflicts()
	if conflicts == nil {
		http.Error(w, "No conflicts to resolve", http.StatusNotFound)
		return
	}

	cf := conflicts.GetConflict(conflictID)
	if cf == nil {
		http.Error(w, "Conflict not found: "+conflictID, http.StatusNotFound)
		return
	}

	data := map[string]interface{}{
		"App":        a.Cfg.App,
		"Navigation": a.navItems(),
		"SyncStatus": a.sync.Status(),
		"ActiveList": "",
		"Conflict":   cf,
	}

	if r.Header.Get("HX-Request") != "" {
		if err := a.tmpl.ExecuteTemplate(w, "conflict-resolve-content", data); err != nil {
			log.Printf("template error: %v", err)
		}
	} else {
		if err := a.tmpl.ExecuteTemplate(w, "conflict-resolve-page", data); err != nil {
			log.Printf("template error: %v", err)
		}
	}
}

// handleConflictSubmit processes a conflict resolution form submission.
// coverage-ignore: HTTP handler
func (a *App) handleConflictSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	conflictID := r.FormValue("conflict_id")
	conflicts := a.getConflicts()
	if conflicts == nil {
		http.Error(w, "No conflicts", http.StatusNotFound)
		return
	}

	cf := conflicts.GetConflict(conflictID)
	if cf == nil {
		http.Error(w, "Conflict not found", http.StatusNotFound)
		return
	}

	// Extract field choices from form
	fieldChoices := make(map[string]string)
	for _, f := range cf.Fields {
		if f.Status == "conflict" {
			choice := r.FormValue("field_" + f.Property)
			if choice == "" {
				http.Error(w, "Missing resolution for field: "+f.Property, http.StatusBadRequest)
				return
			}
			fieldChoices[f.Property] = choice
		}
	}

	// Extract per-hunk choices for body conflicts
	hunkChoices := make(map[int]string)
	if cf.BodyConflict != nil {
		for _, h := range cf.BodyConflict.Hunks {
			if h.Source != "conflict" {
				continue
			}
			key := fmt.Sprintf("hunk_%d", h.Index)
			choice := r.FormValue(key)
			if choice == "" {
				http.Error(w, "Missing resolution for body conflict hunk", http.StatusBadRequest)
				return
			}
			hunkChoices[h.Index] = choice
		}
	}

	// Apply resolution
	resolvedProps, resolvedBody, err := cf.ApplyResolution(fieldChoices, hunkChoices)
	if err != nil {
		http.Error(w, "Resolution error: "+err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Resolved conflict %s: %d properties, %d hunk choices", conflictID, len(resolvedProps), len(hunkChoices))

	// Write the merged file to disk
	merged, err := FormatResolvedDocument(resolvedProps, resolvedBody)
	if err != nil {
		http.Error(w, "Failed to format resolved document: "+err.Error(), http.StatusInternalServerError)
		return
	}

	absPath := filepath.Join(a.sync.RepoRoot(), cf.FilePath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		http.Error(w, "Failed to create directory: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(absPath, []byte(merged), 0o644); err != nil {
		http.Error(w, "Failed to write file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Stage the resolved file
	if backend := a.sync.Backend(); backend != nil {
		if _, gitErr := backend.Git("add", cf.FilePath); gitErr != nil {
			log.Printf("Warning: git add failed for %s: %v", cf.FilePath, gitErr)
		}
	}

	// Redirect back to conflicts list
	w.Header().Set("HX-Redirect", "/conflicts")
	w.WriteHeader(http.StatusOK)
}

// handleConflictStatus returns the current conflict status as JSON.
// coverage-ignore: HTTP handler
func (a *App) handleConflictStatus(w http.ResponseWriter, _ *http.Request) {
	status := struct {
		Count int  `json:"count"`
		Has   bool `json:"has_conflicts"`
	}{
		Count: 0,
		Has:   false,
	}

	if cs := a.getConflicts(); cs != nil {
		status.Count = cs.ConflictCount()
		status.Has = status.Count > 0
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Printf("JSON encode error: %v", err)
	}
}

// handleConflictResolveAll completes the merge after all conflicts are resolved.
// It creates a merge commit with the resolved files and pushes.
// coverage-ignore: HTTP handler
func (a *App) handleConflictResolveAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	conflicts := a.getConflicts()
	if conflicts == nil || len(conflicts.Files) == 0 {
		http.Error(w, "No conflicts to resolve", http.StatusBadRequest)
		return
	}

	// Check all are resolved
	for _, cf := range conflicts.Files {
		if !cf.Resolved {
			http.Error(w, fmt.Sprintf("Conflict %q not yet resolved", cf.ID), http.StatusBadRequest)
			return
		}
	}

	// Complete the merge
	if err := a.sync.CompleteMerge(); err != nil {
		log.Printf("Complete merge failed: %v", err)
		http.Error(w, "Merge failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Rebuild graph after merge
	if err := a.rebuildGraph(); err != nil {
		log.Printf("Warning: graph rebuild after merge failed: %v", err)
	}

	log.Printf("Merge completed: %d conflicts resolved", len(conflicts.Files))
	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

// handleConflictLoadTest loads synthetic test conflicts for UI development.
// coverage-ignore: HTTP handler
func (a *App) handleConflictLoadTest(w http.ResponseWriter, _ *http.Request) {
	a.conflicts = BuildTestConflictSet()
	log.Printf("Loaded %d test conflicts", len(a.conflicts.Files))
	w.Header().Set("HX-Redirect", "/conflicts")
	w.WriteHeader(http.StatusOK)
}
