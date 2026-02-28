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
	docCfg, err := a.getDocumentConfig(docName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if this is the async render request
	if r.URL.Query().Get("render") == "true" {
		a.handleDocumentRender(w, r, entryID, docName, docCfg)
		return
	}

	// Get the entry entity for page title
	entry, _ := a.ws.GetEntity(entryID)

	// Convert config to workspace format
	wsCfg := a.toWorkspaceDocConfig(docCfg)

	// Try to get cached content
	result := a.ws.GetCachedDocument(entryID, wsCfg)
	if result != nil {
		a.renderDocument(w, r, entryID, docName, entry, rewriteDocumentLinks(result.HTML, entryID, docName))
		return
	}

	// Cache miss - render loading page, HTMX will trigger the actual render
	a.renderDocument(w, r, entryID, docName, entry, "")
}

// handleDocumentRender does the actual document rendering (called async via HTMX).
func (a *App) handleDocumentRender(w http.ResponseWriter, _ *http.Request, entryID, docName string, docCfg *DocumentConfig) {
	wsCfg := a.toWorkspaceDocConfig(docCfg)

	result, err := a.ws.RenderDocument(entryID, wsCfg)
	if err != nil {
		a.renderDocumentErrorFragment(w, entryID, docName, err, "Command: "+docCfg.Command)
		return
	}

	content := rewriteDocumentLinks(result.HTML, entryID, docName)

	// Return the content fragment for HTMX to swap in (with wrapper)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	renderURL := "/document/preview?entry=" + entryID + "&doc=" + docName + "&render=true"
	fmt.Fprintf(w, `<div class="document-content" data-render-url="%s">%s</div>`, renderURL, content)
}

// getDocumentConfig finds a document config by name.
// The docName parameter is required.
func (a *App) getDocumentConfig(docName string) (*DocumentConfig, error) {
	if docName == "" {
		return nil, fmt.Errorf("missing 'doc' query parameter")
	}
	if cfg, ok := a.Cfg.Documents[docName]; ok {
		return &cfg, nil
	}
	return nil, fmt.Errorf("document config %q not found", docName)
}

// rewriteDocumentLinks rewrites edit:// and create:// links to form URLs.
func rewriteDocumentLinks(html, entryID, docName string) string {
	returnPath := "/document/preview?entry=" + entryID + "&doc=" + docName
	return workspace.RewriteDocumentLinks(html, returnPath)
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
func (a *App) renderDocument(w http.ResponseWriter, r *http.Request, entryID, docName string, entry *model.Entity, content string) {
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
		"CurrentPath":   "/document/preview?entry=" + entryID + "&doc=" + docName,
		"RenderURL":     "/document/preview?entry=" + entryID + "&doc=" + docName + "&render=true",
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
func (a *App) renderDocumentErrorFragment(w http.ResponseWriter, entryID, docName string, cmdErr error, cmdContext string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := map[string]interface{}{
		"Error":     cmdErr.Error(),
		"Context":   cmdContext,
		"RenderURL": "/document/preview?entry=" + entryID + "&doc=" + docName + "&render=true",
	}
	if err := a.tmpl.ExecuteTemplate(w, "document-error", data); err != nil {
		a.logTemplateError("document-error", err)
	}
}
