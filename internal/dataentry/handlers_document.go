package dataentry

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

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
	entry, _ := a.g.GetNode(entryID)

	// Convert config to workspace format
	wsCfg := a.toWorkspaceDocConfig(docCfg)

	// Try to get cached content
	result, err := a.ws.RenderDocument(entryID, wsCfg)
	if err == nil && result.CacheHit {
		// Rewrite special links for UI
		returnPath := "/document/preview?entry=" + entryID
		html := workspace.RewriteEditLinks(result.HTML, returnPath)
		html = workspace.RewriteCreateLinks(html, returnPath)
		a.renderDocumentPage(w, r, entryID, entry, html)
		return
	}

	// Cache miss or error - render loading page, HTMX will trigger the actual render
	a.renderDocumentLoading(w, r, entryID, entry)
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
	html := workspace.RewriteEditLinks(result.HTML, returnPath)
	html = workspace.RewriteCreateLinks(html, returnPath)

	// Return the content fragment for HTMX to swap in (with wrapper)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if result.CacheHit {
		w.Header().Set("X-Cache", "HIT")
	} else {
		w.Header().Set("X-Cache", "MISS")
	}
	renderURL := "/document/preview?entry=" + entryID + "&render=true"
	fmt.Fprintf(w, `<div class="document-content" data-render-url="%s">%s</div>`, renderURL, html)
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
	entry, ok := a.g.GetNode(entryID)
	if !ok {
		return nil, fmt.Errorf("entry %s not found", entryID)
	}

	// Find a document config that matches the entry type
	for name, cfg := range a.Cfg.Documents {
		if len(cfg.EntryTypes) == 0 {
			// No type restriction, first match wins
			return &cfg, nil
		}
		for _, t := range cfg.EntryTypes {
			if t == entry.Type {
				_ = name // Use name if needed for logging
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

// renderDocumentPage renders the document HTML in a page template.
func (a *App) renderDocumentPage(w http.ResponseWriter, r *http.Request, entryID string, entry *model.Entity, content string) {
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
	}
	a.addGitData(data)

	if r.Header.Get("HX-Request") == "true" {
		a.tmpl.ExecuteTemplate(w, "document-content", data) //nolint:errcheck // template errors logged by http
	} else {
		a.tmpl.ExecuteTemplate(w, "document", data) //nolint:errcheck // template errors logged by http
	}
}

// renderDocumentLoading renders a loading page that triggers async rendering via HTMX.
func (a *App) renderDocumentLoading(w http.ResponseWriter, r *http.Request, entryID string, entry *model.Entity) {
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
		"CurrentPath":   "/document/preview?entry=" + entryID,
		"RenderURL":     "/document/preview?entry=" + entryID + "&render=true",
		"IsHTMX":        r.Header.Get("HX-Request") == "true",
		"Loading":       true,
	}
	a.addGitData(data)

	if r.Header.Get("HX-Request") == "true" {
		a.tmpl.ExecuteTemplate(w, "document-content", data) //nolint:errcheck // template errors logged by http
	} else {
		a.tmpl.ExecuteTemplate(w, "document", data) //nolint:errcheck // template errors logged by http
	}
}

// renderDocumentErrorFragment renders an error fragment for HTMX swap.
func (a *App) renderDocumentErrorFragment(w http.ResponseWriter, _ string, cmdErr error, context string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	errMsg := cmdErr.Error()
	// Escape HTML in error message
	errMsg = strings.ReplaceAll(errMsg, "<", "&lt;")
	errMsg = strings.ReplaceAll(errMsg, ">", "&gt;")
	context = strings.ReplaceAll(context, "<", "&lt;")
	context = strings.ReplaceAll(context, ">", "&gt;")
	fmt.Fprintf(w, `<div class="document-content">
<div class="card" style="padding:20px;margin-bottom:20px;">
  <h3 style="color:var(--error);margin-bottom:12px;">Render Command Failed</h3>
  <pre style="background:var(--bg);padding:16px;border-radius:6px;overflow-x:auto;font-size:13px;white-space:pre-wrap;">%s</pre>
</div>
<details style="margin-top:20px;">
  <summary style="cursor:pointer;padding:12px;background:var(--surface);border-radius:6px;font-weight:600;">
    Show Command
  </summary>
  <div class="card" style="margin-top:8px;padding:16px;">
    <pre style="background:var(--bg);padding:16px;border-radius:6px;overflow-x:auto;font-size:12px;">%s</pre>
  </div>
</details>
<div style="margin-top:16px;">
  <button class="btn btn-secondary btn-sm" onclick="htmx.trigger('#document-body', 'load')">Retry</button>
</div>
</div>`, errMsg, context)
}
