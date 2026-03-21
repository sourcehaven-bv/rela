<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import cytoscape from 'cytoscape'
import type { Core, NodeSingular, EdgeSingular } from 'cytoscape'
import { getGraphData } from '@/api'
import type { GraphData, GraphNode } from '@/api/graph'

const router = useRouter()

// State
const loading = ref(true)
const graphData = ref<GraphData | null>(null)
const mode = ref<'content' | 'metamodel'>('content')
const selectedNode = ref<GraphNode | null>(null)
const cyContainer = ref<HTMLElement | null>(null)

// Graph instance
let cy: Core | null = null

// Layout settings
const currentLayout = ref('force')
const edgeLabelsVisible = ref(false)
const focusMode = ref(false)
const depth = ref(2)

// Filter state
const hiddenEntityTypes = ref<Set<string>>(new Set())
const hiddenRelationTypes = ref<Set<string>>(new Set())

// Computed
const entityTypes = computed(() => graphData.value?.entityTypes || [])
const relationTypes = computed(() => graphData.value?.relationTypes || [])
const typeColors = computed(() => {
  const map: Record<string, string> = {}
  for (const et of entityTypes.value) {
    map[et.type] = et.color
  }
  return map
})

const visibleNodeCount = computed(() => {
  if (!graphData.value) return 0
  // Count nodes not hidden by entity type filter
  return graphData.value.nodes.filter(n => !hiddenEntityTypes.value.has(n.type)).length
})

const visibleEdgeCount = computed(() => {
  if (!graphData.value) return 0
  // Count edges not hidden by relation type filter
  return graphData.value.edges.filter(e => !hiddenRelationTypes.value.has(e.type)).length
})

// Connected edges for selected node
const outgoingRelations = computed((): EdgeSingular[] => {
  if (!cy || !selectedNode.value) return []
  const nodeId = selectedNode.value.id
  const node = cy.getElementById(nodeId)
  return node.connectedEdges().filter((e: EdgeSingular) => e.source().id() === nodeId).toArray() as EdgeSingular[]
})

const incomingRelations = computed((): EdgeSingular[] => {
  if (!cy || !selectedNode.value) return []
  const nodeId = selectedNode.value.id
  const node = cy.getElementById(nodeId)
  return node.connectedEdges().filter((e: EdgeSingular) => e.target().id() === nodeId).toArray() as EdgeSingular[]
})

// Layout configurations
const layouts: Record<string, cytoscape.LayoutOptions> = {
  force: {
    name: 'cose',
    animate: true,
    animationDuration: 800,
    nodeRepulsion: () => 12000,
    idealEdgeLength: () => 100,
    gravity: 0.3,
    padding: 50,
  },
  hierarchy: {
    name: 'breadthfirst',
    animate: true,
    animationDuration: 800,
    directed: true,
    spacingFactor: 1.0,
    padding: 50,
  },
  circle: {
    name: 'circle',
    animate: true,
    animationDuration: 800,
    padding: 50,
  },
  grid: {
    name: 'grid',
    animate: true,
    animationDuration: 800,
    padding: 50,
  },
}

// Methods
async function loadGraphData() {
  loading.value = true
  selectedNode.value = null
  focusMode.value = false

  try {
    graphData.value = await getGraphData(mode.value)
  } catch (err) {
    console.error('Graph load error:', err)
  } finally {
    loading.value = false
    // Wait for DOM to render the cy-container after loading=false
    await nextTick()
    buildGraph()
  }
}

