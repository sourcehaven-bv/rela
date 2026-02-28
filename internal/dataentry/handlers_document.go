package dataentry

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// logTemplateError logs template execution errors.
func (a *App) logTemplateError(tmplName string, err error) {
	log.Printf("Template %q execution error: %v", tmplName, err)
}

// handleDocumentPreview renders a document by executing an external render command.
// URL: /document/preview?entry=<entity-id>&doc=<document-name>
// If render=true query param is set, it does the actual rendering (called via HTMX).
// Otherwise it checks cache first - if valid, shows content; else shows loading spinner.
func (a *App) handleDocumentPreview(w http.ResponseWriter, r *http.Request) {
	entryID := r.URL.Query().Get("entry")
	if entryID == "" {
		http.Error(w, "Missing 'entry' query parameter", http.StatusBadRequest)
		return
	}

	// Get document config
	docName := r.URL.Query().Get("doc")
	docCfg, err := a.getDocumentConfig(entryID, docName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if this is the async render request
	if r.URL.Query().Get("render") == "true" {
		a.handleDocumentRender(w, r, entryID, docCfg)
		return
	}

	// Get the entry entity for page title
	entry, _ := a.ws.GetEntity(entryID)

	// Convert config to workspace format
	wsCfg := a.toWorkspaceDocConfig(docCfg)

	// Try to get cached content
	result, err := a.ws.RenderDocument(entryID, wsCfg)
	if err == nil && result.CacheHit {
		// Rewrite special links for UI
		returnPath := "/document/preview?entry=" + entryID
		content := workspace.RewriteEditLinks(result.HTML, returnPath)
		content = workspace.RewriteCreateLinks(content, returnPath)
		a.renderDocument(w, r, entryID, entry, content)
		return
	}

	// Cache miss or error - render loading page, HTMX will trigger the actual render
	a.renderDocument(w, r, entryID, entry, "")
}

// handleDocumentRender does the actual document rendering (called async via HTMX).
func (a *App) handleDocumentRender(w http.ResponseWriter, _ *http.Request, entryID string, docCfg *DocumentConfig) {
	wsCfg := a.toWorkspaceDocConfig(docCfg)

	result, err := a.ws.RenderDocument(entryID, wsCfg)
	if err != nil {
		a.renderDocumentErrorFragment(w, entryID, err, "Command: "+docCfg.Command)
		return
	}

	// Rewrite special links for UI
	returnPath := "/document/preview?entry=" + entryID
	content := workspace.RewriteEditLinks(result.HTML, returnPath)
	content = workspace.RewriteCreateLinks(content, returnPath)

	// Return the content fragment for HTMX to swap in (with wrapper)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if result.CacheHit {
		w.Header().Set("X-Cache", "HIT")
	} else {
		w.Header().Set("X-Cache", "MISS")
	}
	renderURL := "/document/preview?entry=" + entryID + "&render=true"
	fmt.Fprintf(w, `<div class="document-content" data-render-url="%s">%s</div>`, renderURL, content)
}

// getDocumentConfig finds the appropriate document config for an entry.
// If docName is provided, it uses that specific config.
// Otherwise, it finds a config that matches the entry's type.
func (a *App) getDocumentConfig(entryID, docName string) (*DocumentConfig, error) {
	// If explicit doc name provided, use it
	if docName != "" {
		if cfg, ok := a.Cfg.Documents[docName]; ok {
			return &cfg, nil
		}
		return nil, fmt.Errorf("document config %q not found", docName)
	}

	// Get entry entity to check type
	entry, ok := a.ws.GetEntity(entryID)
	if !ok {
		return nil, fmt.Errorf("entry %s not found", entryID)
	}

	// Find a document config that matches the entry type.
	// Note: If multiple configs match, the first found wins (map order is non-deterministic).
	// In practice, users should configure non-overlapping EntryTypes or use explicit doc names.
	for _, cfg := range a.Cfg.Documents {
		if len(cfg.EntryTypes) == 0 {
			// No type restriction, use this config
			return &cfg, nil
		}
		for _, t := range cfg.EntryTypes {
			if t == entry.Type {
				return &cfg, nil
			}
		}
	}

	return nil, fmt.Errorf("no document config found for entry type %q", entry.Type)
}

// toWorkspaceDocConfig converts dataentry config to workspace config.
func (a *App) toWorkspaceDocConfig(cfg *DocumentConfig) workspace.DocumentConfig {
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return workspace.DocumentConfig{
		View:    cfg.View,
		Command: cfg.Command,
		Timeout: timeout,
	}
}

// renderDocument renders the document page. If content is empty, shows loading state.
func (a *App) renderDocument(w http.ResponseWriter, r *http.Request, entryID string, entry *model.Entity, content string) {
	pageTitle := "Document Preview"
	entryType := ""
	entryTitle := ""

	if entry != nil {
		entryType = entry.Type
		entryTitle = a.entityDisplayTitle(entry)
		pageTitle = entryTitle
	}

	data := map[string]interface{}{
		"App":           a.Cfg.App,
		"ConflictCount": a.conflictCount(),
		"Navigation":    a.navElements("_document"),
		"ActiveList":    "_document",
		"PageTitle":     pageTitle,
		"EntryID":       entryID,
		"EntryType":     entryType,
		"EntryTitle":    entryTitle,
		"Content":       content,
		"CurrentPath":   "/document/preview?entry=" + entryID,
		"RenderURL":     "/document/preview?entry=" + entryID + "&render=true",
		"IsHTMX":        r.Header.Get("HX-Request") == "true",
		"Loading":       content == "",
	}
	a.addGitData(data)

	tmplName := "document"
	if r.Header.Get("HX-Request") == "true" {
		tmplName = "document-content"
	}
	if err := a.tmpl.ExecuteTemplate(w, tmplName, data); err != nil {
		a.logTemplateError(tmplName, err)
	}
}

// renderDocumentErrorFragment renders an error fragment for HTMX swap.
func (a *App) renderDocumentErrorFragment(w http.ResponseWriter, entryID string, cmdErr error, cmdContext string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := map[string]interface{}{
		"Error":     cmdErr.Error(),
		"Context":   cmdContext,
		"RenderURL": "/document/preview?entry=" + entryID + "&render=true",
	}
	if err := a.tmpl.ExecuteTemplate(w, "document-error", data); err != nil {
		a.logTemplateError("document-error", err)
	}
}
