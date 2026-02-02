package dataentry

// allTemplates contains all HTML templates for the data entry application.
// These are parsed at startup and used by all handlers.
const allTemplates = `
{{- define "head" -}}
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<script src="/static/htmx.min.js"></script>
<link rel="stylesheet" href="/static/easymde.min.css">
<script src="/static/easymde.min.js"></script>
<link rel="stylesheet" href="/static/slimselect.css">
<script src="/static/slimselect.min.js"></script>
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
.nav-group { margin: 0; }
.nav-group-toggle { display: flex; align-items: center; gap: 6px; width: 100%; padding: 8px 20px; background: none; border: none; border-left: 3px solid transparent; color: var(--text-sidebar); font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; cursor: pointer; font-family: var(--font); transition: color 0.15s; }
.nav-group-toggle:hover { color: var(--text-sidebar-active); }
.nav-group-chevron { display: inline-block; font-size: 20px; line-height: 1; transition: transform 0.15s ease; transform: rotate(90deg); }
.nav-group-chevron.collapsed { transform: rotate(0deg); }
.nav-group-items { overflow: hidden; }
.nav-group-items.hidden { display: none; }
.sidebar nav .nav-group-items a { padding-left: 40px; }

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
.toast-error { background: #991b1b; }
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

/* Fullscreen editor mode */
.editor-fullscreen-overlay { position: fixed; inset: 0; z-index: 300; background: var(--bg); display: flex; flex-direction: column; }
.editor-fullscreen-overlay .editor-fullscreen-header { display: flex; align-items: center; justify-content: space-between; padding: 10px 20px; border-bottom: 1px solid var(--border); background: var(--bg-card); flex-shrink: 0; }
.editor-fullscreen-overlay .editor-fullscreen-header h3 { font-size: 15px; font-weight: 600; color: var(--text); }
.editor-fullscreen-overlay .editor-fullscreen-body { flex: 1; display: flex; flex-direction: column; padding: 16px 24px; overflow: hidden; }
.editor-fullscreen-overlay .EasyMDEContainer { flex: 1; display: flex; flex-direction: column; border: 1px solid var(--border); border-radius: 6px; }
.editor-fullscreen-overlay .EasyMDEContainer .CodeMirror { flex: 1; }
.editor-fullscreen-overlay .EasyMDEContainer .CodeMirror-scroll { min-height: 0; }

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
.command-toast-msg.warning { color: #b45309; }
.command-toast-msg.error-msg { color: #dc2626; }
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
.command-toast-log.show { display: block; padding: 8px 12px; background: #f8fafc; border-top: 1px solid var(--border); font-family: var(--font-mono); font-size: 11px; max-height: 150px; overflow-y: auto; white-space: pre-wrap; word-break: break-all; color: var(--text-muted); }

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

  var container = document.getElementById('command-toast-container');
  var toast = _createToast(execID, label);
  container.appendChild(toast);

  var qs = new URLSearchParams(params);
  qs.set('exec_id', execID);

  _cmdToasts[execID] = { toast: toast, messages: [], logs: [], hoverPause: false, aborted: false };

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
{{ else }}
    <a href="/list/{{ .List }}"{{ if eq .List .ActiveList }} class="active"{{ end }}
       data-entity-type="{{ .EntityType }}"
       hx-get="/list/{{ .List }}" hx-target="#content" hx-push-url="true">
      {{ .Label }}<span class="nav-count">{{ .Count }}</span>
    </a>
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
       hx-get="/search" hx-target="#content" hx-push-url="true"
       style="border-bottom:1px solid rgba(255,255,255,0.1);margin-bottom:4px;">
      &#128269; Search
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
        {{ template "nav-item" (map "Dashboard" .Dashboard "Graph" .Graph "Label" .Label "List" .List "EntityType" .EntityType "Count" .Count "ActiveList" $.ActiveList) }}
        {{ end }}
      </div>
    </div>
    {{ else }}
    {{ template "nav-item" (map "Dashboard" .Item.Dashboard "Graph" .Item.Graph "Label" .Item.Label "List" .Item.List "EntityType" .Item.EntityType "Count" .Item.Count "ActiveList" $.ActiveList) }}
    {{ end }}
    {{ end }}
  </nav>
</aside>
<script>
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
       hx-get="/form/{{ .List.CreateForm }}" hx-target="#content" hx-push-url="true">+ New</a>
    {{ end }}
    {{ if .Commands }}{{ if gt (len .Commands) 2 }}
    <details class="add-dropdown">
      <summary class="btn btn-secondary btn-sm">Commands &#9662;</summary>
      <div class="add-dropdown-menu">
        {{ range .Commands }}<a href="#" onclick="event.preventDefault();runCommand('{{ .ID }}', {list_id:'{{ $.ListID }}'})" {{ if .Confirm }}data-confirm="{{ .Confirm }}"{{ end }}>{{ .Label }}</a>
        {{ end }}
      </div>
    </details>
    {{ else }}{{ range .Commands }}
    <button class="btn btn-secondary btn-sm" onclick="runCommand('{{ .ID }}', {list_id:'{{ $.ListID }}'})" {{ if .Confirm }}data-confirm="{{ .Confirm }}"{{ end }}>{{ .Label }}</button>
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
          <td style="width:1%;white-space:nowrap;"><a href="#" class="delete-icon" title="Delete"
              onclick="event.preventDefault();confirmDelete('{{ .EntityID }}','/list/{{ $.ListID }}')">&#128465;</a></td>
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
              onclick="confirmDelete('{{ .EntityID }}','{{ .ReturnTo }}')">Delete</button>
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
var _editorInstance = null;
(function() {
  var el = document.getElementById('body-editor');
  if (el) {
    _editorInstance = new EasyMDE({
      element: el,
      spellChecker: false,
      status: false,
      minHeight: '200px',
      toolbar: ['bold', 'italic', 'heading', '|', 'unordered-list', 'ordered-list', '|', 'link', 'image', '|', 'preview', 'side-by-side', '|', {
        name: 'toggle-fullscreen-editor',
        action: toggleFullscreenEditor,
        className: 'fa fa-arrows-alt',
        title: 'Toggle Full Screen Editor',
      }, '|', 'guide'],
      sideBySideFullscreen: false,
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
<div id="command-toast-container"></div>
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
    {{ if .Commands }}{{ if gt (len .Commands) 2 }}
    <details class="add-dropdown">
      <summary class="btn btn-secondary btn-sm">Commands &#9662;</summary>
      <div class="add-dropdown-menu">
        {{ range .Commands }}<a href="#" onclick="event.preventDefault();runCommand('{{ .ID }}', {entity_id:'{{ $.Entity.ID }}',entity_type:'{{ $.Entity.Type }}'})" {{ if .Confirm }}data-confirm="{{ .Confirm }}"{{ end }}>{{ .Label }}</a>
        {{ end }}
      </div>
    </details>
    {{ else }}{{ range .Commands }}
    <button class="btn btn-secondary btn-sm" onclick="runCommand('{{ .ID }}', {entity_id:'{{ $.Entity.ID }}',entity_type:'{{ $.Entity.Type }}'})" {{ if .Confirm }}data-confirm="{{ .Confirm }}"{{ end }}>{{ .Label }}</button>
    {{ end }}{{ end }}{{ end }}
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
<div id="command-toast-container"></div>
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
    {{ if .Commands }}{{ if gt (len .Commands) 2 }}
    <details class="add-dropdown">
      <summary class="btn btn-secondary btn-sm">Commands &#9662;</summary>
      <div class="add-dropdown-menu">
        {{ range .Commands }}<a href="#" onclick="event.preventDefault();runCommand('{{ .ID }}', {entity_id:'{{ $.Entry.ID }}',entity_type:'{{ $.Entry.Type }}',view_id:'{{ $.ViewID }}'})" {{ if .Confirm }}data-confirm="{{ .Confirm }}"{{ end }}>{{ .Label }}</a>
        {{ end }}
      </div>
    </details>
    {{ else }}{{ range .Commands }}
    <button class="btn btn-secondary btn-sm" onclick="runCommand('{{ .ID }}', {entity_id:'{{ $.Entry.ID }}',entity_type:'{{ $.Entry.Type }}',view_id:'{{ $.ViewID }}'})" {{ if .Confirm }}data-confirm="{{ .Confirm }}"{{ end }}>{{ .Label }}</button>
    {{ end }}{{ end }}{{ end }}
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
               hx-get="/form/{{ .EditFormID }}/{{ .EntityID }}?return_to={{ urlquery $returnTo }}" hx-target="#content" hx-push-url="true" title="Edit">&#9998;</a>
               <a href="#" class="delete-icon" title="Delete" style="margin-left:6px;"
                  onclick="event.preventDefault();confirmDelete('{{ .EntityID }}','{{ $returnTo }}')">&#128465;</a></td>{{ end }}
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
                  onclick="event.preventDefault();confirmDelete('{{ .EntityID }}','{{ $returnTo }}')">&#128465;</a></td>
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

{{- define "search-page" -}}
<!DOCTYPE html>
<html lang="en">
<head>
<title>{{ .App.Name }} - Search</title>
{{ template "head" . }}
</head>
<body>
{{ template "sidebar" . }}
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
.search-dd-item:hover { background: #f1f5f9; }
.search-dd-item.active:hover { background: var(--primary-light); }
.search-dd-cat { font-size: 10px; padding: 1px 6px; border-radius: 3px; text-transform: uppercase; font-weight: 600; letter-spacing: 0.5px; }
.search-dd-cat-type { background: #dbeafe; color: #1e40af; }
.search-dd-cat-status { background: #dcfce7; color: #166534; }
.search-dd-cat-property { background: #e9d5ff; color: #6b21a8; }
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
    <code>"exact phrase"</code> exact match &middot;
    plain words (AND logic)
  </div>
</div>

{{ if .ParseErrors }}
<div style="padding:10px 16px;background:#fef2f2;border:1px solid #fecaca;border-radius:6px;margin-bottom:16px;font-size:13px;color:#991b1b;">
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

  var catClass = {type: 'search-chip-type', status: 'search-chip-status', property: 'search-chip-property'};
  var ddCatClass = {type: 'search-dd-cat-type', status: 'search-dd-cat-status', property: 'search-dd-cat-property'};

  function isFilter(val) { return /^(type|status|prop):/.test(val); }

  function detectCategory(val) {
    if (val.lastIndexOf('type:', 0) === 0) return 'type';
    if (val.lastIndexOf('status:', 0) === 0) return 'status';
    if (val.lastIndexOf('prop:', 0) === 0) return 'property';
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
    addChip(sel.value, sel.category);
    // Remove the filter token from the input, keep other text
    var text = input.value;
    var words = text.split(/\s+/);
    var remaining = [];
    var removed = false;
    for (var i = 0; i < words.length; i++) {
      if (!removed && /^(type|status|prop)(:|$)/.test(words[i])) {
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
    if (!last || !/^(type|status|prop)(:|$)/.test(last)) {
      closeDropdown();
      return;
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
    <a href="/entity/{{ .EntityType }}/{{ .ID }}" class="cell-link" style="font-size:15px;font-weight:600;"
       hx-get="/entity/{{ .EntityType }}/{{ .ID }}" hx-target="#content" hx-push-url="true">{{ .Title }}</a>
    <span style="font-size:11px;font-family:var(--font-mono);color:var(--text-muted);background:#f1f5f9;padding:1px 6px;border-radius:3px;">{{ .ID }}</span>
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
        {{ range .Commands }}<a href="#" onclick="event.preventDefault();runCommand('{{ .ID }}', {})" {{ if .Confirm }}data-confirm="{{ .Confirm }}"{{ end }}>{{ .Label }}</a>
        {{ end }}
      </div>
    </details>
    {{ else }}{{ range .Commands }}
    <button class="btn btn-secondary btn-sm" onclick="runCommand('{{ .ID }}', {})" {{ if .Confirm }}data-confirm="{{ .Confirm }}"{{ end }}>{{ .Label }}</button>
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
`