function buildGraph() {
  if (!graphData.value || !cyContainer.value) return

  const elements: cytoscape.ElementDefinition[] = []

  // Add nodes
  for (const node of graphData.value.nodes) {
    elements.push({
      data: {
        id: node.id,
        label: `${node.id}\n${node.title}`,
        type: node.type,
        title: node.title,
        properties: node.properties || {},
      },
    })
  }

  // Add edges
  graphData.value.edges.forEach((edge, i) => {
    elements.push({
      data: {
        id: `e${i}`,
        source: edge.source,
        target: edge.target,
        label: edge.type,
        relType: edge.type,
      },
    })
  })

  // Destroy existing instance
  if (cy) {
    cy.destroy()
  }

  // Create new cytoscape instance
  /* eslint-disable @typescript-eslint/consistent-type-assertions */
  // Cytoscape style definitions require type assertions for dynamic style functions
  cy = cytoscape({
    container: cyContainer.value,
    elements,
    style: [
      {
        selector: 'node',
        style: {
          label: 'data(label)',
          'text-wrap': 'wrap',
          'text-max-width': '100px',
          'font-size': '8px',
          'font-weight': 500,
          'font-family': '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
          'text-valign': 'center',
          'text-halign': 'center',
          width: '110px',
          height: '42px',
          shape: 'roundrectangle',
          'background-color': (ele: NodeSingular) => typeColors.value[ele.data('type')] || '#888',
          'background-opacity': 0.85,
          color: '#fff',
          'border-width': 0,
          'text-outline-width': 0,
          'overlay-padding': '4px',
          'overlay-opacity': 0,
          'shadow-blur': 8,
          'shadow-color': (ele: NodeSingular) => typeColors.value[ele.data('type')] || '#888',
          'shadow-offset-y': 3,
          'shadow-opacity': 0.12,
          'transition-property': 'background-opacity, opacity, width, height, shadow-blur, shadow-opacity',
          'transition-duration': 250,
        } as cytoscape.Css.Node,
      },
      {
        selector: 'node:selected',
        style: {
          'border-width': 2.5,
          'border-color': '#fff',
          'shadow-blur': 18,
          'shadow-opacity': 0.3,
          width: '118px',
          height: '46px',
        } as cytoscape.Css.Node,
      },
      {
        selector: 'node.hover',
        style: {
          'shadow-blur': 14,
          'shadow-opacity': 0.25,
          'background-opacity': 1,
        } as cytoscape.Css.Node,
      },
      {
        selector: 'node.faded',
        style: {
          opacity: 0.08,
        } as cytoscape.Css.Node,
      },
      {
        selector: 'node.highlighted',
        style: {
          'border-width': 2,
          'border-color': '#fff',
          'shadow-blur': 16,
          'shadow-opacity': 0.3,
        } as cytoscape.Css.Node,
      },
      {
        selector: 'edge',
        style: {
          width: 1,
          'line-color': '#c7cdd6',
          'target-arrow-color': '#c7cdd6',
          'target-arrow-shape': 'triangle',
          'arrow-scale': 0.6,
          'curve-style': 'bezier',
          opacity: 0.6,
          'transition-property': 'line-color, target-arrow-color, width, opacity',
          'transition-duration': 250,
        } as cytoscape.Css.Edge,
      },
      {
        selector: 'edge.faded',
        style: {
          opacity: 0.03,
        } as cytoscape.Css.Edge,
      },
      {
        selector: 'edge.highlighted',
        style: {
          width: 2,
          'line-color': '#6366f1',
          'target-arrow-color': '#6366f1',
          opacity: 0.9,
        } as cytoscape.Css.Edge,
      },
      {
        selector: 'edge.show-label',
        style: {
          label: 'data(label)',
          'font-size': '7px',
          'font-weight': 500,
          'font-family': '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
          color: '#64748b',
          'text-background-color': '#fff',
          'text-background-opacity': 0.85,
          'text-background-padding': '2px',
          'text-rotation': 'autorotate',
          'text-margin-y': -6,
        } as cytoscape.Css.Edge,
      },
    ],
    layout: layouts.force,
    wheelSensitivity: 0.3,
    minZoom: 0.15,
    maxZoom: 3,
  })
  /* eslint-enable @typescript-eslint/consistent-type-assertions */

  // Reset filters
  hiddenEntityTypes.value = new Set()
  hiddenRelationTypes.value = new Set()

  // Apply edge labels if enabled
  if (edgeLabelsVisible.value) {
    cy.edges().addClass('show-label')
  }

  // Event handlers
  cy.on('tap', 'node', (evt) => {
    const node = evt.target as NodeSingular
    const data = node.data()
    selectedNode.value = {
      id: data.id,
      type: data.type,
      title: data.title,
      properties: data.properties,
    }
    highlightNeighborhood(node)
  })

  cy.on('tap', (evt) => {
    if (evt.target === cy) {
      clearHighlight()
      selectedNode.value = null
    }
  })

  cy.on('mouseover', 'node', (evt) => {
    if (!selectedNode.value) {
      (evt.target as NodeSingular).addClass('hover')
    }
  })

  cy.on('mouseout', 'node', (evt) => {
    (evt.target as NodeSingular).removeClass('hover')
  })
}

