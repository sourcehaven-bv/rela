// Data Entry App JavaScript

// Scope navigation keyboard shortcuts (left/right arrow keys)
document.addEventListener('keydown', function(e) {
  if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.tagName === 'SELECT') return;
  var nav = document.querySelector('.scope-nav');
  if (!nav) return;
  if (e.key === 'ArrowLeft') { var btn = nav.querySelector('a[data-scope-dir="prev"]'); if (btn) btn.click(); }
  if (e.key === 'ArrowRight') { var btn = nav.querySelector('a[data-scope-dir="next"]'); if (btn) btn.click(); }
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
    // Refresh git status when files change
    if (typeof refreshGitStatus === 'function') refreshGitStatus();
    // Check for page-specific refresh handlers (e.g., document watcher)
    if (window._pageRefreshHandlers && window._pageRefreshHandlers.length > 0) {
      window._pageRefreshHandlers.forEach(function(h) { h.handler(); });
      return;
    }
    if (isOnForm()) {
      showUpdateBanner();
    } else {
      doRefresh();
    }
  }
  function connect() {
    es = new EventSource('/api/events');
    es.addEventListener('refresh', onRefresh);
    es.addEventListener('git', function() {
      // Refresh git status in UI when background fetch completes
      if (typeof refreshGitStatus === 'function') refreshGitStatus();
    });
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

  // --- Git Sync Modal ---
  var _syncModal = null;
  var _syncState = { syncing: false };

  function createSyncModal() {
    if (_syncModal) return;
    var overlay = document.createElement('div');
    overlay.className = 'sync-modal-overlay';
    overlay.id = 'sync-modal';
    overlay.style.display = 'none';
    overlay.innerHTML =
      '<div class="sync-modal">' +
        '<div class="sync-modal-header">' +
          '<h3>Sync Changes</h3>' +
          '<button class="sync-modal-close" onclick="closeSyncModal()">&times;</button>' +
        '</div>' +
        '<div class="sync-modal-body">' +
          '<div class="sync-info">' +
            '<span class="sync-info-label">Branch:</span>' +
            '<span class="sync-info-value" id="sync-branch">main</span>' +
            '<span class="sync-info-label">Local changes:</span>' +
            '<span class="sync-info-value" id="sync-local">0</span>' +
            '<span class="sync-info-label">Remote updates:</span>' +
            '<span class="sync-info-value" id="sync-remote">0</span>' +
          '</div>' +
          '<div id="sync-status"></div>' +
        '</div>' +
        '<div class="sync-modal-footer">' +
          '<button class="btn btn-secondary" id="sync-cancel-btn" onclick="closeSyncModal()">Cancel</button>' +
          '<button class="btn btn-primary" id="sync-now-btn" onclick="doSync()">Sync Now</button>' +
        '</div>' +
      '</div>';
    overlay.addEventListener('click', function(e) { if (e.target === overlay && !_syncState.syncing) closeSyncModal(); });
    document.body.appendChild(overlay);
    _syncModal = overlay;
  }

  function openSyncModal() {
    createSyncModal();
    _syncModal.style.display = '';
    document.getElementById('sync-status').innerHTML = '';
    document.getElementById('sync-now-btn').disabled = false;
    document.getElementById('sync-cancel-btn').disabled = false;
    refreshGitStatus(true);
  }

  function closeSyncModal() {
    if (_syncState.syncing) return;
    if (_syncModal) _syncModal.style.display = 'none';
  }

  function doSync() {
    if (_syncState.syncing) return;
    _syncState.syncing = true;
    document.getElementById('sync-now-btn').disabled = true;
    document.getElementById('sync-cancel-btn').disabled = true;
    document.getElementById('sync-status').innerHTML =
      '<div class="sync-progress"><div class="sync-spinner"></div><span>Syncing...</span></div>';

    fetch('/api/git/sync', { method: 'POST' })
      .then(function(r) { return r.json(); })
      .then(function(data) {
        _syncState.syncing = false;
        if (data.error) {
          var html = '<div class="sync-error"><div class="sync-error-title">Sync failed</div>' + escapeHtml(data.error);
          if (data.conflict_files && data.conflict_files.length) {
            html += '<ul class="sync-error-files">';
            data.conflict_files.forEach(function(f) { html += '<li>' + escapeHtml(f) + '</li>'; });
            html += '</ul>';
          }
          html += '</div>';
          document.getElementById('sync-status').innerHTML = html;
          document.getElementById('sync-cancel-btn').disabled = false;
          document.getElementById('sync-now-btn').textContent = 'Retry';
          document.getElementById('sync-now-btn').disabled = false;
        } else {
          document.getElementById('sync-status').innerHTML =
            '<div class="sync-success">&#10003; Synced successfully</div>';
          setTimeout(function() {
            closeSyncModal();
            refreshGitStatus();
          }, 1500);
        }
      })
      .catch(function(err) {
        _syncState.syncing = false;
        document.getElementById('sync-status').innerHTML =
          '<div class="sync-error"><div class="sync-error-title">Network error</div>' + escapeHtml(err.message) + '</div>';
        document.getElementById('sync-cancel-btn').disabled = false;
        document.getElementById('sync-now-btn').disabled = false;
      });
  }

  function refreshGitStatus(updateModal) {
    fetch('/api/git/status')
      .then(function(r) { return r.json(); })
      .then(function(data) {
        if (!data.available) return;
        var btn = document.getElementById('git-status-btn');
        var branchEl = document.getElementById('git-branch');
        var textEl = document.getElementById('git-status-text');
        if (branchEl) branchEl.textContent = data.branch || 'main';

        var statusParts = [];
        var statusClass = '';
        if (data.remote_ahead > 0) {
          statusParts.push('↓' + data.remote_ahead);
          statusClass = 'has-remote';
        }
        if (data.local_changes > 0) {
          statusParts.push(data.local_changes + ' changes');
          statusClass = data.remote_ahead > 0 ? 'has-both' : 'has-changes';
        }
        if (data.conflict) {
          statusParts = ['Conflict'];
          statusClass = 'conflict';
        }
        if (statusParts.length === 0) {
          statusParts.push('Synced');
        }
        if (textEl) textEl.textContent = statusParts.join(' · ');
        if (btn) {
          btn.className = 'git-status' + (statusClass ? ' ' + statusClass : '');
        }

        if (updateModal && _syncModal && _syncModal.style.display !== 'none') {
          document.getElementById('sync-branch').textContent = data.branch || 'main';
          var localEl = document.getElementById('sync-local');
          localEl.textContent = data.local_changes || 0;
          localEl.className = 'sync-info-value' + (data.local_changes > 0 ? ' changes' : '');
          var remoteEl = document.getElementById('sync-remote');
          remoteEl.textContent = data.remote_ahead || 0;
          remoteEl.className = 'sync-info-value' + (data.remote_ahead > 0 ? ' remote' : '');
        }
      })
      .catch(function() {});
  }

  function escapeHtml(text) {
    var div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  window.openSyncModal = openSyncModal;
  window.closeSyncModal = closeSyncModal;
  window.doSync = doSync;
  window.refreshGitStatus = refreshGitStatus;

  // Initial git status check and periodic refresh
  if (document.getElementById('git-status-btn')) {
    refreshGitStatus();
    setInterval(refreshGitStatus, 30000); // Check every 30s
  }

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
      if (kbdMap['meta+Enter']) {
        e.preventDefault();
        kbdMap['meta+Enter'].click();
      }
      return;
    }

    // Escape: close palette/modal, blur input, or cancel form
    if (e.key === 'Escape') {
      if (isPaletteOpen()) { togglePalette(); return; }
      if (isShortcutsOpen()) { toggleShortcuts(); return; }
      if (hasSearchResultSelected()) { exitSearchResults(); return; }
      if (isInputFocused()) { document.activeElement.blur(); return; }
      // On scope nav — click the Back button (first link in scope-nav)
      var scopeBackBtn = document.querySelector('.scope-nav a.scope-nav-btn');
      if (scopeBackBtn) { scopeBackBtn.click(); return; }
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

  var editor = new EasyMDE({
    element: element,
    spellChecker: false,
    status: false,
    minHeight: options.minHeight || '200px',
    toolbar: toolbar,
    sideBySideFullscreen: false,
  });

  // Sync CodeMirror content to textarea before form submission
  // Using 'changes' event (batched) is more efficient than 'change' (per-keystroke)
  editor.codemirror.on('changes', function() {
    editor.codemirror.save();
  });

  return editor;
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

// Theme toggle
function toggleTheme() {
  var current = document.documentElement.getAttribute('data-theme');
  var next = current === 'dark' ? 'light' : 'dark';
  document.documentElement.setAttribute('data-theme', next);
  localStorage.setItem('theme', next);
}

// Nav group toggle with server-side persistence
function toggleNavGroup(btn) {
  var chevron = btn.querySelector('.nav-group-chevron');
  var items = btn.nextElementSibling;
  var isCollapsed = chevron.classList.toggle('collapsed');
  items.classList.toggle('hidden', isCollapsed);
  var group = btn.getAttribute('data-group');
  fetch('/api/ui/toggle-group', {
    method: 'POST',
    headers: {'Content-Type': 'application/x-www-form-urlencoded'},
    body: 'group=' + encodeURIComponent(group)
  });
}

// Update active sidebar link after HTMX navigation
document.addEventListener('htmx:pushedIntoHistory', function() {
  var path = window.location.pathname;
  var links = document.querySelectorAll('.sidebar nav a');
  var matched = false;
  links.forEach(function(a) {
    var href = a.getAttribute('href');
    if (path === href || path.startsWith(href + '?')) matched = true;
  });
  if (matched) {
    links.forEach(function(a) {
      var href = a.getAttribute('href');
      a.classList.toggle('active', path === href || path.startsWith(href + '?'));
    });
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
  var params = new URLSearchParams(window.location.search);
  var fromList = params.get('from');
  if (fromList) {
    links.forEach(function(a) {
      a.classList.toggle('active', a.getAttribute('href') === '/list/' + fromList);
    });
    return;
  }
  var m = path.match(/^\/entity\/([^/]+)\//);
  if (m) {
    var etype = m[1];
    links.forEach(function(a) {
      a.classList.toggle('active', a.getAttribute('data-entity-type') === etype);
    });
  }
});

// Fullscreen editor toggle
var _editorInstance = null;

function toggleFullscreenEditor() {
  var overlay = document.getElementById('editor-fullscreen-overlay');
  if (overlay) {
    exitFullscreenEditor();
    return;
  }
  if (!_editorInstance) return;

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

  var container = _editorInstance.codemirror.getWrapperElement().closest('.EasyMDEContainer');
  container._originalParent = container.parentNode;
  container._originalNext = container.nextSibling;
  body.appendChild(container);

  document.body.appendChild(overlay);
  _editorInstance.codemirror.refresh();
  _editorInstance.codemirror.focus();

  overlay._keyHandler = function(e) {
    if (e.key === 'Escape') exitFullscreenEditor();
  };
  document.addEventListener('keydown', overlay._keyHandler);
}

function exitFullscreenEditor() {
  var overlay = document.getElementById('editor-fullscreen-overlay');
  if (!overlay || !_editorInstance) return;

  var container = _editorInstance.codemirror.getWrapperElement().closest('.EasyMDEContainer');
  if (container._originalNext) {
    container._originalParent.insertBefore(container, container._originalNext);
  } else {
    container._originalParent.appendChild(container);
  }

  document.removeEventListener('keydown', overlay._keyHandler);
  overlay.remove();
  _editorInstance.codemirror.refresh();
}

// Initialize EasyMDE on form pages
document.addEventListener('DOMContentLoaded', function() {
  var el = document.getElementById('body-editor');
  if (el && typeof createRelaEditor === 'function') {
    _editorInstance = createRelaEditor(el, { fullscreenToggle: toggleFullscreenEditor });
  }
});
document.addEventListener('htmx:afterSettle', function(e) {
  var el = e.detail.target.querySelector('#body-editor');
  if (el && typeof createRelaEditor === 'function' && !el._editorInit) {
    el._editorInit = true;
    _editorInstance = createRelaEditor(el, { fullscreenToggle: toggleFullscreenEditor });
  }
});
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

// Help modal
function openHelpModal(entityType) {
  var modal = document.getElementById('help-modal');
  var title = document.getElementById('help-modal-title');
  var body = document.getElementById('help-modal-body');
  if (!modal || !body) return;

  title.textContent = entityType + ' — Help';
  body.innerHTML = '<p style="color:var(--text-muted);text-align:center;">Loading...</p>';
  modal.style.display = 'flex';

  fetch('/api/help/' + encodeURIComponent(entityType))
    .then(function(r) {
      if (!r.ok) throw new Error('Not found');
      return r.text();
    })
    .then(function(html) {
      body.innerHTML = html;
    })
    .catch(function() {
      body.innerHTML = '<p style="color:var(--danger);text-align:center;">Failed to load help.</p>';
    });
}

function closeHelpModal() {
  var modal = document.getElementById('help-modal');
  if (modal) modal.style.display = 'none';
}

// Close help modal on Escape key
document.addEventListener('keydown', function(e) {
  if (e.key === 'Escape') {
    var modal = document.getElementById('help-modal');
    if (modal && modal.style.display === 'flex') {
      closeHelpModal();
    }
  }
});

// --- Relation cards (advanced mode) ---
var _relPickerState = { relation: '', targetType: '', targetLabel: '', cardsContainer: null };

function openRelationPicker(relation, targetType, targetLabel) {
  _relPickerState.relation = relation;
  _relPickerState.targetType = targetType;
  _relPickerState.targetLabel = targetLabel;
  _relPickerState.cardsContainer = document.querySelector('.relation-cards[data-relation="' + relation + '"]');

  document.getElementById('rel-picker-title').textContent = 'Add ' + targetLabel;
  document.getElementById('rel-picker-search').value = '';
  document.getElementById('rel-picker-results').innerHTML = '<p style="color:var(--text-muted);text-align:center;">Loading...</p>';
  document.getElementById('rel-picker-modal').style.display = 'flex';
  searchRelationCandidates();
  setTimeout(function() { document.getElementById('rel-picker-search').focus(); }, 100);
}

function closeRelationPicker() {
  document.getElementById('rel-picker-modal').style.display = 'none';
  document.getElementById('rel-picker-results').innerHTML = '';
}

var _relPickerDebounce = null;
function searchRelationCandidates() {
  clearTimeout(_relPickerDebounce);
  _relPickerDebounce = setTimeout(function() {
    var q = document.getElementById('rel-picker-search').value;
    // Get already selected IDs to exclude
    var existing = [];
    if (_relPickerState.cardsContainer) {
      _relPickerState.cardsContainer.querySelectorAll('.relation-card').forEach(function(card) {
        existing.push(card.getAttribute('data-target-id'));
      });
    }
    var url = '/api/relation-candidates?type=' + encodeURIComponent(_relPickerState.targetType) +
      '&q=' + encodeURIComponent(q) +
      '&exclude=' + encodeURIComponent(existing.join(','));
    fetch(url)
      .then(function(r) { return r.json(); })
      .then(function(candidates) {
        var container = document.getElementById('rel-picker-results');
        if (!candidates || candidates.length === 0) {
          container.innerHTML = '<p style="color:var(--text-muted);text-align:center;padding:16px 0;">No candidates found</p>';
          return;
        }
        var html = '<div style="display:flex;flex-direction:column;gap:4px;">';
        candidates.forEach(function(c) {
          html += '<div class="rel-picker-item" onclick="selectRelationCandidate(\'' + _escRelAttr(c.id) + '\', \'' + _escRelAttr(c.title) + '\')">' +
            '<div>' +
            '<div style="font-weight:500;font-size:14px;">' + _escRelHtml(c.title) + '</div>' +
            '<div style="font-size:11px;color:var(--text-muted);font-family:var(--font-mono);">' + _escRelHtml(c.id) + '</div>' +
            '</div>' +
            '<button class="btn btn-primary btn-sm" onclick="event.stopPropagation();selectRelationCandidate(\'' + _escRelAttr(c.id) + '\', \'' + _escRelAttr(c.title) + '\')">Add</button>' +
            '</div>';
        });
        html += '</div>';
        container.innerHTML = html;
      })
      .catch(function() {
        document.getElementById('rel-picker-results').innerHTML = '<p style="color:var(--danger);text-align:center;">Failed to load candidates.</p>';
      });
  }, 200);
}

function selectRelationCandidate(id, title) {
  if (!_relPickerState.cardsContainer) return;
  var relation = _relPickerState.relation;
  // Create a new card with HTMX attributes for Edit button
  var card = document.createElement('div');
  card.className = 'relation-card';
  card.id = 'rel-card-' + relation + '-' + id;
  card.setAttribute('data-target-id', id);
  card.innerHTML =
    '<div class="relation-card-header">' +
      '<span class="relation-card-title">' + _escRelHtml(title) + '</span>' +
      '<span class="relation-card-id">' + _escRelHtml(id) + '</span>' +
    '</div>' +
    '<div class="relation-card-actions">' +
      '<button type="button" class="btn btn-sm btn-ghost" ' +
        'hx-get="/api/relation-edit-form?relation=' + encodeURIComponent(relation) + '&target=' + encodeURIComponent(id) + '" ' +
        'hx-target="#rel-edit-body" hx-swap="innerHTML" ' +
        'onclick="document.getElementById(\'rel-edit-title\').textContent=\'Edit: ' + _escRelAttr(id) + '\';document.getElementById(\'rel-edit-modal\').style.display=\'flex\'" ' +
        'title="Edit relation properties">Edit</button>' +
      '<button type="button" class="btn btn-sm btn-danger-outline" onclick="this.closest(\'.relation-card\').remove()" title="Remove relation">Remove</button>' +
    '</div>' +
    '<input type="hidden" name="' + _escRelAttr(relation) + '" value="' + _escRelAttr(id) + '">';
  // Insert before the add button
  var addBtn = _relPickerState.cardsContainer.querySelector('.relation-add-btn');
  _relPickerState.cardsContainer.insertBefore(card, addBtn);
  // Process HTMX attributes on the new card
  htmx.process(card);
  closeRelationPicker();
}

function closeRelationEdit() {
  document.getElementById('rel-edit-modal').style.display = 'none';
  document.getElementById('rel-edit-body').innerHTML = '';
}

function _escRelHtml(s) { var d = document.createElement('div'); d.textContent = s; return d.innerHTML; }
function _escRelAttr(s) { return s.replace(/'/g, "\\'").replace(/"/g, '&quot;'); }

// Side panel toggle (mobile)
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

// Link existing modal
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
              '<div style="font-weight:500;font-size:14px;">' + _escLinkHTML(c.title) + '</div>' +
              '<div style="font-size:11px;color:var(--text-muted);font-family:var(--font-mono);">' + _escLinkHTML(c.id) + ' &middot; ' + _escLinkHTML(c.type) + '</div>' +
              '</div>' +
              '<button class="btn btn-primary btn-sm" onclick="event.stopPropagation();doLinkExisting(\'' + _escLinkAttr(c.id) + '\')">Link</button>' +
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
        window.location.reload();
      })
      .catch(function(e) { alert('Error linking: ' + e); });
  };

  function _escLinkHTML(s) { var d = document.createElement('div'); d.textContent = s; return d.innerHTML; }
  function _escLinkAttr(s) { return s.replace(/'/g, "\\'").replace(/"/g, '&quot;'); }
})();

