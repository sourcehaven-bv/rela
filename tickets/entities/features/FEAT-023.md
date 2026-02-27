---
description: Render documents from external commands in the data-entry UI, enabling live preview of composed documents built from rela entities.
id: FEAT-023
priority: medium
status: proposed
title: Document Rendering in Data Entry Server
type: feature
---

# Document Rendering in Data Entry Server

## Overview

Add the ability to render documents in the data-entry server by invoking an external render command that produces markdown. This enables users to preview composed documents (like technical designs built from multiple entities) directly in the data-entry UI, with live reload on entity changes.

## Background

Projects like `gf` use `mdcomp` to compose documents from rela entities using Jinja2 templates. The current workflow requires:
1. Running `rela view` to collect related entities
2. Transforming output via Python script
3. Rendering with `mdcomp render`
4. Converting to HTML/DOCX/PDF with pandoc

This feature brings step 3 into the data-entry server, providing an integrated preview experience.

## Prototype Scope (V1)

Build a minimal prototype to validate the approach:

### 1. Hardcoded Configuration

For the prototype, hardcode:
- A single render command (e.g., `mdcomp render template.md.j2`)
- A single URL endpoint (e.g., `/document/preview`)
- Context format (YAML via stdin)

### 2. Document Handler

```go
// GET /document/preview?entry=<entity-id>
func (a *App) handleDocumentPreview(w http.ResponseWriter, r *http.Request) {
    entryID := r.URL.Query().Get("entry")
    
    // 1. Execute view (hardcoded view name for prototype)
    result, err := a.executeView(viewConfig, entryID)
    
    // 2. Build context (YAML)
    context := buildContext(result)
    
    // 3. Run render command
    cmd := exec.Command("sh", "-c", renderCommand)
    cmd.Stdin = strings.NewReader(context)
    cmd.Dir = a.ws.Paths().Root
    markdown, err := cmd.Output()
    
    // 4. Convert markdown to HTML
    html := goldmark.Convert(markdown)
    
    // 5. Rewrite edit:// links
    html = rewriteEditLinks(html)
    
    // 6. Wrap in page template and serve
    a.tmpl.ExecuteTemplate(w, "document.html", html)
}
```

### 3. Edit Link Protocol

Templates can include `edit://` links that rela rewrites to form URLs:

**In template:**
```jinja2
### [{{ id }}](edit://component/{{ id }}): {{ title }}
```

**Rendered markdown:**
```markdown
### [NVI-COMP-001](edit://component/NVI-COMP-001): Localization Service
```

**After rewrite:**
```html
<a href="/form/component?id=NVI-COMP-001&return=/document/preview?entry=DOC-001">NVI-COMP-001</a>
```

### 4. Page Template

Simple wrapper with:
- Navigation header (back link, entry entity info)
- Rendered content area
- Live reload script (existing SSE infrastructure)
- Basic styling for markdown content

### 5. Live Reload

Leverage existing file watcher:
- Entity/relation changes trigger SSE event
- Browser reloads document content
- No additional watch configuration needed for prototype

## Implementation Steps

1. **Add goldmark dependency** for markdown→HTML conversion
2. **Create document handler** with hardcoded command
3. **Implement context builder** (serialize view result to YAML)
4. **Add edit:// link rewriter** (regex replacement)
5. **Create document page template** (HTML wrapper)
6. **Wire up route** in router
7. **Test with mdcomp** on a real project

## Open Questions for Prototype

- [ ] How to handle render command errors? (show error in UI vs log)
- [ ] How to handle slow render commands? (loading indicator, timeout)
- [ ] Should we cache rendered output? (probably not for prototype)
- [ ] How to pass entry ID to the render command? (env var, arg, or context only)

## Future Iterations (Post-Prototype)

After validating the approach:

1. **Configuration** - Move to `data-entry.yaml`:
   ```yaml
   documents:
     - id: technical-design
       view: document_publish
       entry_type: document
       render:
         command: "mdcomp render publish/templates/to.md.j2"
         context_format: yaml
   ```

2. **List integration** - Allow linking from entity lists to document views:
   ```yaml
   lists:
     documents:
       type: document
       link_to: document/technical-design
   ```

3. **Navigation** - Add document links to nav menu

4. **Export** - Download rendered markdown/HTML

5. **Edit modes** - Side panel or modal editing (currently: navigate away)

6. **Template watching** - Watch template files for changes (currently: entities only)

## Success Criteria

- [ ] Can render a document at `/document/preview?entry=<id>`
- [ ] Render command receives view context as YAML stdin
- [ ] Markdown output is converted to HTML and displayed
- [ ] `edit://` links are rewritten to form URLs with return parameter
- [ ] Page reloads when entities change (via existing SSE)
- [ ] Errors are handled gracefully (shown in UI or logged)

## Non-Goals (Prototype)

- Configurable documents (hardcoded for now)
- Multiple document types
- Caching
- Export functionality
- Side panel/modal editing
- Template file watching