function highlightNeighborhood(node: NodeSingular) {
  if (!cy) return
  cy.elements().addClass('faded')

  let collected = node.closedNeighborhood()
  for (let i = 1; i < depth.value; i++) {
    collected = collected.closedNeighborhood()
  }

  collected.removeClass('faded').addClass('highlighted')
  node.removeClass('faded').addClass('highlighted')
}

function clearHighlight() {
  if (!cy) return
  cy.elements().removeClass('faded highlighted')
}

function setLayout(name: string) {
  if (!cy) return
  currentLayout.value = name
  cy.elements(':visible').layout(layouts[name]).run()
}

function fitGraph() {
  if (!cy) return
  cy.animate({ fit: { eles: cy.elements(':visible'), padding: 30 } }, { duration: 400 })
}

function toggleEdgeLabels() {
  if (!cy) return
  edgeLabelsVisible.value = !edgeLabelsVisible.value
  if (edgeLabelsVisible.value) {
    cy.edges().addClass('show-label')
  } else {
    cy.edges().removeClass('show-label')
  }
}

function toggleFocusMode() {
  if (!cy) return
  focusMode.value = !focusMode.value

  if (focusMode.value && selectedNode.value) {
    const node = cy.getElementById(selectedNode.value.id)
    highlightNeighborhood(node as NodeSingular)
    cy.elements('.faded').style('display', 'none')
    cy.animate({ fit: { eles: cy.elements(':visible'), padding: 30 } }, { duration: 400 })
  } else {
    restoreFilterVisibility()
    clearHighlight()
  }
}

function toggleEntityType(type: string) {
  if (hiddenEntityTypes.value.has(type)) {
    hiddenEntityTypes.value.delete(type)
  } else {
    hiddenEntityTypes.value.add(type)
  }
  hiddenEntityTypes.value = new Set(hiddenEntityTypes.value)
  applyFilters()
}

function toggleRelationType(type: string) {
  if (hiddenRelationTypes.value.has(type)) {
    hiddenRelationTypes.value.delete(type)
  } else {
    hiddenRelationTypes.value.add(type)
  }
  hiddenRelationTypes.value = new Set(hiddenRelationTypes.value)
  applyFilters()
}

function applyFilters() {
  if (!cy) return

  cy.nodes().forEach((n: NodeSingular) => {
    const hidden = hiddenEntityTypes.value.has(n.data('type'))
    n.style('display', hidden ? 'none' : 'element')
  })

  cy.edges().forEach((e: EdgeSingular) => {
    const hidden = hiddenRelationTypes.value.has(e.data('relType'))
    e.style('display', hidden ? 'none' : 'element')
  })

  // Re-layout after filtering (200ms delay for CSS transitions)
  setTimeout(() => {
    if (cy) {
      const opts: cytoscape.LayoutOptions = { ...layouts[currentLayout.value], animationDuration: 500 }
      cy.elements(':visible').layout(opts).run()
    }
  }, 200)
}

