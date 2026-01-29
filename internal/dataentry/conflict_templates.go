package dataentry

// conflictTemplates contains HTML templates for the conflict resolution UI.
const conflictTemplates = `
{{- define "conflict-styles" -}}
<style>
.conflict-banner { background: #fef2f2; border: 1px solid #fecaca; border-radius: var(--radius); padding: 16px 20px; margin-bottom: 20px; display: flex; align-items: center; gap: 12px; }
.conflict-banner-icon { font-size: 24px; flex-shrink: 0; }
.conflict-banner-text { flex: 1; }
.conflict-banner-text strong { font-size: 15px; color: #991b1b; display: block; margin-bottom: 2px; }
.conflict-banner-text span { font-size: 13px; color: #b91c1c; }

.conflict-list { display: flex; flex-direction: column; gap: 8px; }
.conflict-item { background: var(--bg-card); border: 1px solid var(--border); border-radius: var(--radius); padding: 16px 20px; display: flex; align-items: center; gap: 16px; transition: all 0.15s; }
.conflict-item:hover { border-color: var(--primary); box-shadow: 0 2px 8px rgba(59,130,246,0.1); }
.conflict-item.resolved { opacity: 0.6; border-color: #86efac; background: #f0fdf4; }

.conflict-entity-icon { width: 40px; height: 40px; border-radius: 8px; background: #fee2e2; color: #dc2626; display: flex; align-items: center; justify-content: center; font-size: 18px; font-weight: 700; flex-shrink: 0; }
.conflict-item.resolved .conflict-entity-icon { background: #dcfce7; color: #16a34a; }

.conflict-entity-info { flex: 1; min-width: 0; }
.conflict-entity-title { font-size: 15px; font-weight: 600; color: var(--text); margin-bottom: 2px; }
.conflict-entity-meta { font-size: 12px; color: var(--text-muted); display: flex; gap: 12px; flex-wrap: wrap; }
.conflict-entity-meta code { font-family: var(--font-mono); background: #f1f5f9; padding: 1px 5px; border-radius: 3px; font-size: 11px; }

.conflict-stats { display: flex; gap: 8px; flex-shrink: 0; }
.conflict-stat { padding: 3px 10px; border-radius: 9999px; font-size: 11px; font-weight: 600; }
.conflict-stat-field { background: #fef3c7; color: #92400e; }
.conflict-stat-body { background: #fce7f3; color: #9d174d; }
.conflict-stat-resolved { background: #dcfce7; color: #166534; }

.conflict-resolve-btn { padding: 6px 16px; border: 1px solid var(--primary); background: var(--primary); color: #fff; border-radius: 6px; font-size: 13px; font-weight: 500; cursor: pointer; text-decoration: none; transition: all 0.15s; flex-shrink: 0; }
.conflict-resolve-btn:hover { background: var(--primary-hover); }
.conflict-resolved-label { padding: 6px 16px; border: 1px solid #86efac; background: #dcfce7; color: #166534; border-radius: 6px; font-size: 13px; font-weight: 500; flex-shrink: 0; }

/* Resolution page */
.resolve-header { margin-bottom: 24px; }
.resolve-header h2 { font-size: 20px; font-weight: 700; margin-bottom: 4px; }
.resolve-header p { font-size: 13px; color: var(--text-muted); }

.resolve-section { margin-bottom: 28px; }
.resolve-section-title { font-size: 13px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-muted); margin-bottom: 12px; padding-bottom: 6px; border-bottom: 2px solid var(--border); }

.field-conflict-table { width: 100%; border-collapse: collapse; font-size: 14px; }
.field-conflict-table th { text-align: left; padding: 8px 12px; font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; color: var(--text-muted); border-bottom: 2px solid var(--border); }
.field-conflict-table td { padding: 10px 12px; border-bottom: 1px solid var(--border); vertical-align: middle; }
.field-conflict-table tr:last-child td { border-bottom: none; }

.field-label { font-weight: 600; font-size: 13px; min-width: 100px; }
.field-value { font-size: 13px; font-family: var(--font-mono); word-break: break-word; }
.field-value-empty { color: var(--text-muted); font-style: italic; }

.field-status { display: inline-block; padding: 2px 8px; border-radius: 9999px; font-size: 10px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.03em; }
.field-status-unchanged { background: #f1f5f9; color: #64748b; }
.field-status-auto { background: #dcfce7; color: #166534; }
.field-status-conflict { background: #fee2e2; color: #991b1b; }

.field-radio-group { display: flex; gap: 8px; }
.field-radio-label { display: flex; align-items: center; gap: 6px; padding: 6px 14px; border: 2px solid var(--border); border-radius: 6px; cursor: pointer; font-size: 13px; font-weight: 500; transition: all 0.15s; }
.field-radio-label:hover { border-color: var(--primary); background: var(--primary-light); }
.field-radio-label input[type="radio"] { accent-color: var(--primary); }
.field-radio-label input[type="radio"]:checked + span { color: var(--primary); font-weight: 600; }
.field-radio-label:has(input:checked) { border-color: var(--primary); background: var(--primary-light); }

.field-row-conflict { background: #fffbeb; }

/* Body diff */
.body-diff { background: var(--bg-card); border: 1px solid var(--border); border-radius: var(--radius); overflow: hidden; }
.body-diff-header { padding: 10px 16px; background: #f8fafc; border-bottom: 1px solid var(--border); font-size: 12px; font-weight: 600; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.04em; }
.body-diff-lines { font-family: var(--font-mono); font-size: 13px; line-height: 1.7; overflow-x: auto; }
.body-diff-line { padding: 1px 16px; white-space: pre-wrap; display: flex; gap: 8px; }
.body-diff-line-no { color: var(--text-muted); min-width: 28px; text-align: right; user-select: none; flex-shrink: 0; }
.body-diff-line-content { flex: 1; }
.diff-context { }
.diff-add-ours { background: #dcfce7; color: #166534; }
.diff-add-theirs { background: #dbeafe; color: #1e40af; }
.diff-del-ours { background: #fee2e2; color: #991b1b; text-decoration: line-through; }
.diff-del-theirs { background: #fef3c7; color: #92400e; text-decoration: line-through; }

.diff-legend { display: flex; gap: 16px; font-size: 12px; color: var(--text-muted); margin-top: 8px; }
.diff-legend-item { display: flex; align-items: center; gap: 4px; }
.diff-legend-swatch { width: 14px; height: 14px; border-radius: 3px; }
.diff-legend-swatch.ours-add { background: #dcfce7; border: 1px solid #86efac; }
.diff-legend-swatch.theirs-add { background: #dbeafe; border: 1px solid #93c5fd; }
.diff-legend-swatch.ours-del { background: #fee2e2; border: 1px solid #fecaca; }
.diff-legend-swatch.theirs-del { background: #fef3c7; border: 1px solid #fde68a; }

.hunk-conflict-bar { display: flex; align-items: center; gap: 8px; padding: 6px 16px; background: #fef2f2; border-top: 2px solid #fecaca; border-bottom: 2px solid #fecaca; font-size: 12px; }
.hunk-conflict-bar .hunk-label { font-weight: 600; color: #991b1b; margin-right: auto; }
.hunk-radio-group { display: flex; gap: 6px; }
.hunk-radio-label { display: flex; align-items: center; gap: 4px; padding: 3px 12px; border: 2px solid var(--border); border-radius: 5px; cursor: pointer; font-size: 12px; font-weight: 500; transition: all 0.15s; background: #fff; }
.hunk-radio-label:hover { border-color: var(--primary); background: var(--primary-light); }
.hunk-radio-label:has(input:checked) { border-color: var(--primary); background: var(--primary-light); }
.hunk-radio-label input[type="radio"] { accent-color: var(--primary); }

.resolve-actions { margin-top: 32px; padding-top: 20px; border-top: 2px solid var(--border); display: flex; gap: 12px; align-items: center; }

.empty-state { text-align: center; padding: 60px 20px; }
.empty-state-icon { font-size: 48px; margin-bottom: 16px; }
.empty-state-title { font-size: 18px; font-weight: 600; color: var(--text); margin-bottom: 8px; }
.empty-state-desc { font-size: 14px; color: var(--text-muted); margin-bottom: 24px; }
</style>
{{- end -}}

{{- define "conflicts-page" -}}
<!DOCTYPE html>
<html lang="en">
<head>
<title>{{ .App.Name }} - Sync Conflicts</title>
{{ template "head" . }}
{{ template "conflict-styles" . }}
</head>
<body>
{{ template "sidebar" . }}
<main class="main" id="content">
{{ template "conflicts-content" . }}
</main>
</body>
</html>
{{- end -}}

{{- define "conflicts-content" -}}
<div class="page-header">
  <div>
    <h2>Sync Conflicts</h2>
    <p>Resolve merge conflicts before changes can be pushed</p>
  </div>
  <div style="display:flex;gap:8px;align-items:center;">
    {{ if .Count }}<span style="font-size:13px;color:var(--danger);font-weight:600;">{{ .Count }} unresolved</span>{{ end }}
    <button hx-post="/api/conflicts/load-test" class="btn btn-secondary btn-sm">Load Test Data</button>
  </div>
</div>

{{ if not .Conflicts }}
<div class="card">
  <div class="empty-state">
    <div class="empty-state-icon">&#10003;</div>
    <div class="empty-state-title">No Conflicts</div>
    <div class="empty-state-desc">Everything is in sync. Click "Load Test Data" to test the conflict resolution UI.</div>
  </div>
</div>
{{ else }}

{{ if gt .Count 0 }}
<div class="conflict-banner">
  <span class="conflict-banner-icon">&#9888;</span>
  <div class="conflict-banner-text">
    <strong>{{ .Count }} file{{ if gt .Count 1 }}s{{ end }} with conflicts</strong>
    <span>Resolve each conflict to complete the merge. Other edits can continue while conflicts are pending.</span>
  </div>
</div>
{{ end }}

<div class="conflict-list">
  {{ range .Conflicts }}
  <div class="conflict-item{{ if .Resolved }} resolved{{ end }}">
    <div class="conflict-entity-icon">{{ if .Resolved }}&#10003;{{ else }}!{{ end }}</div>
    <div class="conflict-entity-info">
      <div class="conflict-entity-title">{{ .Title }}</div>
      <div class="conflict-entity-meta">
        <span><code>{{ .EntityType }}</code></span>
        <span>{{ .FilePath }}</span>
      </div>
    </div>
    <div class="conflict-stats">
      {{ if .Resolved }}
        <span class="conflict-stat conflict-stat-resolved">Resolved</span>
      {{ else }}
        {{ if gt .ConflictFields 0 }}<span class="conflict-stat conflict-stat-field">{{ .ConflictFields }} field{{ if gt .ConflictFields 1 }}s{{ end }}</span>{{ end }}
        {{ if .HasBodyConflict }}<span class="conflict-stat conflict-stat-body">body</span>{{ end }}
      {{ end }}
    </div>
    {{ if .Resolved }}
      <span class="conflict-resolved-label">&#10003; Done</span>
    {{ else }}
      <a href="/conflicts/resolve/{{ .ID }}" class="conflict-resolve-btn"
         hx-get="/conflicts/resolve/{{ .ID }}" hx-target="#content" hx-push-url="true">Resolve</a>
    {{ end }}
  </div>
  {{ end }}
</div>

{{ $allResolved := true }}
{{ range .Conflicts }}{{ if not .Resolved }}{{ $allResolved = false }}{{ end }}{{ end }}
{{ if and .Conflicts $allResolved }}
<div style="margin-top:20px;padding:20px;background:#f0fdf4;border:1px solid #86efac;border-radius:var(--radius);text-align:center;">
  <strong style="color:#166534;">All conflicts resolved!</strong>
  <p style="color:#15803d;font-size:13px;margin-top:4px;">Click below to create a merge commit and push the changes.</p>
  <button hx-post="/api/conflicts/resolve-all" class="btn btn-primary" style="margin-top:12px;">Complete Merge</button>
</div>
{{ end }}

{{ end }}
{{- end -}}

{{- define "conflict-resolve-page" -}}
<!DOCTYPE html>
<html lang="en">
<head>
<title>{{ .App.Name }} - Resolve {{ .Conflict.Title }}</title>
{{ template "head" . }}
{{ template "conflict-styles" . }}
</head>
<body>
{{ template "sidebar" . }}
<main class="main" id="content">
{{ template "conflict-resolve-content" . }}
</main>
</body>
</html>
{{- end -}}

{{- define "conflict-resolve-content" -}}
{{ $cf := .Conflict }}
<div class="resolve-header">
  <div style="margin-bottom:12px;">
    <a href="/conflicts" class="btn btn-secondary btn-sm"
       hx-get="/conflicts" hx-target="#content" hx-push-url="true">&larr; Back to Conflicts</a>
  </div>
  <h2>{{ $cf.Title }}</h2>
  <p><code>{{ $cf.EntityType }}</code> &middot; {{ $cf.FilePath }}{{ if not $cf.Base }} &middot; <span class="badge badge-orange">new file on both sides</span>{{ end }}</p>
</div>

<form method="POST" action="/api/conflicts/resolve">
<input type="hidden" name="conflict_id" value="{{ $cf.ID }}">

<!-- Field Conflicts -->
<div class="resolve-section">
  <div class="resolve-section-title">Properties</div>
  <div class="card" style="overflow-x:auto;">
    <table class="field-conflict-table">
      <thead>
        <tr>
          <th>Field</th>
          <th>Base</th>
          <th>Your Change</th>
          <th>Their Change</th>
          <th>Status</th>
          <th>Resolution</th>
        </tr>
      </thead>
      <tbody>
        {{ range $cf.Fields }}
        <tr{{ if eq .Status "conflict" }} class="field-row-conflict"{{ end }}>
          <td class="field-label">{{ if .Label }}{{ .Label }}{{ else }}{{ .Property }}{{ end }}</td>
          <td class="field-value">{{ if .BaseValue }}{{ .BaseValue }}{{ else }}<span class="field-value-empty">&#8212;</span>{{ end }}</td>
          <td class="field-value">{{ if .OurValue }}{{ .OurValue }}{{ else }}<span class="field-value-empty">&#8212;</span>{{ end }}</td>
          <td class="field-value">{{ if .TheirValue }}{{ .TheirValue }}{{ else }}<span class="field-value-empty">&#8212;</span>{{ end }}</td>
          <td>
            {{ if eq .Status "unchanged" }}<span class="field-status field-status-unchanged">unchanged</span>
            {{ else if eq .Status "auto-ours" }}<span class="field-status field-status-auto">auto: yours</span>
            {{ else if eq .Status "auto-theirs" }}<span class="field-status field-status-auto">auto: theirs</span>
            {{ else if eq .Status "conflict" }}<span class="field-status field-status-conflict">conflict</span>
            {{ end }}
          </td>
          <td>
            {{ if eq .Status "conflict" }}
            <div class="field-radio-group">
              <label class="field-radio-label">
                <input type="radio" name="field_{{ .Property }}" value="ours" required>
                <span>Yours</span>
              </label>
              <label class="field-radio-label">
                <input type="radio" name="field_{{ .Property }}" value="theirs">
                <span>Theirs</span>
              </label>
            </div>
            {{ else if eq .Status "auto-ours" }}
            <span style="font-size:12px;color:var(--text-muted);">Taking yours</span>
            {{ else if eq .Status "auto-theirs" }}
            <span style="font-size:12px;color:var(--text-muted);">Taking theirs</span>
            {{ else }}
            <span style="font-size:12px;color:var(--text-muted);">No change</span>
            {{ end }}
          </td>
        </tr>
        {{ end }}
      </tbody>
    </table>
  </div>
</div>

<!-- Body Conflict -->
{{ if $cf.BodyConflict }}
<div class="resolve-section">
  <div class="resolve-section-title">Body Content</div>

  <div class="body-diff">
    <div class="body-diff-header">Three-Way Diff</div>
    <div class="body-diff-lines">
      {{ range $cf.BodyConflict.Hunks }}
        {{ if eq .Source "conflict" }}
        <div class="hunk-conflict-bar">
          <span class="hunk-label">&#9888; Conflict</span>
          <div class="hunk-radio-group">
            <label class="hunk-radio-label">
              <input type="radio" name="hunk_{{ .Index }}" value="ours" required>
              <span>Accept Yours</span>
            </label>
            <label class="hunk-radio-label">
              <input type="radio" name="hunk_{{ .Index }}" value="theirs">
              <span>Accept Theirs</span>
            </label>
          </div>
        </div>
        {{ end }}
        {{ range .Lines }}
        <div class="body-diff-line diff-{{ .Type }}">
          <span class="body-diff-line-no">{{ if ge .LineNo 0 }}{{ .LineNo }}{{ end }}</span>
          <span class="body-diff-line-content">{{ if eq .Type "add-ours" }}+ {{ else if eq .Type "add-theirs" }}+ {{ else if eq .Type "del-ours" }}- {{ else if eq .Type "del-theirs" }}- {{ else }}  {{ end }}{{ .Content }}</span>
        </div>
        {{ end }}
      {{ end }}
    </div>
  </div>

  <div class="diff-legend">
    <div class="diff-legend-item"><div class="diff-legend-swatch ours-add"></div> Added by you</div>
    <div class="diff-legend-item"><div class="diff-legend-swatch theirs-add"></div> Added by them</div>
    <div class="diff-legend-item"><div class="diff-legend-swatch ours-del"></div> Deleted by you</div>
    <div class="diff-legend-item"><div class="diff-legend-swatch theirs-del"></div> Deleted by them</div>
  </div>
</div>
{{ end }}

<div class="resolve-actions">
  <button type="submit" class="btn btn-primary">Save Resolution</button>
  <a href="/conflicts" class="btn btn-secondary"
     hx-get="/conflicts" hx-target="#content" hx-push-url="true">Cancel</a>
</div>

</form>
{{- end -}}
`
