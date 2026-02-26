package dataentry

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/conflict"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// ConflictListItem represents a conflicted file for display.
type ConflictListItem struct {
	Path        string
	RelPath     string
	EntityType  string
	EntityID    string
	MarkerCount int
}

// ConflictResolutionData contains data for the resolution page.
type ConflictResolutionData struct {
	File       *conflict.ConflictedFile
	Info       *conflict.Info
	RelPath    string
	IsEntity   bool
	IsRelation bool
}

// handleConflicts shows a list of files with git conflicts.
func (a *App) handleConflicts(w http.ResponseWriter, r *http.Request) {
	// Check if this is a resolution request (path after /conflicts/)
	path := strings.TrimPrefix(r.URL.Path, "/conflicts")
	if path != "" && path != "/" {
		a.handleConflictResolve(w, r)
		return
	}

	ctx := &project.Context{
		Root:         a.ws.Paths().Root,
		EntitiesDir:  a.ws.Paths().EntitiesDir,
		RelationsDir: a.ws.Paths().RelationsDir,
	}

	result, err := conflict.DetectAll(ctx)
	if err != nil {
		http.Error(w, "Failed to detect conflicts: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Build list items
	items := make([]ConflictListItem, 0, len(result.Files))
	for _, cf := range result.Files {
		relPath, _ := filepath.Rel(ctx.Root, cf.Path)
		items = append(items, ConflictListItem{
			Path:        cf.Path,
			RelPath:     relPath,
			EntityType:  cf.EntityType,
			EntityID:    cf.EntityID,
			MarkerCount: len(cf.Markers),
		})
	}

	data := map[string]interface{}{
		"App":           a.Cfg.App,
		"ConflictCount": len(items),
		"Navigation":    a.navElements("_conflicts"),
		"ActiveList":    "_conflicts",
		"Conflicts":     items,
		"HasConflicts":  len(items) > 0,
		"IsHTMX":        r.Header.Get("HX-Request") == "true",
	}

	if r.Header.Get("HX-Request") == "true" {
		a.tmpl.ExecuteTemplate(w, "conflicts-content", data) //nolint:errcheck // template errors handled by http response
	} else {
		a.tmpl.ExecuteTemplate(w, "conflicts-page", data) //nolint:errcheck // template errors handled by http response
	}
}

// handleConflictResolve shows the resolution UI for a specific file.
func (a *App) handleConflictResolve(w http.ResponseWriter, r *http.Request) {
	// Extract file path from URL
	path := strings.TrimPrefix(r.URL.Path, "/conflicts/")
	if path == "" {
		http.Error(w, "Missing file path", http.StatusBadRequest)
		return
	}

	// Build absolute path
	ctx := a.ws.Paths()
	absPath := filepath.Join(ctx.Root, path)

	// Parse the conflicted file
	cf, err := conflict.ParseConflictedFile(absPath, a.meta)
	if err != nil {
		http.Error(w, "Failed to parse conflict: "+err.Error(), http.StatusInternalServerError)
		return
	}

	info := conflict.AnalyzeConflict(cf)

	resData := &ConflictResolutionData{
		File:       cf,
		Info:       info,
		RelPath:    path,
		IsEntity:   cf.Ours != nil && cf.Ours.Entity != nil,
		IsRelation: cf.Ours != nil && cf.Ours.Relation != nil,
	}

	data := map[string]interface{}{
		"App":           a.Cfg.App,
		"ConflictCount": a.conflictCount(),
		"Navigation":    a.navElements("_conflicts"),
		"ActiveList":    "_conflicts",
		"Resolution":    resData,
		"IsHTMX":        r.Header.Get("HX-Request") == "true",
	}

	if r.Header.Get("HX-Request") == "true" {
		a.tmpl.ExecuteTemplate(w, "conflict-resolve-content", data) //nolint:errcheck // template errors handled by http response
	} else {
		a.tmpl.ExecuteTemplate(w, "conflict-resolve-page", data) //nolint:errcheck // template errors handled by http response
	}
}

// handleConflictApply applies a resolution to a conflicted file.
func (a *App) handleConflictApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	path := r.FormValue("path")
	if path == "" {
		http.Error(w, "Missing path", http.StatusBadRequest)
		return
	}

	action := r.FormValue("action")
	ctx := a.ws.Paths()
	absPath := filepath.Join(ctx.Root, path)

	// Parse the file again
	cf, err := conflict.ParseConflictedFile(absPath, a.meta)
	if err != nil {
		http.Error(w, "Failed to parse conflict: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var resolution *conflict.Resolution

	switch action {
	case "ours":
		resolution = conflict.AcceptOurs(cf)
	case "theirs":
		resolution = conflict.AcceptTheirs(cf)
	case "custom":
		// Build resolution from form values
		resolution = &conflict.Resolution{
			PropertyChoices: make(map[string]conflict.Side),
		}

		// Get property choices
		for key := range r.Form {
			if strings.HasPrefix(key, "prop_") {
				propName := strings.TrimPrefix(key, "prop_")
				value := r.FormValue(key)
				if value == "theirs" {
					resolution.PropertyChoices[propName] = conflict.SideTheirs
				} else {
					resolution.PropertyChoices[propName] = conflict.SideOurs
				}
			}
		}

		// Get content choice
		contentChoice := r.FormValue("content")
		switch contentChoice {
		case "theirs":
			resolution.ContentChoice = conflict.SideTheirs
		case "manual":
			resolution.ManualContent = r.FormValue("manual_content")
		default:
			resolution.ContentChoice = conflict.SideOurs
		}
	default:
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}

	// Apply the resolution
	if err := conflict.ResolveAndWrite(cf, resolution, a.meta); err != nil {
		http.Error(w, "Failed to resolve: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect back to conflicts list
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/conflicts")
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, "/conflicts", http.StatusSeeOther)
	}
}

// conflictCount returns the number of conflicted files for nav badge.
func (a *App) conflictCount() int {
	if a.ws == nil {
		return 0
	}
	ctx := &project.Context{
		Root:         a.ws.Paths().Root,
		EntitiesDir:  a.ws.Paths().EntitiesDir,
		RelationsDir: a.ws.Paths().RelationsDir,
	}

	result, err := conflict.DetectAll(ctx)
	if err != nil {
		return 0
	}
	return len(result.Files)
}