function restoreFilterVisibility() {
  if (!cy) return

  cy.nodes().forEach((n: NodeSingular) => {
    const hidden = hiddenEntityTypes.value.has(n.data('type'))
    n.style('display', hidden ? 'none' : 'element')
  })

  cy.edges().forEach((e: EdgeSingular) => {
    const hidden = hiddenRelationTypes.value.has(e.data('relType'))
    e.style('display', hidden ? 'none' : 'element')
  })
}

function selectRelatedNode(nodeId: string) {
  if (!cy) return
  const node = cy.getElementById(nodeId) as NodeSingular
  if (node.length) {
    cy.animate({ center: { eles: node }, zoom: 1.5 }, { duration: 400 })
    node.select()
    const data = node.data()
    selectedNode.value = {
      id: data.id,
      type: data.type,
      title: data.title,
      properties: data.properties,
    }
    highlightNeighborhood(node)
  }
}

function openNodeDetail() {
  if (!selectedNode.value || mode.value !== 'content') return
  router.push(`/entity/${selectedNode.value.type}/${selectedNode.value.id}`)
}

function closeDetailPanel() {
  selectedNode.value = null
  clearHighlight()
}

// Lifecycle
onMounted(() => {
  loadGraphData()
})

onBeforeUnmount(() => {
  if (cy) {
    cy.destroy()
    cy = null
  }
})

watch(mode, () => {
  loadGraphData()
})

watch(depth, () => {
  if (selectedNode.value && cy) {
    const node = cy.getElementById(selectedNode.value.id) as NodeSingular
    highlightNeighborhood(node)
  }
})
</script>

