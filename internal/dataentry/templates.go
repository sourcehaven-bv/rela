package dataentry

// allTemplates contains all HTML templates for the data entry application.
// These are parsed at startup and used by all handlers.
const allTemplates = `
{{- define "head" -}}
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="icon" type="image/svg+xml" href="/static/favicon.svg">
<script>
(function(){var t=localStorage.getItem('theme');if(t){document.documentElement.setAttribute('data-theme',t)}else if(matchMedia('(prefers-color-scheme:dark)').matches){document.documentElement.setAttribute('data-theme','dark')}})();
</script>
<script src="/static/htmx.min.js"></script>
<link rel="stylesheet" href="/static/easymde.min.css">
<script src="/static/easymde.min.js"></script>
<link rel="stylesheet" href="/static/slimselect.css">
<script src="/static/slimselect.min.js"></script>
<script src="/static/mermaid.min.js"></script>
<style>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
:root {
  --bg: #f8fafc; --bg-card: #fff; --bg-sidebar: #1e293b; --bg-sidebar-hover: #334155;
  --bg-sidebar-active: #0f172a; --text: #1e293b; --text-muted: #64748b;
  --text-sidebar: #cbd5e1; --text-sidebar-active: #fff; --border: #e2e8f0;
  --primary: #3b82f6; --primary-hover: #2563eb; --primary-light: #eff6ff;
  --danger: #ef4444; --danger-light: #fef2f2; --danger-border: #fecaca;
  --warning: #f59e0b; --warning-light: #fffbeb; --warning-text: #b45309;
  --radius: 8px; --font: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  --font-mono: "SF Mono", "Fira Code", monospace;
  --shadow: 0 1px 3px rgba(0,0,0,0.08);
}
[data-theme="dark"] {
  --bg: #0f172a; --bg-card: #1e293b; --bg-sidebar: #020617; --bg-sidebar-hover: #1e293b;
  --bg-sidebar-active: #0f172a; --text: #e2e8f0; --text-muted: #94a3b8;
  --text-sidebar: #94a3b8; --text-sidebar-active: #f1f5f9; --border: #334155;
  --primary: #60a5fa; --primary-hover: #3b82f6; --primary-light: rgba(59,130,246,0.15);
  --danger: #f87171; --danger-light: rgba(239,68,68,0.15); --danger-border: rgba(239,68,68,0.3);
  --warning: #fbbf24; --warning-light: rgba(251,191,36,0.15); --warning-text: #fbbf24;
  --shadow: 0 1px 3px rgba(0,0,0,0.3);
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
.nav-group { margin: 0; }
.nav-group-toggle { display: flex; align-items: center; gap: 6px; width: 100%; padding: 8px 20px; background: none; border: none; border-left: 3px solid transparent; color: var(--text-sidebar); font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; cursor: pointer; font-family: var(--font); transition: color 0.15s; }
.nav-group-toggle:hover { color: var(--text-sidebar-active); }
.nav-group-chevron { display: inline-block; font-size: 20px; line-height: 1; transition: transform 0.15s ease; transform: rotate(90deg); }
.nav-group-chevron.collapsed { transform: rotate(0deg); }
.nav-group-items { overflow: hidden; }
.nav-group-items.hidden { display: none; }
.sidebar nav .nav-group-items a { padding-left: 40px; }

.main { margin-left: 240px; flex: 1; padding: 32px; max-width: 1100px; }
.main.main-wide { max-width: none; }
.page-header { position: sticky; top: 0; z-index: 50; background: var(--bg); padding: 12px 0; margin: -12px 0 16px 0; border-bottom: 1px solid var(--border); display: flex; align-items: center; justify-content: space-between; }
.page-header h2 { font-size: 22px; font-weight: 700; }
.page-header p { color: var(--text-muted); font-size: 14px; margin-top: 2px; }

.theme-toggle { position: fixed; top: 12px; right: 16px; z-index: 200; width: 36px; height: 36px; border-radius: 50%; border: 1px solid var(--border); background: var(--bg-card); color: var(--text); cursor: pointer; display: flex; align-items: center; justify-content: center; font-size: 16px; box-shadow: var(--shadow); transition: all 0.2s; }
.theme-toggle:hover { background: var(--primary-light); border-color: var(--primary); }
.theme-toggle .icon-sun, .theme-toggle .icon-moon { display: none; }
:root:not([data-theme]) .theme-toggle .icon-sun, [data-theme="light"] .theme-toggle .icon-sun { display: block; }
[data-theme="dark"] .theme-toggle .icon-moon { display: block; }
@media (prefers-color-scheme: dark) { :root:not([data-theme]) .theme-toggle .icon-moon { display: block; } :root:not([data-theme]) .theme-toggle .icon-sun { display: none; } }

.card { background: var(--bg-card); border: 1px solid var(--border); border-radius: var(--radius); box-shadow: var(--shadow); }

.filter-bar-sentinel { height: 1px; margin: 0; visibility: hidden; }
.filter-bar { position: sticky; top: 57px; z-index: 40; background: var(--bg); padding: 8px 0 12px 0; display: flex; gap: 12px; align-items: center; flex-wrap: wrap; margin-bottom: 16px; }
.filter-bar.is-stuck { border-bottom: 1px solid var(--border); box-shadow: 0 2px 4px rgba(0,0,0,0.04); }
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
.delete-icon { color: var(--text-muted); text-decoration: none; font-size: 14px; opacity: 0.6; transition: opacity 0.15s; }
.delete-icon:hover { opacity: 1; color: var(--danger); }
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
.badge-gray { background: var(--bg); color: var(--text-muted); }
.badge-red { background: #fee2e2; color: #991b1b; }
.badge-orange { background: #fed7aa; color: #9a3412; }
.badge-yellow { background: #fef9c3; color: #854d0e; }
[data-theme="dark"] .badge-blue { background: rgba(59,130,246,0.2); color: #93c5fd; }
[data-theme="dark"] .badge-purple { background: rgba(168,85,247,0.2); color: #d8b4fe; }
[data-theme="dark"] .badge-green { background: rgba(34,197,94,0.2); color: #86efac; }
[data-theme="dark"] .badge-gray { background: rgba(100,116,139,0.2); color: #cbd5e1; }
[data-theme="dark"] .badge-red { background: rgba(239,68,68,0.2); color: #fca5a5; }
[data-theme="dark"] .badge-orange { background: rgba(249,115,22,0.2); color: #fdba74; }
[data-theme="dark"] .badge-yellow { background: rgba(234,179,8,0.2); color: #fde047; }

.error-box { padding: 10px 16px; background: var(--danger-light); border: 1px solid var(--danger-border); border-radius: 6px; margin-bottom: 16px; font-size: 13px; color: var(--danger); }

.info-bar { display: flex; gap: 6px; flex-wrap: wrap; margin-bottom: 12px; }
.info-chip { display: inline-flex; align-items: center; gap: 4px; padding: 3px 10px; background: var(--primary-light); color: var(--primary); border-radius: 9999px; font-size: 12px; font-weight: 500; }

.btn { padding: 8px 20px; border: 1px solid var(--border); border-radius: 6px; font-size: 14px; font-weight: 500; cursor: pointer; font-family: var(--font); transition: all 0.15s; text-decoration: none; display: inline-flex; align-items: center; gap: 6px; }
.btn-primary { background: var(--primary); color: #fff; border-color: var(--primary); }
.btn-primary:hover { background: var(--primary-hover); }
.btn-secondary { background: var(--bg-card); color: var(--text); }
.btn-secondary:hover { background: var(--bg); }
.btn-sm { padding: 5px 12px; font-size: 13px; }
.btn-danger { background: #fff; color: var(--danger); border-color: var(--danger); }
.btn-danger:hover { background: var(--danger-light); }
[data-theme="dark"] .btn-danger { background: var(--danger-light); color: var(--danger); }
[data-theme="dark"] .btn-danger:hover { background: rgba(239,68,68,0.25); }

.form-card { padding: 28px; max-width: 820px; }
.template-selector { margin-bottom: 16px; display: flex; align-items: center; gap: 12px; max-width: 820px; }
.template-label { font-size: 13px; font-weight: 500; color: var(--text-muted); }
.template-pills { display: flex; gap: 8px; flex-wrap: wrap; }
.template-pill { padding: 6px 14px; border: 1px solid var(--border); border-radius: 16px; background: var(--bg-card); font-size: 13px; cursor: pointer; transition: all 0.15s; color: var(--text); }
.template-pill:hover { border-color: var(--primary); color: var(--primary); }
.template-pill.active { background: var(--primary); color: white; border-color: var(--primary); }
[data-theme="dark"] .template-pill { background: #334155; border-color: #475569; }
[data-theme="dark"] .template-pill:hover { background: #3b82f6; color: #fff; border-color: #3b82f6; }
.template-dropdown { padding: 6px 12px; border: 1px solid var(--border); border-radius: 6px; font-size: 13px; background: var(--bg-card); color: var(--text); }
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

.form-group.has-error input, .form-group.has-error textarea, .form-group.has-error select { border-color: var(--danger); background-color: var(--danger-light); }
.form-group.has-error input:focus, .form-group.has-error textarea:focus, .form-group.has-error select:focus { border-color: var(--danger); box-shadow: 0 0 0 3px rgba(239, 68, 68, 0.1); }
.field-error { color: var(--danger); font-size: 12px; margin-top: 4px; font-weight: 500; }

.transitions-info { margin-top: 6px; padding: 8px 12px; background: var(--bg); border-radius: 6px; border: 1px solid var(--border); }
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
.rel-type { font-size: 11px; font-family: var(--font-mono); color: var(--text-muted); background: var(--bg); padding: 1px 6px; border-radius: 3px; }

.pagination { display: flex; align-items: center; justify-content: space-between; padding: 12px 16px; border-top: 1px solid var(--border); font-size: 13px; color: var(--text-muted); }

.toast { position: fixed; top: 16px; right: 16px; padding: 12px 20px; background: #166534; color: #fff; border-radius: 8px; font-size: 14px; font-weight: 500; z-index: 999; box-shadow: 0 4px 12px rgba(0,0,0,0.15); animation: toastIn 0.3s; }
.toast-error { background: #991b1b; }
@keyframes toastIn { from { opacity: 0; transform: translateY(-8px); } to { opacity: 1; transform: translateY(0); } }

.conflict-banner { position: fixed; top: 0; left: 240px; right: 0; z-index: 90; background: linear-gradient(90deg, #fef3c7, #fde68a); border-bottom: 1px solid #fcd34d; padding: 8px 20px; display: flex; align-items: center; justify-content: space-between; font-size: 14px; color: #92400e; }
.conflict-banner-icon { font-size: 16px; margin-right: 8px; }
.conflict-banner a { color: #92400e; font-weight: 600; text-decoration: underline; }
.conflict-banner a:hover { color: #78350f; }
.conflict-banner-close { background: none; border: none; font-size: 18px; cursor: pointer; color: #92400e; opacity: 0.7; padding: 4px 8px; }
.conflict-banner-close:hover { opacity: 1; }
.has-conflict-banner .main { padding-top: 56px; }
.has-conflict-banner .page-header { top: 44px; }

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
.EasyMDEContainer .CodeMirror { border: none; border-radius: 0 0 6px 6px; font-family: var(--font-mono); font-size: 14px; background: var(--bg-card); color: var(--text); }
.EasyMDEContainer .editor-toolbar { border-bottom: 1px solid var(--border); border-radius: 6px 6px 0 0; background: var(--bg); }
[data-theme="dark"] .EasyMDEContainer .CodeMirror .CodeMirror-cursor { border-left-color: var(--text); }
[data-theme="dark"] .EasyMDEContainer .editor-toolbar button { color: var(--text); }
[data-theme="dark"] .EasyMDEContainer .editor-toolbar button:hover { background: var(--bg-card); }
[data-theme="dark"] .EasyMDEContainer .editor-preview { background: var(--bg-card); color: var(--text); }
.EasyMDEContainer .editor-preview { padding: 12px 16px; }
.EasyMDEContainer .editor-preview ul, .EasyMDEContainer .editor-preview ol { padding-left: 24px; }
.EasyMDEContainer .editor-preview li { margin: 2px 0; }
.EasyMDEContainer .editor-preview ul.contains-task-list { list-style: none; padding-left: 4px; }
.EasyMDEContainer .editor-preview .task-list-item { display: flex; align-items: baseline; gap: 6px; }
.EasyMDEContainer .editor-preview .task-list-item input[type="checkbox"] { margin: 0; position: relative; top: 1px; }

/* Fullscreen editor mode */
.editor-fullscreen-overlay { position: fixed; inset: 0; z-index: 300; background: var(--bg); display: flex; flex-direction: column; }
.editor-fullscreen-overlay .editor-fullscreen-header { display: flex; align-items: center; justify-content: space-between; padding: 10px 20px; border-bottom: 1px solid var(--border); background: var(--bg-card); flex-shrink: 0; }
.editor-fullscreen-overlay .editor-fullscreen-header h3 { font-size: 15px; font-weight: 600; color: var(--text); }
.editor-fullscreen-overlay .editor-fullscreen-body { flex: 1; display: flex; flex-direction: column; padding: 16px 24px; overflow: hidden; }
.editor-fullscreen-overlay .EasyMDEContainer { flex: 1; display: flex; flex-direction: column; border: 1px solid var(--border); border-radius: 6px; }
.editor-fullscreen-overlay .EasyMDEContainer .CodeMirror { flex: 1; }
.editor-fullscreen-overlay .EasyMDEContainer .CodeMirror-scroll { min-height: 0; }
.editor-fullscreen-overlay .EasyMDEContainer.sided--no-fullscreen { display: grid; grid-template-columns: 1fr 1fr; grid-template-rows: auto 1fr; }
.editor-fullscreen-overlay .EasyMDEContainer.sided--no-fullscreen .editor-toolbar { grid-column: 1 / -1; }
.editor-fullscreen-overlay .EasyMDEContainer.sided--no-fullscreen .CodeMirror { min-height: 0; width: 100% !important; }
.editor-fullscreen-overlay .EasyMDEContainer.sided--no-fullscreen .editor-preview-active-side { overflow: auto; width: 100% !important; }

.view-section-heading { font-size: 15px; font-weight: 700; color: var(--text); margin: 0 0 10px; padding-bottom: 6px; border-bottom: 2px solid var(--border); }
.view-content-entity .markdown-body { font-size: 14px; line-height: 1.7; color: var(--text); }
.markdown-body h3 { font-size: 15px; font-weight: 600; margin: 16px 0 6px; }
.markdown-body h4 { font-size: 14px; font-weight: 600; margin: 14px 0 4px; }
.markdown-body h5 { font-size: 13px; font-weight: 600; margin: 12px 0 4px; }
.markdown-body p { margin: 8px 0; }
.markdown-body ul, .markdown-body ol { margin: 8px 0; padding-left: 24px; }
.markdown-body li { margin: 2px 0; }
.markdown-body pre { background: var(--bg); padding: 12px; border-radius: 6px; overflow-x: auto; font-family: var(--font-mono); font-size: 13px; margin: 8px 0; border: 1px solid var(--border); }
.markdown-body code { background: var(--bg); padding: 1px 4px; border-radius: 3px; font-family: var(--font-mono); font-size: 0.9em; }
.markdown-body pre code { background: none; padding: 0; border: none; }
.markdown-body strong { font-weight: 600; }
.markdown-body em { font-style: italic; }
.markdown-body ul.task-list { list-style: none; padding-left: 4px; }
.markdown-body .task-item { margin: 4px 0; }
.markdown-body .task-item label { display: flex; align-items: baseline; gap: 6px; cursor: pointer; }
.markdown-body .task-item input[type="checkbox"] { cursor: pointer; margin: 0; position: relative; top: 1px; }
.cb-stats { font-size: 13px; font-weight: 400; color: var(--text-muted); margin-left: 6px; }

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
.jump-bar { position: sticky; top: 57px; z-index: 40; display: flex; gap: 6px; flex-wrap: wrap; margin-bottom: 20px; padding: 8px 12px; background: var(--bg-card); border: 1px solid var(--border); border-radius: var(--radius); box-shadow: 0 2px 4px rgba(0,0,0,0.04); }
.jump-link { font-size: 13px; color: var(--primary); text-decoration: none; padding: 2px 10px; border-radius: 9999px; transition: all 0.15s; }
.jump-link:hover { background: var(--primary-light); }
.nav-count { margin-left: auto; font-size: 11px; color: rgba(255,255,255,0.4); font-weight: 400; }

/* Kanban board */
.kanban-board { display: flex; gap: 12px; padding: 16px 0; overflow-x: auto; min-height: 400px; align-items: flex-start; }
.kanban-column { flex: 0 0 280px; background: var(--bg-card); border-radius: var(--radius); border: 1px solid var(--border); display: flex; flex-direction: column; max-height: calc(100vh - 200px); }
.kanban-column-header { padding: 12px 16px; font-weight: 600; font-size: 13px; border-bottom: 1px solid var(--border); display: flex; align-items: center; justify-content: space-between; position: sticky; top: 0; background: var(--bg-card); border-radius: var(--radius) var(--radius) 0 0; z-index: 1; }
.kanban-column-header .kanban-count { font-size: 11px; font-weight: 500; color: var(--text-muted); background: var(--bg); padding: 2px 8px; border-radius: 9999px; }
.kanban-cards { flex: 1; padding: 8px; display: flex; flex-direction: column; gap: 8px; min-height: 80px; overflow-y: auto; background: var(--bg); }
.kanban-card { background: var(--bg-card); border: 1px solid var(--border); border-radius: 6px; padding: 10px 12px; cursor: pointer; transition: box-shadow 0.15s, transform 0.15s, opacity 0.15s; border-left: 3px solid var(--border); position: relative; }
.kanban-card:hover { box-shadow: 0 4px 12px rgba(0,0,0,0.08); transform: translateY(-1px); }
.kanban-card.dragging { opacity: 0.5; transform: rotate(2deg); }
.kanban-card-title { font-weight: 500; font-size: 13px; margin-bottom: 8px; line-height: 1.4; color: var(--text); }
.kanban-card-fields { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; }
.kanban-card-fields .badge { font-size: 10px; padding: 2px 6px; }
.kanban-column.drag-over { background: var(--primary-light); }
.kanban-column.drag-over .kanban-cards { background: var(--primary-light); }
/* Card accent colors based on first field */
.kanban-card[data-accent="red"] { border-left-color: #ef4444; }
.kanban-card[data-accent="orange"] { border-left-color: #f97316; }
.kanban-card[data-accent="yellow"] { border-left-color: #eab308; }
.kanban-card[data-accent="green"] { border-left-color: #22c55e; }
.kanban-card[data-accent="blue"] { border-left-color: #3b82f6; }
.kanban-card[data-accent="purple"] { border-left-color: #a855f7; }
.kanban-card[data-accent="gray"] { border-left-color: #6b7280; }

/* Kanban swimlanes */
.kanban-board.with-swimlanes { display: grid; gap: 0; overflow: auto; border-radius: var(--radius); border: 1px solid var(--border); padding: 0; }
.kanban-swimlane-header { display: contents; }
.kanban-swimlane-header .kanban-corner { background: transparent; padding: 12px; border-bottom: 1px solid var(--border); }
.kanban-swimlane-header .kanban-col-header { background: var(--bg-card); padding: 12px 16px; font-weight: 600; font-size: 12px; text-align: center; color: var(--text); text-transform: uppercase; letter-spacing: 0.03em; border-bottom: 1px solid var(--border); border-left: 1px solid var(--border); }
.kanban-swimlane { display: contents; }
.kanban-swimlane-label { background: var(--bg-card); color: var(--text); padding: 16px 12px; font-weight: 600; font-size: 13px; min-width: 100px; display: flex; align-items: flex-start; justify-content: flex-start; border-right: 3px solid var(--primary); border-bottom: 1px solid var(--border); }
.kanban-swimlane:last-child .kanban-swimlane-label { border-bottom: none; }
.kanban-cell { background: var(--bg); padding: 8px; min-height: 120px; display: flex; flex-direction: column; gap: 8px; align-content: flex-start; align-self: stretch; border-left: 1px solid var(--border); border-bottom: 1px solid var(--border); }
.kanban-swimlane:last-child .kanban-cell { border-bottom: none; }
.kanban-cell.drag-over { background: var(--primary-light); }

/* Command toast */
#command-toast-container { position: fixed; bottom: 16px; right: 16px; z-index: 1000; display: flex; flex-direction: column-reverse; gap: 8px; max-width: 380px; }
.command-toast { background: var(--bg-card); border: 1px solid var(--border); border-radius: 10px; box-shadow: 0 4px 16px rgba(0,0,0,0.12); overflow: hidden; animation: toastIn 0.3s; font-size: 13px; }
.command-toast-header { display: flex; align-items: center; gap: 8px; padding: 10px 12px; border-bottom: 1px solid var(--border); background: var(--bg); }
.command-toast-label { font-weight: 600; font-size: 13px; flex: 1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.command-toast-icon { width: 18px; height: 18px; flex-shrink: 0; display: flex; align-items: center; justify-content: center; font-size: 14px; }
.command-toast-btn { background: none; border: none; cursor: pointer; font-size: 16px; color: var(--text-muted); padding: 2px 4px; border-radius: 4px; line-height: 1; }
.command-toast-btn:hover { background: var(--bg); color: var(--text); }
.command-toast-body { padding: 8px 12px; max-height: 200px; overflow-y: auto; }
.command-toast-body:empty { display: none; }
.command-toast-msg { padding: 3px 0; color: var(--text-muted); line-height: 1.4; }
.command-toast-msg.warning { color: var(--warning-text); }
.command-toast-msg.error-msg { color: var(--danger); }
.command-toast-file, .command-toast-entity { display: flex; align-items: center; gap: 6px; padding: 4px 0; }
.command-toast-file a, .command-toast-entity a { font-size: 12px; color: var(--primary); text-decoration: none; padding: 2px 8px; border: 1px solid var(--primary); border-radius: 4px; white-space: nowrap; }
.command-toast-file a:hover, .command-toast-entity a:hover { background: var(--primary-light); }
.command-toast-file span, .command-toast-entity span { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.command-toast-expand { font-size: 12px; color: var(--primary); cursor: pointer; padding: 4px 0; border: none; background: none; }
.command-toast-expand:hover { text-decoration: underline; }
.command-toast-group-label { font-size: 12px; font-weight: 600; color: var(--text); padding: 4px 0 2px; cursor: pointer; }
.command-toast-group-label::before { content: '\25B6'; font-size: 9px; margin-right: 4px; display: inline-block; transition: transform 0.15s; }
.command-toast-group-label.open::before { transform: rotate(90deg); }
.command-toast-group-items { display: none; padding-left: 14px; }
.command-toast-group-label.open + .command-toast-group-items { display: block; }
.command-toast.running .command-toast-header { border-left: 3px solid var(--primary); }
.command-toast.success .command-toast-header { border-left: 3px solid #16a34a; }
.command-toast.error .command-toast-header { border-left: 3px solid #dc2626; }
.command-toast.cancelled .command-toast-header { border-left: 3px solid var(--text-muted); }
@keyframes cmdSpin { to { transform: rotate(360deg); } }
.cmd-spinner { display: inline-block; width: 14px; height: 14px; border: 2px solid var(--border); border-top-color: var(--primary); border-radius: 50%; animation: cmdSpin 0.6s linear infinite; }
.command-toast-log { display: none; }
.command-toast-log.show { display: block; padding: 8px 12px; background: var(--bg); border-top: 1px solid var(--border); font-family: var(--font-mono); font-size: 11px; max-height: 150px; overflow-y: auto; white-space: pre-wrap; word-break: break-all; color: var(--text-muted); }

/* Keyboard shortcut hints */
kbd { display: inline-flex; align-items: center; justify-content: center; min-width: 18px; height: 18px; padding: 0 4px; background: var(--bg); border: 1px solid var(--border); border-bottom-width: 2px; border-radius: 3px; font-family: var(--font-mono); font-size: 10px; color: var(--text-muted); line-height: 1; vertical-align: middle; }
kbd + kbd { margin-left: 2px; }
.btn kbd { background: rgba(255,255,255,0.2); border-color: rgba(255,255,255,0.3); color: rgba(255,255,255,0.8); font-size: 10px; height: 16px; min-width: 16px; margin-left: 4px; }
.btn-secondary kbd { background: var(--bg); border-color: var(--border); color: var(--text-muted); }
.sidebar kbd { background: rgba(255,255,255,0.1); border-color: rgba(255,255,255,0.2); color: rgba(255,255,255,0.4); }
tbody tr.row-selected { background: #dbeafe; outline: 2px solid var(--primary); outline-offset: -2px; }
#search-results .card.result-selected { outline: 2px solid var(--primary); outline-offset: -2px; background: var(--primary-light); }

/* Sidebar footer */
.sidebar-footer { display: flex; align-items: center; justify-content: space-between; padding: 12px 20px; border-top: 1px solid rgba(255,255,255,0.1); margin-top: auto; }
.sidebar-footer a, .sidebar-footer button { display: flex; align-items: center; gap: 6px; padding: 4px 0; background: none; border: none; color: var(--text-sidebar); font-size: 13px; cursor: pointer; font-family: var(--font); text-decoration: none; transition: color 0.15s; }
.sidebar-footer a:hover, .sidebar-footer button:hover { color: var(--text-sidebar-active); }

/* Command palette */
.cmd-palette-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.4); z-index: 500; display: flex; align-items: flex-start; justify-content: center; padding-top: 15vh; animation: fadeIn 0.1s; }
.cmd-palette { background: var(--bg-card); border-radius: 12px; box-shadow: 0 16px 48px rgba(0,0,0,0.2), 0 0 0 1px rgba(0,0,0,0.05); width: 520px; max-height: 400px; overflow: hidden; display: flex; flex-direction: column; }
.cmd-palette-input-wrap { padding: 12px 16px; border-bottom: 1px solid var(--border); display: flex; align-items: center; gap: 10px; }
.cmd-palette-input-wrap svg { flex-shrink: 0; color: var(--text-muted); }
.cmd-palette-input { flex: 1; border: none; outline: none; font-size: 15px; font-family: var(--font); background: none; color: var(--text); }
.cmd-palette-input::placeholder { color: #94a3b8; }
.cmd-palette-results { overflow-y: auto; padding: 6px; }
.cmd-palette-section { padding: 6px 10px 4px; font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-muted); }
.cmd-palette-item { display: flex; align-items: center; gap: 10px; padding: 8px 10px; border-radius: 6px; cursor: pointer; font-size: 14px; color: var(--text); transition: background 0.1s; }
.cmd-palette-item:hover, .cmd-palette-item.active { background: var(--primary-light); }
.cmd-palette-item.active { outline: 2px solid var(--primary); outline-offset: -2px; }
.cmd-palette-icon { width: 28px; height: 28px; display: flex; align-items: center; justify-content: center; background: var(--bg); border-radius: 6px; font-size: 14px; flex-shrink: 0; }
.cmd-palette-label { flex: 1; }
.cmd-palette-shortcut { display: flex; gap: 3px; }
.cmd-palette-footer { padding: 8px 16px; border-top: 1px solid var(--border); display: flex; gap: 16px; font-size: 12px; color: var(--text-muted); }
@keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }

/* Shortcuts help modal */
.shortcuts-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.4); z-index: 500; display: flex; align-items: center; justify-content: center; animation: fadeIn 0.1s; }
.shortcuts-modal { background: var(--bg-card); border-radius: 12px; box-shadow: 0 16px 48px rgba(0,0,0,0.2); width: 560px; max-height: 80vh; overflow-y: auto; }
.shortcuts-modal-header { padding: 20px 24px 16px; border-bottom: 1px solid var(--border); display: flex; align-items: center; justify-content: space-between; }
.shortcuts-modal-header h3 { font-size: 18px; font-weight: 700; }
.shortcuts-modal-close { background: none; border: none; font-size: 22px; cursor: pointer; color: var(--text-muted); padding: 4px 8px; border-radius: 4px; }
.shortcuts-modal-close:hover { background: var(--bg); color: var(--text); }
.shortcuts-body { padding: 16px 24px 24px; }
.shortcuts-group { margin-bottom: 20px; }
.shortcuts-group h4 { font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-muted); margin-bottom: 8px; }
.shortcut-row { display: flex; align-items: center; justify-content: space-between; padding: 6px 0; font-size: 14px; }
.shortcut-row + .shortcut-row { border-top: 1px solid var(--border); }
.shortcut-keys { display: flex; gap: 4px; align-items: center; font-size: 12px; color: var(--text-muted); }
.scope-nav { display:flex; align-items:center; gap:8px; padding:6px 12px; margin-bottom:12px; background:var(--bg-card); border:1px solid var(--border); border-radius:6px; font-size:13px; }
.scope-nav-btn { text-decoration:none; color:var(--primary); padding:2px 8px; border-radius:4px; transition:background 0.15s; }
.scope-nav-btn:hover { background:var(--primary-light); }
.scope-nav-disabled { opacity:0.35; pointer-events:none; color:var(--text-muted); padding:2px 8px; }
.scope-nav-progress { font-weight:600; font-family:var(--font-mono); }
.scope-nav-label { color:var(--text-muted); }

/* ── Side panel (form context panel) ── */
.main-with-panel { max-width: none; display: flex; align-items: stretch; min-height: 100vh; padding: 0; }
.main-with-panel .form-column { flex: 0 1 820px; min-width: 0; padding: 32px; }
.main-with-panel .form-column .form-card { max-width: none; }
.side-panel { flex: 0 0 auto; width: min(420px, 30vw); min-width: 260px; margin-left: auto; background: var(--bg); border-left: 1px solid var(--border); padding: 32px 16px; overflow-y: auto; position: sticky; top: 0; height: 100vh; }
.side-panel-section { background: var(--bg-card); border: 1px solid var(--border); border-radius: var(--radius); box-shadow: var(--shadow); overflow: hidden; }
.side-panel-section + .side-panel-section { margin-top: 12px; }
.side-panel-toggle { display: flex; align-items: center; gap: 8px; width: 100%; padding: 12px 16px; background: none; border: none; cursor: pointer; font-family: var(--font); font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-muted); transition: color 0.15s; text-align: left; }
.side-panel-toggle:hover { color: var(--text); }
.sp-chevron { display: inline-block; font-size: 16px; line-height: 1; transition: transform 0.15s ease; transform: rotate(90deg); }
.sp-chevron.collapsed { transform: rotate(0deg); }
.side-panel-body { padding: 0 16px 14px; }
.side-panel-body.hidden { display: none; }
.side-panel-header { display: none; }
.side-panel-close-btn { display: none; background: none; border: none; cursor: pointer; font-size: 22px; color: var(--text-muted); padding: 2px 6px; border-radius: 4px; margin-left: auto; }
.side-panel-close-btn:hover { background: var(--border); color: var(--text); }
.side-panel-overlay { display: none; }
.side-panel-edge-bar { display: none; position: fixed; top: 0; right: 0; bottom: 0; z-index: 150; width: 32px; background: var(--bg); border-left: 1px solid var(--border); cursor: pointer; flex-direction: column; align-items: center; justify-content: center; transition: background 0.15s; }
.side-panel-edge-bar:hover { background: #e2e8f0; }
.sp-edge-icon { width: 18px; height: 16px; position: relative; }
.sp-edge-icon::before { content: ''; position: absolute; inset: 0; border: 1.5px solid var(--text-muted); border-radius: 2px; }
.sp-edge-icon::after { content: ''; position: absolute; top: 3px; bottom: 3px; right: 5px; width: 1.5px; background: var(--text-muted); }
.side-panel-edge-bar:hover .sp-edge-icon::before, .side-panel-edge-bar:hover .sp-edge-icon::after { border-color: var(--primary); background-color: var(--primary); }
.side-panel-edge-bar:hover .sp-edge-icon::before { background-color: transparent; }
/* Panel card styling */
.sp-card { padding: 10px 12px; border: 1px solid var(--border); border-radius: 6px; font-size: 13px; }
.sp-card + .sp-card { margin-top: 8px; }
.sp-card-title { font-weight: 600; font-size: 13px; color: var(--primary); text-decoration: none; }
.sp-card-title:hover { text-decoration: underline; }
.sp-card-meta { font-size: 12px; color: var(--text-muted); margin-top: 2px; display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
@media (max-width: 1100px) {
  .side-panel-edge-bar { display: flex; }
  .main-with-panel .form-column { flex: 1; max-width: 820px; padding-right: 48px; }
  .side-panel { position: fixed; top: 0; right: 0; bottom: 0; z-index: 200; max-width: 380px; width: 85vw; min-width: 0; border-radius: 0; border: none; border-left: 1px solid var(--border); padding: 0; overflow-y: auto; height: auto; transform: translateX(100%); transition: transform 0.25s ease; box-shadow: -4px 0 20px rgba(0,0,0,0.1); }
  .side-panel.open { transform: translateX(0); }
  .side-panel-header { display: flex; align-items: center; padding: 16px; border-bottom: 1px solid var(--border); background: var(--bg); }
  .side-panel-header-title { font-size: 14px; font-weight: 600; color: var(--text); }
  .side-panel-close-btn { display: block; }
  .side-panel-sections { padding: 16px; }
  .side-panel-overlay { display: none; position: fixed; inset: 0; background: rgba(0,0,0,0.3); z-index: 199; }
  .side-panel-overlay.open { display: block; }
}

/* Settings page */
.settings-row { display: flex; align-items: center; gap: 8px; padding: 8px 0; border-bottom: 1px solid var(--border); }
.settings-row:first-child { border-top: 1px solid var(--border); }
.settings-row-label { font-size: 13px; font-weight: 500; font-family: var(--font-mono); color: var(--text); min-width: 140px; flex-shrink: 0; }
.settings-row-value { flex: 1; }
.settings-row-value select, .settings-row-value input { width: 100%; padding: 6px 10px; border: 1px solid var(--border); border-radius: 6px; font-size: 13px; font-family: var(--font); background: var(--bg-card); color: var(--text); }
.settings-row-value select:focus, .settings-row-value input:focus { outline: none; border-color: var(--primary); box-shadow: 0 0 0 3px var(--primary-light); }
.settings-row-remove { background: none; border: none; cursor: pointer; color: var(--text-muted); font-size: 18px; padding: 4px; border-radius: 4px; transition: all 0.15s; line-height: 1; flex-shrink: 0; }
.settings-row-remove:hover { color: var(--danger); background: #fef2f2; }
.override-group { border: 1px solid var(--border); border-radius: var(--radius); padding: 20px; margin-top: 12px; background: var(--bg); }
.override-header { display: flex; align-items: start; gap: 12px; }
.settings-stale-badge { display: inline-block; font-size: 10px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.03em; padding: 2px 6px; border-radius: 4px; background: #fef3c7; color: #92400e; border: 1px solid #fcd34d; margin-left: 6px; flex-shrink: 0; white-space: nowrap; }
.stale-option { color: #92400e; font-style: italic; }

/* Conflicts list page */
.conflict-summary { margin-bottom: 16px; }
.conflict-chip { display: inline-flex; align-items: center; gap: 6px; padding: 8px 16px; border-radius: 8px; font-size: 14px; font-weight: 600; }
.conflict-chip-warning { background: #fef3c7; color: #92400e; border: 1px solid #fcd34d; }
.conflict-path { font-family: var(--font-mono); font-size: 13px; }
.conflict-path-link { display: block; text-decoration: none; color: inherit; }
.conflict-path-link:hover .conflict-path { color: var(--primary); text-decoration: underline; }
.conflict-id { font-size: 11px; color: var(--text-muted); margin-top: 2px; }
.conflict-empty { text-align: center; padding: 60px 20px; }
.conflict-empty-icon { font-size: 48px; color: #22c55e; margin-bottom: 16px; }
.conflict-empty h3 { font-size: 18px; color: var(--text); margin-bottom: 8px; }
.conflict-empty p { color: var(--text-muted); }

/* Conflict resolution page */
.resolve-actions-top { display: flex; gap: 12px; margin-bottom: 16px; }
.resolve-actions-bottom { display: flex; gap: 12px; margin-top: 16px; justify-content: flex-end; }
.resolve-card { padding: 20px; margin-bottom: 16px; }
.resolve-card h3 { font-size: 14px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; color: var(--text-muted); margin-bottom: 16px; }
.resolve-table { width: 100%; }
.resolve-table th { text-align: left; padding: 8px 12px; font-size: 12px; font-weight: 600; color: var(--text-muted); border-bottom: 2px solid var(--border); }
.resolve-table td { padding: 10px 12px; border-bottom: 1px solid var(--border); vertical-align: top; }
.resolve-prop-name { font-family: var(--font-mono); font-size: 13px; font-weight: 500; }
.resolve-value { font-size: 13px; max-width: 200px; overflow: hidden; text-overflow: ellipsis; }
.resolve-value em { color: var(--text-muted); }
.resolve-choice { white-space: nowrap; }
.resolve-choice label { display: inline-flex; align-items: center; gap: 4px; margin-right: 12px; font-size: 13px; cursor: pointer; }
.resolve-same { background: #f0fdf4; }
.resolve-same-label { font-size: 11px; color: #15803d; font-weight: 500; }
.resolve-radio-hidden { position: absolute; opacity: 0; pointer-events: none; }
.resolve-value-selectable { cursor: pointer; transition: all 0.15s ease; position: relative; border-left: 3px solid transparent; }
.resolve-value-selectable:hover { background: #eff6ff; }
.resolve-value-selected { background: #eff6ff; border-left-color: #3b82f6; }
.resolve-value-unselected { color: var(--text-muted); opacity: 0.6; }
.resolve-value-unselected:hover { opacity: 1; }
.resolve-value-same { text-align: center; color: var(--text-muted); }
.resolve-row-focused td { outline: 2px solid #3b82f6; outline-offset: -2px; }
.resolve-row-focused td:first-child { border-radius: 4px 0 0 4px; }
.resolve-row-focused td:last-child { border-radius: 0 4px 4px 0; }
.resolve-content-choice { display: flex; gap: 20px; margin-bottom: 16px; }
.resolve-content-choice label { display: flex; align-items: center; gap: 6px; font-size: 14px; cursor: pointer; }
.resolve-content-compare { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.resolve-content-side { border: 1px solid var(--border); border-radius: 6px; overflow: hidden; }
.resolve-content-label { background: var(--bg); padding: 8px 12px; font-size: 12px; font-weight: 600; color: var(--text-muted); border-bottom: 1px solid var(--border); }
.resolve-content-pre { padding: 12px; margin: 0; font-size: 12px; font-family: var(--font-mono); white-space: pre-wrap; word-break: break-word; max-height: 300px; overflow-y: auto; background: var(--bg-card); }
.resolve-manual-edit { margin-top: 16px; }
.resolve-manual-edit label { display: block; font-size: 12px; font-weight: 600; color: var(--text-muted); margin-bottom: 8px; }
.resolve-manual-edit textarea { width: 100%; padding: 12px; border: 1px solid var(--border); border-radius: 6px; font-size: 13px; font-family: var(--font-mono); resize: vertical; }

/* Diff highlighting */
.diff-line { display: block; padding: 1px 4px; margin: 0 -4px; border-radius: 2px; }
.diff-line-add { background: #f0fdf4; }
.diff-line-remove { background: #fef2f2; }
.diff-line-change { background: transparent; }
.diff-word-add { background: #bbf7d0; border-radius: 2px; padding: 0 2px; }
.diff-word-remove { background: #fecaca; border-radius: 2px; padding: 0 2px; }

/* Dark mode: Conflict resolution */
[data-theme="dark"] .conflict-banner { background: linear-gradient(90deg, rgba(251,191,36,0.15), rgba(251,191,36,0.1)); border-bottom-color: #92400e; color: #fcd34d; }
[data-theme="dark"] .conflict-banner a { color: #fcd34d; }
[data-theme="dark"] .conflict-banner a:hover { color: #fef3c7; }
[data-theme="dark"] .conflict-banner-close { color: #fcd34d; }
[data-theme="dark"] .conflict-chip-warning { background: rgba(251,191,36,0.15); color: #fcd34d; border-color: #92400e; }
[data-theme="dark"] .conflict-empty-icon { color: #4ade80; }
[data-theme="dark"] .resolve-same { background: rgba(34,197,94,0.1); }
[data-theme="dark"] .resolve-same-label { color: #4ade80; }
[data-theme="dark"] .resolve-value-selectable:hover { background: rgba(59,130,246,0.15); }
[data-theme="dark"] .resolve-value-selected { background: rgba(59,130,246,0.2); border-left-color: #60a5fa; }
[data-theme="dark"] .resolve-row-focused td { outline-color: #60a5fa; }
[data-theme="dark"] .diff-line-add { background: rgba(34,197,94,0.15); }
[data-theme="dark"] .diff-line-remove { background: rgba(239,68,68,0.15); }
[data-theme="dark"] .diff-word-add { background: rgba(34,197,94,0.3); }
[data-theme="dark"] .diff-word-remove { background: rgba(239,68,68,0.3); }

</style>
<script>
// Scope navigation keyboard shortcuts (left/right arrow keys)
document.addEventListener('keydown', function(e) {
  if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.tagName === 'SELECT') return;
  var nav = document.querySelector('.scope-nav');
  if (!nav) return;
  if (e.key === 'ArrowLeft') { var btn = nav.querySelector('a.scope-nav-btn'); if (btn) btn.click(); }
  if (e.key === 'ArrowRight') { var btns = nav.querySelectorAll('a.scope-nav-btn'); if (btns.length > 0) btns[btns.length-1].click(); }
});

// Sticky detection for filter-bar
(function() {
  var filterBar = document.querySelector('.filter-bar');
  if (!filterBar) return;
  var sentinel = document.createElement('div');
  sentinel.className = 'filter-bar-sentinel';
  filterBar.parentNode.insertBefore(sentinel, filterBar);
  var observer = new IntersectionObserver(function(entries) {
    filterBar.classList.toggle('is-stuck', !entries[0].isIntersecting);
  }, { threshold: 0, rootMargin: '-58px 0px 0px 0px' });
  observer.observe(sentinel);
})();

// Mermaid diagram rendering
if (typeof mermaid !== 'undefined') {
  mermaid.initialize({ startOnLoad: false, theme: 'neutral' });
  function renderMermaid(root) {
    var nodes = (root || document).querySelectorAll('pre.mermaid:not([data-mermaid-processed])');
    if (nodes.length > 0) {
      nodes.forEach(function(n) { n.setAttribute('data-mermaid-processed', 'true'); });
      mermaid.run({ nodes: nodes });
    }
  }
  document.addEventListener('DOMContentLoaded', function() { renderMermaid(); });
  document.addEventListener('htmx:afterSettle', function(e) { renderMermaid(e.detail.target); });
}

// Checkbox toggle enhancement
function enhanceCheckboxes(root) {
  (root || document).querySelectorAll('.markdown-body input[type="checkbox"][data-cb-idx]').forEach(function(cb) {
    if (cb.dataset.enhanced) return;
    cb.dataset.enhanced = 'true';
    cb.addEventListener('change', function() {
      var container = cb.closest('[data-entity-id]');
      if (!container) return;
      var body = new FormData();
      body.append('entity_id', container.dataset.entityId);
      body.append('index', cb.dataset.cbIdx);
      fetch('/api/toggle-checkbox', { method: 'POST', body: body })
        .then(function(r) { return r.text(); })
        .then(function(html) {
          var target = cb.closest('.markdown-body');
          if (target) { target.innerHTML = html; enhanceCheckboxes(target); }
          var stats = container.querySelector('.cb-stats');
          if (stats) {
            var checked = target.querySelectorAll('input[type="checkbox"]:checked').length;
            var total = target.querySelectorAll('input[type="checkbox"][data-cb-idx]').length;
            stats.textContent = checked + '/' + total;
          }
        });
    });
  });
}
document.addEventListener('DOMContentLoaded', function() { enhanceCheckboxes(); });
document.addEventListener('htmx:afterSettle', function(e) { enhanceCheckboxes(e.detail.target); });

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
document.addEventListener('htmx:beforeSwap', function(evt) {
  // Allow 422 validation error responses to be swapped (HTMX ignores non-2xx by default).
  // The server sends HX-Retarget and HX-Reswap headers to control where/how the swap happens.
  if (evt.detail.xhr.status === 422) {
    evt.detail.shouldSwap = true;
    evt.detail.isError = false;
  }
  // Destroy SlimSelect instances before swap to prevent orphaned elements
  var target = evt.detail.target;
  if (target) {
    target.querySelectorAll('select').forEach(function(sel) {
      if (sel._slimSelect) {
        try { sel._slimSelect.destroy(); } catch(e) {}
        sel._slimSelect = null;
      }
    });
  }
});
document.addEventListener('htmx:afterSettle', function(evt) {
  enhanceSelects(evt.detail.target);
  // Scroll to first validation error if present
  var firstError = evt.detail.target.querySelector('[data-has-error]');
  if (firstError) {
    firstError.scrollIntoView({ behavior: 'smooth', block: 'center' });
    var input = firstError.querySelector('input, textarea, select');
    if (input) setTimeout(function() { input.focus(); }, 300);
  }
});
// Clear validation error styling when user modifies a field.
// Only respond to trusted (real user) events, not programmatic ones (e.g. SlimSelect init).
function clearFieldError(evt) {
  if (!evt.isTrusted) return;
  var group = evt.target.closest('.form-group.has-error');
  if (!group) return;
  group.classList.remove('has-error');
  group.removeAttribute('data-has-error');
  var err = group.querySelector('.field-error');
  if (err) err.remove();
}
document.addEventListener('input', clearFieldError);
document.addEventListener('change', clearFieldError);
document.addEventListener('htmx:responseError', function(evt) {
  var xhr = evt.detail.xhr;
  var msg = xhr.responseText || ('Request failed: ' + xhr.status);
  var div = document.createElement('div');
  div.className = 'toast toast-error';
  div.textContent = msg;
  document.body.appendChild(div);
  setTimeout(function() { div.remove(); }, 5000);
});
function confirmDelete(entityID, returnTo) {
  var existing = document.getElementById('delete-confirm-modal');
  if (existing) existing.remove();
  var overlay = document.createElement('div');
  overlay.id = 'delete-confirm-modal';
  overlay.className = 'modal-overlay';
  overlay.innerHTML = '<div class="modal" style="width:380px;">' +
    '<div class="modal-header"><h3>Confirm Delete</h3>' +
    '<button class="modal-close" onclick="this.closest(\'.modal-overlay\').remove()">&times;</button></div>' +
    '<div class="modal-body"><p>Delete <strong>' + entityID + '</strong>?</p>' +
    '<p style="font-size:13px;color:var(--text-muted);margin-top:8px;">This cannot be undone. The entity and all its relations will be permanently removed.</p></div>' +
    '<div class="modal-footer">' +
    '<button class="btn btn-secondary" onclick="this.closest(\'.modal-overlay\').remove()">Cancel</button>' +
    '<button class="btn btn-danger" id="delete-confirm-btn">Delete</button></div></div>';
  document.body.appendChild(overlay);
  overlay.addEventListener('click', function(e) { if (e.target === overlay) overlay.remove(); });
  document.getElementById('delete-confirm-btn').addEventListener('click', function() {
    htmx.ajax('POST', '/api/delete', {values: {'_entity_id': entityID, '_return_to': returnTo || ''}, swap: 'none'});
    overlay.remove();
  });
}

// --- Template switching ---
var _formDirty = false;
document.addEventListener('input', function(e) {
  if (e.target.closest('form.form-card form, .form-card form, form[hx-post]')) _formDirty = true;
});
document.addEventListener('htmx:beforeSwap', function(e) {
  if (e.detail.target && e.detail.target.id === 'content') _formDirty = false;
});

function switchTemplate(templateName) {
  var formID = document.querySelector('input[name="_form_id"]');
  if (!formID) return;
  var url = '/form/' + formID.value + '?template=' + encodeURIComponent(templateName);
  if (_formDirty && !confirm('Discard changes and switch template?')) return;
  htmx.ajax('GET', url, {target: '#content', swap: 'innerHTML'}).then(function() {
    history.pushState({}, '', url);
  });
}

// Intercept pill button switches for dirty form warning
document.addEventListener('htmx:confirm', function(e) {
  var elt = e.detail.elt;
  if (elt.classList.contains('template-pill') && !elt.classList.contains('active')) {
    if (_formDirty) {
      e.preventDefault();
      if (confirm('Discard changes and switch template?')) {
        _formDirty = false;
        htmx.trigger(elt, 'htmx:confirm');
      }
    }
  }
});

// --- Command execution ---
var _cmdToasts = {};
var _CMD_MAX_VISIBLE = 5;

function runCommand(commandID, params) {
  var btn = event.currentTarget;
  // Close parent dropdown if command was picked from a menu
  var dd = btn.closest('details.add-dropdown');
  if (dd) dd.removeAttribute('open');
  var confirmMsg = btn.getAttribute('data-confirm');
  if (confirmMsg && !window.confirm(confirmMsg)) return;

  var execID = 'cmd-' + Date.now() + '-' + Math.random().toString(36).substr(2, 6);
  var label = btn.textContent.trim();
  var autoOpen = btn.getAttribute('data-auto-open') === 'true';

  var container = document.getElementById('command-toast-container');
  var toast = _createToast(execID, label);
  container.appendChild(toast);

  var qs = new URLSearchParams(params);
  qs.set('exec_id', execID);

  _cmdToasts[execID] = { toast: toast, messages: [], logs: [], hoverPause: false, aborted: false, autoOpen: autoOpen, files: [] };

  toast.addEventListener('mouseenter', function() { _cmdToasts[execID].hoverPause = true; });
  toast.addEventListener('mouseleave', function() { _cmdToasts[execID].hoverPause = false; });

  // Use fetch+ReadableStream instead of EventSource for Wails compatibility.
  var url = '/api/command/' + encodeURIComponent(commandID) + '?' + qs.toString();
  fetch(url).then(function(resp) {
    if (!resp.ok) {
      return resp.text().then(function(t) { throw new Error(t || resp.statusText); });
    }
    var reader = resp.body.getReader();
    var decoder = new TextDecoder();
    var buf = '';
    function pump() {
      return reader.read().then(function(result) {
        if (result.done) return;
        buf += decoder.decode(result.value, {stream: true});
        var lines = buf.split('\n');
        buf = lines.pop(); // keep incomplete last line
        for (var i = 0; i < lines.length; i++) _processSSELine(execID, lines[i]);
        return pump();
      });
    }
    return pump();
  }).then(function() {
    // If stream ended without a done event, finish as success.
    var state = _cmdToasts[execID];
    if (state && !state.finished) _finishToast(execID, true);
  }).catch(function(err) {
    var state = _cmdToasts[execID];
    if (state && !state.aborted && !state.finished) {
      _addMsg(execID, {type: 'error', text: err.message || 'Connection failed'});
      _finishToast(execID, false);
    }
  });
}

// Parse SSE lines from the fetch stream.
var _sseEvent = {};
function _processSSELine(execID, line) {
  if (line.indexOf('event: ') === 0) {
    _sseEvent[execID] = line.substring(7).trim();
  } else if (line.indexOf('data: ') === 0) {
    var evtType = _sseEvent[execID] || 'message';
    var data = line.substring(6);
    _sseEvent[execID] = '';
    _dispatchSSE(execID, evtType, data);
  }
  // blank lines (SSE delimiter) are handled by clearing event state above
}

function _dispatchSSE(execID, evtType, raw) {
  var d;
  try { d = JSON.parse(raw); } catch(e) { return; }
  switch (evtType) {
    case 'message': _addMsg(execID, d); break;
    case 'file':    _addFile(execID, d); break;
    case 'entity':  _addEntity(execID, d); break;
    case 'open':    _handleOpen(d); break;
    case 'log':     _addLog(execID, d); break;
    case 'group':   _startGroup(execID, d); break;
    case 'endgroup': _endGroup(execID); break;
    case 'error':
      _addMsg(execID, {type: 'error', text: d.text || 'Command error'});
      _finishToast(execID, false);
      break;
    case 'done':
      _finishToast(execID, !!d.success);
      break;
  }
}

function cancelCommand(execID) {
  fetch('/api/command-cancel/' + execID, { method: 'POST' });
  var state = _cmdToasts[execID];
  if (!state) return;
  state.aborted = true;
  state.finished = true;
  var t = state.toast;
  t.className = 'command-toast cancelled';
  t.querySelector('.command-toast-icon').innerHTML = '&#8709;';
  var btnEl = t.querySelector('.command-toast-btn');
  btnEl.innerHTML = '&times;';
  btnEl.onclick = function() { t.remove(); delete _cmdToasts[execID]; };
  _autoHide(execID, 3000);
}

function _createToast(execID, label) {
  var t = document.createElement('div');
  t.className = 'command-toast running';
  t.id = 'toast-' + execID;
  t.innerHTML =
    '<div class="command-toast-header">' +
      '<span class="command-toast-icon"><span class="cmd-spinner"></span></span>' +
      '<span class="command-toast-label">' + _esc(label) + '</span>' +
      '<button class="command-toast-btn" onclick="cancelCommand(\'' + execID + '\')" title="Cancel">&#9632;</button>' +
    '</div>' +
    '<div class="command-toast-body" id="toast-body-' + execID + '"></div>' +
    '<div class="command-toast-log" id="toast-log-' + execID + '"></div>';
  return t;
}

function _addMsg(execID, msg) {
  var state = _cmdToasts[execID];
  if (!state) return;
  var cls = 'command-toast-msg';
  if (msg.level === 'warning') cls += ' warning';
  if (msg.type === 'error') cls += ' error-msg';
  if (msg.level === 'debug') return;
  _appendBody(execID, '<div class="' + cls + '">' + _esc(msg.text) + '</div>');
}

function _addFile(execID, msg) {
  var state = _cmdToasts[execID];
  if (!state) return;
  var label = msg.label || msg.path.split('/').pop();
  var action = msg.action || 'none';
  if (state.files) state.files.push({ path: msg.path, action: action });
  var actionHtml = '';
  if (action === 'open') {
    actionHtml = '<a href="#" onclick="event.preventDefault();_openFile(\'' + _escAttr(execID) + '\',\'' + _escAttr(msg.path) + '\',\'open\')">Open</a>' +
      '<a href="#" onclick="event.preventDefault();_openFile(\'' + _escAttr(execID) + '\',\'' + _escAttr(msg.path) + '\',\'reveal\')">Reveal</a>';
  } else if (action === 'reveal') {
    actionHtml = '<a href="#" onclick="event.preventDefault();_openFile(\'' + _escAttr(execID) + '\',\'' + _escAttr(msg.path) + '\',\'reveal\')">Reveal</a>';
  }
  _appendBody(execID, '<div class="command-toast-file"><span title="' + _escAttr(msg.path) + '">&#128196; ' + _esc(label) + '</span>' + actionHtml + '</div>');
  state.hasActions = true;
}

function _addEntity(execID, msg) {
  var state = _cmdToasts[execID];
  if (!state) return;
  var verb = msg.action || 'updated';
  var link = '/entity/' + encodeURIComponent(msg.entity_type) + '/' + encodeURIComponent(msg.id);
  _appendBody(execID,
    '<div class="command-toast-entity">' +
      '<span>' + _esc(msg.id) + ' ' + verb + '</span>' +
      '<a href="' + link + '" hx-get="' + link + '" hx-target="#content" hx-push-url="true">Go to</a>' +
    '</div>');
  state.hasActions = true;
}

function _handleOpen(msg) {
  if (msg.url) {
    fetch('/api/open-url?url=' + encodeURIComponent(msg.url), { method: 'POST' });
  }
}

function _addLog(execID, msg) {
  var state = _cmdToasts[execID];
  if (!state) return;
  state.logs.push(msg.text || '');
}

function _startGroup(execID, msg) {
  var state = _cmdToasts[execID];
  if (!state) return;
  state._groupID = 'grp-' + Date.now();
  _appendBody(execID,
    '<div class="command-toast-group-label" onclick="this.classList.toggle(\'open\')">' + _esc(msg.label || 'Group') + '</div>' +
    '<div class="command-toast-group-items" id="' + state._groupID + '"></div>');
}

function _endGroup(execID) {
  var state = _cmdToasts[execID];
  if (state) state._groupID = null;
}

function _appendBody(execID, html) {
  var state = _cmdToasts[execID];
  if (!state) return;
  state.messages.push(html);
  var target;
  if (state._groupID) {
    target = document.getElementById(state._groupID);
  }
  if (!target) {
    target = document.getElementById('toast-body-' + execID);
  }
  if (!target) return;
  // Message limiting: hide older items beyond _CMD_MAX_VISIBLE
  var body = document.getElementById('toast-body-' + execID);
  var items = body.children;
  target.insertAdjacentHTML('beforeend', html);
  // Re-check visible count (only direct children of body, not group contents)
  var directItems = [];
  for (var i = 0; i < body.children.length; i++) {
    var ch = body.children[i];
    if (!ch.classList.contains('command-toast-expand')) directItems.push(ch);
  }
  if (directItems.length > _CMD_MAX_VISIBLE + 1) {
    // Hide overflow items and show expand link
    var hidden = 0;
    for (var j = 0; j < directItems.length - _CMD_MAX_VISIBLE; j++) {
      directItems[j].style.display = 'none';
      hidden++;
    }
    var existing = body.querySelector('.command-toast-expand');
    if (existing) existing.remove();
    var expand = document.createElement('button');
    expand.className = 'command-toast-expand';
    expand.textContent = hidden + ' more messages';
    expand.onclick = function() {
      for (var k = 0; k < body.children.length; k++) body.children[k].style.display = '';
      expand.remove();
    };
    body.insertBefore(expand, body.firstChild);
  }
}

function _finishToast(execID, success) {
  var state = _cmdToasts[execID];
  if (!state || state.finished) return;
  state.finished = true;
  var t = state.toast;
  var btnEl = t.querySelector('.command-toast-btn');
  btnEl.innerHTML = '&times;';
  btnEl.onclick = function() { t.remove(); delete _cmdToasts[execID]; };

  if (success) {
    t.className = 'command-toast success';
    t.querySelector('.command-toast-icon').innerHTML = '&#10003;';
    // Auto-open: open all files with action "open" and dismiss toast.
    if (state.autoOpen && state.files && state.files.length > 0) {
      var opened = 0;
      for (var i = 0; i < state.files.length; i++) {
        var f = state.files[i];
        if (f.action === 'open') {
          fetch('/api/open-file?path=' + encodeURIComponent(f.path) + '&action=open', { method: 'POST' });
          opened++;
        }
      }
      if (opened > 0) {
        _dismissToast(execID);
        return;
      }
    }
    if (!state.hasActions) _autoHide(execID, 5000);
  } else {
    t.className = 'command-toast error';
    t.querySelector('.command-toast-icon').innerHTML = '&#10007;';
    // Show log output on error
    if (state.logs.length > 0) {
      var logEl = document.getElementById('toast-log-' + execID);
      logEl.textContent = state.logs.join('\n');
      var showBtn = document.createElement('button');
      showBtn.className = 'command-toast-expand';
      showBtn.textContent = 'Show output';
      showBtn.onclick = function() { logEl.classList.toggle('show'); showBtn.textContent = logEl.classList.contains('show') ? 'Hide output' : 'Show output'; };
      var body = document.getElementById('toast-body-' + execID);
      body.appendChild(showBtn);
    }
  }
}

function _autoHide(execID, ms) {
  setTimeout(function _tick() {
    var state = _cmdToasts[execID];
    if (!state) return;
    if (state.hoverPause) { setTimeout(_tick, 500); return; }
    var t = state.toast;
    t.style.opacity = '0';
    t.style.transition = 'opacity 0.3s';
    setTimeout(function() { t.remove(); delete _cmdToasts[execID]; }, 300);
  }, ms);
}

function _openFile(execID, path, action) {
  fetch('/api/open-file?path=' + encodeURIComponent(path) + '&action=' + encodeURIComponent(action), { method: 'POST' });
  _dismissToast(execID);
}

function _dismissToast(execID) {
  var state = _cmdToasts[execID];
  if (!state) return;
  var t = state.toast;
  t.style.opacity = '0';
  t.style.transition = 'opacity 0.3s';
  setTimeout(function() { t.remove(); delete _cmdToasts[execID]; }, 300);
}

function _esc(s) { var d = document.createElement('div'); d.textContent = s; return d.innerHTML; }
function _escAttr(s) { return s.replace(/'/g, "\\'").replace(/"/g, '&quot;'); }

// Close dropdown menus on outside click
document.addEventListener('click', function(e) {
  document.querySelectorAll('details.add-dropdown[open]').forEach(function(d) {
    if (!d.contains(e.target)) d.removeAttribute('open');
  });
});

// Live-reload: listen for server-sent events and refresh content + sidebar.
// On form pages, show a non-intrusive banner instead of refreshing.
(function() {
  var es;
  var reconnectDelay = 1000;
  function isOnForm() {
    return !!document.querySelector('#content form[hx-post]');
  }
  function doRefresh() {
    fetch(window.location.pathname + window.location.search)
      .then(function(r) { return r.text(); })
      .then(function(html) {
        var doc = new DOMParser().parseFromString(html, 'text/html');
        var content = document.getElementById('content');
        var newContent = doc.getElementById('content');
        if (content && newContent) {
          content.innerHTML = newContent.innerHTML;
          htmx.process(content);
        }
        doc.querySelectorAll('.sidebar .nav-count').forEach(function(el) {
          var link = el.closest('a');
          if (!link) return;
          var href = link.getAttribute('href');
          var cur = document.querySelector('.sidebar a[href="' + href + '"] .nav-count');
          if (cur) cur.textContent = el.textContent;
        });
      });
  }
  function showUpdateBanner() {
    if (document.getElementById('live-reload-banner')) return;
    var banner = document.createElement('div');
    banner.id = 'live-reload-banner';
    banner.style.cssText = 'position:fixed;top:0;left:0;right:0;z-index:9999;background:#1e40af;color:#fff;padding:8px 16px;display:flex;align-items:center;justify-content:center;gap:12px;font-size:14px;box-shadow:0 2px 8px rgba(0,0,0,0.15);';
    banner.innerHTML = '<span>Project files have changed.</span>'
      + '<button onclick="this.parentElement._doRefresh()" style="background:#fff;color:#1e40af;border:none;border-radius:4px;padding:4px 12px;cursor:pointer;font-weight:600;font-size:13px;">Refresh</button>'
      + '<button onclick="this.parentElement.remove()" style="background:transparent;color:rgba(255,255,255,0.8);border:1px solid rgba(255,255,255,0.3);border-radius:4px;padding:4px 12px;cursor:pointer;font-size:13px;">Dismiss</button>';
    banner._doRefresh = function() { banner.remove(); doRefresh(); };
    document.body.appendChild(banner);
  }
  function onRefresh() {
    if (isOnForm()) {
      showUpdateBanner();
    } else {
      doRefresh();
    }
  }
  function connect() {
    es = new EventSource('/api/events');
    es.addEventListener('refresh', onRefresh);
    es.onopen = function() { reconnectDelay = 1000; };
    es.onerror = function() {
      es.close();
      setTimeout(connect, reconnectDelay);
      reconnectDelay = Math.min(reconnectDelay * 2, 30000);
    };
  }
  connect();
})();

// --- Keyboard shortcuts, command palette, help modal ---
(function() {
  var _selectedRow = -1;
  var _searchSelectedResult = -1;
  var _gPending = false;
  var _gTimer = null;
  var _cmdPaletteEl = null;
  var _shortcutsEl = null;

  // --- Helpers ---
  function isInputFocused() {
    var el = document.activeElement;
    if (!el) return false;
    var tag = el.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true;
    if (el.closest && el.closest('.CodeMirror')) return true;
    if (el.isContentEditable) return true;
    return false;
  }

  function getListRows() {
    var content = document.getElementById('content');
    if (!content) return [];
    return content.querySelectorAll('table tbody tr');
  }

  function isListPage() { return getListRows().length > 0; }
  function isFormPage() { return !!document.querySelector('#content form[hx-post]'); }
  function isSearchPage() { return !!document.getElementById('search-input'); }

  // --- Row selection ---
  function updateRowSelection() {
    var rows = getListRows();
    for (var i = 0; i < rows.length; i++) {
      rows[i].classList.toggle('row-selected', i === _selectedRow);
    }
    if (_selectedRow >= 0 && _selectedRow < rows.length) {
      var row = rows[_selectedRow];
      if (row.scrollIntoView) row.scrollIntoView({block: 'nearest'});
    }
  }

  function selectRow(delta) {
    var rows = getListRows();
    if (rows.length === 0) return;
    _selectedRow = Math.max(0, Math.min(rows.length - 1, _selectedRow + delta));
    updateRowSelection();
  }

  // --- Search result selection ---
  function getSearchResults() {
    var container = document.getElementById('search-results');
    return container ? container.querySelectorAll('.card') : [];
  }

  function updateSearchSelection() {
    var results = getSearchResults();
    for (var i = 0; i < results.length; i++) {
      results[i].classList.toggle('result-selected', i === _searchSelectedResult);
    }
    if (_searchSelectedResult >= 0 && _searchSelectedResult < results.length) {
      results[_searchSelectedResult].scrollIntoView({block: 'nearest'});
    }
  }

  function selectSearchResult(delta) {
    var results = getSearchResults();
    if (results.length === 0) return;
    _searchSelectedResult = Math.max(0, Math.min(results.length - 1, _searchSelectedResult + delta));
    updateSearchSelection();
  }

  function enterSearchResults() {
    var results = getSearchResults();
    if (results.length === 0) return false;
    _searchSelectedResult = 0;
    updateSearchSelection();
    document.getElementById('search-input').blur();
    return true;
  }

  function exitSearchResults() {
    _searchSelectedResult = -1;
    updateSearchSelection();
    var input = document.getElementById('search-input');
    if (input) input.focus();
  }

  function hasSearchResultSelected() {
    return _searchSelectedResult >= 0 && getSearchResults().length > 0;
  }

  // Reset selection on HTMX content swap; auto-focus first form field
  document.addEventListener('htmx:afterSettle', function() {
    _selectedRow = -1;
    _searchSelectedResult = -1;
    if (isFormPage()) {
      var first = document.querySelector('#content form input:not([type=hidden]), #content form textarea, #content form select');
      if (first) first.focus();
    }
  });

  // --- DOM-driven shortcut scanning ---
  // Finds all <kbd> elements inside clickable parents (a, button) and builds
  // a keymap: key → clickable element. This means adding a shortcut is just
  // putting <kbd>X</kbd> inside a button — no JS changes needed.
  //
  // Returns { key: element } where key is the lowercase text content of the kbd.
  // Modifier combos (e.g. ⌘↵) are returned with a 'meta+' prefix.
  function scanKbdShortcuts() {
    var map = {};
    // Scan sidebar and #content (not the shortcuts modal or command palette)
    var scopes = [document.querySelector('.sidebar'), document.getElementById('content')];
    scopes.forEach(function(scope) {
      if (!scope) return;
      scope.querySelectorAll('kbd').forEach(function(kbd) {
        var clickable = kbd.closest('a, button');
        if (!clickable) return;
        // Skip disabled/invisible elements
        if (clickable.style.pointerEvents === 'none' || clickable.closest('[style*="pointer-events:none"]')) return;
        var raw = kbd.textContent.trim();
        if (!raw) return;
        // Normalize: detect modifier combos (⌘↵ = meta+Enter)
        var key = _normalizeKbdKey(raw);
        if (key) map[key] = clickable;
      });
    });
    return map;
  }

  // Map display symbols back to event key names
  function _normalizeKbdKey(raw) {
    // Modifier combo: ⌘↵ → meta+Enter
    if (raw === '\u2318\u21B5' || raw === '\u2318Enter') return 'meta+Enter';
    // Single chars
    var sym = {'\u21B5': 'Enter', '\u2318': 'meta', '\u232B': 'Backspace'};
    if (sym[raw]) return sym[raw];
    // Simple single-char shortcuts: N, E, H, L, /, ?
    if (raw.length === 1) return raw.toLowerCase();
    return raw.toLowerCase();
  }

  // --- Command Palette ---
  function _extractLabel(el) {
    var label = '';
    for (var n = el.firstChild; n; n = n.nextSibling) {
      if (n.nodeType === 3) label += n.textContent;
    }
    return label.replace(/[\u{1F300}-\u{1F9FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}]/gu, '').trim();
  }

  function _extractShortcut(el) {
    var kbd = el.querySelector('kbd');
    return kbd ? kbd.textContent.trim() : '';
  }

  function buildPaletteItems() {
    var items = [];
    // Navigation from sidebar links — read shortcuts from their <kbd> elements
    var navLinks = document.querySelectorAll('.sidebar nav a');
    navLinks.forEach(function(a) {
      var href = a.getAttribute('href');
      var label = _extractLabel(a);
      if (!label || !href) return;
      var icon = '&#128196;';
      if (href === '/search') icon = '&#128269;';
      else if (href === '/dashboard') icon = '&#128202;';
      else if (href === '/graph') icon = '&#128312;';
      var shortcut = _extractShortcut(a);
      items.push({section: 'Navigation', icon: icon, label: 'Go to ' + label, shortcut: shortcut, action: function() {
        a.click();
      }});
    });
    // Actions from #content — any link/button with a <kbd> becomes an action
    var content = document.getElementById('content');
    if (content) {
      content.querySelectorAll('a[href] kbd, button kbd').forEach(function(kbd) {
        var clickable = kbd.closest('a, button');
        if (!clickable) return;
        var label = _extractLabel(clickable);
        var shortcut = kbd.textContent.trim();
        if (!label) return;
        items.push({section: 'Actions', icon: '&#9654;', label: label, shortcut: shortcut, action: function() { clickable.click(); }});
      });
    }
    // Commands from current page (these don't have <kbd> but should still appear)
    var cmdLinks = document.querySelectorAll('#content .add-dropdown-menu a[onclick*="runCommand"], #content button[onclick*="runCommand"]');
    cmdLinks.forEach(function(el) {
      var label = el.textContent.trim();
      if (label) {
        items.push({section: 'Commands', icon: '&#9654;', label: label, shortcut: '', action: function() { el.click(); }});
      }
    });
    return items;
  }

  function createPalette() {
    if (_cmdPaletteEl) return;
    var overlay = document.createElement('div');
    overlay.className = 'cmd-palette-overlay';
    overlay.id = 'cmd-palette';
    overlay.style.display = 'none';
    overlay.innerHTML =
      '<div class="cmd-palette">' +
        '<div class="cmd-palette-input-wrap">' +
          '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/></svg>' +
          '<input class="cmd-palette-input" id="cmd-palette-input" placeholder="Type a command or search..." autocomplete="off">' +
        '</div>' +
        '<div class="cmd-palette-results" id="cmd-palette-results"></div>' +
        '<div class="cmd-palette-footer">' +
          '<span><kbd>&uarr;</kbd><kbd>&darr;</kbd> Navigate</span>' +
          '<span><kbd>&#8629;</kbd> Select</span>' +
          '<span><kbd>Esc</kbd> Close</span>' +
        '</div>' +
      '</div>';
    overlay.addEventListener('click', function(e) { if (e.target === overlay) togglePalette(); });
    document.body.appendChild(overlay);
    _cmdPaletteEl = overlay;
  }

  var _paletteItems = [];
  var _paletteFiltered = [];
  var _paletteIdx = 0;

  function renderPaletteResults(query) {
    var results = document.getElementById('cmd-palette-results');
    if (!results) return;
    var q = (query || '').toLowerCase();
    _paletteFiltered = q ? _paletteItems.filter(function(item) {
      return item.label.toLowerCase().indexOf(q) >= 0;
    }) : _paletteItems.slice();
    _paletteIdx = 0;
    var html = '';
    var lastSection = '';
    for (var i = 0; i < _paletteFiltered.length; i++) {
      var item = _paletteFiltered[i];
      if (item.section !== lastSection) {
        html += '<div class="cmd-palette-section">' + _esc(item.section) + '</div>';
        lastSection = item.section;
      }
      var shortcutHtml = '';
      if (item.shortcut) {
        shortcutHtml = '<div class="cmd-palette-shortcut"><kbd>' + _esc(item.shortcut) + '</kbd></div>';
      }
      html += '<div class="cmd-palette-item' + (i === 0 ? ' active' : '') + '" data-idx="' + i + '">' +
        '<div class="cmd-palette-icon">' + item.icon + '</div>' +
        '<div class="cmd-palette-label">' + _esc(item.label) + '</div>' +
        shortcutHtml +
      '</div>';
    }
    if (_paletteFiltered.length === 0) {
      html = '<div style="padding:16px;text-align:center;color:var(--text-muted);font-size:14px;">No results</div>';
    }
    results.innerHTML = html;
    results.querySelectorAll('.cmd-palette-item').forEach(function(el) {
      el.addEventListener('mouseenter', function() {
        _paletteIdx = parseInt(el.getAttribute('data-idx'));
        updatePaletteActive();
      });
      el.addEventListener('click', function() {
        executePaletteItem(_paletteIdx);
      });
    });
  }

  function updatePaletteActive() {
    var items = document.querySelectorAll('#cmd-palette-results .cmd-palette-item');
    items.forEach(function(el, i) { el.classList.toggle('active', i === _paletteIdx); });
    if (items[_paletteIdx]) items[_paletteIdx].scrollIntoView({block: 'nearest'});
  }

  function executePaletteItem(idx) {
    if (idx >= 0 && idx < _paletteFiltered.length) {
      togglePalette();
      _paletteFiltered[idx].action();
    }
  }

  function togglePalette() {
    createPalette();
    var el = _cmdPaletteEl;
    var visible = el.style.display !== 'none';
    if (visible) {
      el.style.display = 'none';
    } else {
      _paletteItems = buildPaletteItems();
      el.style.display = '';
      var input = document.getElementById('cmd-palette-input');
      input.value = '';
      renderPaletteResults('');
      setTimeout(function() { input.focus(); }, 10);
    }
  }

  function isPaletteOpen() {
    return _cmdPaletteEl && _cmdPaletteEl.style.display !== 'none';
  }

  // Palette keyboard nav
  document.addEventListener('keydown', function(e) {
    if (!isPaletteOpen()) return;
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      _paletteIdx = Math.min(_paletteIdx + 1, _paletteFiltered.length - 1);
      updatePaletteActive();
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      _paletteIdx = Math.max(_paletteIdx - 1, 0);
      updatePaletteActive();
    } else if (e.key === 'Enter') {
      e.preventDefault();
      executePaletteItem(_paletteIdx);
    }
  });
  document.addEventListener('input', function(e) {
    if (e.target.id === 'cmd-palette-input') {
      renderPaletteResults(e.target.value);
    }
  });

  // --- Shortcuts Help Modal ---
  function createShortcutsModal() {
    if (_shortcutsEl) return;
    var overlay = document.createElement('div');
    overlay.className = 'shortcuts-overlay';
    overlay.id = 'shortcuts-modal';
    overlay.style.display = 'none';
    var isMac = /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent);
    var mod = isMac ? '&#8984;' : 'Ctrl';
    overlay.innerHTML =
      '<div class="shortcuts-modal">' +
        '<div class="shortcuts-modal-header">' +
          '<h3>Keyboard shortcuts</h3>' +
          '<button class="shortcuts-modal-close" onclick="document.getElementById(\'shortcuts-modal\').style.display=\'none\'">&times;</button>' +
        '</div>' +
        '<div class="shortcuts-body">' +
          '<div class="shortcuts-group"><h4>Global</h4>' +
            _shortcutRow('Open command palette', mod + ' + K') +
            _shortcutRow('Focus search', '/') +
            _shortcutRow('Show keyboard shortcuts', '?') +
            _shortcutRow('Close modal / cancel', 'Esc') +
          '</div>' +
          '<div class="shortcuts-group"><h4>Navigation</h4>' +
            _shortcutRow('Go to Dashboard', 'G then D') +
            _shortcutRow('Go to Graph', 'G then G') +
          '</div>' +
          '<div class="shortcuts-group"><h4>List view</h4>' +
            _shortcutRow('Move selection down', 'J or &darr;') +
            _shortcutRow('Move selection up', 'K or &uarr;') +
            _shortcutRow('Open selected entity', 'Enter or O') +
            _shortcutRow('Edit selected entity', 'E') +
            _shortcutRow('Create new entity', 'N') +
            _shortcutRow('Delete selected entity', 'Del') +
            _shortcutRow('Previous page', 'H') +
            _shortcutRow('Next page', 'L') +
          '</div>' +
          '<div class="shortcuts-group"><h4>Search results</h4>' +
            _shortcutRow('Enter results from input', 'Tab or &darr;') +
            _shortcutRow('Navigate results', 'J or K') +
            _shortcutRow('Open selected result', 'Enter or O') +
            _shortcutRow('Return to search input', 'Esc or /') +
          '</div>' +
          '<div class="shortcuts-group"><h4>Entity detail</h4>' +
            _shortcutRow('Edit entity', 'E') +
          '</div>' +
          '<div class="shortcuts-group"><h4>Form / editor</h4>' +
            _shortcutRow('Save / submit', mod + ' + Enter') +
            _shortcutRow('Cancel and go back', 'Esc') +
          '</div>' +
        '</div>' +
      '</div>';
    overlay.addEventListener('click', function(e) { if (e.target === overlay) toggleShortcuts(); });
    document.body.appendChild(overlay);
    _shortcutsEl = overlay;
  }

  function _shortcutRow(label, keys) {
    return '<div class="shortcut-row"><span>' + label + '</span><div class="shortcut-keys">' +
      keys.split(' ').map(function(k) {
        if (k === 'or' || k === 'then' || k === '+') return '<span style="margin:0 2px;">' + k + '</span>';
        return '<kbd>' + k + '</kbd>';
      }).join('') +
    '</div></div>';
  }

  function toggleShortcuts() {
    createShortcutsModal();
    _shortcutsEl.style.display = _shortcutsEl.style.display === 'none' ? '' : 'none';
  }

  function isShortcutsOpen() {
    return _shortcutsEl && _shortcutsEl.style.display !== 'none';
  }

  // Expose toggles for inline onclick usage
  window._toggleCmdPalette = togglePalette;
  window._toggleShortcuts = toggleShortcuts;
  window._enterSearchResults = enterSearchResults;

  // --- Main keyboard handler ---
  document.addEventListener('keydown', function(e) {
    // Cmd/Ctrl+K: command palette (works always, even in inputs)
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault();
      if (isShortcutsOpen()) toggleShortcuts();
      togglePalette();
      return;
    }

    // Cmd/Ctrl+Enter: scan DOM for a matching <kbd> on a submit button
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
      var kbdMap = scanKbdShortcuts();
      if (kbdMap['meta+enter']) {
        e.preventDefault();
        kbdMap['meta+enter'].click();
      }
      return;
    }

    // Escape: close palette/modal, blur input, or cancel form
    if (e.key === 'Escape') {
      if (isPaletteOpen()) { togglePalette(); return; }
      if (isShortcutsOpen()) { toggleShortcuts(); return; }
      if (hasSearchResultSelected()) { exitSearchResults(); return; }
      if (isInputFocused()) { document.activeElement.blur(); return; }
      // On form, entity detail, or view pages — click the Back/Cancel button
      var backBtn = document.querySelector('#content .btn-secondary[hx-get]');
      if (backBtn) backBtn.click();
      return;
    }

    // Don't handle single-key shortcuts in palette, modals, or inputs
    if (isPaletteOpen() || isShortcutsOpen() || isInputFocused()) return;

    // --- Search results navigation (after input blur via Tab/ArrowDown) ---
    if (hasSearchResultSelected()) {
      if (e.key === 'j' || e.key === 'ArrowDown') {
        e.preventDefault();
        selectSearchResult(1);
        return;
      }
      if (e.key === 'k' || e.key === 'ArrowUp') {
        e.preventDefault();
        if (_searchSelectedResult === 0) { exitSearchResults(); return; }
        selectSearchResult(-1);
        return;
      }
      if (e.key === 'Enter' || e.key === 'o') {
        var results = getSearchResults();
        var link = results[_searchSelectedResult] && results[_searchSelectedResult].querySelector('.cell-link');
        if (link) link.click();
        return;
      }
      if (e.key === '/' || e.key === 'Tab') {
        e.preventDefault();
        exitSearchResults();
        return;
      }
      // Any printable character: refocus input and let it through
      if (e.key.length === 1 && !e.metaKey && !e.ctrlKey) {
        exitSearchResults();
        return;  // let the keydown propagate to the now-focused input
      }
      return;
    }

    // G-prefix sequences
    if (_gPending) {
      _gPending = false;
      clearTimeout(_gTimer);
      if (e.key === 'd') {
        var dashLink = document.querySelector('.sidebar nav a[href="/dashboard"]');
        if (dashLink) dashLink.click();
        return;
      }
      if (e.key === 'g') {
        var graphLink = document.querySelector('.sidebar nav a[href="/graph"]');
        if (graphLink) { window.location.href = '/graph'; }
        return;
      }
      return;
    }

    // ? = shortcuts help
    if (e.key === '?') { toggleShortcuts(); return; }

    // / = focus search (not on search page — handled via DOM <kbd> on sidebar)
    if (e.key === '/' && !isSearchPage()) {
      e.preventDefault();
      var kbdMap = scanKbdShortcuts();
      if (kbdMap['/']) { kbdMap['/'].click(); return; }
      var searchLink = document.querySelector('.sidebar nav a[href="/search"]');
      if (searchLink) searchLink.click();
      return;
    }

    // g = start G-sequence
    if (e.key === 'g') {
      _gPending = true;
      _gTimer = setTimeout(function() { _gPending = false; }, 1000);
      return;
    }

    // --- List-specific behavioral shortcuts (no DOM element to click) ---
    if (isListPage()) {
      if (e.key === 'j' || e.key === 'ArrowDown') {
        e.preventDefault();
        selectRow(_selectedRow < 0 ? 0 : 1);
        return;
      }
      if (e.key === 'k' || e.key === 'ArrowUp') {
        e.preventDefault();
        if (_selectedRow < 0) { selectRow(0); } else { selectRow(-1); }
        return;
      }
      if ((e.key === 'Enter' || e.key === 'o') && _selectedRow >= 0) {
        var rows = getListRows();
        var row = rows[_selectedRow];
        // Click first link in row (primary action)
        var link = row && (row.querySelector('.cell-link') || row.querySelector('a[href]'));
        if (link) { link.click(); return; }
        return;
      }
      if (e.key === 'e' && _selectedRow >= 0) {
        var rows = getListRows();
        var row = rows[_selectedRow];
        if (row) {
          var editHref = row.getAttribute('data-edit-href');
          if (editHref) {
            // Create a temporary HTMX link to trigger proper navigation
            var tmp = document.createElement('a');
            tmp.href = editHref;
            tmp.setAttribute('hx-get', editHref);
            tmp.setAttribute('hx-target', '#content');
            tmp.setAttribute('hx-push-url', 'true');
            tmp.style.display = 'none';
            document.body.appendChild(tmp);
            htmx.process(tmp);
            tmp.click();
            tmp.remove();
            return;
          }
          // Fallback: open detail view
          var link = row.querySelector('.cell-link');
          if (link) link.click();
        }
        return;
      }
      if ((e.key === 'Backspace' || e.key === 'Delete') && _selectedRow >= 0) {
        var rows = getListRows();
        var delIcon = rows[_selectedRow] && rows[_selectedRow].querySelector('.delete-icon');
        if (delIcon) delIcon.click();
        return;
      }
    }

    // --- DOM-driven shortcuts: scan <kbd> elements and click their parent ---
    var kbdMap = scanKbdShortcuts();
    var target = kbdMap[e.key.toLowerCase()];
    if (target) {
      e.preventDefault();
      target.click();
    }
  });
})();

// Shared EasyMDE factory - creates editor with consistent config
function createRelaEditor(element, options) {
  options = options || {};
  var toolbar = ['bold', 'italic', 'heading', '|', 'unordered-list', 'ordered-list', {
    name: 'checklist',
    action: function(editor) {
      var cm = editor.codemirror;
      var sel = cm.getSelection();
      if (sel) {
        cm.replaceSelection(sel.split('\n').map(function(l) { return '- [ ] ' + l; }).join('\n'));
      } else {
        cm.replaceSelection('- [ ] ');
      }
      cm.focus();
    },
    className: 'fa fa-check-square-o',
    title: 'Checklist (Ctrl+Shift+L)',
  }, '|', 'link', 'image', '|', 'preview', 'side-by-side'];

  // Add fullscreen toggle if callback provided
  if (options.fullscreenToggle) {
    toolbar.push('|', {
      name: 'toggle-fullscreen-editor',
      action: options.fullscreenToggle,
      className: 'fa fa-arrows-alt',
      title: 'Toggle Full Screen Editor',
    });
  }
  toolbar.push('|', 'guide');

  return new EasyMDE({
    element: element,
    spellChecker: false,
    status: false,
    minHeight: options.minHeight || '200px',
    toolbar: toolbar,
    sideBySideFullscreen: false,
  });
}

// Kanban board functions
function applyKanbanFilter(sel, kanbanId) {
  var params = new URLSearchParams(window.location.search);
  if (sel.value) {
    params.set(sel.name, sel.value);
  } else {
    params.delete(sel.name);
  }
  var url = '/kanban/' + kanbanId + (params.toString() ? '?' + params.toString() : '');
  htmx.ajax('GET', url, { target: '#content', pushUrl: true });
}

// Kanban keyboard shortcuts
document.addEventListener('keydown', function(e) {
  if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.tagName === 'SELECT' || e.target.isContentEditable) return;
  if (e.key === 'n' || e.key === 'N') {
    var btn = document.getElementById('kanban-new-btn');
    if (btn) { e.preventDefault(); btn.click(); }
  }
});

// Kanban drag and drop
(function() {
  document.addEventListener('dragstart', function(e) {
    var card = e.target.closest('.kanban-card');
    if (card) {
      e.dataTransfer.setData('text/plain', card.dataset.entityId);
      e.dataTransfer.effectAllowed = 'move';
      card.classList.add('dragging');
    }
  });

  document.addEventListener('dragend', function(e) {
    var card = e.target.closest('.kanban-card');
    if (card) {
      card.classList.remove('dragging');
    }
    document.querySelectorAll('.drag-over').forEach(function(el) {
      el.classList.remove('drag-over');
    });
  });

  document.addEventListener('dragover', function(e) {
    var target = e.target.closest('.kanban-column, .kanban-cell');
    if (target) {
      e.preventDefault();
      e.dataTransfer.dropEffect = 'move';
      document.querySelectorAll('.drag-over').forEach(function(el) {
        if (el !== target) el.classList.remove('drag-over');
      });
      target.classList.add('drag-over');
    }
  });

  document.addEventListener('dragleave', function(e) {
    var target = e.target.closest('.kanban-column, .kanban-cell');
    if (target && !target.contains(e.relatedTarget)) {
      target.classList.remove('drag-over');
    }
  });

  document.addEventListener('drop', function(e) {
    var target = e.target.closest('.kanban-column, .kanban-cell');
    if (!target) return;
    e.preventDefault();
    target.classList.remove('drag-over');

    var entityId = e.dataTransfer.getData('text/plain');
    var column = target.dataset.column;
    var swimlane = target.dataset.swimlane || '';
    var board = target.closest('.kanban-board');
    var kanbanId = board ? board.dataset.kanbanId : '';

    // Build filter params from current URL
    var params = new URLSearchParams(window.location.search);
    var filterParams = params.toString() ? '?' + params.toString() : '';

    htmx.ajax('POST', '/api/kanban/move' + filterParams, {
      values: { entity_id: entityId, column: column, swimlane: swimlane, kanban_id: kanbanId },
      target: '#content',
      swap: 'innerHTML'
    });
  });
})();
</script>
{{- end -}}

{{- define "nav-item" -}}
{{ if .Dashboard }}
    <a href="/dashboard"{{ if eq "_dashboard" .ActiveList }} class="active"{{ end }}
       hx-get="/dashboard" hx-target="#content" hx-push-url="true">
      {{ .Label }}
    </a>
{{ else if .Graph }}
    <a href="/graph"{{ if eq "_graph" .ActiveList }} class="active"{{ end }}>
      {{ .Label }}
    </a>
{{ else if .Kanban }}
    <a href="/kanban/{{ .Kanban }}"{{ if eq (printf "_kanban_%s" .Kanban) .ActiveList }} class="active"{{ end }}
       hx-get="/kanban/{{ .Kanban }}" hx-target="#content" hx-push-url="true">
      {{ .Label }}
    </a>
{{ else }}
    <a href="/list/{{ .List }}"{{ if eq .List .ActiveList }} class="active"{{ end }}
       data-entity-type="{{ .EntityType }}"
       hx-get="/list/{{ .List }}" hx-target="#content" hx-push-url="true">
      {{ .Label }}<span class="nav-count">{{ .Count }}</span>
    </a>
{{ end }}
{{- end -}}

{{- define "conflict-bar" -}}
{{ if and .ConflictCount (gt .ConflictCount 0) }}
<div class="conflict-banner" id="conflict-banner">
  <div>
    <span class="conflict-banner-icon">⚠️</span>
    <strong>{{ .ConflictCount }} file{{ if gt .ConflictCount 1 }}s{{ end }} with merge conflicts</strong> —
    <a href="/conflicts" hx-get="/conflicts" hx-target="#content" hx-push-url="true">Resolve now</a>
  </div>
  <button class="conflict-banner-close" onclick="this.parentElement.remove();document.body.classList.remove('has-conflict-banner');" title="Dismiss">×</button>
</div>
<script>document.body.classList.add('has-conflict-banner');</script>
{{ end }}
{{- end -}}

{{- define "sidebar" -}}
<aside class="sidebar">
  <div class="sidebar-header">
    <h1>{{ .App.Name }}</h1>
    {{ if .App.Description }}<p>{{ .App.Description }}</p>{{ end }}
  </div>
  <nav>
    <a href="/search"{{ if eq $.ActiveList "_search" }} class="active"{{ end }}
       hx-get="/search" hx-target="#content" hx-push-url="true">
      &#128269; Search <kbd style="margin-left:auto;">/</kbd>
    </a>
    <a href="/analyze"{{ if eq $.ActiveList "_analyze" }} class="active"{{ end }}
       hx-get="/analyze" hx-target="#content" hx-push-url="true"
       style="border-bottom:1px solid rgba(255,255,255,0.1);margin-bottom:4px;">
      &#9888; Analysis
    </a>
    {{ range .Navigation }}
    {{ if .Group }}
    <div class="nav-group">
      <button class="nav-group-toggle" onclick="toggleNavGroup(this)" data-group="{{ .Group.Group }}">
        <span class="nav-group-chevron{{ if .Group.Collapsed }} collapsed{{ end }}">&#9656;</span>
        {{ .Group.Group }}
      </button>
      <div class="nav-group-items{{ if .Group.Collapsed }} hidden{{ end }}">
        {{ range .Group.Items }}
        {{ template "nav-item" (map "Dashboard" .Dashboard "Graph" .Graph "Kanban" .Kanban "Label" .Label "List" .List "EntityType" .EntityType "Count" .Count "ActiveList" $.ActiveList) }}
        {{ end }}
      </div>
    </div>
    {{ else }}
    {{ template "nav-item" (map "Dashboard" .Item.Dashboard "Graph" .Item.Graph "Label" .Item.Label "List" .Item.List "EntityType" .Item.EntityType "Count" .Item.Count "ActiveList" $.ActiveList) }}
    {{ end }}
    {{ end }}
  </nav>
  <div class="sidebar-footer">
    <a href="/settings"{{ if eq $.ActiveList "_settings" }} class="active"{{ end }}
       hx-get="/settings" hx-target="#content" hx-push-url="true">&#9881; Settings</a>
    <button onclick="_toggleShortcuts()"><kbd>?</kbd> Shortcuts</button>
  </div>
</aside>
<button class="theme-toggle" onclick="toggleTheme()" title="Toggle dark mode">
  <span class="icon-sun">&#9788;</span>
  <span class="icon-moon">&#9790;</span>
</button>
<script>
(function(){
  var stored = localStorage.getItem('theme');
  if (stored) {
    document.documentElement.setAttribute('data-theme', stored);
  } else if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
    document.documentElement.setAttribute('data-theme', 'dark');
  }
})();
function toggleTheme() {
  var current = document.documentElement.getAttribute('data-theme');
  var next = current === 'dark' ? 'light' : 'dark';
  document.documentElement.setAttribute('data-theme', next);
  localStorage.setItem('theme', next);
}
function toggleNavGroup(btn) {
  var chevron = btn.querySelector('.nav-group-chevron');
  var items = btn.nextElementSibling;
  var isCollapsed = chevron.classList.toggle('collapsed');
  items.classList.toggle('hidden', isCollapsed);
  // Persist state to server in the background
  var group = btn.getAttribute('data-group');
  fetch('/api/ui/toggle-group', {
    method: 'POST',
    headers: {'Content-Type': 'application/x-www-form-urlencoded'},
    body: 'group=' + encodeURIComponent(group)
  });
}
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
    // Auto-expand group containing the active link
    links.forEach(function(a) {
      if (a.classList.contains('active')) {
        var group = a.closest('.nav-group-items');
        if (group && group.classList.contains('hidden')) {
          group.classList.remove('hidden');
          var chevron = group.previousElementSibling.querySelector('.nav-group-chevron');
          if (chevron) chevron.classList.remove('collapsed');
        }
      }
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
{{ template "conflict-bar" . }}
<main class="main" id="content">
{{ template "list-content" . }}
</main>
<div id="command-toast-container"></div>
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
       hx-get="/form/{{ .List.CreateForm }}" hx-target="#content" hx-push-url="true">+ New <kbd>N</kbd></a>
    {{ end }}
    {{ if .Commands }}{{ if gt (len .Commands) 2 }}
    <details class="add-dropdown">
      <summary class="btn btn-secondary btn-sm">Commands &#9662;</summary>
      <div class="add-dropdown-menu">
        {{ range .Commands }}<a href="#" onclick="event.preventDefault();runCommand('{{ .ID }}', {list_id:'{{ $.ListID }}'})" {{ if .Confirm }}data-confirm="{{ .Confirm }}"{{ end }}{{ if boolTrue .AutoOpen }} data-auto-open="true"{{ end }}>{{ .Label }}</a>
        {{ end }}
      </div>
    </details>
    {{ else }}{{ range .Commands }}
    <button class="btn btn-secondary btn-sm" onclick="runCommand('{{ .ID }}', {list_id:'{{ $.ListID }}'})" {{ if .Confirm }}data-confirm="{{ .Confirm }}"{{ end }}{{ if boolTrue .AutoOpen }} data-auto-open="true"{{ end }}>{{ .Label }}</button>
    {{ end }}{{ end }}{{ end }}
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
          <th></th>
        </tr>
      </thead>
      <tbody>
        {{ range .Rows }}
        <tr{{ if $.EditForm }} data-edit-href="/form/{{ $.EditForm }}/{{ .EntityID }}?return_to=/list/{{ $.ListID }}"{{ end }}>
          {{ $dlp := $.DetailLinkPrefix }}
          {{ range .Cells }}
          <td>
            {{ if .Link }}<a href="{{ $dlp }}{{ .EntityID }}?from={{ $.ListID }}&scope=list:{{ $.ListID }}{{ $.ScopeParams }}" class="cell-link"
               hx-get="{{ $dlp }}{{ .EntityID }}?from={{ $.ListID }}&scope=list:{{ $.ListID }}{{ $.ScopeParams }}" hx-target="#content" hx-push-url="true">{{ .Value }}</a>
            {{ else if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>
            {{ else }}{{ if .Value }}{{ .Value }}{{ else }}&mdash;{{ end }}{{ end }}
          </td>
          {{ end }}
          <td style="width:1%;white-space:nowrap;"><a href="#" class="delete-icon" title="Delete"
              onclick="event.preventDefault();confirmDelete('{{ .EntityID }}','/list/{{ $.ListID }}')"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"/></svg></a></td>
        </tr>
        {{ end }}
        {{ if not .Rows }}
        <tr><td colspan="{{ add (len .Columns) 1 }}" style="text-align:center;padding:32px;color:var(--text-muted);">No items found</td></tr>
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
         hx-include=".filter-bar select, .filter-bar input">&larr; Prev <kbd>H</kbd></a>
      {{ else }}<span class="btn btn-secondary btn-sm" style="opacity:0.4;pointer-events:none;">&larr; Prev <kbd>H</kbd></span>{{ end }}
      {{ if .NextPageURL }}<a href="{{ .NextPageURL }}" class="btn btn-secondary btn-sm"
         hx-get="{{ .NextPageURL }}" hx-target="#content" hx-push-url="true"
         hx-include=".filter-bar select, .filter-bar input">Next &rarr; <kbd>L</kbd></a>
      {{ else }}<span class="btn btn-secondary btn-sm" style="opacity:0.4;pointer-events:none;">Next &rarr; <kbd>L</kbd></span>{{ end }}
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
{{ template "conflict-bar" . }}
<main class="main{{ if .SidePanelSections }} main-with-panel{{ end }}" id="content">
{{ template "form-content" . }}
</main>
</body>
</html>
{{- end -}}

{{- define "form-content" -}}
{{ if .SidePanelSections }}<div class="form-column">{{ end }}
<div class="page-header">
  <div>
    <h2>{{ .Form.Title }}{{ if .EntityID }} — {{ .EntityID }}{{ end }}</h2>
    {{ if .Form.Description }}<p>{{ .Form.Description }}</p>{{ end }}
  </div>
  <a href="{{ .BackURL }}" class="btn btn-secondary btn-sm"
     hx-get="{{ .BackURL }}" hx-target="#content" hx-push-url="true">&larr; Back <kbd>Esc</kbd></a>
</div>

{{ if .ShowTemplates }}
<div class="template-selector">
  {{ if .UsePills }}
  <div class="template-pills">
    {{ range .Templates }}
    <button type="button" class="template-pill{{ if .Selected }} active{{ end }}"
            {{ if not .Selected }}hx-get="/form/{{ $.FormID }}?template={{ .Name }}" hx-target="#content" hx-push-url="true"{{ end }}>{{ .Label }}</button>
    {{ end }}
  </div>
  {{ else }}
  <label class="template-label">Template:</label>
  <select class="template-dropdown" onchange="switchTemplate(this.value)">
    {{ range .Templates }}
    <option value="{{ .Name }}"{{ if .Selected }} selected{{ end }}>{{ .Label }}</option>
    {{ end }}
  </select>
  {{ end }}
</div>
{{ end }}

<div class="card form-card">
  <form {{ if eq .Mode "edit" }}hx-post="/api/update"{{ else }}hx-post="/api/create"{{ end }}
        hx-swap="none" novalidate>
    <input type="hidden" name="_form_id" value="{{ .FormID }}">
    <input type="hidden" name="_entity_id" value="{{ .EntityID }}">
    <input type="hidden" name="_template" value="{{ .SelectedTemplate }}">
    {{ if .ReturnTo }}<input type="hidden" name="_return_to" value="{{ .ReturnTo }}">{{ end }}
    {{ if .LinkRelation }}<input type="hidden" name="_link_relation" value="{{ .LinkRelation }}">
    <input type="hidden" name="_link_peer" value="{{ .LinkPeer }}">
    <input type="hidden" name="_link_as" value="{{ .LinkAs }}">{{ end }}

    {{ range .Fields }}
    {{ if .Hidden }}
    <input type="hidden" name="{{ .Property }}" value="{{ .Value }}">
    {{ else if eq .Widget "checkbox" }}
    <div class="form-group{{ if .Error }} has-error{{ end }}"{{ if .Error }} data-has-error="true"{{ end }}>
      <div class="form-row-checkbox">
        <input type="checkbox" name="{{ .Property }}" value="true" id="f-{{ .Property }}"{{ if eq .Value "true" }} checked{{ end }}>
        <label for="f-{{ .Property }}">{{ .Label }}</label>
      </div>
      {{ if .Error }}<p class="field-error">{{ .Error }}</p>{{ end }}
      {{ if .Help }}<p class="help-text">{{ .Help }}</p>{{ end }}
    </div>
    {{ else if eq .Widget "textarea" }}
    <div class="form-group{{ if .Error }} has-error{{ end }}"{{ if .Error }} data-has-error="true"{{ end }}>
      <label for="f-{{ .Property }}">{{ .Label }}{{ if .Required }}<span class="required">*</span>{{ end }}</label>
      <textarea name="{{ .Property }}" id="f-{{ .Property }}" placeholder="{{ .Placeholder }}"{{ if .Required }} required{{ end }}>{{ .Value }}</textarea>
      {{ if .Error }}<p class="field-error">{{ .Error }}</p>{{ end }}
      {{ if .Help }}<p class="help-text">{{ .Help }}</p>{{ end }}
    </div>
    {{ else if or (eq .Widget "select") (eq .Widget "multi-select") }}
    <div class="form-group{{ if .Error }} has-error{{ end }}"{{ if .Error }} data-has-error="true"{{ end }}>
      <label for="f-{{ .Property }}">{{ .Label }}{{ if .Required }}<span class="required">*</span>{{ end }}</label>
      <select name="{{ .Property }}" id="f-{{ .Property }}"{{ if eq .Widget "multi-select" }} multiple{{ end }}{{ if .Required }} required{{ end }}>
        {{ if ne .Widget "multi-select" }}<option value="">Select...</option>{{ end }}
        {{ $val := .Value }}
        {{ range .Values }}<option value="{{ . }}"{{ if eq . $val }} selected{{ end }}>{{ . }}</option>{{ end }}
      </select>
      {{ if .Error }}<p class="field-error">{{ .Error }}</p>{{ end }}
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
    <div class="form-group{{ if .Error }} has-error{{ end }}"{{ if .Error }} data-has-error="true"{{ end }}>
      <label for="f-{{ .Property }}">{{ .Label }}{{ if .Required }}<span class="required">*</span>{{ end }}</label>
      <input type="{{ .InputType }}" name="{{ .Property }}" id="f-{{ .Property }}"
             placeholder="{{ .Placeholder }}" value="{{ .Value }}"{{ if .Required }} required{{ end }}>
      {{ if .Error }}<p class="field-error">{{ .Error }}</p>{{ end }}
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
      <button type="submit" class="btn btn-primary">Save Changes <kbd>&#8984;&#8629;</kbd></button>
      <button type="button" class="btn btn-danger"
              onclick="confirmDelete('{{ .EntityID }}','{{ .ReturnTo }}')">Delete</button>
      {{ else }}
      <button type="submit" class="btn btn-primary">Create <kbd>&#8984;&#8629;</kbd></button>
      {{ end }}
      <a href="{{ .BackURL }}" class="btn btn-secondary"
         hx-get="{{ .BackURL }}" hx-target="#content" hx-push-url="true">Cancel <kbd>Esc</kbd></a>
    </div>
  </form>
</div>

{{ if .SidePanelSections }}
</div>{{/* close .form-column */}}

{{/* Edge bar toggle (small screens) */}}
<div class="side-panel-edge-bar" onclick="openSidePanel()" role="button" tabindex="0" aria-label="Open side panel">
  <div class="sp-edge-icon"></div>
</div>
<div class="side-panel-overlay" onclick="closeSidePanel()"></div>

<aside class="side-panel">
  <div class="side-panel-header">
    <span class="side-panel-header-title">Context</span>
    <button type="button" class="side-panel-close-btn" onclick="closeSidePanel()" aria-label="Close side panel">&times;</button>
  </div>
  <div class="side-panel-sections">
  {{ range .SidePanelSections }}
  <div class="side-panel-section">
    <button type="button" class="side-panel-toggle" onclick="toggleSidePanel(this)">
      <span class="sp-chevron">›</span>
      {{ .Heading }}
    </button>
    <div class="side-panel-body">

    {{/* display: properties (entry source — single entity) */}}
    {{ if and (eq .Display "properties") (not .Entities) }}
    <div class="detail-grid">
      {{ range .Fields }}
      <div class="detail-label">{{ .Label }}</div>
      <div class="detail-value">
        {{ if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>
        {{ else }}{{ if .Value }}{{ formatValue .Value }}{{ else }}&mdash;{{ end }}{{ end }}
      </div>
      {{ end }}
    </div>
    {{ end }}

    {{/* display: properties (collection source — multiple entities as cards) */}}
    {{ if and (eq .Display "properties") .Entities }}
    {{ if .IsEmpty }}
    <p style="font-size:13px;color:var(--text-muted);padding:4px 0;">{{ if .EmptyMessage }}{{ .EmptyMessage }}{{ else }}No items{{ end }}</p>
    {{ else }}
    {{ range .Entities }}
    <div class="sp-card">
      <a href="/entity/{{ .Type }}/{{ .ID }}" class="sp-card-title"
         hx-get="/entity/{{ .Type }}/{{ .ID }}" hx-target="#content" hx-push-url="true">{{ .Title }}</a>
      <div class="sp-card-meta">
        {{ range .Fields }}{{ if .Value }}{{ if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>{{ else }}<span>{{ .Value }}</span>{{ end }}{{ end }}{{ end }}
      </div>
    </div>
    {{ end }}
    {{ end }}
    {{ end }}

    {{/* display: cards */}}
    {{ if eq .Display "cards" }}
    {{ if .IsEmpty }}
    <p style="font-size:13px;color:var(--text-muted);padding:4px 0;">{{ if .EmptyMessage }}{{ .EmptyMessage }}{{ else }}No items{{ end }}</p>
    {{ else }}
    {{ range .Entities }}
    <div class="sp-card">
      <a href="/entity/{{ .Type }}/{{ .ID }}" class="sp-card-title"
         hx-get="/entity/{{ .Type }}/{{ .ID }}" hx-target="#content" hx-push-url="true">{{ .Title }}</a>
      <div class="sp-card-meta">
        {{ range .Fields }}{{ if .Value }}{{ if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>{{ else }}<span>{{ .Value }}</span>{{ end }}{{ end }}{{ end }}
      </div>
      {{ if .HasContent }}
      <div class="markdown-body" style="border-top:1px solid var(--border);padding-top:6px;margin-top:6px;font-size:12px;">
        {{ renderMarkdown .Content }}
      </div>
      {{ end }}
    </div>
    {{ end }}
    {{ end }}
    {{ end }}

    {{/* display: content (entry) */}}
    {{ if and (eq .Display "content") .HasContent (not .Entities) }}
    <div class="markdown-body" style="font-size:13px;">{{ renderMarkdown .Content }}</div>
    {{ end }}

    {{/* display: content (collection) */}}
    {{ if and (eq .Display "content") .Entities }}
    {{ if .IsEmpty }}
    <p style="font-size:13px;color:var(--text-muted);padding:4px 0;">{{ if .EmptyMessage }}{{ .EmptyMessage }}{{ else }}No items{{ end }}</p>
    {{ else }}
    {{ range .Entities }}
    <div class="sp-card">
      <a href="/entity/{{ .Type }}/{{ .ID }}" class="sp-card-title"
         hx-get="/entity/{{ .Type }}/{{ .ID }}" hx-target="#content" hx-push-url="true">{{ .Title }}</a>
      {{ if .Fields }}
      <div class="sp-card-meta">
        {{ range .Fields }}{{ if .Value }}{{ if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>{{ else }}<span>{{ .Value }}</span>{{ end }}{{ end }}{{ end }}
      </div>
      {{ end }}
      {{ if .HasContent }}
      <div class="markdown-body" style="border-top:1px solid var(--border);padding-top:6px;margin-top:6px;font-size:12px;">
        {{ renderMarkdown .Content }}
      </div>
      {{ end }}
    </div>
    {{ end }}
    {{ end }}
    {{ end }}

    {{/* display: list */}}
    {{ if eq .Display "list" }}
    {{ if .IsEmpty }}
    <p style="font-size:13px;color:var(--text-muted);padding:4px 0;">{{ if .EmptyMessage }}{{ .EmptyMessage }}{{ else }}No items{{ end }}</p>
    {{ else }}
    <ul class="rel-list">
      {{ range .Entities }}
      <li>
        <a href="/entity/{{ .Type }}/{{ .ID }}" class="cell-link" style="font-size:13px;"
           hx-get="/entity/{{ .Type }}/{{ .ID }}" hx-target="#content" hx-push-url="true">{{ .Title }}</a>
        {{ range .Fields }}{{ if .Value }}{{ if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>{{ else }}<span style="font-size:12px;color:var(--text-muted);">{{ .Value }}</span>{{ end }}{{ end }}{{ end }}
      </li>
      {{ end }}
    </ul>
    {{ end }}
    {{ end }}

    {{/* display: table */}}
    {{ if eq .Display "table" }}
    {{ if .IsEmpty }}
    <p style="font-size:13px;color:var(--text-muted);padding:4px 0;">{{ if .EmptyMessage }}{{ .EmptyMessage }}{{ else }}No items{{ end }}</p>
    {{ else }}
    <div style="overflow-x:auto;margin:0 -16px;">
      <table style="font-size:13px;">
        <thead>
          <tr>{{ range .Columns }}<th style="padding:6px 12px;">{{ if .Label }}{{ .Label }}{{ else }}{{ .Property }}{{ end }}</th>{{ end }}</tr>
        </thead>
        <tbody>
          {{ range .Rows }}
          <tr>
            {{ range .Cells }}
            <td style="padding:6px 12px;">
              {{ if .Link }}<a href="/entity/{{ .EntityType }}/{{ .EntityID }}" class="cell-link" hx-get="/entity/{{ .EntityType }}/{{ .EntityID }}" hx-target="#content" hx-push-url="true">{{ .Value }}</a>
              {{ else if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>
              {{ else }}{{ if .Value }}{{ .Value }}{{ else }}&mdash;{{ end }}{{ end }}
            </td>
            {{ end }}
          </tr>
          {{ end }}
        </tbody>
      </table>
    </div>
    {{ end }}
    {{ end }}

    </div>{{/* close .side-panel-body */}}
  </div>{{/* close .side-panel-section */}}
  {{ end }}
  </div>{{/* close .side-panel-sections */}}
</aside>
{{ end }}

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
var _editorInstance = null;
(function() {
  var el = document.getElementById('body-editor');
  if (el) {
    _editorInstance = createRelaEditor(el, {
      fullscreenToggle: toggleFullscreenEditor
    });
  }
})();

// Fullscreen editor toggle
function toggleFullscreenEditor() {
  var overlay = document.getElementById('editor-fullscreen-overlay');
  if (overlay) {
    exitFullscreenEditor();
    return;
  }
  if (!_editorInstance) return;

  // Create overlay
  overlay = document.createElement('div');
  overlay.id = 'editor-fullscreen-overlay';
  overlay.className = 'editor-fullscreen-overlay';

  var header = document.createElement('div');
  header.className = 'editor-fullscreen-header';
  var title = document.createElement('h3');
  title.textContent = 'Body (Markdown)';
  var exitBtn = document.createElement('button');
  exitBtn.className = 'btn btn-secondary btn-sm';
  exitBtn.textContent = 'Exit Full Screen';
  exitBtn.onclick = exitFullscreenEditor;
  header.appendChild(title);
  header.appendChild(exitBtn);

  var body = document.createElement('div');
  body.className = 'editor-fullscreen-body';

  overlay.appendChild(header);
  overlay.appendChild(body);

  // Move the EasyMDE container into the overlay
  var container = _editorInstance.codemirror.getWrapperElement().closest('.EasyMDEContainer');
  container._originalParent = container.parentNode;
  container._originalNext = container.nextSibling;
  body.appendChild(container);

  document.body.appendChild(overlay);
  _editorInstance.codemirror.refresh();
  _editorInstance.codemirror.focus();

  // Escape key to exit
  overlay._keyHandler = function(e) {
    if (e.key === 'Escape') exitFullscreenEditor();
  };
  document.addEventListener('keydown', overlay._keyHandler);
}

function exitFullscreenEditor() {
  var overlay = document.getElementById('editor-fullscreen-overlay');
  if (!overlay || !_editorInstance) return;

  var container = _editorInstance.codemirror.getWrapperElement().closest('.EasyMDEContainer');
  // Move editor back to original location
  if (container._originalNext) {
    container._originalParent.insertBefore(container, container._originalNext);
  } else {
    container._originalParent.appendChild(container);
  }

  document.removeEventListener('keydown', overlay._keyHandler);
  overlay.remove();
  _editorInstance.codemirror.refresh();
}

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

// Side panel toggle
function toggleSidePanel(btn) {
  var chevron = btn.querySelector('.sp-chevron');
  var body = btn.nextElementSibling;
  chevron.classList.toggle('collapsed');
  body.classList.toggle('hidden');
}
function openSidePanel() {
  var p = document.querySelector('.side-panel');
  var o = document.querySelector('.side-panel-overlay');
  if (p) p.classList.add('open');
  if (o) o.classList.add('open');
  document.body.style.overflow = 'hidden';
}
function closeSidePanel() {
  var p = document.querySelector('.side-panel');
  var o = document.querySelector('.side-panel-overlay');
  if (p) p.classList.remove('open');
  if (o) o.classList.remove('open');
  document.body.style.overflow = '';
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
{{ template "conflict-bar" . }}
<main class="main" id="content">
{{ template "entity-content" . }}
</main>
<div id="command-toast-container"></div>
</body>
</html>
{{- end -}}

{{- define "scope-nav" -}}
{{ if .Scope }}
<div class="scope-nav">
  {{ if .Scope.PrevURL }}
  <a href="{{ .Scope.PrevURL }}" hx-get="{{ .Scope.PrevURL }}" hx-target="#content" hx-push-url="true" class="scope-nav-btn">&larr; Prev</a>
  {{ else }}
  <span class="scope-nav-disabled">&larr; Prev</span>
  {{ end }}
  <span class="scope-nav-progress">{{ .Scope.Progress }}</span>
  <span class="scope-nav-label">{{ .Scope.Label }}</span>
  {{ if .Scope.NextURL }}
  <a href="{{ .Scope.NextURL }}" hx-get="{{ .Scope.NextURL }}" hx-target="#content" hx-push-url="true" class="scope-nav-btn">Next &rarr;</a>
  {{ else }}
  <span class="scope-nav-disabled">Next &rarr;</span>
  {{ end }}
</div>
{{ end }}
{{- end -}}

{{- define "entity-content" -}}
{{ template "scope-nav" . }}
<div class="page-header">
  <div>
    <h2>{{ .Entity.Title }}{{ if not .Entity.Title }}{{ .Entity.ID }}{{ end }}</h2>
    <p style="font-family:var(--font-mono);font-size:13px;color:var(--text-muted);">{{ .Entity.ID }} &middot; {{ .Entity.Type }}</p>
  </div>
  <div style="display:flex;gap:8px;">
    {{ if .EditFormID }}
    <a href="/form/{{ .EditFormID }}/{{ .Entity.ID }}?return_to={{ urlquery .ReturnTo }}" class="btn btn-primary btn-sm"
       hx-get="/form/{{ .EditFormID }}/{{ .Entity.ID }}?return_to={{ urlquery .ReturnTo }}" hx-target="#content" hx-push-url="true">Edit <kbd>E</kbd></a>
    {{ end }}
    {{ if .Commands }}{{ if gt (len .Commands) 2 }}
    <details class="add-dropdown">
      <summary class="btn btn-secondary btn-sm">Commands &#9662;</summary>
      <div class="add-dropdown-menu">
        {{ range .Commands }}<a href="#" onclick="event.preventDefault();runCommand('{{ .ID }}', {entity_id:'{{ $.Entity.ID }}',entity_type:'{{ $.Entity.Type }}'})" {{ if .Confirm }}data-confirm="{{ .Confirm }}"{{ end }}{{ if boolTrue .AutoOpen }} data-auto-open="true"{{ end }}>{{ .Label }}</a>
        {{ end }}
      </div>
    </details>
    {{ else }}{{ range .Commands }}
    <button class="btn btn-secondary btn-sm" onclick="runCommand('{{ .ID }}', {entity_id:'{{ $.Entity.ID }}',entity_type:'{{ $.Entity.Type }}'})" {{ if .Confirm }}data-confirm="{{ .Confirm }}"{{ end }}{{ if boolTrue .AutoOpen }} data-auto-open="true"{{ end }}>{{ .Label }}</button>
    {{ end }}{{ end }}{{ end }}
  </div>
</div>

<div class="jump-bar">
  <a href="#properties" class="jump-link">Properties</a>
  {{ if .Relations }}<a href="#relations" class="jump-link">Relations ({{ len .Relations }})</a>{{ end }}
  {{ if .Entity.Content }}<a href="#content" class="jump-link">Content{{ with checkboxStats .Entity.Content }}{{ if gt .Total 0 }} ({{ .Checked }}/{{ .Total }}){{ end }}{{ end }}</a>{{ end }}
</div>

<div class="card" style="padding:24px;">
  <div id="properties" class="detail-grid">
    {{ $propTypes := .PropTypes }}
    {{ $props := .Entity.Properties }}
    {{ range $key := sortedKeys $props }}
    {{ $val := index $props $key }}
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
        <span style="font-size:11px;color:var(--text-muted);background:var(--bg);padding:1px 6px;border-radius:3px;">{{ .Key }}: {{ .Value }}</span>
        {{ end }}
      </li>
      {{ end }}
    </ul>
  </div>
  {{ end }}

  {{ if .Entity.Content }}
  <div id="entity-content" class="detail-section">
    <h3>Content{{ with checkboxStats .Entity.Content }}{{ if gt .Total 0 }} <span class="cb-stats">{{ .Checked }}/{{ .Total }}</span>{{ end }}{{ end }}</h3>
    <div class="markdown-body" data-entity-id="{{ .Entity.ID }}" style="padding:12px;background:var(--bg);border-radius:6px;font-size:14px;">{{ renderMarkdown .Entity.Content }}</div>
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
{{ template "conflict-bar" . }}
<main class="main" id="content">
{{ template "view-content" . }}
</main>
<div id="command-toast-container"></div>
</body>
</html>
{{- end -}}

{{- define "view-content" -}}
{{ template "scope-nav" . }}
<div class="page-header">
  <div>
    <h2>{{ .EntryTitle }}</h2>
    <p style="font-family:var(--font-mono);font-size:13px;color:var(--text-muted);">{{ .Entry.ID }} &middot; {{ .Entry.Type }} &middot; {{ .View.Title }}</p>
  </div>
  <div style="display:flex;gap:8px;">
    {{ if .EditFormID }}
    <a href="/form/{{ .EditFormID }}/{{ .Entry.ID }}?return_to={{ urlquery .ReturnTo }}" class="btn btn-primary btn-sm"
       hx-get="/form/{{ .EditFormID }}/{{ .Entry.ID }}?return_to={{ urlquery .ReturnTo }}" hx-target="#content" hx-push-url="true">Edit <kbd>E</kbd></a>
    {{ end }}
    {{ if .Commands }}{{ if gt (len .Commands) 2 }}
    <details class="add-dropdown">
      <summary class="btn btn-secondary btn-sm">Commands &#9662;</summary>
      <div class="add-dropdown-menu">
        {{ range .Commands }}<a href="#" onclick="event.preventDefault();runCommand('{{ .ID }}', {entity_id:'{{ $.Entry.ID }}',entity_type:'{{ $.Entry.Type }}',view_id:'{{ $.ViewID }}'})" {{ if .Confirm }}data-confirm="{{ .Confirm }}"{{ end }}{{ if boolTrue .AutoOpen }} data-auto-open="true"{{ end }}>{{ .Label }}</a>
        {{ end }}
      </div>
    </details>
    {{ else }}{{ range .Commands }}
    <button class="btn btn-secondary btn-sm" onclick="runCommand('{{ .ID }}', {entity_id:'{{ $.Entry.ID }}',entity_type:'{{ $.Entry.Type }}',view_id:'{{ $.ViewID }}'})" {{ if .Confirm }}data-confirm="{{ .Confirm }}"{{ end }}{{ if boolTrue .AutoOpen }} data-auto-open="true"{{ end }}>{{ .Label }}</button>
    {{ end }}{{ end }}{{ end }}
    <a href="{{ .BackURL }}" class="btn btn-secondary btn-sm"
       hx-get="{{ .BackURL }}" hx-target="#content" hx-push-url="true">&larr; Back <kbd>Esc</kbd></a>
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
    <div style="display:flex;gap:6px;">
    {{ if .LinkInfo }}
    {{ $lnk := .LinkInfo }}
    <button class="btn btn-secondary btn-sm" onclick="openLinkExisting('{{ $lnk.Relation }}','{{ $lnk.LinkAs }}','{{ $lnk.PeerID }}','{{ join $lnk.EntityTypes "," }}','{{ .SectionID }}')">&#128279; Link existing</button>
    {{ end }}
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
      <span style="font-size:11px;font-family:var(--font-mono);color:var(--text-muted);background:var(--bg);padding:1px 6px;border-radius:3px;">{{ .ID }}</span>
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
      <span style="font-size:11px;font-family:var(--font-mono);color:var(--text-muted);background:var(--bg);padding:1px 6px;border-radius:3px;">{{ .ID }}</span>
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
               hx-get="/form/{{ .EditFormID }}/{{ .EntityID }}?return_to={{ urlquery $returnTo }}" hx-target="#content" hx-push-url="true" title="Edit">&#9998;</a>
               <a href="#" class="delete-icon" title="Delete" style="margin-left:6px;"
                  onclick="event.preventDefault();confirmDelete('{{ .EntityID }}','{{ $returnTo }}')"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"/></svg></a></td>{{ end }}
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
               hx-get="/form/{{ .EditFormID }}/{{ .EntityID }}?return_to={{ urlquery $returnTo }}" hx-target="#content" hx-push-url="true" title="Edit">&#9998;</a>
               <a href="#" class="delete-icon" title="Delete" style="margin-left:6px;"
                  onclick="event.preventDefault();confirmDelete('{{ .EntityID }}','{{ $returnTo }}')"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"/></svg></a></td>
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

<div id="link-existing-modal" class="modal-overlay" style="display:none;" onclick="if(event.target===this)closeLinkExisting()">
  <div class="modal" style="width:520px;">
    <div class="modal-header">
      <h3>Link Existing</h3>
      <button class="modal-close" onclick="closeLinkExisting()">&times;</button>
    </div>
    <div class="modal-body" style="padding:12px 20px;">
      <input type="text" id="link-existing-search" class="search-input" placeholder="Search..." autocomplete="off"
             style="margin-bottom:12px;" oninput="searchLinkCandidates()">
      <div id="link-existing-results" style="max-height:320px;overflow-y:auto;"></div>
    </div>
  </div>
</div>

<script>
(function() {
  var _leRelation = '', _leLinkAs = '', _lePeer = '', _leEntityTypes = '', _leSectionID = '';
  var _leDebounce = null;

  window.openLinkExisting = function(relation, linkAs, peer, entityTypes, sectionID) {
    _leRelation = relation;
    _leLinkAs = linkAs;
    _lePeer = peer;
    _leEntityTypes = entityTypes;
    _leSectionID = sectionID;
    document.getElementById('link-existing-search').value = '';
    document.getElementById('link-existing-results').innerHTML = '<p style="color:var(--text-muted);text-align:center;">Loading...</p>';
    document.getElementById('link-existing-modal').style.display = 'flex';
    searchLinkCandidates();
    setTimeout(function() { document.getElementById('link-existing-search').focus(); }, 100);
  };

  window.closeLinkExisting = function() {
    document.getElementById('link-existing-modal').style.display = 'none';
    document.getElementById('link-existing-results').innerHTML = '';
  };

  window.searchLinkCandidates = function() {
    clearTimeout(_leDebounce);
    _leDebounce = setTimeout(function() {
      var q = document.getElementById('link-existing-search').value;
      var url = '/api/link-candidates?relation=' + encodeURIComponent(_leRelation) +
        '&link_as=' + encodeURIComponent(_leLinkAs) +
        '&peer=' + encodeURIComponent(_lePeer) +
        '&entity_types=' + encodeURIComponent(_leEntityTypes) +
        '&q=' + encodeURIComponent(q);
      fetch(url)
        .then(function(r) { return r.json(); })
        .then(function(candidates) {
          var container = document.getElementById('link-existing-results');
          if (candidates.length === 0) {
            container.innerHTML = '<p style="color:var(--text-muted);text-align:center;padding:16px 0;">No candidates found</p>';
            return;
          }
          var html = '<div style="display:flex;flex-direction:column;gap:4px;">';
          candidates.forEach(function(c) {
            html += '<div style="display:flex;align-items:center;justify-content:space-between;padding:8px 12px;border:1px solid var(--border);border-radius:6px;cursor:pointer;" ' +
              'onmouseenter="this.style.background=\'var(--primary-light)\'" onmouseleave="this.style.background=\'\'">' +
              '<div>' +
              '<div style="font-weight:500;font-size:14px;">' + escHTML(c.title) + '</div>' +
              '<div style="font-size:11px;color:var(--text-muted);font-family:var(--font-mono);">' + escHTML(c.id) + ' &middot; ' + escHTML(c.type) + '</div>' +
              '</div>' +
              '<button class="btn btn-primary btn-sm" onclick="event.stopPropagation();doLinkExisting(\'' + escAttr(c.id) + '\')">Link</button>' +
              '</div>';
          });
          html += '</div>';
          container.innerHTML = html;
        })
        .catch(function() {
          document.getElementById('link-existing-results').innerHTML = '<p style="color:var(--danger);text-align:center;">Failed to load candidates.</p>';
        });
    }, 200);
  };

  window.doLinkExisting = function(targetID) {
    var formData = new FormData();
    formData.append('relation', _leRelation);
    formData.append('link_as', _leLinkAs);
    formData.append('peer', _lePeer);
    formData.append('target', targetID);
    fetch('/api/link-existing', { method: 'POST', body: formData })
      .then(function(r) { return r.json(); })
      .then(function(data) {
        if (data.error) { alert('Error: ' + data.error); return; }
        closeLinkExisting();
        // Reload the current view to reflect the new link
        window.location.reload();
      })
      .catch(function(e) { alert('Error linking: ' + e); });
  };

  function escHTML(s) { var d = document.createElement('div'); d.textContent = s; return d.innerHTML; }
  function escAttr(s) { return s.replace(/'/g, "\\'").replace(/"/g, '&quot;'); }
})();
</script>
{{- end -}}

{{- define "search-page" -}}
<!DOCTYPE html>
<html lang="en">
<head>
<title>{{ .App.Name }} - Search</title>
{{ template "head" . }}
</head>
<body>
{{ template "sidebar" . }}
{{ template "conflict-bar" . }}
<main class="main" id="content">
{{ template "search-content" . }}
</main>
</body>
</html>
{{- end -}}

{{- define "search-content" -}}
<div class="page-header">
  <div>
    <h2>Search</h2>
    <p>Search across all entities by text, type, or property filters</p>
  </div>
</div>

<style>
.search-box { position: relative; }
.search-chips { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 8px; min-height: 0; }
.search-chips:empty { display: none; }
.search-chip { display: inline-flex; align-items: center; gap: 4px; padding: 3px 10px; border-radius: 9999px; font-size: 13px; font-family: var(--font-mono); line-height: 1.4; }
.search-chip-type { background: #dbeafe; color: #1e40af; }
.search-chip-status { background: #dcfce7; color: #166534; }
.search-chip-property { background: #e9d5ff; color: #6b21a8; }
.search-chip-sort { background: #fce7f3; color: #9d174d; }
[data-theme="dark"] .search-chip-type { background: rgba(59,130,246,0.2); color: #93c5fd; }
[data-theme="dark"] .search-chip-status { background: rgba(34,197,94,0.2); color: #86efac; }
[data-theme="dark"] .search-chip-property { background: rgba(168,85,247,0.2); color: #d8b4fe; }
[data-theme="dark"] .search-chip-sort { background: rgba(236,72,153,0.2); color: #f9a8d4; }
.search-chip button { background: none; border: none; cursor: pointer; padding: 0 0 0 2px; font-size: 15px; line-height: 1; opacity: 0.6; color: inherit; }
.search-chip button:hover { opacity: 1; }
.search-input { width: 100%; padding: 10px 12px; border: 1px solid var(--border); border-radius: 6px; font-family: var(--font); font-size: 14px; color: var(--text); background: var(--bg-card); outline: none; }
.search-input:focus { border-color: var(--primary); box-shadow: 0 0 0 2px rgba(99,102,241,0.15); }
.search-dropdown { position: absolute; left: 0; right: 0; top: 100%; margin-top: 4px; background: var(--bg-card); border: 1px solid var(--border); border-radius: 6px; box-shadow: 0 4px 12px rgba(0,0,0,0.1); max-height: 240px; overflow-y: auto; z-index: 100; display: none; }
.search-dropdown.open { display: block; }
.search-dd-item { padding: 7px 12px; font-size: 13px; cursor: pointer; display: flex; align-items: center; gap: 8px; }
.search-dd-item:first-child { border-radius: 6px 6px 0 0; }
.search-dd-item:last-child { border-radius: 0 0 6px 6px; }
.search-dd-item.active { background: var(--primary-light); color: var(--primary); }
.search-dd-item:hover { background: var(--bg); }
.search-dd-item.active:hover { background: var(--primary-light); }
.search-dd-cat { font-size: 10px; padding: 1px 6px; border-radius: 3px; text-transform: uppercase; font-weight: 600; letter-spacing: 0.5px; }
.search-dd-cat-type { background: #dbeafe; color: #1e40af; }
.search-dd-cat-status { background: #dcfce7; color: #166534; }
.search-dd-cat-property { background: #e9d5ff; color: #6b21a8; }
.search-dd-cat-sort { background: #fce7f3; color: #9d174d; }
[data-theme="dark"] .search-dd-cat-type { background: rgba(59,130,246,0.2); color: #93c5fd; }
[data-theme="dark"] .search-dd-cat-status { background: rgba(34,197,94,0.2); color: #86efac; }
[data-theme="dark"] .search-dd-cat-property { background: rgba(168,85,247,0.2); color: #d8b4fe; }
[data-theme="dark"] .search-dd-cat-sort { background: rgba(236,72,153,0.2); color: #f9a8d4; }
</style>

<div class="card" style="padding:20px;margin-bottom:20px;">
  <div class="search-box" id="search-box">
    <div class="search-chips" id="search-chips"></div>
    <input id="search-input" class="search-input" type="text" data-query="{{ .Query }}"
           placeholder="Type to search or filter..." autocomplete="off">
    <div class="search-dropdown" id="search-dropdown"></div>
  </div>
  <div style="margin-top:10px;font-size:12px;color:var(--text-muted);line-height:1.8;">
    <strong>Syntax:</strong>
    <code>type:ticket</code> filter by entity type &middot;
    <code>status:open</code> filter by status &middot;
    <code>prop:priority=high</code> filter by property &middot;
    <code>sort:priority:desc</code> sort results &middot;
    <code>"exact phrase"</code> exact match &middot;
    plain words (fuzzy, ranked)
  </div>
</div>

{{ if .ParseErrors }}
<div class="error-box">
  {{ .ParseErrors }}
</div>
{{ end }}

<div id="search-results">
{{ template "search-results" . }}
</div>

<script>
(function() {
  var input = document.getElementById('search-input');
  if (!input || input._searchInit) return;
  input._searchInit = true;

  var chipsEl = document.getElementById('search-chips');
  var dropdown = document.getElementById('search-dropdown');
  var suggestions = {{ .SuggestionsJSON }};
  var chips = [];
  var activeIdx = -1;
  var filtered = [];

  var catClass = {type: 'search-chip-type', status: 'search-chip-status', property: 'search-chip-property', sort: 'search-chip-sort'};
  var ddCatClass = {type: 'search-dd-cat-type', status: 'search-dd-cat-status', property: 'search-dd-cat-property', sort: 'search-dd-cat-sort'};

  function isFilter(val) { return /^(type|status|prop|sort):/.test(val); }

  function detectCategory(val) {
    if (val.lastIndexOf('type:', 0) === 0) return 'type';
    if (val.lastIndexOf('status:', 0) === 0) return 'status';
    if (val.lastIndexOf('prop:', 0) === 0) return 'property';
    if (val.lastIndexOf('sort:', 0) === 0) return 'sort';
    return '';
  }

  function renderChips() {
    chipsEl.innerHTML = '';
    for (var i = 0; i < chips.length; i++) {
      var c = chips[i];
      var span = document.createElement('span');
      span.className = 'search-chip ' + (catClass[c.category] || '');
      span.textContent = c.value;
      var btn = document.createElement('button');
      btn.textContent = '\u00d7';
      btn.setAttribute('data-idx', i);
      btn.onclick = function() {
        chips.splice(parseInt(this.getAttribute('data-idx')), 1);
        renderChips();
        doSearch();
        input.focus();
      };
      span.appendChild(btn);
      chipsEl.appendChild(span);
    }
  }

  function addChip(value, category) {
    for (var i = 0; i < chips.length; i++) {
      if (chips[i].value === value) return;
    }
    chips.push({value: value, category: category || detectCategory(value)});
    renderChips();
    doSearch();
  }

  function closeDropdown() {
    dropdown.classList.remove('open');
    dropdown.innerHTML = '';
    activeIdx = -1;
    filtered = [];
  }

  function showDropdown(matches) {
    filtered = matches;
    activeIdx = 0;
    dropdown.innerHTML = '';
    for (var i = 0; i < matches.length; i++) {
      var item = document.createElement('div');
      item.className = 'search-dd-item' + (i === 0 ? ' active' : '');
      item.setAttribute('data-idx', i);
      var label = document.createElement('span');
      label.textContent = matches[i].value;
      item.appendChild(label);
      var badge = document.createElement('span');
      badge.className = 'search-dd-cat ' + (ddCatClass[matches[i].category] || '');
      badge.textContent = matches[i].category;
      item.appendChild(badge);
      item.onmousedown = function(e) {
        e.preventDefault();
        var idx = parseInt(this.getAttribute('data-idx'));
        selectItem(idx);
      };
      item.onmouseenter = function() {
        setActive(parseInt(this.getAttribute('data-idx')));
      };
      dropdown.appendChild(item);
    }
    dropdown.classList.add('open');
  }

  function setActive(idx) {
    var items = dropdown.children;
    if (activeIdx >= 0 && activeIdx < items.length) items[activeIdx].classList.remove('active');
    activeIdx = idx;
    if (activeIdx >= 0 && activeIdx < items.length) {
      items[activeIdx].classList.add('active');
      items[activeIdx].scrollIntoView({block: 'nearest'});
    }
  }

  function selectItem(idx) {
    if (idx < 0 || idx >= filtered.length) return;
    var sel = filtered[idx];
    // For first-stage sort suggestions (no direction yet), advance to direction picker
    if (sel.category === 'sort' && sel.value.split(':').length === 2) {
      // Replace the current token with sort:prop: to trigger direction dropdown
      var text = input.value;
      var words = text.split(/\s+/);
      var replaced = false;
      for (var i = words.length - 1; i >= 0; i--) {
        if (!replaced && /^sort(:|$)/.test(words[i])) {
          words[i] = sel.value + ':';
          replaced = true;
        }
      }
      input.value = words.join(' ');
      closeDropdown();
      input.focus();
      updateDropdown();
      return;
    }
    addChip(sel.value, sel.category);
    // Remove the filter token from the input, keep other text
    var text = input.value;
    var words = text.split(/\s+/);
    var remaining = [];
    var removed = false;
    for (var i = 0; i < words.length; i++) {
      if (!removed && /^(type|status|prop|sort)(:|$)/.test(words[i])) {
        removed = true;
      } else {
        remaining.push(words[i]);
      }
    }
    input.value = remaining.length > 0 ? remaining.join(' ') + ' ' : '';
    closeDropdown();
    input.focus();
  }

  // Find matches for the last typed token
  function updateDropdown() {
    var text = input.value;
    var words = text.split(/\s+/);
    var last = words[words.length - 1] || '';
    if (!last || !/^(type|status|prop|sort)(:|$)/.test(last)) {
      closeDropdown();
      return;
    }
    // Second-stage: sort direction picker (e.g. "sort:priority:" -> asc/desc)
    var sortDirMatch = last.match(/^(sort:[^:]+):(.*)$/);
    if (sortDirMatch) {
      var base = sortDirMatch[1];
      var dirPrefix = sortDirMatch[2].toLowerCase();
      // Only offer directions if base matches a known sort suggestion
      var knownBase = false;
      for (var s = 0; s < suggestions.length; s++) {
        if (suggestions[s].value === base && suggestions[s].category === 'sort') { knownBase = true; break; }
      }
      if (knownBase) {
        var dirs = [{value: base + ':asc', category: 'sort'}, {value: base + ':desc', category: 'sort'}];
        var dirMatches = [];
        for (var d = 0; d < dirs.length; d++) {
          if (dirs[d].value.toLowerCase().indexOf(last.toLowerCase()) === 0) dirMatches.push(dirs[d]);
        }
        if (dirMatches.length > 0) { showDropdown(dirMatches); return; }
        closeDropdown();
        return;
      }
    }
    var matches = [];
    var q = last.toLowerCase();
    for (var i = 0; i < suggestions.length; i++) {
      if (suggestions[i].value.toLowerCase().indexOf(q) === 0) {
        // Skip already-added chips
        var dupe = false;
        for (var j = 0; j < chips.length; j++) {
          if (chips[j].value === suggestions[i].value) { dupe = true; break; }
        }
        if (!dupe) matches.push(suggestions[i]);
      }
    }
    if (matches.length > 0) {
      showDropdown(matches.slice(0, 20));
    } else {
      closeDropdown();
    }
  }

  var searchTimer = null;
  function doSearch() {
    if (searchTimer) clearTimeout(searchTimer);
    searchTimer = setTimeout(function() {
      var parts = [];
      for (var i = 0; i < chips.length; i++) parts.push(chips[i].value);
      var freeText = input.value.trim();
      if (freeText) parts.push(freeText);
      var q = parts.join(' ');
      var url = '/search?q=' + encodeURIComponent(q);
      htmx.ajax('GET', url, {target: '#search-results', swap: 'innerHTML'});
      history.replaceState(null, '', url);
    }, 300);
  }

  input.addEventListener('input', function() {
    updateDropdown();
    doSearch();
  });

  input.addEventListener('keydown', function(e) {
    var isOpen = dropdown.classList.contains('open');
    if (isOpen) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        if (activeIdx < filtered.length - 1) setActive(activeIdx + 1);
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        if (activeIdx > 0) setActive(activeIdx - 1);
      } else if (e.key === 'Enter' || e.key === 'Tab') {
        if (activeIdx >= 0 && activeIdx < filtered.length) {
          e.preventDefault();
          selectItem(activeIdx);
        }
      } else if (e.key === 'Escape') {
        e.preventDefault();
        closeDropdown();
      }
    } else {
      // Backspace on empty input removes the last chip
      if (e.key === 'Backspace' && input.value === '' && chips.length > 0) {
        chips.pop();
        renderChips();
        doSearch();
      }
      // Tab or ArrowDown with dropdown closed: enter search results
      if (e.key === 'Tab' || e.key === 'ArrowDown') {
        if (typeof _enterSearchResults === 'function' && _enterSearchResults()) {
          e.preventDefault();
        }
      }
    }
  });

  input.addEventListener('blur', function() {
    // Small delay to allow mousedown on dropdown items to fire
    setTimeout(closeDropdown, 150);
  });

  // Parse initial query: filters become chips, text stays in input
  var rawQuery = input.getAttribute('data-query') || '';
  if (rawQuery.trim()) {
    var tokens = rawQuery.match(/"[^"]*"|\S+/g) || [];
    var textParts = [];
    for (var i = 0; i < tokens.length; i++) {
      var t = tokens[i];
      if (isFilter(t)) {
        addChip(t, detectCategory(t));
      } else {
        textParts.push(t);
      }
    }
    if (textParts.length) input.value = textParts.join(' ');
  }

  setTimeout(function() { input.focus(); }, 50);
})();
</script>
{{- end -}}

{{- define "search-results" -}}
{{ if .HasQuery }}
<div style="font-size:13px;color:var(--text-muted);margin-bottom:12px;">{{ .ResultCount }} results</div>
{{ range .Results }}
<div class="card" style="padding:16px;margin-bottom:8px;">
  <div style="display:flex;align-items:center;gap:10px;margin-bottom:4px;">
    <a href="/entity/{{ .EntityType }}/{{ .ID }}?{{ $.ScopeParams }}" class="cell-link" style="font-size:15px;font-weight:600;"
       hx-get="/entity/{{ .EntityType }}/{{ .ID }}?{{ $.ScopeParams }}" hx-target="#content" hx-push-url="true">{{ .Title }}</a>
    <span style="font-size:11px;font-family:var(--font-mono);color:var(--text-muted);background:var(--bg);padding:1px 6px;border-radius:3px;">{{ .ID }}</span>
    <span class="badge badge-blue" style="font-size:10px;">{{ .EntityType }}</span>
  </div>
  {{ if .Properties }}
  <div style="display:flex;gap:12px;flex-wrap:wrap;">
    {{ range .Properties }}
    <span style="font-size:12px;color:var(--text-muted);">
      {{ .Key }}:
      {{ if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>
      {{ else }}<strong>{{ formatValue .Value }}</strong>{{ end }}
    </span>
    {{ end }}
  </div>
  {{ end }}
</div>
{{ end }}
{{ if not .Results }}
<div class="card" style="padding:32px;text-align:center;color:var(--text-muted);">No results found</div>
{{ end }}
{{ end }}
{{- end -}}

{{- define "dashboard-page" -}}
<!DOCTYPE html>
<html lang="en">
<head>
<title>{{ .App.Name }} - {{ .Dashboard.Title }}</title>
{{ template "head" . }}
</head>
<body>
{{ template "sidebar" . }}
{{ template "conflict-bar" . }}
<main class="main" id="content">
{{ template "dashboard-content" . }}
</main>
<div id="command-toast-container"></div>
</body>
</html>
{{- end -}}

{{- define "dashboard-content" -}}
<div class="page-header">
  <div>
    <h2>{{ .Dashboard.Title }}</h2>
    {{ if .Dashboard.Description }}<p>{{ .Dashboard.Description }}</p>{{ end }}
  </div>
  {{ if .Commands }}
  <div style="display:flex;gap:8px;">
    {{ if gt (len .Commands) 2 }}
    <details class="add-dropdown">
      <summary class="btn btn-secondary btn-sm">Commands &#9662;</summary>
      <div class="add-dropdown-menu">
        {{ range .Commands }}<a href="#" onclick="event.preventDefault();runCommand('{{ .ID }}', {})" {{ if .Confirm }}data-confirm="{{ .Confirm }}"{{ end }}{{ if boolTrue .AutoOpen }} data-auto-open="true"{{ end }}>{{ .Label }}</a>
        {{ end }}
      </div>
    </details>
    {{ else }}{{ range .Commands }}
    <button class="btn btn-secondary btn-sm" onclick="runCommand('{{ .ID }}', {})" {{ if .Confirm }}data-confirm="{{ .Confirm }}"{{ end }}{{ if boolTrue .AutoOpen }} data-auto-open="true"{{ end }}>{{ .Label }}</button>
    {{ end }}{{ end }}
  </div>
  {{ end }}
</div>

<div class="dashboard-grid">
{{ range .Cards }}
<div class="card dashboard-card">
  <div class="dashboard-card-header">
    <h3>{{ .Title }}</h3>
    <a href="/search?q={{ urlquery .Query }}" class="dashboard-query-link"
       hx-get="/search?q={{ urlquery .Query }}" hx-target="#content" hx-push-url="true"
       title="View in search">&#8599;</a>
  </div>

  {{ if eq .Display "count" }}
  <div class="dashboard-count">
    <span class="dashboard-count-number">{{ .Count }}</span>
  </div>

  {{ else if eq .Display "breakdown" }}
  <div class="dashboard-breakdown">
    {{ range .BreakdownItems }}
    <div class="dashboard-breakdown-row">
      <span class="dashboard-breakdown-label">
        {{ if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>
        {{ else }}{{ .Value }}{{ end }}
      </span>
      <div class="dashboard-bar-track">
        <div class="dashboard-bar-fill {{ if isBadgeType .PropType }}{{ badgeClass .PropType .Value }}{{ else }}badge-blue{{ end }}"
             style="width:{{ printf "%.0f" .Percentage }}%"></div>
      </div>
      <span class="dashboard-breakdown-count">{{ .Count }}</span>
    </div>
    {{ end }}
    {{ if not .BreakdownItems }}
    <div style="color:var(--text-muted);font-size:13px;padding:8px 0;">No data</div>
    {{ end }}
  </div>

  {{ else if eq .Display "table" }}
  {{ if .Rows }}
  <div style="overflow-x:auto;">
    <table class="dashboard-table">
      <thead>
        <tr>{{ range .Columns }}<th>{{ .Label }}</th>{{ end }}</tr>
      </thead>
      <tbody>
        {{ range .Rows }}
        <tr>
          {{ range . }}
          <td>
            {{ if .Link }}<a href="{{ .Link }}" class="cell-link"
               hx-get="{{ .Link }}" hx-target="#content" hx-push-url="true">{{ end }}
            {{ if isBadgeType .PropType }}<span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>
            {{ else }}{{ formatValue .Value }}{{ end }}
            {{ if .Link }}</a>{{ end }}
          </td>
          {{ end }}
        </tr>
        {{ end }}
      </tbody>
    </table>
  </div>
  {{ else }}
  <div style="color:var(--text-muted);font-size:13px;padding:8px 16px;">No results</div>
  {{ end }}

  {{ end }}
</div>
{{ end }}
</div>

<div class="card dashboard-validation-card" style="margin-top:20px;">
  <div class="dashboard-card-header">
    <h3>&#9888; Validation</h3>
    <a href="/analyze" class="dashboard-query-link"
       hx-get="/analyze" hx-target="#content" hx-push-url="true"
       title="View full analysis">&#8599;</a>
  </div>
  <div style="padding:16px;display:flex;align-items:center;gap:12px;">
    {{ if and (eq .AnalysisErrors 0) (eq .AnalysisWarnings 0) }}
    <span style="color:#166534;font-weight:600;font-size:14px;">&#10003; All checks passed</span>
    {{ else }}
    {{ if gt .AnalysisErrors 0 }}<span class="badge badge-red" style="font-size:12px;">{{ .AnalysisErrors }} {{ if eq .AnalysisErrors 1 }}error{{ else }}errors{{ end }}</span>{{ end }}
    {{ if gt .AnalysisWarnings 0 }}<span class="badge badge-orange" style="font-size:12px;">{{ .AnalysisWarnings }} {{ if eq .AnalysisWarnings 1 }}warning{{ else }}warnings{{ end }}</span>{{ end }}
    <a href="/analyze" style="margin-left:auto;font-size:13px;color:var(--primary);text-decoration:none;font-weight:500;"
       hx-get="/analyze" hx-target="#content" hx-push-url="true">View details &rarr;</a>
    {{ end }}
  </div>
</div>

<style>
.dashboard-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(340px, 1fr)); gap: 16px; }
.dashboard-card { padding: 0; overflow: hidden; }
.dashboard-card-header { display: flex; align-items: center; justify-content: space-between; padding: 14px 16px 10px; border-bottom: 1px solid var(--border); }
.dashboard-card-header h3 { font-size: 14px; font-weight: 600; color: var(--text); }
.dashboard-query-link { font-size: 14px; color: var(--text-muted); text-decoration: none; line-height: 1; }
.dashboard-query-link:hover { color: var(--primary); }
.dashboard-count { display: flex; align-items: center; justify-content: center; padding: 24px 16px; }
.dashboard-count-number { font-size: 48px; font-weight: 700; color: var(--text); line-height: 1; }
.dashboard-breakdown { padding: 12px 16px; }
.dashboard-breakdown-row { display: flex; align-items: center; gap: 10px; padding: 4px 0; }
.dashboard-breakdown-label { min-width: 90px; font-size: 13px; }
.dashboard-bar-track { flex: 1; height: 8px; background: var(--bg); border-radius: 4px; overflow: hidden; }
.dashboard-bar-fill { height: 100%; border-radius: 4px; transition: width 0.3s; }
.dashboard-bar-fill.badge-blue { background: #3b82f6; }
.dashboard-bar-fill.badge-purple { background: #8b5cf6; }
.dashboard-bar-fill.badge-green { background: #22c55e; }
.dashboard-bar-fill.badge-gray { background: #94a3b8; }
.dashboard-bar-fill.badge-red { background: #ef4444; }
.dashboard-bar-fill.badge-orange { background: #f97316; }
.dashboard-bar-fill.badge-yellow { background: #eab308; }
.dashboard-breakdown-count { font-size: 13px; font-weight: 600; color: var(--text); min-width: 24px; text-align: right; }
.dashboard-table { width: 100%; font-size: 13px; }
.dashboard-table thead th { padding: 8px 16px; font-size: 11px; }
.dashboard-table tbody td { padding: 6px 16px; }
</style>
{{- end -}}

{{- define "analyze-page" -}}
<!DOCTYPE html>
<html lang="en">
<head>
<title>{{ .App.Name }} - Analysis</title>
{{ template "head" . }}
</head>
<body>
{{ template "sidebar" . }}
{{ template "conflict-bar" . }}
<main class="main" id="content">
{{ template "analyze-content" . }}
</main>
<div id="command-toast-container"></div>
</body>
</html>
{{- end -}}

{{- define "analyze-content" -}}
<div class="page-header">
  <div>
    <h2>Analysis</h2>
    <p>Validation checks across all entities and relations</p>
  </div>
</div>

<div class="analyze-summary">
  {{ if and (eq .Analysis.ErrorCount 0) (eq .Analysis.WarningCount 0) }}
  <span class="analyze-chip analyze-chip-ok">&#10003; All checks passed</span>
  {{ else }}
  {{ if gt .Analysis.ErrorCount 0 }}<span class="analyze-chip analyze-chip-error">{{ .Analysis.ErrorCount }} {{ if eq .Analysis.ErrorCount 1 }}error{{ else }}errors{{ end }}</span>{{ end }}
  {{ if gt .Analysis.WarningCount 0 }}<span class="analyze-chip analyze-chip-warning">{{ .Analysis.WarningCount }} {{ if eq .Analysis.WarningCount 1 }}warning{{ else }}warnings{{ end }}</span>{{ end }}
  {{ end }}
</div>

{{ range .Analysis.Sections }}
<div class="card analyze-section">
  <div class="analyze-section-header">
    <div class="analyze-section-title">
      <h3>{{ .Name }}</h3>
      {{ if .Issues }}
      {{ if gt (.ErrorCount) 0 }}<span class="badge badge-red">{{ .ErrorCount }}</span>{{ end }}
      {{ if gt (.WarningCount) 0 }}<span class="badge badge-orange">{{ .WarningCount }}</span>{{ end }}
      {{ else }}
      <span class="badge badge-green">0</span>
      {{ end }}
    </div>
    <span class="analyze-section-desc">{{ .Description }}</span>
  </div>
  {{ if .Issues }}
  <table>
    <thead>
      <tr>
        <th>Entity</th>
        <th>Type</th>
        <th>Message</th>
        <th>Severity</th>
      </tr>
    </thead>
    <tbody>
      {{ range .Issues }}
      <tr>
        <td>{{ if .EntityID }}<a href="/entity/{{ .EntityType }}/{{ .EntityID }}" class="cell-link"
               hx-get="/entity/{{ .EntityType }}/{{ .EntityID }}" hx-target="#content" hx-push-url="true"
            >{{ if .Title }}{{ .Title }}{{ else }}{{ .EntityID }}{{ end }}</a>
            <div style="font-size:11px;color:var(--text-muted);">{{ .EntityID }}</div>
            {{ else }}&mdash;{{ end }}</td>
        <td>{{ if .EntityType }}<span class="badge badge-gray">{{ .EntityType }}</span>{{ else }}&mdash;{{ end }}</td>
        <td>{{ .Message }}</td>
        <td>{{ if eq .Severity "error" }}<span class="badge badge-red">error</span>{{ else }}<span class="badge badge-orange">warning</span>{{ end }}</td>
      </tr>
      {{ end }}
    </tbody>
  </table>
  {{ else }}
  <div class="analyze-section-ok">&#10003; No issues</div>
  {{ end }}
</div>
{{ end }}

<style>
.analyze-summary { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 20px; }
.analyze-chip { display: inline-flex; align-items: center; gap: 4px; padding: 6px 14px; border-radius: 9999px; font-size: 13px; font-weight: 600; }
.analyze-chip-error { background: var(--danger-light); color: var(--danger); }
.analyze-chip-warning { background: var(--warning-light); color: var(--warning-text); }
.analyze-chip-ok { background: rgba(34,197,94,0.15); color: #22c55e; }
[data-theme="dark"] .analyze-chip-ok { background: rgba(34,197,94,0.2); color: #4ade80; }
.analyze-section { margin-bottom: 12px; overflow: hidden; }
.analyze-section-header { padding: 14px 16px; }
.analyze-section-title { display: flex; align-items: center; gap: 8px; }
.analyze-section-title h3 { font-size: 14px; font-weight: 600; margin: 0; }
.analyze-section-desc { font-size: 12px; color: var(--text-muted); margin-top: 2px; }
.analyze-section table { font-size: 13px; border-top: 1px solid var(--border); }
.analyze-section thead th { padding: 8px 16px; font-size: 11px; }
.analyze-section tbody td { padding: 8px 16px; vertical-align: top; }
.analyze-section-ok { padding: 16px; color: #166534; font-size: 13px; font-weight: 500; border-top: 1px solid var(--border); }
</style>
{{- end -}}

{{- define "settings-page" -}}
<!DOCTYPE html>
<html lang="en">
<head>
<title>{{ .App.Name }} - Settings</title>
{{ template "head" . }}
</head>
<body>
{{ template "sidebar" . }}
{{ template "conflict-bar" . }}
<main class="main" id="content">
{{ template "settings-content" . }}
</main>
</body>
</html>
{{- end -}}

{{- define "settings-content" -}}
<div class="page-header">
  <div>
    <h2>Settings</h2>
    <p>Configure default values for new entities</p>
    <p style="font-size:12px;color:var(--text-muted);font-family:var(--font-mono);margin-top:4px;">.rela/user-defaults.yaml</p>
  </div>
</div>

<div class="card form-card" style="max-width:720px;">
  <form hx-post="/api/settings" hx-swap="none">

    <h3 style="font-size:15px;font-weight:600;margin-bottom:4px;">Property Defaults</h3>
    <p style="font-size:13px;color:var(--text-muted);margin-bottom:16px;">
      Default values applied when creating any entity type.
    </p>

    <div id="prop-defaults">
    {{ range $propName, $propVal := .UserDefaults.Defaults }}
      <div class="settings-row" data-property="{{ $propName }}">
        <span class="settings-row-label">{{ $propName }}</span>
        <div class="settings-row-value">
          {{ $found := false }}
          {{ range $.AllProperties }}{{ if eq .Name $propName }}{{ $found = true }}
            {{ if .Values }}
            <select name="default_prop[{{ $propName }}]">
              <option value="">—</option>
              {{ $matched := false }}
              {{ range .Values }}<option value="{{ . }}"{{ if eq . $propVal }} selected{{ end }}>{{ . }}</option>{{ if eq . $propVal }}{{ $matched = true }}{{ end }}{{ end }}
              {{ if and (ne $propVal "") (not $matched) }}<option value="{{ $propVal }}" selected class="stale-option">{{ $propVal }} (not in schema)</option>{{ end }}
            </select>
            {{ if and (ne $propVal "") (not $matched) }}<span class="settings-stale-badge" title="This value is no longer in the metamodel schema">stale</span>{{ end }}
            {{ else if eq .Type "boolean" }}
            <select name="default_prop[{{ $propName }}]">
              <option value="">—</option>
              <option value="true"{{ if eq $propVal "true" }} selected{{ end }}>true</option>
              <option value="false"{{ if eq $propVal "false" }} selected{{ end }}>false</option>
            </select>
            {{ else if eq .Type "date" }}
            <input type="date" name="default_prop[{{ $propName }}]" value="{{ $propVal }}">
            {{ else if eq .Type "integer" }}
            <input type="number" name="default_prop[{{ $propName }}]" value="{{ $propVal }}">
            {{ else }}
            <input type="text" name="default_prop[{{ $propName }}]" value="{{ $propVal }}">
            {{ end }}
          {{ end }}{{ end }}
          {{ if not $found }}
          <input type="text" name="default_prop[{{ $propName }}]" value="{{ $propVal }}">
          <span class="settings-stale-badge" title="This property is not in the current metamodel">unknown</span>
          {{ end }}
        </div>
        <button type="button" class="settings-row-remove" onclick="this.closest('.settings-row').remove()" title="Remove">&times;</button>
      </div>
    {{ end }}
    </div>

    <div style="margin-top:12px;">
      <select id="add-prop-select" style="width:100%;" onchange="addPropertyDefault(this)">
        <option value="">Add property default...</option>
        {{ range .AllProperties }}
        <option value="{{ .Name }}" data-type="{{ .Type }}" data-values="{{ join .Values "," }}">{{ .Name }} ({{ .Type }})</option>
        {{ end }}
      </select>
    </div>

    <p class="form-section-label">Relation Defaults</p>
    <p style="font-size:13px;color:var(--text-muted);margin-bottom:16px;">
      Default relations created when making a new entity.
    </p>

    <div id="rel-defaults">
    {{ range $relName, $relVal := .UserDefaults.RelationDefaults }}
      <div class="settings-row" data-relation="{{ $relName }}">
        <span class="settings-row-label">{{ $relName }}</span>
        <div class="settings-row-value">
          {{ $found := false }}
          {{ range $.AllRelations }}{{ if eq .Name $relName }}{{ $found = true }}
            <select name="default_rel[{{ $relName }}]">
              <option value="">—</option>
              {{ $matched := false }}
              {{ range .Targets }}<option value="{{ .ID }}"{{ if eq .ID $relVal }} selected{{ end }}>{{ .Title }}</option>{{ if eq .ID $relVal }}{{ $matched = true }}{{ end }}{{ end }}
              {{ if and (ne $relVal "") (not $matched) }}<option value="{{ $relVal }}" selected class="stale-option">{{ $relVal }} (not found)</option>{{ end }}
            </select>
            {{ if and (ne $relVal "") (not $matched) }}<span class="settings-stale-badge" title="This target entity no longer exists">stale</span>{{ end }}
          {{ end }}{{ end }}
          {{ if not $found }}
          <input type="text" name="default_rel[{{ $relName }}]" value="{{ $relVal }}" readonly>
          <span class="settings-stale-badge" title="This relation type is not in the current metamodel">unknown</span>
          {{ end }}
        </div>
        <button type="button" class="settings-row-remove" onclick="this.closest('.settings-row').remove()" title="Remove">&times;</button>
      </div>
    {{ end }}
    </div>

    <div style="margin-top:12px;">
      <select id="add-rel-select" style="width:100%;" onchange="addRelationDefault(this)">
        <option value="">Add relation default...</option>
        {{ range .AllRelations }}<option value="{{ .Name }}" data-targets="{{ json .Targets }}">{{ .Name }}{{ if .TargetType }} &rarr; {{ .TargetType }}{{ end }}</option>{{ end }}
      </select>
    </div>

    <p class="form-section-label">Overrides</p>
    <p style="font-size:13px;color:var(--text-muted);margin-bottom:16px;">
      Override defaults for specific entity types. First matching override takes precedence.
    </p>

    <div id="overrides">
    {{ range $idx, $override := .UserDefaults.Overrides }}
      <div class="override-group" data-idx="{{ $idx }}">
        <div class="override-header">
          <div style="flex:1;">
            <label style="font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:0.04em;color:var(--text-muted);margin-bottom:6px;display:block;">Entity Types</label>
            <select name="override[{{ $idx }}][types]" multiple style="width:100%;">
              {{ range $.EntityTypes }}<option value="{{ . }}"{{ if contains $override.Types . }} selected{{ end }}>{{ . }}</option>{{ end }}
              {{ range $override.Types }}{{ $t := . }}{{ $known := false }}{{ range $.EntityTypes }}{{ if eq . $t }}{{ $known = true }}{{ end }}{{ end }}{{ if not $known }}<option value="{{ $t }}" selected class="stale-option">{{ $t }} (unknown)</option>{{ end }}{{ end }}
            </select>
          </div>
          <button type="button" class="settings-row-remove" style="font-size:20px;align-self:start;margin-top:18px;" onclick="this.closest('.override-group').remove()" title="Remove group">&times;</button>
        </div>

        <div style="margin-top:12px;">
          <label style="font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:0.04em;color:var(--text-muted);margin-bottom:6px;display:block;">Properties</label>
          <div class="override-props">
          {{ range $propName, $propVal := $override.Defaults }}
            <div class="settings-row" data-property="{{ $propName }}">
              <span class="settings-row-label">{{ $propName }}</span>
              <div class="settings-row-value">
                {{ $found := false }}
                {{ range $.AllProperties }}{{ if eq .Name $propName }}{{ $found = true }}
                  {{ if .Values }}
                  <select name="override[{{ $idx }}][prop][{{ $propName }}]">
                    <option value="">—</option>
                    {{ $matched := false }}
                    {{ range .Values }}<option value="{{ . }}"{{ if eq . $propVal }} selected{{ end }}>{{ . }}</option>{{ if eq . $propVal }}{{ $matched = true }}{{ end }}{{ end }}
                    {{ if and (ne $propVal "") (not $matched) }}<option value="{{ $propVal }}" selected class="stale-option">{{ $propVal }} (not in schema)</option>{{ end }}
                  </select>
                  {{ if and (ne $propVal "") (not $matched) }}<span class="settings-stale-badge" title="This value is no longer in the metamodel schema">stale</span>{{ end }}
                  {{ else if eq .Type "boolean" }}
                  <select name="override[{{ $idx }}][prop][{{ $propName }}]">
                    <option value="">—</option>
                    <option value="true"{{ if eq $propVal "true" }} selected{{ end }}>true</option>
                    <option value="false"{{ if eq $propVal "false" }} selected{{ end }}>false</option>
                  </select>
                  {{ else if eq .Type "date" }}
                  <input type="date" name="override[{{ $idx }}][prop][{{ $propName }}]" value="{{ $propVal }}">
                  {{ else if eq .Type "integer" }}
                  <input type="number" name="override[{{ $idx }}][prop][{{ $propName }}]" value="{{ $propVal }}">
                  {{ else }}
                  <input type="text" name="override[{{ $idx }}][prop][{{ $propName }}]" value="{{ $propVal }}">
                  {{ end }}
                {{ end }}{{ end }}
                {{ if not $found }}
                <input type="text" name="override[{{ $idx }}][prop][{{ $propName }}]" value="{{ $propVal }}">
                <span class="settings-stale-badge" title="This property is not in the current metamodel">unknown</span>
                {{ end }}
              </div>
              <button type="button" class="settings-row-remove" onclick="this.closest('.settings-row').remove()" title="Remove">&times;</button>
            </div>
          {{ end }}
          </div>
          <select class="override-add-prop" style="width:100%;margin-top:8px;" onchange="addOverrideProp(this)">
            <option value="">Add property...</option>
            {{ range $.AllProperties }}<option value="{{ .Name }}" data-type="{{ .Type }}" data-values="{{ join .Values "," }}">{{ .Name }} ({{ .Type }})</option>{{ end }}
          </select>
        </div>

        <div style="margin-top:12px;">
          <label style="font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:0.04em;color:var(--text-muted);margin-bottom:6px;display:block;">Relations</label>
          <div class="override-rels">
          {{ range $relName, $relVal := $override.RelationDefaults }}
            <div class="settings-row" data-relation="{{ $relName }}">
              <span class="settings-row-label">{{ $relName }}</span>
              <div class="settings-row-value">
                {{ $found := false }}
                {{ range $.AllRelations }}{{ if eq .Name $relName }}{{ $found = true }}
                  <select name="override[{{ $idx }}][rel][{{ $relName }}]">
                    <option value="">—</option>
                    {{ $matched := false }}
                    {{ range .Targets }}<option value="{{ .ID }}"{{ if eq .ID $relVal }} selected{{ end }}>{{ .Title }}</option>{{ if eq .ID $relVal }}{{ $matched = true }}{{ end }}{{ end }}
                    {{ if and (ne $relVal "") (not $matched) }}<option value="{{ $relVal }}" selected class="stale-option">{{ $relVal }} (not found)</option>{{ end }}
                  </select>
                  {{ if and (ne $relVal "") (not $matched) }}<span class="settings-stale-badge" title="This target entity no longer exists">stale</span>{{ end }}
                {{ end }}{{ end }}
                {{ if not $found }}
                <input type="text" name="override[{{ $idx }}][rel][{{ $relName }}]" value="{{ $relVal }}" readonly>
                <span class="settings-stale-badge" title="This relation type is not in the current metamodel">unknown</span>
                {{ end }}
              </div>
              <button type="button" class="settings-row-remove" onclick="this.closest('.settings-row').remove()" title="Remove">&times;</button>
            </div>
          {{ end }}
          </div>
          <select class="override-add-rel" style="width:100%;margin-top:8px;" onchange="addOverrideRel(this)">
            <option value="">Add relation...</option>
            {{ range $.AllRelations }}<option value="{{ .Name }}" data-targets="{{ json .Targets }}">{{ .Name }}{{ if .TargetType }} &rarr; {{ .TargetType }}{{ end }}</option>{{ end }}
          </select>
        </div>
      </div>
    {{ end }}
    </div>

    <button type="button" class="btn btn-secondary btn-sm" style="margin-top:16px;" onclick="addOverrideGroup()">+ Add override group</button>

    <div class="form-actions">
      <button type="submit" class="btn btn-primary">Save</button>
      <a href="/settings" class="btn btn-secondary">Reset</a>
    </div>
  </form>
</div>

<script>
var overrideCounter = {{ len .UserDefaults.Overrides }};
var allPropertiesJSON = {{ jsJSON .AllProperties }};
var allRelationsJSON = {{ jsJSON .AllRelations }};
var entityTypesJSON = {{ jsJSON .EntityTypes }};

function makeValueInput(name, propInfo, currentVal) {
  if (propInfo && propInfo.Values && propInfo.Values.length > 0) {
    var sel = '<select name="' + name + '"><option value="">—</option>';
    propInfo.Values.forEach(function(v) {
      sel += '<option value="' + v + '"' + (v === currentVal ? ' selected' : '') + '>' + v + '</option>';
    });
    sel += '</select>';
    return sel;
  }
  if (propInfo) {
    if (propInfo.Type === 'boolean') {
      return '<select name="' + name + '"><option value="">—</option>' +
        '<option value="true"' + (currentVal === 'true' ? ' selected' : '') + '>true</option>' +
        '<option value="false"' + (currentVal === 'false' ? ' selected' : '') + '>false</option></select>';
    }
    if (propInfo.Type === 'date') {
      return '<input type="date" name="' + name + '" value="' + (currentVal || '') + '">';
    }
    if (propInfo.Type === 'integer') {
      return '<input type="number" name="' + name + '" value="' + (currentVal || '') + '">';
    }
  }
  return '<input type="text" name="' + name + '" value="' + (currentVal || '') + '">';
}

function makeRelationSelect(name, targets, currentVal) {
  var sel = '<select name="' + name + '"><option value="">—</option>';
  (targets || []).forEach(function(t) {
    sel += '<option value="' + t.ID + '"' + (t.ID === currentVal ? ' selected' : '') + '>' + t.Title + '</option>';
  });
  sel += '</select>';
  return sel;
}

function findPropInfo(propName) {
  for (var i = 0; i < allPropertiesJSON.length; i++) {
    if (allPropertiesJSON[i].Name === propName) return allPropertiesJSON[i];
  }
  return null;
}

function findRelInfo(relName) {
  for (var i = 0; i < allRelationsJSON.length; i++) {
    if (allRelationsJSON[i].Name === relName) return allRelationsJSON[i];
  }
  return null;
}

function addPropertyDefault(selectEl) {
  var propName = selectEl.value;
  if (!propName) return;
  selectEl.value = '';
  if (selectEl._slimSelect) selectEl._slimSelect.setSelected('');
  var container = document.getElementById('prop-defaults');
  if (container.querySelector('[data-property="' + propName + '"]')) return;
  var propInfo = findPropInfo(propName);
  var html = '<div class="settings-row" data-property="' + propName + '">' +
    '<span class="settings-row-label">' + propName + '</span>' +
    '<div class="settings-row-value">' + makeValueInput('default_prop[' + propName + ']', propInfo, '') + '</div>' +
    '<button type="button" class="settings-row-remove" onclick="this.closest(\'.settings-row\').remove()" title="Remove">&times;</button>' +
    '</div>';
  container.insertAdjacentHTML('beforeend', html);
  enhanceSelects(container.lastElementChild);
}

function addRelationDefault(selectEl) {
  var relName = selectEl.value;
  if (!relName) return;
  selectEl.value = '';
  if (selectEl._slimSelect) selectEl._slimSelect.setSelected('');
  var container = document.getElementById('rel-defaults');
  if (container.querySelector('[data-relation="' + relName + '"]')) return;
  var relInfo = findRelInfo(relName);
  var targets = relInfo ? relInfo.Targets : [];
  var html = '<div class="settings-row" data-relation="' + relName + '">' +
    '<span class="settings-row-label">' + relName + '</span>' +
    '<div class="settings-row-value">' + makeRelationSelect('default_rel[' + relName + ']', targets, '') + '</div>' +
    '<button type="button" class="settings-row-remove" onclick="this.closest(\'.settings-row\').remove()" title="Remove">&times;</button>' +
    '</div>';
  container.insertAdjacentHTML('beforeend', html);
  enhanceSelects(container.lastElementChild);
}

function addOverrideProp(selectEl) {
  var propName = selectEl.value;
  if (!propName) return;
  selectEl.value = '';
  if (selectEl._slimSelect) selectEl._slimSelect.setSelected('');
  var group = selectEl.closest('.override-group');
  var idx = group.getAttribute('data-idx');
  var container = group.querySelector('.override-props');
  if (container.querySelector('[data-property="' + propName + '"]')) return;
  var propInfo = findPropInfo(propName);
  var html = '<div class="settings-row" data-property="' + propName + '">' +
    '<span class="settings-row-label">' + propName + '</span>' +
    '<div class="settings-row-value">' + makeValueInput('override[' + idx + '][prop][' + propName + ']', propInfo, '') + '</div>' +
    '<button type="button" class="settings-row-remove" onclick="this.closest(\'.settings-row\').remove()" title="Remove">&times;</button>' +
    '</div>';
  container.insertAdjacentHTML('beforeend', html);
  enhanceSelects(container.lastElementChild);
}

function addOverrideRel(selectEl) {
  var relName = selectEl.value;
  if (!relName) return;
  selectEl.value = '';
  if (selectEl._slimSelect) selectEl._slimSelect.setSelected('');
  var group = selectEl.closest('.override-group');
  var idx = group.getAttribute('data-idx');
  var container = group.querySelector('.override-rels');
  if (container.querySelector('[data-relation="' + relName + '"]')) return;
  var relInfo = findRelInfo(relName);
  var targets = relInfo ? relInfo.Targets : [];
  var html = '<div class="settings-row" data-relation="' + relName + '">' +
    '<span class="settings-row-label">' + relName + '</span>' +
    '<div class="settings-row-value">' + makeRelationSelect('override[' + idx + '][rel][' + relName + ']', targets, '') + '</div>' +
    '<button type="button" class="settings-row-remove" onclick="this.closest(\'.settings-row\').remove()" title="Remove">&times;</button>' +
    '</div>';
  container.insertAdjacentHTML('beforeend', html);
  enhanceSelects(container.lastElementChild);
}

function addOverrideGroup() {
  var idx = overrideCounter++;
  var html = '<div class="override-group" data-idx="' + idx + '">' +
    '<div class="override-header">' +
    '<div style="flex:1;">' +
    '<label style="font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:0.04em;color:var(--text-muted);margin-bottom:6px;display:block;">Entity Types</label>' +
    '<select name="override[' + idx + '][types]" multiple style="width:100%;">';
  entityTypesJSON.forEach(function(t) {
    html += '<option value="' + t + '">' + t + '</option>';
  });
  html += '</select></div>' +
    '<button type="button" class="settings-row-remove" style="font-size:20px;align-self:start;margin-top:18px;" onclick="this.closest(\'.override-group\').remove()" title="Remove group">&times;</button>' +
    '</div>' +
    '<div style="margin-top:12px;">' +
    '<label style="font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:0.04em;color:var(--text-muted);margin-bottom:6px;display:block;">Properties</label>' +
    '<div class="override-props"></div>' +
    '<select class="override-add-prop" style="width:100%;margin-top:8px;" onchange="addOverrideProp(this)"><option value="">Add property...</option>';
  allPropertiesJSON.forEach(function(p) {
    html += '<option value="' + p.Name + '" data-type="' + p.Type + '" data-values="' + (p.Values||[]).join(',') + '">' + p.Name + ' (' + p.Type + ')</option>';
  });
  html += '</select></div>' +
    '<div style="margin-top:12px;">' +
    '<label style="font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:0.04em;color:var(--text-muted);margin-bottom:6px;display:block;">Relations</label>' +
    '<div class="override-rels"></div>' +
    '<select class="override-add-rel" style="width:100%;margin-top:8px;" onchange="addOverrideRel(this)"><option value="">Add relation...</option>';
  allRelationsJSON.forEach(function(r) {
    html += '<option value="' + r.Name + '">'+r.Name + (r.TargetType ? ' → ' + r.TargetType : '') + '</option>';
  });
  html += '</select></div></div>';
  document.getElementById('overrides').insertAdjacentHTML('beforeend', html);
  var lastGroup = document.getElementById('overrides').lastElementChild;
  enhanceSelects(lastGroup);
}
</script>
{{- end -}}

{{- define "settings-head" -}}
<style>
.settings-row { display: flex; align-items: center; gap: 8px; padding: 8px 0; border-bottom: 1px solid var(--border); }
.settings-row:first-child { border-top: 1px solid var(--border); }
.settings-row-label { font-size: 13px; font-weight: 500; font-family: var(--font-mono); color: var(--text); min-width: 140px; flex-shrink: 0; }
.settings-row-value { flex: 1; }
.settings-row-value select, .settings-row-value input { width: 100%; padding: 6px 10px; border: 1px solid var(--border); border-radius: 6px; font-size: 13px; font-family: var(--font); background: var(--bg-card); color: var(--text); }
.settings-row-value select:focus, .settings-row-value input:focus { outline: none; border-color: var(--primary); box-shadow: 0 0 0 3px var(--primary-light); }
.settings-row-remove { background: none; border: none; cursor: pointer; color: var(--text-muted); font-size: 18px; padding: 4px; border-radius: 4px; transition: all 0.15s; line-height: 1; flex-shrink: 0; }
.settings-row-remove:hover { color: var(--danger); background: var(--danger-light); }
.override-group { border: 1px solid var(--border); border-radius: var(--radius); padding: 20px; margin-top: 12px; background: var(--bg); }
.override-header { display: flex; align-items: start; gap: 12px; }
.settings-stale-badge { display: inline-block; font-size: 10px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.03em; padding: 2px 6px; border-radius: 4px; background: #fef3c7; color: #92400e; border: 1px solid #fcd34d; margin-left: 6px; flex-shrink: 0; white-space: nowrap; }
.stale-option { color: #92400e; font-style: italic; }
</style>
{{- end -}}

{{- define "conflicts-page" -}}
<!DOCTYPE html>
<html lang="en">
<head>
<title>{{ .App.Name }} - Conflicts</title>
{{ template "head" . }}
</head>
<body>
{{ template "sidebar" . }}
{{ template "conflict-bar" . }}
<main class="main" id="content">
{{ template "conflicts-content" . }}
</main>
<div id="command-toast-container"></div>
</body>
</html>
{{- end -}}

{{- define "conflicts-content" -}}
<div class="page-header">
  <div>
    <h2>Merge Conflicts</h2>
    <p>Files with unresolved git conflicts</p>
  </div>
</div>

{{ if .HasConflicts }}
<div class="conflict-summary">
  <span class="conflict-chip conflict-chip-warning">⚠ {{ len .Conflicts }} file{{ if gt (len .Conflicts) 1 }}s{{ end }} with conflicts</span>
</div>

<div class="card" style="margin-top: 16px;">
  <table>
    <thead>
      <tr>
        <th>File</th>
        <th>Type</th>
        <th>Conflicts</th>
      </tr>
    </thead>
    <tbody>
      {{ range .Conflicts }}
      <tr>
        <td>
          <a href="/conflicts/{{ .RelPath }}" class="conflict-path-link"
             hx-get="/conflicts/{{ .RelPath }}" hx-target="#content" hx-push-url="true">
            <div class="conflict-path">{{ .RelPath }}</div>
            {{ if .EntityID }}<div class="conflict-id">{{ .EntityID }}</div>{{ end }}
          </a>
        </td>
        <td>{{ if .EntityType }}<span class="badge badge-gray">{{ .EntityType }}</span>{{ else }}<span class="badge badge-purple">relation</span>{{ end }}</td>
        <td><span class="badge badge-orange">{{ .MarkerCount }}</span></td>
      </tr>
      {{ end }}
    </tbody>
  </table>
</div>
{{ else }}
<div class="conflict-empty">
  <div class="conflict-empty-icon">✓</div>
  <h3>No conflicts detected</h3>
  <p>All entity and relation files are clean.</p>
</div>
{{ end }}
{{- end -}}


{{- define "conflict-resolve-page" -}}
<!DOCTYPE html>
<html lang="en">
<head>
<title>{{ .App.Name }} - Resolve Conflict</title>
{{ template "head" . }}
</head>
<body>
{{ template "sidebar" . }}
{{ template "conflict-bar" . }}
<main class="main" id="content">
{{ template "conflict-resolve-content" . }}
</main>
<div id="command-toast-container"></div>
</body>
</html>
{{- end -}}

{{- define "conflict-resolve-content" -}}
<div class="page-header">
  <div>
    <h2>Resolve Conflict</h2>
    <p>{{ .Resolution.RelPath }}</p>
  </div>
  <a href="/conflicts" class="btn btn-secondary" hx-get="/conflicts" hx-target="#content" hx-push-url="true">← Back to Conflicts</a>
</div>

<form method="POST" action="/api/conflict-resolve" id="resolve-form">
  <input type="hidden" name="path" value="{{ .Resolution.RelPath }}">

  <div class="resolve-actions-top">
    <button type="button" class="btn btn-secondary" onclick="selectAllSide('ours')">Select All Ours <kbd>O</kbd></button>
    <button type="button" class="btn btn-secondary" onclick="selectAllSide('theirs')">Select All Theirs <kbd>T</kbd></button>
  </div>

  <div class="card resolve-card">
    <h3>Properties</h3>
    <table class="resolve-table">
      <thead>
        <tr>
          <th>Property</th>
          <th>Ours (HEAD)</th>
          <th>Theirs</th>
        </tr>
      </thead>
      <tbody>
        {{ range .Resolution.Info.PropertyDiffs }}
        <tr class="{{ if .IsSame }}resolve-same{{ else }}resolve-diff{{ end }}" data-prop="{{ .Property }}">
          <td class="resolve-prop-name">{{ .Property }}</td>
          {{ if .IsSame }}
          <td class="resolve-value resolve-value-same" colspan="2">{{ if .OursValue }}{{ .OursValue }}{{ else }}<em>empty</em>{{ end }}</td>
          <input type="hidden" name="prop_{{ .Property }}" value="ours">
          {{ else }}
          <td class="resolve-value resolve-value-selectable resolve-value-selected" data-side="ours" data-prop="{{ .Property }}">
            <input type="radio" name="prop_{{ .Property }}" value="ours" checked class="resolve-radio-hidden">
            <span class="resolve-value-text">{{ if .OursValue }}{{ .OursValue }}{{ else }}<em>empty</em>{{ end }}</span>
          </td>
          <td class="resolve-value resolve-value-selectable resolve-value-unselected" data-side="theirs" data-prop="{{ .Property }}">
            <input type="radio" name="prop_{{ .Property }}" value="theirs" class="resolve-radio-hidden">
            <span class="resolve-value-text">{{ if .TheirsValue }}{{ .TheirsValue }}{{ else }}<em>empty</em>{{ end }}</span>
          </td>
          {{ end }}
        </tr>
        {{ end }}
      </tbody>
    </table>
  </div>

  {{ if not .Resolution.Info.ContentSame }}
  <div class="card resolve-card">
    <h3>Content</h3>
    <div class="resolve-content-choice">
      <label><input type="radio" name="content" value="ours" checked> Use Ours</label>
      <label><input type="radio" name="content" value="theirs"> Use Theirs</label>
      <label><input type="radio" name="content" value="manual" id="content-manual-radio"> Edit Manually</label>
    </div>
    <div class="resolve-content-compare">
      <div class="resolve-content-side">
        <div class="resolve-content-label">Ours (HEAD)</div>
        <pre class="resolve-content-pre" id="diff-ours">{{ .Resolution.Info.ContentDiffOurs }}</pre>
      </div>
      <div class="resolve-content-side">
        <div class="resolve-content-label">Theirs</div>
        <pre class="resolve-content-pre" id="diff-theirs">{{ .Resolution.Info.ContentDiffTheirs }}</pre>
      </div>
    </div>
    <div class="resolve-manual-edit" id="manual-edit-container" style="display:none;">
      <label>Manual Content</label>
      <textarea name="manual_content" id="manual-content-editor" rows="10">{{ .Resolution.Info.ContentDiffOurs }}</textarea>
    </div>
  </div>
  {{ else }}
  <input type="hidden" name="content" value="ours">
  {{ end }}

  <div class="resolve-actions-bottom">
    <button type="submit" name="action" value="custom" class="btn btn-primary">Apply Resolution <kbd>&#8984;&#8629;</kbd></button>
  </div>
</form>

<script>
(function() {
  var manualRadio = document.getElementById('content-manual-radio');
  var container = document.getElementById('manual-edit-container');
  var editorEl = document.getElementById('manual-content-editor');
  var editorInstance = null;

  function showManualEdit() {
    container.style.display = 'block';
    if (!editorInstance && editorEl) {
      editorInstance = createRelaEditor(editorEl, { minHeight: '250px' });
    }
    if (editorInstance) {
      setTimeout(function() { editorInstance.codemirror.refresh(); }, 10);
    }
  }

  function hideManualEdit() {
    container.style.display = 'none';
  }

  if (manualRadio) {
    manualRadio.addEventListener('change', function() {
      if (this.checked) showManualEdit();
    });
  }

  document.querySelectorAll('input[name="content"]').forEach(function(radio) {
    radio.addEventListener('change', function() {
      if (manualRadio && manualRadio.checked) {
        showManualEdit();
      } else {
        hideManualEdit();
      }
    });
  });

  // Diff highlighting
  function escapeHtml(text) {
    var div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  function computeLineDiff(oursLines, theirsLines) {
    // Simple LCS-based diff
    var m = oursLines.length, n = theirsLines.length;
    var dp = [];
    for (var i = 0; i <= m; i++) {
      dp[i] = [];
      for (var j = 0; j <= n; j++) {
        if (i === 0 || j === 0) dp[i][j] = 0;
        else if (oursLines[i-1] === theirsLines[j-1]) dp[i][j] = dp[i-1][j-1] + 1;
        else dp[i][j] = Math.max(dp[i-1][j], dp[i][j-1]);
      }
    }
    // Backtrack to find diff
    var oursResult = [], theirsResult = [];
    var i = m, j = n;
    while (i > 0 || j > 0) {
      if (i > 0 && j > 0 && oursLines[i-1] === theirsLines[j-1]) {
        oursResult.unshift({ text: oursLines[i-1], type: 'same' });
        theirsResult.unshift({ text: theirsLines[j-1], type: 'same' });
        i--; j--;
      } else if (j > 0 && (i === 0 || dp[i][j-1] >= dp[i-1][j])) {
        theirsResult.unshift({ text: theirsLines[j-1], type: 'add' });
        j--;
      } else {
        oursResult.unshift({ text: oursLines[i-1], type: 'remove' });
        i--;
      }
    }
    return { ours: oursResult, theirs: theirsResult };
  }

  function highlightWordDiff(line1, line2, addClass, removeClass) {
    var words1 = line1.split(/(\s+)/), words2 = line2.split(/(\s+)/);
    var result1 = '', result2 = '';
    var i = 0, j = 0;
    while (i < words1.length || j < words2.length) {
      if (i < words1.length && j < words2.length && words1[i] === words2[j]) {
        result1 += escapeHtml(words1[i]);
        result2 += escapeHtml(words2[j]);
        i++; j++;
      } else if (j < words2.length && (i >= words1.length || words1.indexOf(words2[j], i) === -1)) {
        result2 += '<span class="' + addClass + '">' + escapeHtml(words2[j]) + '</span>';
        j++;
      } else {
        result1 += '<span class="' + removeClass + '">' + escapeHtml(words1[i]) + '</span>';
        i++;
      }
    }
    return { line1: result1, line2: result2 };
  }

  function renderDiff() {
    var oursEl = document.getElementById('diff-ours');
    var theirsEl = document.getElementById('diff-theirs');
    if (!oursEl || !theirsEl) return;

    var oursText = oursEl.textContent;
    var theirsText = theirsEl.textContent;
    var oursLines = oursText.split('\n');
    var theirsLines = theirsText.split('\n');

    var diff = computeLineDiff(oursLines, theirsLines);

    var oursHtml = '', theirsHtml = '';
    var oi = 0, ti = 0;
    while (oi < diff.ours.length || ti < diff.theirs.length) {
      var oLine = diff.ours[oi], tLine = diff.theirs[ti];
      if (oLine && tLine && oLine.type === 'same' && tLine.type === 'same') {
        oursHtml += '<span class="diff-line">' + escapeHtml(oLine.text) + '</span>\n';
        theirsHtml += '<span class="diff-line">' + escapeHtml(tLine.text) + '</span>\n';
        oi++; ti++;
      } else if (oLine && oLine.type === 'remove' && tLine && tLine.type === 'add') {
        // Changed line - highlight word differences
        var wordDiff = highlightWordDiff(oLine.text, tLine.text, 'diff-word-add', 'diff-word-remove');
        oursHtml += '<span class="diff-line diff-line-change">' + wordDiff.line1 + '</span>\n';
        theirsHtml += '<span class="diff-line diff-line-change">' + wordDiff.line2 + '</span>\n';
        oi++; ti++;
      } else if (oLine && oLine.type === 'remove') {
        oursHtml += '<span class="diff-line diff-line-remove">' + escapeHtml(oLine.text) + '</span>\n';
        oi++;
      } else if (tLine && tLine.type === 'add') {
        theirsHtml += '<span class="diff-line diff-line-add">' + escapeHtml(tLine.text) + '</span>\n';
        ti++;
      } else {
        oi++; ti++;
      }
    }

    oursEl.innerHTML = oursHtml.replace(/\n$/, '');
    theirsEl.innerHTML = theirsHtml.replace(/\n$/, '');
  }

  renderDiff();

  // Select all properties and content to one side
  window.selectAllSide = function(side) {
    // Select all property values
    document.querySelectorAll('.resolve-value-selectable[data-side="' + side + '"]').forEach(function(cell) {
      cell.click();
    });
    // Select content radio
    var contentRadio = document.querySelector('input[name="content"][value="' + side + '"]');
    if (contentRadio) contentRadio.click();
  };

  // Click-to-select property values
  document.querySelectorAll('.resolve-value-selectable').forEach(function(cell) {
    cell.addEventListener('click', function() {
      var prop = this.getAttribute('data-prop');
      var side = this.getAttribute('data-side');
      var row = this.closest('tr');

      // Check the radio button
      var radio = this.querySelector('input[type="radio"]');
      if (radio) radio.checked = true;

      // Update visual states
      row.querySelectorAll('.resolve-value-selectable').forEach(function(c) {
        if (c.getAttribute('data-side') === side) {
          c.classList.add('resolve-value-selected');
          c.classList.remove('resolve-value-unselected');
        } else {
          c.classList.remove('resolve-value-selected');
          c.classList.add('resolve-value-unselected');
        }
      });
    });
  });

  // Keyboard navigation for conflict resolution
  (function() {
    var rows = Array.from(document.querySelectorAll('.resolve-table tbody tr'));
    var focusedIndex = -1;

    function setFocusedRow(index) {
      rows.forEach(function(r) { r.classList.remove('resolve-row-focused'); });
      if (index >= 0 && index < rows.length) {
        focusedIndex = index;
        rows[index].classList.add('resolve-row-focused');
        rows[index].scrollIntoView({ block: 'nearest', behavior: 'smooth' });
      }
    }

    function selectSide(side) {
      if (focusedIndex < 0 || focusedIndex >= rows.length) return;
      var row = rows[focusedIndex];
      var cell = row.querySelector('.resolve-value-selectable[data-side="' + side + '"]');
      if (cell) cell.click();
    }

    document.addEventListener('keydown', function(e) {
      // Skip if typing in an input/textarea
      var tag = e.target.tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
      if (e.target.isContentEditable) return;

      // Only handle if we're on the conflict resolution page
      var table = document.querySelector('.resolve-table');
      if (!table) return;

      switch(e.key) {
        case 'ArrowDown':
        case 'j':
          e.preventDefault();
          if (focusedIndex < 0) {
            // First press: focus first differing row
            for (var i = 0; i < rows.length; i++) {
              if (rows[i].querySelector('.resolve-value-selectable')) {
                setFocusedRow(i);
                break;
              }
            }
          } else {
            setFocusedRow(Math.min(focusedIndex + 1, rows.length - 1));
          }
          break;
        case 'ArrowUp':
        case 'k':
          e.preventDefault();
          if (focusedIndex < 0) {
            // First press: focus last differing row
            for (var i = rows.length - 1; i >= 0; i--) {
              if (rows[i].querySelector('.resolve-value-selectable')) {
                setFocusedRow(i);
                break;
              }
            }
          } else {
            setFocusedRow(Math.max(focusedIndex - 1, 0));
          }
          break;
        case 'ArrowLeft':
        case 'h':
        case '1':
          e.preventDefault();
          selectSide('ours');
          break;
        case 'ArrowRight':
        case 'l':
        case '2':
          e.preventDefault();
          selectSide('theirs');
          break;
        case 'Escape':
          e.preventDefault();
          var backLink = document.querySelector('a[href="/conflicts"]');
          if (backLink) backLink.click();
          break;
        case 'O':
          e.preventDefault();
          selectAllSide('ours');
          break;
        case 'T':
          e.preventDefault();
          selectAllSide('theirs');
          break;
      }
    });
  })();
})();
</script>
{{- end -}}

{{- define "kanban-page" -}}
<!DOCTYPE html>
<html lang="en">
<head>
<title>{{ .App.Name }} - {{ .Kanban.Title }}</title>
{{ template "head" . }}
</head>
<body>
{{ template "sidebar" . }}
<main class="main main-wide" id="content">
{{ template "kanban-content" . }}
</main>
<div id="command-toast-container"></div>
</body>
</html>
{{- end -}}

{{- define "kanban-content" -}}
<script>document.getElementById('content').classList.add('main-wide');</script>
<div class="page-header">
  <div>
    <h2>{{ .Kanban.Title }}</h2>
  </div>
  <div style="display:flex;gap:8px;align-items:center;">
    <span style="font-size:13px;color:var(--text-muted);">{{ .TotalCount }} items</span>
    {{ if .CreateForm }}
    <a id="kanban-new-btn" href="/form/{{ .CreateForm }}" class="btn btn-primary btn-sm"
       hx-get="/form/{{ .CreateForm }}" hx-target="#content" hx-push-url="true">+ New <kbd>N</kbd></a>
    {{ end }}
  </div>
</div>

{{ if .FilterControls }}
<div class="filter-bar-sentinel"></div>
<div class="filter-bar">
  {{ range .FilterControls }}
  <div>
    <label>{{ .Label }}</label>
    <select name="filter_{{ .Property }}" onchange="applyKanbanFilter(this, '{{ $.KanbanID }}')">
      <option value="">All</option>
      {{ $cur := .Current }}
      {{ range .Values }}<option value="{{ . }}"{{ if eq . $cur }} selected{{ end }}>{{ . }}</option>{{ end }}
    </select>
  </div>
  {{ end }}
</div>
{{ end }}

{{ if .HasSwimlanes }}
<div class="kanban-board with-swimlanes" data-kanban-id="{{ .KanbanID }}"
     style="grid-template-columns: auto repeat({{ len .Columns }}, minmax(240px, 1fr));">
  <div class="kanban-swimlane-header">
    <div class="kanban-corner"></div>
    {{ range .Columns }}
    <div class="kanban-col-header">{{ .Label }}</div>
    {{ end }}
  </div>
  {{ range $lane := .Swimlanes }}
  <div class="kanban-swimlane" data-swimlane="{{ $lane.Value }}">
    <div class="kanban-swimlane-label">{{ $lane.Label }}</div>
    {{ range $col := $.Columns }}
    <div class="kanban-cell" data-column="{{ $col.Value }}" data-swimlane="{{ $lane.Value }}">
      {{ $cards := index (index $.Cells $col.Value) $lane.Value }}
      {{ range $cards }}
      {{ template "kanban-card" (map "Card" . "EditForm" $.EditForm "KanbanID" $.KanbanID "FilterParams" $.FilterParams) }}
      {{ end }}
    </div>
    {{ end }}
  </div>
  {{ end }}
</div>
{{ else }}
<div class="kanban-board" data-kanban-id="{{ .KanbanID }}">
  {{ range $col := .Columns }}
  <div class="kanban-column" data-column="{{ $col.Value }}">
    <div class="kanban-column-header">
      <span>{{ $col.Label }}</span>
      {{ $cards := index (index $.Cells $col.Value) "" }}
      <span class="kanban-count">{{ len $cards }}</span>
    </div>
    <div class="kanban-cards">
      {{ range $cards }}
      {{ template "kanban-card" (map "Card" . "EditForm" $.EditForm "KanbanID" $.KanbanID "FilterParams" $.FilterParams) }}
      {{ end }}
    </div>
  </div>
  {{ end }}
</div>
{{ end }}
{{- end -}}

{{- define "kanban-card" -}}
<div class="kanban-card" draggable="true" data-entity-id="{{ .Card.ID }}"
     data-accent="{{ accentColor .Card.AccentType .Card.AccentValue }}"
     hx-get="/form/{{ .EditForm }}/{{ .Card.ID }}?from=/kanban/{{ .KanbanID }}{{ .FilterParams }}"
     hx-target="#content" hx-push-url="true">
  <div class="kanban-card-title">{{ .Card.Title }}</div>
  {{ if .Card.Fields }}
  <div class="kanban-card-fields">
    {{ range .Card.Fields }}
    <span class="badge {{ badgeClass .PropType .Value }}">{{ .Value }}</span>
    {{ end }}
  </div>
  {{ end }}
</div>
{{- end -}}
`
