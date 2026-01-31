package dataentry

// graphTemplates contains HTML templates for the graph visualization page.
const graphTemplates = `
{{- define "graph-page" -}}
<!DOCTYPE html>
<html lang="en">
<head>
<title>{{ .App.Name }} - Graph Explorer</title>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<script src="/static/cytoscape.min.js"></script>
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
  --accent: #6366f1; --accent-light: rgba(99,102,241,0.12);
  --surface: rgba(255,255,255,0.72); --blur: blur(20px);
}
body { font-family: var(--font); color: var(--text); line-height: 1.6; display: flex; min-height: 100vh; margin: 0; overflow: hidden; }

/* ── App sidebar (shared with data-entry) ── */
.sidebar { width: 240px; background: var(--bg-sidebar); position: fixed; top: 0; left: 0; bottom: 0; overflow-y: auto; z-index: 100; display: flex; flex-direction: column; }
.sidebar-header { padding: 20px 20px 16px; border-bottom: 1px solid rgba(255,255,255,0.1); }
.sidebar-header h1 { font-size: 16px; font-weight: 700; color: #fff; }
.sidebar-header p { font-size: 12px; color: var(--text-sidebar); margin-top: 4px; }
.sidebar nav { padding: 8px 0; flex: 1; }
.sidebar nav a { display: flex; align-items: center; gap: 10px; padding: 8px 20px; color: var(--text-sidebar); text-decoration: none; font-size: 14px; font-weight: 500; transition: all 0.15s; border-left: 3px solid transparent; }
.sidebar nav a:hover { background: var(--bg-sidebar-hover); color: var(--text-sidebar-active); }
.sidebar nav a.active { background: var(--bg-sidebar-active); color: var(--text-sidebar-active); border-left-color: var(--primary); }
.nav-count { margin-left: auto; font-size: 11px; color: rgba(255,255,255,0.4); font-weight: 400; }

/* ── Graph area ── */
.graph-container {
  margin-left: 240px; flex: 1; display: flex; flex-direction: column; height: 100vh;
  background: var(--bg);
  background-image:
    radial-gradient(at 20% 20%, rgba(99,102,241,0.07) 0%, transparent 50%),
    radial-gradient(at 80% 80%, rgba(168,85,247,0.05) 0%, transparent 50%),
    radial-gradient(at 50% 0%, rgba(6,182,212,0.04) 0%, transparent 50%);
}

/* ── Graph header ── */
.graph-header {
  display: flex; align-items: center; gap: 12px;
  padding: 10px 20px;
  background: var(--surface); backdrop-filter: var(--blur);
  border-bottom: 1px solid rgba(0,0,0,0.06); z-index: 20;
}
.graph-header h2 { font-size: 15px; font-weight: 700; letter-spacing: -0.3px; white-space: nowrap; }
.graph-search { flex: 1; max-width: 300px; position: relative; }
.graph-search input {
  width: 100%; padding: 7px 12px 7px 32px; border: 1px solid rgba(0,0,0,0.06);
  border-radius: 10px; font-size: 13px; font-family: inherit;
  background: rgba(255,255,255,0.6); color: var(--text); outline: none; transition: all 0.2s;
}
.graph-search input:focus { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-light); }
.graph-search::before { content: '\2315'; position: absolute; left: 10px; top: 50%; transform: translateY(-50%); color: var(--text-muted); font-size: 14px; }
.graph-header-right { display: flex; align-items: center; gap: 8px; margin-left: auto; }
.view-toggle { display: flex; background: rgba(0,0,0,0.04); border-radius: 8px; padding: 2px; }
.view-toggle button {
  padding: 5px 14px; border: none; background: none; border-radius: 6px;
  font-size: 12px; font-weight: 600; font-family: inherit; cursor: pointer;
  color: var(--text-muted); transition: all 0.2s;
}
.view-toggle button.active { background: var(--bg-card); color: var(--text); box-shadow: 0 1px 3px rgba(0,0,0,0.06); }
.graph-stats { font-size: 11px; color: var(--text-muted); font-weight: 500; white-space: nowrap; }
.graph-stats strong { color: var(--accent); font-weight: 700; }

/* ── Graph main layout ── */
.graph-main { flex: 1; display: flex; overflow: hidden; position: relative; }

/* ── Filter sidebar ── */
.filter-sidebar {
  width: 220px; background: var(--surface); backdrop-filter: var(--blur);
  border-right: 1px solid rgba(0,0,0,0.06); padding: 16px 12px;
  overflow-y: auto; z-index: 10; display: flex; flex-direction: column; gap: 12px;
}
.filter-section h3 {
  font-size: 10px; text-transform: uppercase; letter-spacing: 0.8px;
  color: var(--text-muted); font-weight: 700; margin-bottom: 8px;
  display: flex; align-items: center; justify-content: space-between;
}
.filter-section h3 .toggle-all {
  font-size: 10px; color: var(--accent); cursor: pointer; text-transform: none;
  letter-spacing: 0; font-weight: 600;
}
.filter-section h3 .toggle-all:hover { text-decoration: underline; }
.type-item {
  display: flex; align-items: center; gap: 8px;
  padding: 6px 8px; border-radius: 8px; cursor: pointer;
  transition: all 0.15s; user-select: none; font-size: 12px; font-weight: 500;
}
.type-item:hover { background: rgba(0,0,0,0.03); }
.type-item.inactive { opacity: 0.4; }
.type-dot { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; transition: transform 0.2s; }
.type-item:hover .type-dot { transform: scale(1.2); }
.type-label { flex: 1; }
.type-count {
  font-size: 10px; color: var(--text-muted); background: rgba(0,0,0,0.04);
  padding: 1px 6px; border-radius: 10px; font-weight: 600;
}
.filter-section.relations .type-dot { width: 10px; height: 3px; border-radius: 2px; }

/* ── Cytoscape canvas ── */
#cy { flex: 1; z-index: 1; }

/* ── Detail panel ── */
.detail-panel {
  width: 340px; background: var(--surface); backdrop-filter: var(--blur);
  border-left: 1px solid rgba(0,0,0,0.06); overflow-y: auto;
  transform: translateX(100%); transition: transform 0.35s cubic-bezier(0.32, 0.72, 0, 1);
  z-index: 10; position: relative;
}
.detail-panel.open { transform: translateX(0); }
.detail-panel-header { padding: 20px 20px 16px; }
.close-btn {
  position: absolute; top: 14px; right: 14px;
  background: rgba(0,0,0,0.05); border: none; font-size: 14px; cursor: pointer;
  color: var(--text-muted); width: 26px; height: 26px; border-radius: 8px;
  display: flex; align-items: center; justify-content: center; transition: all 0.15s;
}
.close-btn:hover { background: rgba(0,0,0,0.1); color: var(--text); }
.detail-badge {
  display: inline-flex; align-items: center; gap: 5px;
  padding: 3px 10px; border-radius: 8px; font-size: 10px; font-weight: 700;
  text-transform: uppercase; letter-spacing: 0.5px; color: white; margin-bottom: 10px;
}
.detail-panel h2 { font-size: 17px; font-weight: 700; letter-spacing: -0.3px; margin-bottom: 3px; }
.detail-id { font-size: 12px; color: var(--text-muted); font-family: var(--font-mono); }
.detail-panel-body { padding: 0 20px 20px; }
.props-list { list-style: none; margin-bottom: 16px; }
.props-list li {
  display: flex; justify-content: space-between; padding: 7px 0;
  border-bottom: 1px solid rgba(0,0,0,0.04); font-size: 12px;
}
.props-list li:last-child { border: none; }
.pk { color: var(--text-muted); }
.pv { font-weight: 600; text-align: right; max-width: 180px; overflow: hidden; text-overflow: ellipsis; }
.rels-section h4 {
  font-size: 10px; text-transform: uppercase; letter-spacing: 0.6px;
  color: var(--text-muted); font-weight: 700; margin: 12px 0 6px;
}
.rel-item {
  display: flex; align-items: center; gap: 8px;
  padding: 6px 10px; border-radius: 8px; margin-bottom: 3px;
  font-size: 12px; cursor: pointer; transition: all 0.15s;
}
.rel-item:hover { background: rgba(0,0,0,0.03); transform: translateX(3px); }
.rel-dot { width: 6px; height: 6px; border-radius: 50%; flex-shrink: 0; }
.rel-type { font-size: 10px; color: var(--text-muted); }
.rel-name { font-weight: 500; }
.detail-link {
  display: inline-block; margin-top: 12px; padding: 5px 14px; border-radius: 8px;
  font-size: 12px; font-weight: 600; background: var(--accent-light); color: var(--accent);
  text-decoration: none; transition: all 0.15s;
}
.detail-link:hover { background: var(--accent); color: #fff; }

/* ── Toolbar ── */
.graph-toolbar {
  position: absolute; bottom: 20px; left: 50%; transform: translateX(-50%);
  display: flex; align-items: center; gap: 3px;
  background: var(--surface); backdrop-filter: var(--blur);
  padding: 5px; border-radius: 14px;
  box-shadow: 0 4px 24px rgba(0,0,0,0.08), 0 0 0 1px rgba(0,0,0,0.06);
  z-index: 15;
}
.graph-toolbar button {
  padding: 6px 14px; border: none; background: none; border-radius: 9px;
  font-size: 11px; font-weight: 600; font-family: inherit; cursor: pointer;
  color: var(--text-muted); transition: all 0.15s; white-space: nowrap;
}
.graph-toolbar button:hover { background: rgba(0,0,0,0.04); color: var(--text); }
.graph-toolbar button.active { background: var(--accent); color: white; box-shadow: 0 2px 8px rgba(99,102,241,0.25); }
.graph-toolbar .sep { width: 1px; height: 18px; background: rgba(0,0,0,0.06); margin: 0 2px; }
.depth-ctrl {
  display: flex; align-items: center; gap: 5px; padding: 0 8px;
  font-size: 11px; font-weight: 600; color: var(--text-muted);
}
.depth-ctrl input { width: 60px; accent-color: var(--accent); }
.depth-num {
  background: var(--accent-light); color: var(--accent); font-weight: 700;
  width: 20px; height: 20px; border-radius: 6px;
  display: flex; align-items: center; justify-content: center; font-size: 10px;
}

/* ── Loading state ── */
.graph-loading {
  position: absolute; inset: 0; display: flex; align-items: center; justify-content: center;
  background: var(--surface); backdrop-filter: var(--blur); z-index: 50;
  font-size: 14px; font-weight: 600; color: var(--text-muted);
}
.graph-loading.hidden { display: none; }
</style>
</head>
<body>
{{ template "sidebar" . }}
<div class="graph-container">
  <div class="graph-header">
    <h2>Graph Explorer</h2>
    <div class="graph-search"><input type="text" placeholder="Search entities..." id="searchInput"></div>
    <div class="graph-header-right">
      <div class="view-toggle">
        <button class="active" id="btnContent">Content</button>
        <button id="btnMetamodel">Metamodel</button>
      </div>
      <div class="graph-stats"><strong id="nodeCount">0</strong> nodes &middot; <strong id="edgeCount">0</strong> edges</div>
    </div>
  </div>
  <div class="graph-main">
    <div class="filter-sidebar" id="filterSidebar"></div>
    <div id="cy"></div>
    <div class="detail-panel" id="detailPanel">
      <div class="detail-panel-header">
        <button class="close-btn" id="closePanel">&times;</button>
        <div id="detailHead"></div>
      </div>
      <div class="detail-panel-body" id="detailBody"></div>
    </div>
    <div class="graph-loading" id="graphLoading">Loading graph data...</div>
  </div>
  <div class="graph-toolbar">
    <button class="active" id="btnForce">Force</button>
    <button id="btnHierarchy">Hierarchy</button>
    <button id="btnCircle">Circle</button>
    <button id="btnGrid">Grid</button>
    <div class="sep"></div>
    <button id="btnFit">Fit</button>
    <button id="btnLabels">Labels</button>
    <button id="btnFocus">Focus</button>
    <div class="sep"></div>
    <div class="depth-ctrl">
      Depth <input type="range" min="1" max="5" value="2" id="depthSlider">
      <div class="depth-num" id="depthVal">2</div>
    </div>
  </div>
</div>

<script>
(function() {
  var currentMode = 'content';
  var graphData = null;
  var typeColors = {};
  var cy = null;
  var selectedNode = null;
  var focusMode = false;
  var currentLayout = 'force';
  var edgeLabelsVisible = false;

  var layouts = {
    force: { name: 'cose', animate: true, animationDuration: 800, nodeRepulsion: function() { return 12000; }, idealEdgeLength: function() { return 100; }, gravity: 0.3, padding: 30 },
    hierarchy: { name: 'breadthfirst', animate: true, animationDuration: 800, directed: true, spacingFactor: 1.0, padding: 30 },
    circle: { name: 'circle', animate: true, animationDuration: 800, padding: 30 },
    grid: { name: 'grid', animate: true, animationDuration: 800, padding: 30 }
  };

  function loadData(mode) {
    currentMode = mode;
    document.getElementById('graphLoading').classList.remove('hidden');
    fetch('/api/graph-data?mode=' + mode)
      .then(function(r) { return r.json(); })
      .then(function(data) {
        graphData = data;
        typeColors = {};
        data.entityTypes.forEach(function(et) { typeColors[et.type] = et.color; });
        buildFilters(data);
        buildGraph(data);
        document.getElementById('graphLoading').classList.add('hidden');
      })
      .catch(function(err) {
        document.getElementById('graphLoading').textContent = 'Failed to load graph data';
        console.error(err);
      });
  }

  function buildFilters(data) {
    var sidebar = document.getElementById('filterSidebar');
    var html = '<div class="filter-section"><h3>Entity Types <span class="toggle-all" id="toggleTypes">hide all</span></h3>';
    data.entityTypes.forEach(function(et) {
      html += '<div class="type-item" data-type="' + escapeAttr(et.type) + '">' +
        '<div class="type-dot" style="background:' + escapeAttr(et.color) + '"></div>' +
        '<span class="type-label">' + escapeHTML(et.label) + '</span>' +
        '<span class="type-count">' + et.count + '</span></div>';
    });
    html += '</div>';

    html += '<div class="filter-section relations"><h3>Relation Types <span class="toggle-all" id="toggleRels">hide all</span></h3>';
    data.relationTypes.forEach(function(rt) {
      html += '<div class="type-item" data-rel="' + escapeAttr(rt.type) + '">' +
        '<div class="type-dot" style="background:var(--text-muted)"></div>' +
        '<span class="type-label">' + escapeHTML(rt.label) + '</span>' +
        '<span class="type-count">' + rt.count + '</span></div>';
    });
    html += '</div>';
    sidebar.innerHTML = html;
    attachFilterListeners();
  }

  function buildGraph(data) {
    var elements = [];
    data.nodes.forEach(function(n) {
      elements.push({ data: { id: n.id, label: n.id + '\n' + n.title, type: n.type, title: n.title, properties: n.properties || {} } });
    });
    data.edges.forEach(function(e, i) {
      elements.push({ data: { id: 'e' + i, source: e.source, target: e.target, label: e.type, relType: e.type } });
    });

    if (cy) { cy.destroy(); }

    cy = cytoscape({
      container: document.getElementById('cy'),
      elements: elements,
      style: [
        {
          selector: 'node',
          style: {
            'label': 'data(label)', 'text-wrap': 'wrap', 'text-max-width': '100px',
            'font-size': '8px', 'font-weight': '500',
            'font-family': '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
            'text-valign': 'center', 'text-halign': 'center',
            'width': '110px', 'height': '42px', 'shape': 'roundrectangle',
            'background-color': function(ele) { return typeColors[ele.data('type')] || '#888'; },
            'background-opacity': 0.85, 'color': '#fff', 'border-width': 0,
            'text-outline-width': 0, 'overlay-padding': '4px', 'overlay-opacity': 0,
            'shadow-blur': 8,
            'shadow-color': function(ele) { return typeColors[ele.data('type')] || '#888'; },
            'shadow-offset-y': 3, 'shadow-opacity': 0.12,
            'transition-property': 'background-opacity, opacity, width, height, shadow-blur, shadow-opacity',
            'transition-duration': '0.25s'
          }
        },
        { selector: 'node:selected', style: { 'border-width': 2.5, 'border-color': '#fff', 'shadow-blur': 18, 'shadow-opacity': 0.3, 'width': '118px', 'height': '46px' } },
        { selector: 'node.hover', style: { 'shadow-blur': 14, 'shadow-opacity': 0.25, 'background-opacity': 1 } },
        { selector: 'node.faded', style: { 'opacity': 0.08 } },
        { selector: 'node.highlighted', style: { 'border-width': 2, 'border-color': '#fff', 'shadow-blur': 16, 'shadow-opacity': 0.3 } },
        {
          selector: 'edge',
          style: {
            'width': 1, 'line-color': '#c7cdd6', 'target-arrow-color': '#c7cdd6',
            'target-arrow-shape': 'triangle', 'arrow-scale': 0.6,
            'curve-style': 'bezier', 'opacity': 0.6,
            'transition-property': 'line-color, target-arrow-color, width, opacity',
            'transition-duration': '0.25s'
          }
        },
        { selector: 'edge.faded', style: { 'opacity': 0.03 } },
        { selector: 'edge.highlighted', style: { 'width': 2, 'line-color': '#6366f1', 'target-arrow-color': '#6366f1', 'opacity': 0.9 } },
        { selector: 'edge.show-label', style: { 'label': 'data(label)', 'font-size': '7px', 'font-weight': '500', 'font-family': '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif', 'color': '#64748b', 'text-background-color': '#fff', 'text-background-opacity': 0.85, 'text-background-padding': '2px', 'text-rotation': 'autorotate', 'text-margin-y': '-6px' } }
      ],
      layout: layouts.force,
      wheelSensitivity: 0.3, minZoom: 0.15, maxZoom: 3
    });

    document.getElementById('nodeCount').textContent = data.nodes.length;
    document.getElementById('edgeCount').textContent = data.edges.length;
    selectedNode = null;
    focusMode = false;
    document.getElementById('btnFocus').classList.remove('active');
    if (edgeLabelsVisible) cy.edges().addClass('show-label');

    // Interactions
    cy.on('tap', 'node', function(evt) {
      selectedNode = evt.target;
      showDetail(evt.target.data());
      highlightNeighborhood(evt.target);
    });
    cy.on('tap', function(evt) {
      if (evt.target === cy) { clearHighlight(); closeDetail(); selectedNode = null; }
    });
    cy.on('mouseover', 'node', function(evt) { if (!selectedNode) evt.target.addClass('hover'); document.body.style.cursor = 'pointer'; });
    cy.on('mouseout', 'node', function(evt) { evt.target.removeClass('hover'); document.body.style.cursor = 'default'; });
  }

  function highlightNeighborhood(node) {
    cy.elements().addClass('faded');
    var depth = parseInt(document.getElementById('depthSlider').value);
    var collected = node.closedNeighborhood();
    for (var i = 1; i < depth; i++) collected = collected.closedNeighborhood();
    collected.removeClass('faded').addClass('highlighted');
    node.removeClass('faded').addClass('highlighted');
  }

  function clearHighlight() { cy.elements().removeClass('faded highlighted'); }

  function showDetail(data) {
    var color = typeColors[data.type] || '#888';
    var headHTML = '<div class="detail-badge" style="background:' + escapeAttr(color) + '">' + escapeHTML(data.type) + '</div>' +
      '<h2>' + escapeHTML(data.title) + '</h2>' +
      '<div class="detail-id">' + escapeHTML(data.id) + '</div>';
    document.getElementById('detailHead').innerHTML = headHTML;

    var props = data.properties || {};
    var keys = Object.keys(props).sort();
    var html = '<ul class="props-list">';
    keys.forEach(function(k) {
      html += '<li><span class="pk">' + escapeHTML(k) + '</span><span class="pv">' + escapeHTML(props[k]) + '</span></li>';
    });
    html += '</ul>';

    // Relations from Cytoscape edges
    var ce = cy.getElementById(data.id).connectedEdges();
    var out = ce.filter(function(e) { return e.source().id() === data.id; });
    var inc = ce.filter(function(e) { return e.target().id() === data.id; });

    html += renderRels(out, 'Outgoing', data.id);
    html += renderRels(inc, 'Incoming', data.id);

    // Link to entity detail page (content mode only)
    if (currentMode === 'content') {
      html += '<a class="detail-link" href="/entity/' + encodeURIComponent(data.type) + '/' + encodeURIComponent(data.id) + '">View details &rarr;</a>';
    }

    document.getElementById('detailBody').innerHTML = html;
    document.getElementById('detailPanel').classList.add('open');
  }

  function renderRels(rels, direction, sourceId) {
    if (!rels.length) return '';
    var h = '<div class="rels-section"><h4>' + direction + ' (' + rels.length + ')</h4>';
    rels.forEach(function(e) {
      var other = direction === 'Outgoing' ? e.target().data() : e.source().data();
      var otherColor = typeColors[other.type] || '#888';
      h += '<div class="rel-item" data-id="' + escapeAttr(other.id) + '">' +
        '<div class="rel-dot" style="background:' + escapeAttr(otherColor) + '"></div>' +
        '<div><div class="rel-type">' + escapeHTML(e.data('relType')) + '</div>' +
        '<div class="rel-name">' + escapeHTML(other.id) + ' &middot; ' + escapeHTML(other.title) + '</div></div></div>';
    });
    return h + '</div>';
  }

  function closeDetail() { document.getElementById('detailPanel').classList.remove('open'); }

  function escapeHTML(str) {
    if (!str) return '';
    return str.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
  }
  function escapeAttr(str) {
    if (!str) return '';
    return str.replace(/&/g,'&amp;').replace(/"/g,'&quot;').replace(/'/g,'&#39;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
  }

  // Layout controls
  function runLayout(opts) {
    cy.elements(':visible').layout(opts).run();
  }

  var reLayoutTimer = null;
  function reLayout() {
    clearTimeout(reLayoutTimer);
    reLayoutTimer = setTimeout(function() {
      runLayout(Object.assign({}, layouts[currentLayout], { animationDuration: 500 }));
    }, 200);
  }

  function setLayout(name) {
    currentLayout = name;
    document.querySelectorAll('.graph-toolbar button').forEach(function(b) { b.classList.remove('active'); });
    document.getElementById('btn' + name.charAt(0).toUpperCase() + name.slice(1)).classList.add('active');
    runLayout(layouts[name]);
  }

  document.getElementById('btnForce').addEventListener('click', function() { setLayout('force'); });
  document.getElementById('btnHierarchy').addEventListener('click', function() { setLayout('hierarchy'); });
  document.getElementById('btnCircle').addEventListener('click', function() { setLayout('circle'); });
  document.getElementById('btnGrid').addEventListener('click', function() { setLayout('grid'); });
  document.getElementById('btnFit').addEventListener('click', function() {
    if (cy) cy.animate({ fit: { padding: 30 } }, { duration: 400 });
  });

  // Edge labels toggle
  document.getElementById('btnLabels').addEventListener('click', function() {
    edgeLabelsVisible = !edgeLabelsVisible;
    this.classList.toggle('active', edgeLabelsVisible);
    if (cy) {
      if (edgeLabelsVisible) cy.edges().addClass('show-label');
      else cy.edges().removeClass('show-label');
    }
  });

  // Depth slider
  document.getElementById('depthSlider').addEventListener('input', function() {
    document.getElementById('depthVal').textContent = this.value;
    if (selectedNode) highlightNeighborhood(selectedNode);
  });

  // Focus mode
  document.getElementById('btnFocus').addEventListener('click', function() {
    focusMode = !focusMode;
    this.classList.toggle('active', focusMode);
    if (focusMode && selectedNode) {
      highlightNeighborhood(selectedNode);
      cy.elements('.faded').style('display', 'none');
      cy.animate({ fit: { eles: cy.elements(':visible'), padding: 30 } }, { duration: 400 });
    } else if (cy) {
      // Restore type filter visibility
      restoreFilterVisibility();
      clearHighlight();
    }
    updateStats();
  });

  // Close panel
  document.getElementById('closePanel').addEventListener('click', function() {
    closeDetail();
    if (cy) clearHighlight();
    selectedNode = null;
  });

  // Event delegation for relation item clicks in detail panel
  document.getElementById('detailBody').addEventListener('click', function(evt) {
    var el = evt.target.closest('.rel-item');
    if (!el || !el.dataset.id) return;
    var n = cy.getElementById(el.dataset.id);
    if (n.length) {
      cy.animate({ center: { eles: n }, zoom: 1.5 }, { duration: 400 });
      n.select();
      showDetail(n.data());
      highlightNeighborhood(n);
      selectedNode = n;
    }
  });

  // View toggle
  document.getElementById('btnContent').addEventListener('click', function() {
    if (currentMode === 'content') return;
    document.getElementById('btnContent').classList.add('active');
    document.getElementById('btnMetamodel').classList.remove('active');
    loadData('content');
  });
  document.getElementById('btnMetamodel').addEventListener('click', function() {
    if (currentMode === 'metamodel') return;
    document.getElementById('btnMetamodel').classList.add('active');
    document.getElementById('btnContent').classList.remove('active');
    loadData('metamodel');
  });

  // Filter listeners
  function attachFilterListeners() {
    document.querySelectorAll('.type-item[data-type]').forEach(function(item) {
      item.addEventListener('click', function() {
        this.classList.toggle('inactive');
        var type = this.dataset.type;
        var visible = !this.classList.contains('inactive');
        cy.nodes('[type="' + type + '"]').forEach(function(n) { n.style('display', visible ? 'element' : 'none'); });
        updateStats();
        reLayout();
      });
    });

    document.querySelectorAll('.type-item[data-rel]').forEach(function(item) {
      item.addEventListener('click', function() {
        this.classList.toggle('inactive');
        var rt = this.dataset.rel;
        var visible = !this.classList.contains('inactive');
        cy.edges('[relType="' + rt + '"]').forEach(function(e) { e.style('display', visible ? 'element' : 'none'); });
        updateStats();
      });
    });

    var toggleTypesEl = document.getElementById('toggleTypes');
    if (toggleTypesEl) {
      toggleTypesEl.addEventListener('click', function() {
        var items = document.querySelectorAll('.type-item[data-type]');
        var allActive = Array.from(items).every(function(i) { return !i.classList.contains('inactive'); });
        items.forEach(function(i) {
          if (allActive) i.classList.add('inactive'); else i.classList.remove('inactive');
          var type = i.dataset.type;
          cy.nodes('[type="' + type + '"]').forEach(function(n) { n.style('display', allActive ? 'none' : 'element'); });
        });
        this.textContent = allActive ? 'show all' : 'hide all';
        updateStats();
        reLayout();
      });
    }

    var toggleRelsEl = document.getElementById('toggleRels');
    if (toggleRelsEl) {
      toggleRelsEl.addEventListener('click', function() {
        var items = document.querySelectorAll('.type-item[data-rel]');
        var allActive = Array.from(items).every(function(i) { return !i.classList.contains('inactive'); });
        items.forEach(function(i) {
          if (allActive) i.classList.add('inactive'); else i.classList.remove('inactive');
          var rt = i.dataset.rel;
          cy.edges('[relType="' + rt + '"]').forEach(function(e) { e.style('display', allActive ? 'none' : 'element'); });
        });
        this.textContent = allActive ? 'show all' : 'hide all';
        updateStats();
      });
    }
  }

  function restoreFilterVisibility() {
    var hiddenTypes = new Set();
    document.querySelectorAll('.type-item[data-type].inactive').forEach(function(i) { hiddenTypes.add(i.dataset.type); });
    var hiddenRels = new Set();
    document.querySelectorAll('.type-item[data-rel].inactive').forEach(function(i) { hiddenRels.add(i.dataset.rel); });
    cy.nodes().forEach(function(n) { n.style('display', hiddenTypes.has(n.data('type')) ? 'none' : 'element'); });
    cy.edges().forEach(function(e) { e.style('display', hiddenRels.has(e.data('relType')) ? 'none' : 'element'); });
  }

  function updateStats() {
    if (!cy) return;
    document.getElementById('nodeCount').textContent = cy.nodes(':visible').length;
    document.getElementById('edgeCount').textContent = cy.edges(':visible').length;
  }

  // Search
  var searchTimer = null;
  document.getElementById('searchInput').addEventListener('input', function() {
    var q = this.value.toLowerCase().trim();
    var hiddenTypes = new Set();
    document.querySelectorAll('.type-item[data-type].inactive').forEach(function(i) { hiddenTypes.add(i.dataset.type); });
    if (!q) {
      cy.nodes().forEach(function(n) {
        n.style('display', hiddenTypes.has(n.data('type')) ? 'none' : 'element');
      });
      clearHighlight();
      updateStats();
      reLayout();
      return;
    }
    cy.nodes().forEach(function(n) {
      var d = n.data();
      var matches = d.id.toLowerCase().indexOf(q) >= 0 || d.title.toLowerCase().indexOf(q) >= 0;
      var hidden = hiddenTypes.has(d.type);
      n.style('display', (matches && !hidden) ? 'element' : 'none');
    });
    updateStats();
    clearTimeout(searchTimer);
    searchTimer = setTimeout(function() { reLayout(); }, 400);
  });

  // Initial load
  loadData('content');
})();
</script>
</body>
</html>
{{- end -}}
`