<template>
  <div class="graph-view">
    <header class="page-header">
      <h1>Graph Explorer</h1>
      <div class="header-actions">
        <select v-model="mode" class="mode-select">
          <option value="content">Content</option>
          <option value="metamodel">Metamodel</option>
        </select>
        <div class="graph-stats">
          <strong>{{ visibleNodeCount }}</strong> nodes &middot;
          <strong>{{ visibleEdgeCount }}</strong> edges
        </div>
        <button class="btn btn-secondary" :disabled="loading" @click="loadGraphData">
          {{ loading ? 'Loading...' : 'Refresh' }}
        </button>
      </div>
    </header>

    <div v-if="loading" class="loading-state">
      <div class="spinner" />
      <span>Loading graph...</span>
    </div>

    <div v-else class="graph-container">
      <!-- Sidebar filters -->
      <aside class="graph-sidebar">
        <div class="filter-section">
          <h4>Entity Types</h4>
          <div class="filter-list">
            <label
              v-for="et in entityTypes"
              :key="et.type"
              class="filter-item"
              :class="{ inactive: hiddenEntityTypes.has(et.type) }"
              @click="toggleEntityType(et.type)"
            >
              <span class="color-dot" :style="{ background: et.color }" />
              <span class="filter-label">{{ et.label }}</span>
              <span class="filter-count">{{ et.count }}</span>
            </label>
          </div>
        </div>

        <div class="filter-section">
          <h4>Relation Types</h4>
          <div class="filter-list">
            <label
              v-for="rt in relationTypes"
              :key="rt.type"
              class="filter-item"
              :class="{ inactive: hiddenRelationTypes.has(rt.type) }"
              @click="toggleRelationType(rt.type)"
            >
              <span class="filter-label">{{ rt.label }}</span>
              <span class="filter-count">{{ rt.count }}</span>
            </label>
          </div>
        </div>

        <!-- Depth control -->
        <div class="filter-section">
          <h4>Neighborhood Depth</h4>
          <div class="depth-control">
            <input v-model.number="depth" type="range" min="1" max="5" />
            <span class="depth-value">{{ depth }}</span>
          </div>
        </div>

        <!-- Node detail panel -->
        <div v-if="selectedNode" class="detail-panel">
          <div class="detail-header">
            <span class="detail-badge" :style="{ background: typeColors[selectedNode.type] }">
              {{ selectedNode.type }}
            </span>
            <button class="close-btn" @click="closeDetailPanel">&times;</button>
          </div>
          <h3>{{ selectedNode.title }}</h3>
          <p class="detail-id">{{ selectedNode.id }}</p>

          <div v-if="Object.keys(selectedNode.properties).length > 0" class="detail-properties">
            <div v-for="(value, key) in selectedNode.properties" :key="key" class="detail-prop">
              <span class="prop-key">{{ key }}:</span>
              <span class="prop-value">{{ value }}</span>
            </div>
          </div>

          <!-- Outgoing relations -->
          <div v-if="outgoingRelations.length" class="relations-section">
            <h4>Outgoing ({{ outgoingRelations.length }})</h4>
            <div
              v-for="edge in outgoingRelations"
              :key="edge.id()"
              class="rel-item"
              @click="selectRelatedNode(edge.target().id())"
            >
              <span
                class="rel-dot"
                :style="{ background: typeColors[edge.target().data('type')] || '#888' }"
              />
              <div>
                <div class="rel-type">{{ edge.data('relType') }}</div>
                <div class="rel-name">{{ edge.target().data('id') }}</div>
              </div>
            </div>
          </div>

          <!-- Incoming relations -->
          <div v-if="incomingRelations.length" class="relations-section">
            <h4>Incoming ({{ incomingRelations.length }})</h4>
            <div
              v-for="edge in incomingRelations"
              :key="edge.id()"
              class="rel-item"
              @click="selectRelatedNode(edge.source().id())"
            >
              <span
                class="rel-dot"
                :style="{ background: typeColors[edge.source().data('type')] || '#888' }"
              />
              <div>
                <div class="rel-type">{{ edge.data('relType') }}</div>
                <div class="rel-name">{{ edge.source().data('id') }}</div>
              </div>
            </div>
          </div>

          <button v-if="mode === 'content'" class="btn btn-primary btn-sm" @click="openNodeDetail">
            View Details
          </button>
        </div>
      </aside>

      <!-- Cytoscape canvas -->
      <div class="graph-canvas">
        <div
          ref="cyContainer"
          class="cy-container"
          :data-node-count="visibleNodeCount"
          :data-edge-count="visibleEdgeCount"
        />

        <!-- Toolbar -->
        <div class="graph-toolbar">
          <button :class="{ active: currentLayout === 'force' }" @click="setLayout('force')">
            Force
          </button>
          <button :class="{ active: currentLayout === 'hierarchy' }" @click="setLayout('hierarchy')">
            Hierarchy
          </button>
          <button :class="{ active: currentLayout === 'circle' }" @click="setLayout('circle')">
            Circle
          </button>
          <button :class="{ active: currentLayout === 'grid' }" @click="setLayout('grid')">
            Grid
          </button>
          <div class="sep" />
          <button @click="fitGraph">Fit</button>
          <button :class="{ active: edgeLabelsVisible }" @click="toggleEdgeLabels">Labels</button>
          <button :class="{ active: focusMode }" @click="toggleFocusMode">Focus</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.graph-view {
  height: 100%;
  display: flex;
  flex-direction: column;
}

/* Uses global .page-header, .header-actions from App.vue */
.page-header {
  margin-bottom: 16px;
}

.mode-select {
  padding: 8px 12px;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 6px;
  font-size: 14px;
}

.graph-stats {
  font-size: 12px;
  color: var(--muted-text);
}

