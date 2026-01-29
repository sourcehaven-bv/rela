package dataentry

// syncTemplates contains templates for the sync status indicator in the sidebar.
const syncTemplates = `
{{- define "sync-indicator" -}}
<div class="sidebar-footer" id="sync-indicator"
     hx-get="/api/sync/indicator" hx-trigger="every 30s" hx-swap="innerHTML">
  {{ template "sync-indicator-content" . }}
</div>
<script>
(function() {
  var indicator = document.getElementById('sync-indicator');
  if (!indicator || typeof EventSource === 'undefined') return;
  var es = new EventSource('/api/sync/sse');
  es.onmessage = function(e) {
    indicator.innerHTML = e.data;
  };
  es.onerror = function() {
    // SSE failed — polling fallback is already active via hx-trigger
    es.close();
  };
})();
</script>
{{- end -}}

{{- define "sync-indicator-content" -}}
{{ if .SyncStatus.Enabled }}
<div class="sync-status sync-{{ .SyncStatus.State }}">
  <span class="sync-dot"></span>
  {{ if .SyncStatus.Protected }}<span class="sync-lock" title="Protected branch">&#128274;</span>{{ end }}
  {{ if eq .SyncStatus.State "conflict" }}
  <a href="/conflicts" class="sync-label sync-label-link" title="Resolve conflicts">{{ .SyncStatus.Message }}</a>
  {{ else }}
  <span class="sync-label">{{ .SyncStatus.Message }}</span>
  {{ end }}
  {{ if or (eq .SyncStatus.State "error") (eq .SyncStatus.State "offline") }}
  <button class="sync-retry-btn" onclick="syncPull()" title="Retry sync">&#8635;</button>
  {{ end }}
  {{ if not (or (eq .SyncStatus.State "conflict") (eq .SyncStatus.State "offline")) }}
  <button class="sync-action-btn" onclick="syncPull()" title="Pull from remote">&#8595;</button>
  {{ end }}
  <button class="sync-branch-btn" onclick="toggleBranchDropdown(event)">
    {{ .SyncStatus.Branch }} &#9662;
  </button>
</div>
<div class="branch-dropdown" id="branch-dropdown" style="display:none;">
  <div class="branch-dropdown-header">Branches</div>
  <div id="branch-list" hx-get="/api/sync/branches-list" hx-trigger="click from:.sync-branch-btn" hx-swap="innerHTML">
    <div class="branch-loading">Loading...</div>
  </div>
  <div class="branch-dropdown-footer">
    <input type="text" id="new-branch-name" placeholder="New branch name..." class="branch-input">
    <button class="btn btn-sm btn-primary" onclick="createBranch()">Create</button>
  </div>
</div>
{{ else }}
<div class="sync-status sync-disabled">
  <span class="sync-dot"></span>
  <span class="sync-label">{{ .SyncStatus.Message }}</span>
</div>
{{ end }}
<script>
function toggleBranchDropdown(e) {
  e.stopPropagation();
  var dd = document.getElementById('branch-dropdown');
  dd.style.display = dd.style.display === 'none' ? 'block' : 'none';
}
function switchBranch(name) {
  fetch('/api/sync/branch', {
    method: 'POST',
    headers: {'Content-Type': 'application/x-www-form-urlencoded'},
    body: 'action=switch&name=' + encodeURIComponent(name)
  }).then(function(r) {
    if (r.ok) { window.location.href = '/'; }
    else { r.text().then(function(t) { alert('Switch failed: ' + t); }); }
  });
}
function createBranch() {
  var name = document.getElementById('new-branch-name').value.trim();
  if (!name) return;
  fetch('/api/sync/branch', {
    method: 'POST',
    headers: {'Content-Type': 'application/x-www-form-urlencoded'},
    body: 'action=create&name=' + encodeURIComponent(name)
  }).then(function(r) {
    if (r.ok) { window.location.href = '/'; }
    else { r.text().then(function(t) { alert('Create failed: ' + t); }); }
  });
}
function syncPush() {
  fetch('/api/sync/push', { method: 'POST' })
    .then(function(r) {
      htmx.trigger('#sync-indicator', 'htmx:load');
    });
}
function syncPull() {
  fetch('/api/sync/pull', { method: 'POST' })
    .then(function(r) {
      htmx.trigger('#sync-indicator', 'htmx:load');
      if (r.ok) { window.location.reload(); }
    });
}
document.addEventListener('click', function(e) {
  var dd = document.getElementById('branch-dropdown');
  if (dd && !dd.contains(e.target) && !e.target.classList.contains('sync-branch-btn')) {
    dd.style.display = 'none';
  }
});
</script>
{{- end -}}

{{- define "branch-list-content" -}}
{{ range .Branches.Local }}
<a class="branch-item{{ if eq . $.Branches.Current }} branch-current{{ end }}"
   {{ if ne . $.Branches.Current }}onclick="switchBranch('{{ . }}')"{{ end }}>
  {{ if eq . $.Branches.Current }}<span class="branch-check">&#10003;</span>{{ end }}
  {{ . }}
</a>
{{ end }}
{{ if .Branches.Remote }}
<div class="branch-separator">Remote</div>
{{ range .Branches.Remote }}
<a class="branch-item" onclick="switchBranch('{{ . }}')">{{ . }}</a>
{{ end }}
{{ end }}
{{- end -}}
`