// Conflict resolution page initialization
(function() {
  function initConflictResolution() {
    var manualRadio = document.getElementById('content-manual-radio');
    var container = document.getElementById('manual-edit-container');
    var editorEl = document.getElementById('manual-content-editor');
    var editorInstance = null;

    if (!container) return;

    function showManualEdit() {
      container.style.display = 'block';
      if (!editorInstance && editorEl && typeof createRelaEditor === 'function') {
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
      document.querySelectorAll('.resolve-value-selectable[data-side="' + side + '"]').forEach(function(cell) {
        cell.click();
      });
      var contentRadio = document.querySelector('input[name="content"][value="' + side + '"]');
      if (contentRadio) contentRadio.click();
    };

    // Click-to-select property values
    document.querySelectorAll('.resolve-value-selectable').forEach(function(cell) {
      cell.addEventListener('click', function() {
        var prop = this.getAttribute('data-prop');
        var side = this.getAttribute('data-side');
        var row = this.closest('tr');

        var radio = this.querySelector('input[type="radio"]');
        if (radio) radio.checked = true;

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

    document.addEventListener('keydown', function conflictKeyHandler(e) {
      var tag = e.target.tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
      if (e.target.isContentEditable) return;

      var table = document.querySelector('.resolve-table');
      if (!table) return;

      switch(e.key) {
        case 'ArrowDown':
        case 'j':
          e.preventDefault();
          if (focusedIndex < 0) {
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
  }

  document.addEventListener('DOMContentLoaded', initConflictResolution);
  document.addEventListener('htmx:afterSettle', initConflictResolution);
})();

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