.graph-stats strong {
  color: var(--accent-color, #6366f1);
  font-weight: 700;
}

/* Uses global .btn, .btn-sm, .btn-secondary, .btn-primary, .loading-state, .spinner from App.vue */

.graph-container {
  flex: 1;
  display: flex;
  gap: 16px;
  min-height: 0;
}

.graph-sidebar {
  width: 260px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
  overflow-y: auto;
}

.filter-section {
  background: var(--card-bg);
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 8px;
  padding: 12px;
}

.filter-section h4 {
  margin: 0 0 8px;
  font-size: 11px;
  font-weight: 600;
  color: var(--muted-text);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.filter-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.filter-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  transition: all 0.15s;
}

.filter-item:hover {
  background: var(--hover-bg);
}

.filter-item.inactive {
  opacity: 0.4;
}

.color-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}

.filter-label {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.filter-count {
  color: var(--muted-text);
  font-size: 11px;
  background: var(--hover-bg);
  padding: 1px 6px;
  border-radius: 10px;
}

.depth-control {
  display: flex;
  align-items: center;
  gap: 12px;
}

.depth-control input[type='range'] {
  flex: 1;
  accent-color: var(--accent-color, #6366f1);
}

.depth-value {
  background: var(--accent-color, #6366f1);
  color: white;
  font-weight: 700;
  width: 24px;
  height: 24px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
}

.detail-panel {
  background: var(--card-bg);
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 8px;
  padding: 16px;
}

.detail-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 8px;
}

.detail-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  color: white;
}

.close-btn {
  background: var(--hover-bg);
  border: none;
  font-size: 14px;
  cursor: pointer;
  color: var(--muted-text);
  width: 24px;
  height: 24px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.close-btn:hover {
  filter: brightness(0.9);
  color: var(--text-color);
}

.detail-panel h3 {
  margin: 0 0 4px;
  font-size: 15px;
  font-weight: 600;
}

.detail-id {
  font-size: 12px;
  color: var(--muted-text);
  font-family: monospace;
  margin: 0 0 12px;
}

.detail-properties {
  margin-bottom: 12px;
}

.detail-prop {
  display: flex;
  justify-content: space-between;
  padding: 4px 0;
  border-bottom: 1px solid var(--border-color);
  font-size: 12px;
}

.detail-prop:last-child {
  border: none;
}

.prop-key {
  color: var(--muted-text);
}

.prop-value {
  font-weight: 500;
  text-align: right;
  max-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.relations-section {
  margin-top: 12px;
}

.relations-section h4 {
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--muted-text);
  font-weight: 700;
  margin: 0 0 6px;
}

.rel-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-radius: 6px;
  margin-bottom: 2px;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s;
}

.rel-item:hover {
  background: var(--hover-bg);
  transform: translateX(3px);
}

.rel-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}

.rel-type {
  font-size: 10px;
  color: var(--muted-text);
}

.rel-name {
  font-weight: 500;
}

.graph-canvas {
  flex: 1;
  position: relative;
  background: var(--card-bg);
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 8px;
  overflow: hidden;
}

.cy-container {
  width: 100%;
  height: calc(100% - 60px);
}

.graph-toolbar {
  position: absolute;
  bottom: 20px;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  align-items: center;
  gap: 3px;
  background: var(--card-bg);
  backdrop-filter: blur(10px);
  padding: 5px;
  border-radius: 12px;
  box-shadow:
    0 4px 24px rgba(0, 0, 0, 0.15),
    0 0 0 1px var(--border-color);
}

.graph-toolbar button {
  padding: 6px 14px;
  border: none;
  background: none;
  border-radius: 8px;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  color: var(--muted-text);
  transition: all 0.15s;
  white-space: nowrap;
}

.graph-toolbar button:hover {
  background: var(--hover-bg);
  color: var(--text-color);
}

.graph-toolbar button.active {
  background: var(--accent-color, #6366f1);
  color: white;
  box-shadow: 0 2px 8px rgba(99, 102, 241, 0.25);
}

.graph-toolbar .sep {
  width: 1px;
  height: 18px;
  background: var(--border-color);
  margin: 0 2px;
}
</style>
