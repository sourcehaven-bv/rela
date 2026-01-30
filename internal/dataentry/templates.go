package dataentry

// allTemplates contains all HTML templates for the data entry application.
// These are parsed at startup and used by all handlers.
const allTemplates = `
{{- define "head" -}}
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<script src="https://unpkg.com/htmx.org@2.0.4"></script>
<link rel="stylesheet" href="https://unpkg.com/easymde@2.18.0/dist/easymde.min.css">
<script src="https://unpkg.com/easymde@2.18.0/dist/easymde.min.js"></script>
<link rel="stylesheet" href="https://unpkg.com/slim-select@2.9.2/dist/slimselect.css">
<script src="https://unpkg.com/slim-select@2.9.2/dist/slimselect.min.js"></script>
<style>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
:root {
  --bg: #f8fafc; --bg-card: #fff; --bg-sidebar: #1e293b; --bg-sidebar-hover: #334155;
  --bg-sidebar-active: #0f172a; --text: #1e293b; --text-muted: #64748b;
  --text-sidebar: #cbd5e1; --text-sidebar-active: #fff; --border: #e2e8f0;
  --primary: #3b82f6; --primary-hover: #2563eb; --primary-light: #eff6ff;
  --danger: #ef4444; --radius: 8px; --font: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  --font-mono: "SF Mono", "Fira Code", monospace;
  --shadow: 0 1px 3px rgba(0,0,0,0.08);
}
body { font-family: var(--font); background: var(--bg); color: var(--text); line-height: 1.6; display: flex; min-height: 100vh; }

.sidebar { width: 240px; background: var(--bg-sidebar); position: fixed; top: 0; left: 0; bottom: 0; overflow-y: auto; z-index: 100; display: flex; flex-direction: column; }
.sidebar-header { padding: 20px 20px 16px; border-bottom: 1px solid rgba(255,255,255,0.1); }
.sidebar-header h1 { font-size: 16px; font-weight: 700; color: #fff; }
.sidebar-header p { font-size: 12px; color: var(--text-sidebar); margin-top: 4px; }
.sidebar nav { padding: 8px 0; flex: 1; }
.sidebar nav a { display: flex; align-items: center; gap: 10px; padding: 8px 20px; color: var(--text-sidebar); text-decoration: none; font-size: 14px; font-weight: 500; transition: all 0.15s; border-left: 3px solid transparent; }
.sidebar nav a:hover { background: var(--bg-sidebar-hover); color: var(--text-sidebar-active); }
.sidebar nav a.active { background: var(--bg-sidebar-active); color: var(--text-sidebar-active); border-left-color: var(--primary); }

.main { margin-left: 240px; flex: 1; padding: 32px; max-width: 1100px; }
.page-header { margin-bottom: 24px; display: flex; align-items: center; justify-content: space-between; }
.page-header h2 { font-size: 22px; font-weight: 700; }
.page-header p { color: var(--text-muted); font-size: 14px; margin-top: 2px; }

.card { background: var(--bg-card); border: 1px solid var(--border); border-radius: var(--radius); box-shadow: var(--shadow); }

.filter-bar { display: flex; gap: 12px; align-items: center; flex-wrap: wrap; margin-bottom: 16px; }
.filter-bar label { font-size: 12px; color: var(--text-muted); font-weight: 500; }
.filter-bar select, .filter-bar input { padding: 6px 10px; border: 1px solid var(--border); border-radius: 6px; font-size: 13px; font-family: var(--font); background: var(--bg-card); min-width: 140px; }

table { width: 100%; border-collapse: collapse; font-size: 14px; }
thead th { text-align: left; padding: 10px 16px; font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; color: var(--text-muted); border-bottom: 2px solid var(--border); white-space: nowrap; }
thead th.sortable { cursor: pointer; }
thead th.sortable:hover { color: var(--text); }
tbody td { padding: 12px 16px; border-bottom: 1px solid var(--border); }
tbody tr:hover { background: var(--primary-light); }
tbody tr:last-child td { border-bottom: none; }
.cell-link { color: var(--primary); text-decoration: none; font-weight: 500; }
.cell-link:hover { text-decoration: underline; }
.edit-icon { color: var(--text-muted); text-decoration: none; font-size: 14px; opacity: 0.6; transition: opacity 0.15s; }
.edit-icon:hover { opacity: 1; color: var(--primary); }
.add-dropdown { position: relative; }
.add-dropdown summary { list-style: none; cursor: pointer; }
.add-dropdown summary::-webkit-details-marker { display: none; }
.add-dropdown-menu { position: absolute; right: 0; top: 100%; margin-top: 4px; background: var(--bg-card); border: 1px solid var(--border); border-radius: 6px; box-shadow: 0 4px 12px rgba(0,0,0,0.1); z-index: 10; min-width: 160px; padding: 4px 0; }
.add-dropdown-menu a { display: block; padding: 8px 16px; font-size: 13px; color: var(--text); text-decoration: none; }
.add-dropdown-menu a:hover { background: var(--primary-light); color: var(--primary); }

.badge { display: inline-block; padding: 2px 8px; border-radius: 9999px; font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.03em; }
.badge-blue { background: #dbeafe; color: #1e40af; }
.badge-purple { background: #e9d5ff; color: #6b21a8; }
.badge-green { background: #dcfce7; color: #166534; }
.badge-gray { background: #f1f5f9; color: #475569; }
.badge-red { background: #fee2e2; color: #991b1b; }
.badge-orange { background: #fed7aa; color: #9a3412; }
.badge-yellow { background: #fef9c3; color: #854d0e; }

.info-bar { display: flex; gap: 6px; flex-wrap: wrap; margin-bottom: 12px; }
.info-chip { display: inline-flex; align-items: center; gap: 4px; padding: 3px 10px; background: var(--primary-light); color: var(--primary); border-radius: 9999px; font-size: 12px; font-weight: 500; }

.btn { padding: 8px 20px; border: 1px solid var(--border); border-radius: 6px; font-size: 14px; font-weight: 500; cursor: pointer; font-family: var(--font); transition: all 0.15s; text-decoration: none; display: inline-flex; align-items: center; gap: 6px; }
.btn-primary { background: var(--primary); color: #fff; border-color: var(--primary); }
.btn-primary:hover { background: var(--primary-hover); }
.btn-secondary { background: var(--bg-card); color: var(--text); }
.btn-secondary:hover { background: var(--bg); }
.btn-sm { padding: 5px 12px; font-size: 13px; }
.btn-danger { background: #fff; color: var(--danger); border-color: var(--danger); }
.btn-danger:hover { background: #fef2f2; }

.form-card { padding: 28px; max-width: 640px; }
.form-desc { color: var(--text-muted); font-size: 13px; margin-bottom: 24px; }
.form-group { margin-bottom: 20px; }
.form-group label { display: block; font-size: 13px; font-weight: 600; margin-bottom: 6px; }
.form-group .required { color: var(--danger); margin-left: 2px; }
.form-group input[type="text"], .form-group input[type="date"], .form-group input[type="number"],
.form-group textarea, .form-group select { width: 100%; padding: 8px 12px; border: 1px solid var(--border); border-radius: 6px; font-size: 14px; font-family: var(--font); background: var(--bg-card); color: var(--text); transition: border-color 0.15s, box-shadow 0.15s; }
.form-group input:focus, .form-group textarea:focus, .form-group select:focus { outline: none; border-color: var(--primary); box-shadow: 0 0 0 3px var(--primary-light); }
.form-group textarea { min-height: 100px; resize: vertical; }
.form-group .help-text { font-size: 12px; color: var(--text-muted); margin-top: 4px; }
.form-group .field-meta { font-size: 11px; color: var(--text-muted); margin-top: 2px; font-family: var(--font-mono); }
.form-row-checkbox { display: flex; align-items: center; gap: 8px; }
.form-row-checkbox input[type="checkbox"] { width: 16px; height: 16px; accent-color: var(--primary); }
.form-row-checkbox label { margin-bottom: 0; font-weight: 500; }
.form-section-label { font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-muted); margin: 28px 0 16px; padding-top: 20px; border-top: 1px solid var(--border); }
.form-actions { margin-top: 28px; padding-top: 20px; border-top: 1px solid var(--border); display: flex; gap: 12px; }

.transitions-info { margin-top: 6px; padding: 8px 12px; background: #f8fafc; border-radius: 6px; border: 1px solid var(--border); }
.transitions-info .t-title { font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; color: var(--text-muted); margin-bottom: 4px; }
.transitions-info .t-row { font-size: 12px; font-family: var(--font-mono); color: var(--text-muted); line-height: 1.8; }
.t-arrow { color: var(--primary); margin: 0 4px; }

.detail-grid { display: grid; grid-template-columns: 140px 1fr; gap: 8px 16px; font-size: 14px; }
.detail-label { color: var(--text-muted); font-weight: 500; font-size: 13px; }
.detail-value { font-weight: 400; }

.detail-section { margin-top: 24px; }
.detail-section h3 { font-size: 14px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; color: var(--text-muted); margin-bottom: 12px; }

.rel-list { list-style: none; }
.rel-list li { padding: 6px 0; border-bottom: 1px solid var(--border); font-size: 14px; display: flex; gap: 8px; align-items: center; }
.rel-list li:last-child { border-bottom: none; }
.rel-type { font-size: 11px; font-family: var(--font-mono); color: var(--text-muted); background: #f1f5f9; padding: 1px 6px; border-radius: 3px; }

.pagination { display: flex; align-items: center; justify-content: space-between; padding: 12px 16px; border-top: 1px solid var(--border); font-size: 13px; color: var(--text-muted); }

.toast { position: fixed; top: 16px; right: 16px; padding: 12px 20px; background: #166534; color: #fff; border-radius: 8px; font-size: 14px; font-weight: 500; z-index: 999; box-shadow: 0 4px 12px rgba(0,0,0,0.15); animation: toastIn 0.3s; }
@keyframes toastIn { from { opacity: 0; transform: translateY(-8px); } to { opacity: 1; transform: translateY(0); } }

.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.4); z-index: 200; display: flex; align-items: center; justify-content: center; animation: fadeIn 0.15s; }
.modal { background: var(--bg-card); border-radius: 12px; box-shadow: 0 8px 32px rgba(0,0,0,0.2); width: 480px; max-width: 90vw; max-height: 80vh; overflow-y: auto; }
.modal-header { padding: 16px 20px; border-bottom: 1px solid var(--border); display: flex; align-items: center; justify-content: space-between; }
.modal-header h3 { font-size: 16px; font-weight: 600; }
.modal-close { background: none; border: none; font-size: 20px; cursor: pointer; color: var(--text-muted); padding: 4px 8px; border-radius: 4px; }
.modal-close:hover { background: var(--bg); color: var(--text); }
.modal-body { padding: 20px; }
.modal-body .form-group { margin-bottom: 16px; }
.modal-footer { padding: 12px 20px; border-top: 1px solid var(--border); display: flex; gap: 8px; justify-content: flex-end; }

.rel-row { display: flex; gap: 8px; align-items: flex-start; margin-bottom: 8px; }
.rel-row .rel-select-wrap { flex: 1; }
.rel-row .rel-props { display: flex; gap: 6px; align-items: center; }
.rel-row .rel-props input { width: 120px; padding: 6px 8px; border: 1px solid var(--border); border-radius: 6px; font-size: 13px; }
.btn-icon { width: 34px; height: 34px; padding: 0; display: inline-flex; align-items: center; justify-content: center; border: 1px solid var(--border); border-radius: 6px; background: var(--bg-card); cursor: pointer; font-size: 18px; color: var(--primary); transition: all 0.15s; flex-shrink: 0; }
.btn-icon:hover { background: var(--primary-light); border-color: var(--primary); }

.EasyMDEContainer { border: 1px solid var(--border); border-radius: 6px; }
.EasyMDEContainer .CodeMirror { border: none; border-radius: 0 0 6px 6px; font-family: var(--font-mono); font-size: 14px; }
.EasyMDEContainer .editor-toolbar { border-bottom: 1px solid var(--border); border-radius: 6px 6px 0 0; }

.view-section-heading { font-size: 15px; font-weight: 700; color: var(--text); margin: 0 0 10px; padding-bottom: 6px; border-bottom: 2px solid var(--border); }
.view-content-entity .markdown-body { font-size: 14px; line-height: 1.7; color: var(--text); }
.markdown-body h3 { font-size: 15px; font-weight: 600; margin: 16px 0 6px; }
.markdown-body h4 { font-size: 14px; font-weight: 600; margin: 14px 0 4px; }
.markdown-body h5 { font-size: 13px; font-weight: 600; margin: 12px 0 4px; }
.markdown-body p { margin: 8px 0; }
.markdown-body ul, .markdown-body ol { margin: 8px 0; padding-left: 24px; }
.markdown-body li { margin: 2px 0; }
.markdown-body pre { background: #f1f5f9; padding: 12px; border-radius: 6px; overflow-x: auto; font-family: var(--font-mono); font-size: 13px; margin: 8px 0; }
.markdown-body code { background: #f1f5f9; padding: 1px 4px; border-radius: 3px; font-family: var(--font-mono); font-size: 0.9em; }
.markdown-body pre code { background: none; padding: 0; }
.markdown-body strong { font-weight: 600; }
.markdown-body em { font-style: italic; }

/* SlimSelect theme overrides */
:root {
  --ss-primary-color: var(--primary);
  --ss-bg-color: var(--bg-card);
  --ss-font-color: var(--text);
  --ss-font-placeholder-color: var(--text-muted);
  --ss-border-color: var(--border);
  --ss-highlight-color: var(--primary-light);
  --ss-border-radius: 6px;
  --ss-spacing-s: 4px;
  --ss-spacing-m: 8px;
  --ss-spacing-l: 12px;
  --ss-animation-timing: 0.15s;
  --ss-font-size: 14px;
}
.ss-main { font-family: var(--font); min-height: 38px; }
.ss-main:focus { box-shadow: 0 0 0 3px var(--primary-light); }
.ss-main .ss-values .ss-value { background-color: var(--primary); }
.ss-content { font-family: var(--font); }
.filter-bar .ss-main { min-height: 32px; --ss-font-size: 13px; min-width: 140px; }

html { scroll-behavior: smooth; }
thead th.sortable { user-select: none; }
.sort-indicator { font-size: 10px; margin-left: 2px; opacity: 0.7; }
.jump-bar { display: flex; gap: 6px; flex-wrap: wrap; margin-bottom: 20px; padding: 8px 12px; background: var(--bg-card); border: 1px solid var(--border); border-radius: var(--radius); }
.jump-link { font-size: 13px; color: var(--primary); text-decoration: none; padding: 2px 10px; border-radius: 9999px; transition: all 0.15s; }
.jump-link:hover { background: var(--primary-light); }
.nav-count { margin-left: auto; font-size: 11px; color: rgba(255,255,255,0.4); font-weight: 400; }

</style>
<script>
// SlimSelect progressive enhancement
function enhanceSelects(root) {
  if (typeof SlimSelect === 'undefined') return;
  (root || document).querySelectorAll('select:not([data-ssid])').forEach(function(sel) {
    var settings = {
      select: sel,
      settings: {
        showSearch: sel.options.length > 6,
        allowDeselect: !sel.required && !sel.multiple,
        placeholderText: '',
        searchHighlight: true,
        closeOnSelect: !sel.multiple
      }
    };
    try {
      var instance = new SlimSelect(settings);
      sel._slimSelect = instance;
    } catch(e) { /* skip if SlimSelect fails on this element */ }
  });
}
document.addEventListener('DOMContentLoaded', function() {
  enhanceSelects();
  var params = new URLSearchParams(window.location.search);
  var toast = params.get('_toast');
  if (toast) {
    var div = document.createElement('div');
    div.className = 'toast';
    div.textContent = toast;
    document.body.appendChild(div);
    setTimeout(function() {
      div.style.opacity = '0';
      div.style.transition = 'opacity 0.3s';
      setTimeout(function() { div.remove(); }, 300);
    }, 2700);
    params.delete('_toast');
    var clean = window.location.pathname;
    var remaining = params.toString();
    if (remaining) clean += '?' + remaining;
    if (window.location.hash) clean += window.location.hash;
    history.replaceState(null, '', clean);
  }
});
document.addEventListener('htmx:afterSettle', function(evt) { enhanceSelects(evt.detail.target); });
</script>
{{- end -}}

{{- define "sidebar" -}}
<aside class="sidebar">
  <div class="sidebar-header">
    <h1>{{ .App.Name }}</h1>
    {{ if .App.Description }}<p>{{ .App.Description }}</p>{{ end }}
  </div>
  <nav>
    {{ range .Navigation }}
    <a href="/list/{{ .List }}"{{ if eq .List $.ActiveList }} class="active"{{ end }}
       data-entity-type="{{ .EntityType }}"
       hx-get="/list/{{ .List }}" hx-target="#content" hx-push-url="true">
      {{ .Label }}<span class="nav-count">{{ .Count }}</span>
    </a>
    {{ end }}
  </nav>
</aside>
<script>
document.body.addEventListener('htmx:pushedIntoHistory', function() {
  var path = window.location.pathname;
  var links = document.querySelectorAll('.sidebar nav a');
  var matched = false;
  // Direct match: /list/{listID}
  links.forEach(function(a) {
    var href = a.getAttribute('href');
    if (path === href || path.startsWith(href + '?')) matched = true;
  });
  if (matched) {
    links.forEach(function(a) {
      var href = a.getAttribute('href');
      a.classList.toggle('active', path === href || path.startsWith(href + '?'));
    });
    return;
  }
  // Explicit source list: ?from={listID}
  var params = new URLSearchParams(window.location.search);
  var fromList = params.get('from');
  if (fromList) {
    links.forEach(function(a) {
      a.classList.toggle('active', a.getAttribute('href') === '/list/' + fromList);
    });
    return;
  }
  // Entity type match: /entity/{type}/{id}
  var m = path.match(/^\/entity\/([^/]+)\//);
  if (m) {
    var etype = m[1];
    links.forEach(function(a) {
      a.classList.toggle('active', a.getAttribute('data-entity-type') === etype);
    });
  }
});
</script>
{{- end -}}

{{- define "page" -}}
<!DOCTYPE html>
<html lang="en">
<head>
<title>{{ .App.Name }} - {{ .List.Title }}</title>
{{ template "head" . }}
</head>
<body>
{{ template "sidebar" . }}
<main class="main" id="content">
{{ template "list-content" . }}
</main>
</body>
</html>
{{- end -}}

{{- define "list-content" -}}
<div class="page-header">
  <div>
    <h2>{{ .List.Title }}</h2>
    {{ if .List.Description }}<p>{{ .List.Description }}</p>{{ end }}
  </div>
  <div style="display:flex;gap:8px;align-items:center;">
    <span style="font-size:13px;color:var(--text-muted);">{{ .TotalCount }} items</span>
    {{ if .List.CreateForm }}
    <a href="/form/{{ .List.CreateForm }}" class="btn btn-primary btn-sm"
       hx-get="/form/{{ .List.CreateForm }}" hx-target="#content" hx-push-url="true">+ New</a>
    {{ end }}
  </div>
</div>

{{ if .FilterControls }}
<div class="filter-bar">
  {{ range .FilterControls }}
  <div>
    <label>{{ .Label }}</label><br>
    {{ if or (eq .Widget "select") (eq .Widget "multi-select") }}
    <select name="filter_{{ .Property }}"
            hx-get="/list/{{ $.ListID }}" hx-target="#content" hx-push-url="true"
            hx-include=".filter-bar select, .filter-bar input">
      <option value="">All</option>
      {{ $current := .Current }}
      {{ range .Values }}<option value="{{ . }}"{{ if eq . $current }} selected{{ end }}>{{ . }}</option>{{ end }}
    </select>
    {{ else }}
    <input type="text" placeholder="Search..." name="filter_{{ .Property }}"
           hx-get="/list/{{ $.ListID }}" hx-target="#content" hx-push-url="true"
           hx-trigger="keyup changed delay:300ms"
           hx-include=".filter-bar select, .filter-bar input">
    {{ end }}
  </div>
  {{ end }}
</div>
{{ end }}

{{ if .List.Filters }}
<div class="info-bar">
  {{ range .List.Filters }}
  <span class="info-chip">{{ .Property }} {{ .Operator }} {{ .Value }}</span>
  {{ end }}
</div>
{{ end }}

<div class="card">
  <div style="overflow-x:auto;">
    <table>
      <thead>
        <tr>
          {{ range .Columns }}<th{{ if .Sortable }} class="sortable" hx-get="{{ .SortURL }}" hx-target="#content" hx-push-url="true" hx-include=".filter-bar select, .filter-bar input"{{ end }}>{{ if .Label }}{{ .Label }}{{ else }}{{ .Property }}{{ end }}{{ if .IsSorted }}<span class="sort-indicator">{{ if eq .SortDir "desc" }}&#9660;{{ else }}&#9650;{{ end }}</span>{{ end }}</th>{{ end }}
        </tr>
      </thead>
      <tbody>
        {{ range .Rows }}
        <tr>
          {{ $dlp := $.DetailLinkPrefix }}
          {{ range .Cells }}
          <td>
            {{ if .Link }}<a href="{{ $dlp }}{{ .EntityID }}?from={{ $.ListID }}" class="cell-link"
               hx-get="{{ $dlp }}{{ .EntityID }}?from={{ $.ListID }}" hx-target="#content" hx-push-url="true">{{ .Value }}</a>
            {{ else if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>
            {{ else }}{{ if .Value }}{{ .Value }}{{ else }}&mdash;{{ end }}{{ end }}
          </td>
          {{ end }}
        </tr>
        {{ end }}
        {{ if not .Rows }}
        <tr><td colspan="{{ len .Columns }}" style="text-align:center;padding:32px;color:var(--text-muted);">No items found</td></tr>
        {{ end }}
      </tbody>
    </table>
  </div>
  <div class="pagination">
    <span>{{ .TotalCount }} items{{ if .HasPagination }} &middot; Page {{ .Page }} of {{ .TotalPages }}{{ end }}</span>
    {{ if .HasPagination }}
    <div style="display:flex;gap:6px;">
      {{ if .PrevPageURL }}<a href="{{ .PrevPageURL }}" class="btn btn-secondary btn-sm"
         hx-get="{{ .PrevPageURL }}" hx-target="#content" hx-push-url="true"
         hx-include=".filter-bar select, .filter-bar input">&larr; Prev</a>
      {{ else }}<span class="btn btn-secondary btn-sm" style="opacity:0.4;pointer-events:none;">&larr; Prev</span>{{ end }}
      {{ if .NextPageURL }}<a href="{{ .NextPageURL }}" class="btn btn-secondary btn-sm"
         hx-get="{{ .NextPageURL }}" hx-target="#content" hx-push-url="true"
         hx-include=".filter-bar select, .filter-bar input">Next &rarr;</a>
      {{ else }}<span class="btn btn-secondary btn-sm" style="opacity:0.4;pointer-events:none;">Next &rarr;</span>{{ end }}
    </div>
    {{ end }}
  </div>
</div>
{{- end -}}

{{- define "form-page" -}}
<!DOCTYPE html>
<html lang="en">
<head>
<title>{{ .App.Name }} - {{ .Form.Title }}</title>
{{ template "head" . }}
</head>
<body>
{{ template "sidebar" . }}
<main class="main" id="content">
{{ template "form-content" . }}
</main>
</body>
</html>
{{- end -}}

{{- define "form-content" -}}
<div class="page-header">
  <div>
    <h2>{{ .Form.Title }}{{ if .EntityID }} — {{ .EntityID }}{{ end }}</h2>
    {{ if .Form.Description }}<p>{{ .Form.Description }}</p>{{ end }}
  </div>
  <a href="javascript:history.back()" class="btn btn-secondary btn-sm">&larr; Back</a>
</div>

<div class="card form-card">
  <form {{ if eq .Mode "edit" }}hx-post="/api/update"{{ else }}hx-post="/api/create"{{ end }}
        hx-swap="none">
    <input type="hidden" name="_form_id" value="{{ .FormID }}">
    <input type="hidden" name="_entity_id" value="{{ .EntityID }}">
    {{ if .ReturnTo }}<input type="hidden" name="_return_to" value="{{ .ReturnTo }}">{{ end }}
    {{ if .LinkRelation }}<input type="hidden" name="_link_relation" value="{{ .LinkRelation }}">
    <input type="hidden" name="_link_peer" value="{{ .LinkPeer }}">
    <input type="hidden" name="_link_as" value="{{ .LinkAs }}">{{ end }}

    {{ range .Fields }}
    {{ if .Hidden }}
    <input type="hidden" name="{{ .Property }}" value="{{ .Value }}">
    {{ else if eq .Widget "checkbox" }}
    <div class="form-group">
      <div class="form-row-checkbox">
        <input type="checkbox" name="{{ .Property }}" value="true" id="f-{{ .Property }}"{{ if eq .Value "true" }} checked{{ end }}>
        <label for="f-{{ .Property }}">{{ .Label }}</label>
      </div>
      {{ if .Help }}<p class="help-text">{{ .Help }}</p>{{ end }}
    </div>
    {{ else if eq .Widget "textarea" }}
    <div class="form-group">
      <label for="f-{{ .Property }}">{{ .Label }}{{ if .Required }}<span class="required">*</span>{{ end }}</label>
      <textarea name="{{ .Property }}" id="f-{{ .Property }}" placeholder="{{ .Placeholder }}"{{ if .Required }} required{{ end }}>{{ .Value }}</textarea>
      {{ if .Help }}<p class="help-text">{{ .Help }}</p>{{ end }}
    </div>
    {{ else if or (eq .Widget "select") (eq .Widget "multi-select") }}
    <div class="form-group">
      <label for="f-{{ .Property }}">{{ .Label }}{{ if .Required }}<span class="required">*</span>{{ end }}</label>
      <select name="{{ .Property }}" id="f-{{ .Property }}"{{ if eq .Widget "multi-select" }} multiple{{ end }}{{ if .Required }} required{{ end }}>
        {{ if ne .Widget "multi-select" }}<option value="">Select...</option>{{ end }}
        {{ $val := .Value }}
        {{ range .Values }}<option value="{{ . }}"{{ if eq . $val }} selected{{ end }}>{{ . }}</option>{{ end }}
      </select>
      {{ if .Help }}<p class="help-text">{{ .Help }}</p>{{ end }}
      {{ if .Transitions }}
      <div class="transitions-info">
        <p class="t-title">Allowed transitions</p>
        {{ range $from, $tos := .Transitions }}
        <div class="t-row">{{ $from }} <span class="t-arrow">&rarr;</span> {{ join $tos ", " }}</div>
        {{ end }}
      </div>
      {{ end }}
    </div>
    {{ else }}
    <div class="form-group">
      <label for="f-{{ .Property }}">{{ .Label }}{{ if .Required }}<span class="required">*</span>{{ end }}</label>
      <input type="{{ .InputType }}" name="{{ .Property }}" id="f-{{ .Property }}"
             placeholder="{{ .Placeholder }}" value="{{ .Value }}"{{ if .Required }} required{{ end }}>
      {{ if .Help }}<p class="help-text">{{ .Help }}</p>{{ end }}
    </div>
    {{ end }}
    {{ end }}

    {{ if .ShowBody }}
    <p class="form-section-label">Content</p>
    <div class="form-group">
      <label for="body-editor">Body (Markdown)</label>
      <textarea name="_body" id="body-editor">{{ .Body }}</textarea>
    </div>
    {{ end }}

    {{ if .Relations }}
    <p class="form-section-label">Relations</p>
    {{ range .Relations }}
    <div class="form-group">
      <div style="display:flex;align-items:center;gap:8px;margin-bottom:6px;">
        <label for="r-{{ .Relation }}" style="margin-bottom:0;">{{ .Label }}{{ if .Required }}<span class="required">*</span>{{ end }}</label>
        {{ if .AllowCreate }}
        <button type="button" class="btn-icon" onclick="openInlineCreate('{{ .CreateForm }}', '{{ .Relation }}', '{{ .TargetLabel }}')" title="Add new {{ .TargetLabel }}">+</button>
        {{ end }}
      </div>
      {{ if eq .Widget "multi-select" }}
      <select name="{{ .Relation }}" id="r-{{ .Relation }}" multiple{{ if .Required }} required{{ end }}>
        {{ $selected := .Selected }}
        {{ range .Options }}<option value="{{ .ID }}"{{ if contains $selected .ID }} selected{{ end }}>{{ .Title }}</option>{{ end }}
      </select>
      {{ else if eq .Widget "search" }}
      <input type="text" list="dl-{{ .Relation }}" name="{{ .Relation }}" placeholder="Search {{ .TargetLabel }}...">
      <datalist id="dl-{{ .Relation }}">
        {{ range .Options }}<option value="{{ .ID }}">{{ .Title }}</option>{{ end }}
      </datalist>
      {{ else }}
      <select name="{{ .Relation }}" id="r-{{ .Relation }}"{{ if .Required }} required{{ end }}>
        <option value="">Select {{ .TargetLabel }}...</option>
        {{ $selected := .Selected }}
        {{ range .Options }}<option value="{{ .ID }}"{{ if contains $selected .ID }} selected{{ end }}>{{ .Title }}</option>{{ end }}
      </select>
      {{ end }}
      {{ if .Properties }}
      <div class="rel-props-section" style="margin-top:8px;">
        <p style="font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:0.04em;color:var(--text-muted);margin-bottom:6px;">Relation Properties</p>
        {{ $rel := . }}
        {{ range .Properties }}
        {{ $rp := . }}
        <div style="display:flex;gap:8px;align-items:center;margin-bottom:4px;">
          <label style="font-size:12px;color:var(--text-muted);min-width:80px;margin-bottom:0;">{{ if .Label }}{{ .Label }}{{ else }}{{ .Property }}{{ end }}</label>
          <input type="text" name="_relprop_{{ $rel.Relation }}__{{ $rp.Property }}" placeholder="{{ .Property }}"
                 style="flex:1;padding:5px 8px;border:1px solid var(--border);border-radius:4px;font-size:13px;"
                 data-relprop-relation="{{ $rel.Relation }}" data-relprop-property="{{ $rp.Property }}">
        </div>
        {{ end }}
      </div>
      {{ end }}
      <p class="field-meta">{{ .Relation }} &rarr; {{ .TargetType }}</p>
    </div>
    {{ end }}
    {{ end }}

    <div class="form-actions">
      {{ if eq .Mode "edit" }}
      <button type="submit" class="btn btn-primary">Save Changes</button>
      <button type="button" class="btn btn-danger"
              hx-post="/api/delete" hx-vals='{"_entity_id":"{{ .EntityID }}"}'
              hx-confirm="Delete {{ .EntityID }}? This cannot be undone."
              hx-swap="none">Delete</button>
      {{ else }}
      <button type="submit" class="btn btn-primary">Create</button>
      {{ end }}
      <a href="javascript:history.back()" class="btn btn-secondary">Cancel</a>
    </div>
  </form>
</div>

<div id="inline-create-modal" class="modal-overlay" style="display:none;" onclick="if(event.target===this)closeInlineCreate()">
  <div class="modal">
    <div class="modal-header">
      <h3 id="inline-create-title">Add New</h3>
      <button class="modal-close" onclick="closeInlineCreate()">&times;</button>
    </div>
    <div class="modal-body" id="inline-create-body">
      <p style="color:var(--text-muted);">Loading...</p>
    </div>
    <div class="modal-footer">
      <button class="btn btn-secondary btn-sm" onclick="closeInlineCreate()">Cancel</button>
      <button class="btn btn-primary btn-sm" onclick="submitInlineCreate()">Create</button>
    </div>
  </div>
</div>

<script>
// EasyMDE initialization
(function() {
  var el = document.getElementById('body-editor');
  if (el) {
    new EasyMDE({
      element: el,
      spellChecker: false,
      status: false,
      minHeight: '200px',
      toolbar: ['bold', 'italic', 'heading', '|', 'unordered-list', 'ordered-list', '|', 'link', 'image', '|', 'preview', 'side-by-side', '|', 'guide'],
      sideBySideFullscreen: false,
    });
  }
})();

// Inline create modal
var _inlineRelation = '';
var _inlineFormID = '';

function openInlineCreate(formID, relation, targetLabel) {
  _inlineFormID = formID;
  _inlineRelation = relation;
  document.getElementById('inline-create-title').textContent = 'Add New ' + targetLabel;
  fetch('/api/inline-form/' + formID)
    .then(function(r) { return r.text(); })
    .then(function(html) {
      document.getElementById('inline-create-body').innerHTML = html;
    })
    .catch(function() {
      document.getElementById('inline-create-body').innerHTML = '<p style="color:var(--danger);">Failed to load form.</p>';
    });
  document.getElementById('inline-create-modal').style.display = 'flex';
}

function closeInlineCreate() {
  document.getElementById('inline-create-modal').style.display = 'none';
  document.getElementById('inline-create-body').innerHTML = '';
}

function submitInlineCreate() {
  var body = document.getElementById('inline-create-body');
  var inputs = body.querySelectorAll('input, textarea, select');
  var formData = new FormData();
  formData.append('_form_id', _inlineFormID);
  inputs.forEach(function(inp) {
    if (inp.name) {
      if (inp.type === 'checkbox') {
        if (inp.checked) formData.append(inp.name, inp.value);
      } else {
        formData.append(inp.name, inp.value);
      }
    }
  });

  fetch('/api/inline-create', { method: 'POST', body: formData })
    .then(function(r) { return r.json(); })
    .then(function(data) {
      if (data.error) { alert('Error: ' + data.error); return; }
      var sel = document.getElementById('r-' + _inlineRelation);
      if (sel) {
        var opt = document.createElement('option');
        opt.value = data.id;
        opt.textContent = data.title;
        opt.selected = true;
        sel.appendChild(opt);
        // Refresh SlimSelect instance if present
        if (sel._slimSelect) {
          sel._slimSelect.destroy();
          var wrap = sel.closest('.rel-select-wrap') || sel.parentNode;
          enhanceSelects(wrap);
        }
      }
      closeInlineCreate();
    })
    .catch(function(e) { alert('Error creating: ' + e); });
}
</script>
{{- end -}}

{{- define "entity-page" -}}
<!DOCTYPE html>
<html lang="en">
<head>
<title>{{ .App.Name }} - {{ .Entity.ID }}</title>
{{ template "head" . }}
</head>
<body>
{{ template "sidebar" . }}
<main class="main" id="content">
{{ template "entity-content" . }}
</main>
</body>
</html>
{{- end -}}

{{- define "entity-content" -}}
<div class="page-header">
  <div>
    <h2>{{ .Entity.Title }}{{ if not .Entity.Title }}{{ .Entity.ID }}{{ end }}</h2>
    <p style="font-family:var(--font-mono);font-size:13px;color:var(--text-muted);">{{ .Entity.ID }} &middot; {{ .Entity.Type }}</p>
  </div>
  <div style="display:flex;gap:8px;">
    {{ if .EditFormID }}
    <a href="/form/{{ .EditFormID }}/{{ .Entity.ID }}" class="btn btn-primary btn-sm"
       hx-get="/form/{{ .EditFormID }}/{{ .Entity.ID }}" hx-target="#content" hx-push-url="true">Edit</a>
    {{ end }}
    <a href="javascript:history.back()" class="btn btn-secondary btn-sm">&larr; Back</a>
  </div>
</div>

<div class="jump-bar">
  <a href="#properties" class="jump-link">Properties</a>
  {{ if .Relations }}<a href="#relations" class="jump-link">Relations ({{ len .Relations }})</a>{{ end }}
  {{ if .Entity.Content }}<a href="#content" class="jump-link">Content</a>{{ end }}
</div>

<div class="card" style="padding:24px;">
  <div id="properties" class="detail-grid">
    {{ $propTypes := .PropTypes }}
    {{ range $key, $val := .Entity.Properties }}
    {{ $ptype := index $propTypes $key }}
    <div class="detail-label">{{ $key }}</div>
    <div class="detail-value">
      {{ if isBadgeType $ptype }}<span class="badge {{ badgeClass $ptype (printf "%v" $val) }}">{{ $val }}</span>
      {{ else }}{{ if $val }}{{ $val }}{{ else }}&mdash;{{ end }}{{ end }}
    </div>
    {{ end }}
  </div>

  {{ if .Relations }}
  <div id="relations" class="detail-section">
    <h3>Relations</h3>
    <ul class="rel-list">
      {{ range .Relations }}
      <li>
        <span class="rel-type">{{ .Direction }} {{ .Type }}</span>
        <a href="/entity/{{ .TargetType }}/{{ .TargetID }}" class="cell-link"
           hx-get="/entity/{{ .TargetType }}/{{ .TargetID }}" hx-target="#content" hx-push-url="true">{{ .TargetTitle }}</a>
        {{ range .Properties }}
        <span style="font-size:11px;color:var(--text-muted);background:#f1f5f9;padding:1px 6px;border-radius:3px;">{{ .Key }}: {{ .Value }}</span>
        {{ end }}
      </li>
      {{ end }}
    </ul>
  </div>
  {{ end }}

  {{ if .Entity.Content }}
  <div id="entity-content" class="detail-section">
    <h3>Content</h3>
    <div class="markdown-body" style="padding:12px;background:#f8fafc;border-radius:6px;font-size:14px;">{{ renderMarkdown .Entity.Content }}</div>
  </div>
  {{ end }}
</div>
{{- end -}}

{{- define "view-page" -}}
<!DOCTYPE html>
<html lang="en">
<head>
<title>{{ .App.Name }} - {{ .View.Title }}: {{ .EntryTitle }}</title>
{{ template "head" . }}
</head>
<body>
{{ template "sidebar" . }}
<main class="main" id="content">
{{ template "view-content" . }}
</main>
</body>
</html>
{{- end -}}

{{- define "view-content" -}}
<div class="page-header">
  <div>
    <h2>{{ .EntryTitle }}</h2>
    <p style="font-family:var(--font-mono);font-size:13px;color:var(--text-muted);">{{ .Entry.ID }} &middot; {{ .Entry.Type }} &middot; {{ .View.Title }}</p>
  </div>
  <div style="display:flex;gap:8px;">
    {{ if .EditFormID }}
    <a href="/form/{{ .EditFormID }}/{{ .Entry.ID }}?return_to={{ urlquery .ReturnTo }}" class="btn btn-primary btn-sm"
       hx-get="/form/{{ .EditFormID }}/{{ .Entry.ID }}?return_to={{ urlquery .ReturnTo }}" hx-target="#content" hx-push-url="true">Edit</a>
    {{ end }}
    <a href="javascript:history.back()" class="btn btn-secondary btn-sm">&larr; Back</a>
  </div>
</div>

{{ if .Sections }}
<div class="jump-bar">
  {{ range .Sections }}{{ if .Heading }}<a href="#{{ .SectionID }}" class="jump-link">{{ .Heading }}</a>{{ end }}{{ end }}
</div>
{{ end }}

{{ range .Sections }}
<div class="view-section" id="{{ .SectionID }}" style="margin-bottom:24px;">

  {{ if .Heading }}
  <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:8px;">
    <h3 class="view-section-heading" style="margin-bottom:0;">{{ .Heading }}</h3>
    {{ if .AddInfo }}
    {{ $info := .AddInfo }}
    {{ $returnTo := printf "%s#%s" $.ReturnTo .SectionID }}
    {{ if eq (len $info.Targets) 1 }}
    {{ $t := index $info.Targets 0 }}
    <a href="/form/{{ $t.FormID }}?return_to={{ urlquery $returnTo }}&link_relation={{ $info.Relation }}&link_peer={{ $info.PeerID }}&link_as={{ $info.LinkAs }}"
       class="btn btn-secondary btn-sm"
       hx-get="/form/{{ $t.FormID }}?return_to={{ urlquery $returnTo }}&link_relation={{ $info.Relation }}&link_peer={{ $info.PeerID }}&link_as={{ $info.LinkAs }}"
       hx-target="#content" hx-push-url="true">+ Add {{ $t.Label }}</a>
    {{ else }}
    <details class="add-dropdown">
      <summary class="btn btn-secondary btn-sm">+ Add&hellip;</summary>
      <div class="add-dropdown-menu">
        {{ range $info.Targets }}
        <a href="/form/{{ .FormID }}?return_to={{ urlquery $returnTo }}&link_relation={{ $info.Relation }}&link_peer={{ $info.PeerID }}&link_as={{ $info.LinkAs }}"
           hx-get="/form/{{ .FormID }}?return_to={{ urlquery $returnTo }}&link_relation={{ $info.Relation }}&link_peer={{ $info.PeerID }}&link_as={{ $info.LinkAs }}"
           hx-target="#content" hx-push-url="true">{{ .Label }}</a>
        {{ end }}
      </div>
    </details>
    {{ end }}
    {{ end }}
  </div>
  {{ end }}

  {{/* display: properties */}}
  {{ if eq .Display "properties" }}
  {{ if or .Entities .IsEmpty }}
  {{/* Collection source: render each entity as a card with header */}}
  {{ if .IsEmpty }}
  <div class="card" style="padding:24px;text-align:center;color:var(--text-muted);">
    {{ if .EmptyMessage }}{{ .EmptyMessage }}{{ else }}No items{{ end }}
  </div>
  {{ else }}
  {{ $returnTo := printf "%s#%s" $.ReturnTo .SectionID }}
  {{ range .Entities }}
  <div class="card" style="padding:20px;margin-bottom:12px;">
    <div style="display:flex;align-items:center;gap:10px;margin-bottom:12px;">
      <a href="/entity/{{ .Type }}/{{ .ID }}" class="cell-link" style="font-size:16px;font-weight:600;"
         hx-get="/entity/{{ .Type }}/{{ .ID }}" hx-target="#content" hx-push-url="true">{{ .Title }}</a>
      <span style="font-size:11px;font-family:var(--font-mono);color:var(--text-muted);background:#f1f5f9;padding:1px 6px;border-radius:3px;">{{ .ID }}</span>
      {{ if .EditFormID }}<a href="/form/{{ .EditFormID }}/{{ .ID }}?return_to={{ urlquery $returnTo }}" class="edit-icon"
         hx-get="/form/{{ .EditFormID }}/{{ .ID }}?return_to={{ urlquery $returnTo }}" hx-target="#content" hx-push-url="true" title="Edit">&#9998;</a>{{ end }}
    </div>
    <div class="detail-grid">
      {{ range .Fields }}
      <div class="detail-label">{{ .Label }}</div>
      <div class="detail-value">
        {{ if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>
        {{ else }}{{ if .Value }}{{ formatValue .Value }}{{ else }}&mdash;{{ end }}{{ end }}
      </div>
      {{ end }}
    </div>
  </div>
  {{ end }}
  {{ end }}
  {{ else }}
  {{/* Entry source: single card */}}
  <div class="card" style="padding:20px;">
    <div class="detail-grid">
      {{ range .Fields }}
      <div class="detail-label">{{ .Label }}</div>
      <div class="detail-value">
        {{ if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>
        {{ else }}{{ if .Value }}{{ formatValue .Value }}{{ else }}&mdash;{{ end }}{{ end }}
      </div>
      {{ end }}
    </div>
  </div>
  {{ end }}
  {{ end }}

  {{/* display: content (entry) */}}
  {{ if and (eq .Display "content") .HasContent (not .Entities) }}
  <div class="card" style="padding:20px;">
    <div class="markdown-body">{{ renderMarkdown .Content }}</div>
  </div>
  {{ end }}

  {{/* display: content (collection) */}}
  {{ if and (eq .Display "content") .Entities }}
  {{ if .IsEmpty }}
  <div class="card" style="padding:24px;text-align:center;color:var(--text-muted);">
    {{ if .EmptyMessage }}{{ .EmptyMessage }}{{ else }}No items{{ end }}
  </div>
  {{ else }}
  {{ $returnTo := printf "%s#%s" $.ReturnTo .SectionID }}
  {{ range .Entities }}
  <div class="card view-content-entity" style="padding:20px;margin-bottom:12px;">
    <div style="display:flex;align-items:center;gap:10px;margin-bottom:8px;">
      <a href="/entity/{{ .Type }}/{{ .ID }}" class="cell-link" style="font-size:16px;font-weight:600;"
         hx-get="/entity/{{ .Type }}/{{ .ID }}" hx-target="#content" hx-push-url="true">{{ .Title }}</a>
      <span style="font-size:11px;font-family:var(--font-mono);color:var(--text-muted);background:#f1f5f9;padding:1px 6px;border-radius:3px;">{{ .ID }}</span>
      {{ if .EditFormID }}<a href="/form/{{ .EditFormID }}/{{ .ID }}?return_to={{ urlquery $returnTo }}" class="edit-icon"
         hx-get="/form/{{ .EditFormID }}/{{ .ID }}?return_to={{ urlquery $returnTo }}" hx-target="#content" hx-push-url="true" title="Edit">&#9998;</a>{{ end }}
    </div>
    {{ if .Fields }}
    <div style="display:flex;gap:12px;flex-wrap:wrap;margin-bottom:10px;">
      {{ range .Fields }}
      {{ if .Value }}
      <span style="font-size:12px;color:var(--text-muted);">
        {{ .Label }}:
        {{ if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>
        {{ else }}<strong>{{ formatValue .Value }}</strong>{{ end }}
      </span>
      {{ end }}
      {{ end }}
    </div>
    {{ end }}
    {{ if .HasContent }}
    <div class="markdown-body" style="border-top:1px solid var(--border);padding-top:12px;margin-top:4px;">
      {{ renderMarkdown .Content }}
    </div>
    {{ end }}
  </div>
  {{ end }}
  {{ end }}
  {{ end }}

  {{/* display: table */}}
  {{ if eq .Display "table" }}
  {{ if .IsEmpty }}
  <div class="card" style="padding:24px;text-align:center;color:var(--text-muted);">
    {{ if .EmptyMessage }}{{ .EmptyMessage }}{{ else }}No items{{ end }}
  </div>
  {{ else if .IsGrouped }}
  {{ $returnTo := printf "%s#%s" $.ReturnTo .SectionID }}
  {{ range .Groups }}
  <h4 style="font-size:13px;font-weight:600;color:var(--text-muted);margin:16px 0 8px;text-transform:uppercase;letter-spacing:0.04em;">{{ .GroupName }}</h4>
  <div class="card" style="margin-bottom:12px;">
    <div style="overflow-x:auto;">
      <table>
        <tbody>
          {{ range .Rows }}
          <tr>
            {{ range .Cells }}
            <td>
              {{ if .Link }}<a href="/entity/{{ .EntityType }}/{{ .EntityID }}" class="cell-link"
                 hx-get="/entity/{{ .EntityType }}/{{ .EntityID }}" hx-target="#content" hx-push-url="true">{{ .Value }}</a>
              {{ else if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>
              {{ else }}{{ if .Value }}{{ .Value }}{{ else }}&mdash;{{ end }}{{ end }}
            </td>
            {{ end }}
            {{ if .EditFormID }}<td style="width:1%;white-space:nowrap;"><a href="/form/{{ .EditFormID }}/{{ .EntityID }}?return_to={{ urlquery $returnTo }}" class="edit-icon"
               hx-get="/form/{{ .EditFormID }}/{{ .EntityID }}?return_to={{ urlquery $returnTo }}" hx-target="#content" hx-push-url="true" title="Edit">&#9998;</a></td>{{ end }}
          </tr>
          {{ end }}
        </tbody>
      </table>
    </div>
  </div>
  {{ end }}
  {{ else }}
  {{ $returnTo := printf "%s#%s" $.ReturnTo .SectionID }}
  <div class="card">
    <div style="overflow-x:auto;">
      <table>
        <thead>
          <tr>
            {{ range .Columns }}
            <th>{{ if .Label }}{{ .Label }}{{ else }}{{ .Property }}{{ end }}</th>
            {{ end }}
            <th></th>
          </tr>
        </thead>
        <tbody>
          {{ range .Rows }}
          <tr>
            {{ range .Cells }}
            <td>
              {{ if .Link }}<a href="/entity/{{ .EntityType }}/{{ .EntityID }}" class="cell-link"
                 hx-get="/entity/{{ .EntityType }}/{{ .EntityID }}" hx-target="#content" hx-push-url="true">{{ .Value }}</a>
              {{ else if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>
              {{ else }}{{ if .Value }}{{ .Value }}{{ else }}&mdash;{{ end }}{{ end }}
            </td>
            {{ end }}
            {{ if .EditFormID }}<td style="width:1%;white-space:nowrap;"><a href="/form/{{ .EditFormID }}/{{ .EntityID }}?return_to={{ urlquery $returnTo }}" class="edit-icon"
               hx-get="/form/{{ .EditFormID }}/{{ .EntityID }}?return_to={{ urlquery $returnTo }}" hx-target="#content" hx-push-url="true" title="Edit">&#9998;</a></td>
            {{ else }}<td></td>{{ end }}
          </tr>
          {{ end }}
        </tbody>
      </table>
    </div>
  </div>
  {{ end }}
  {{ end }}

  {{/* display: cards */}}
  {{ if eq .Display "cards" }}
  {{ if .IsEmpty }}
  <div class="card" style="padding:24px;text-align:center;color:var(--text-muted);">
    {{ if .EmptyMessage }}{{ .EmptyMessage }}{{ else }}No items{{ end }}
  </div>
  {{ else }}
  {{ $returnTo := printf "%s#%s" $.ReturnTo .SectionID }}
  <div style="display:grid;grid-template-columns:repeat(auto-fill, minmax(300px, 1fr));gap:12px;">
    {{ range .Entities }}
    <div class="card" style="padding:16px;">
      <div style="margin-bottom:8px;display:flex;align-items:center;gap:6px;">
        <a href="/entity/{{ .Type }}/{{ .ID }}" class="cell-link" style="font-size:14px;font-weight:600;"
           hx-get="/entity/{{ .Type }}/{{ .ID }}" hx-target="#content" hx-push-url="true">{{ .Title }}</a>
        <span style="font-size:10px;font-family:var(--font-mono);color:var(--text-muted);">{{ .ID }}</span>
        {{ if .EditFormID }}<a href="/form/{{ .EditFormID }}/{{ .ID }}?return_to={{ urlquery $returnTo }}" class="edit-icon"
           hx-get="/form/{{ .EditFormID }}/{{ .ID }}?return_to={{ urlquery $returnTo }}" hx-target="#content" hx-push-url="true" title="Edit">&#9998;</a>{{ end }}
      </div>
      {{ range .Fields }}
      {{ if .Value }}
      <div style="font-size:12px;margin-bottom:2px;">
        <span style="color:var(--text-muted);">{{ .Label }}:</span>
        {{ if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>
        {{ else }}<strong>{{ formatValue .Value }}</strong>{{ end }}
      </div>
      {{ end }}
      {{ end }}
      {{ if .HasContent }}
      <div class="markdown-body" style="border-top:1px solid var(--border);padding-top:8px;margin-top:8px;font-size:13px;">
        {{ renderMarkdown .Content }}
      </div>
      {{ end }}
    </div>
    {{ end }}
  </div>
  {{ end }}
  {{ end }}

  {{/* display: list */}}
  {{ if eq .Display "list" }}
  {{ if .IsEmpty }}
  <div class="card" style="padding:24px;text-align:center;color:var(--text-muted);">
    {{ if .EmptyMessage }}{{ .EmptyMessage }}{{ else }}No items{{ end }}
  </div>
  {{ else }}
  {{ $returnTo := printf "%s#%s" $.ReturnTo .SectionID }}
  <div class="card" style="padding:12px 20px;">
    <ul class="rel-list">
      {{ range .Entities }}
      <li>
        <a href="/entity/{{ .Type }}/{{ .ID }}" class="cell-link"
           hx-get="/entity/{{ .Type }}/{{ .ID }}" hx-target="#content" hx-push-url="true">{{ .Title }}</a>
        {{ if .EditFormID }}<a href="/form/{{ .EditFormID }}/{{ .ID }}?return_to={{ urlquery $returnTo }}" class="edit-icon"
           hx-get="/form/{{ .EditFormID }}/{{ .ID }}?return_to={{ urlquery $returnTo }}" hx-target="#content" hx-push-url="true" title="Edit">&#9998;</a>{{ end }}
        {{ range .Fields }}
        {{ if .Value }}
        {{ if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>
        {{ else }}<span style="font-size:12px;color:var(--text-muted);">{{ .Value }}</span>{{ end }}
        {{ end }}
        {{ end }}
      </li>
      {{ end }}
    </ul>
  </div>
  {{ end }}
  {{ end }}

</div>
{{ end }}
{{- end -}}
`
