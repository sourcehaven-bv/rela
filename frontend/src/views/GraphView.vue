<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRouter } from 'vue-router'
import { getGraphData } from '@/api'
import type { GraphData, GraphNode, GraphEdge } from '@/api/graph'

const router = useRouter()

// State
const loading = ref(true)
const graphData = ref<GraphData | null>(null)
const mode = ref<'content' | 'metamodel'>('content')
const selectedEntityTypes = ref<Set<string>>(new Set())
const selectedRelationTypes = ref<Set<string>>(new Set())
const selectedNode = ref<GraphNode | null>(null)
const svgRef = ref<SVGSVGElement | null>(null)

// Force simulation state
interface NodePosition {
  x: number
  y: number
  vx: number
  vy: number
}
const nodePositions = ref<Map<string, NodePosition>>(new Map())
let animationFrame: number | null = null

// Computed
const entityTypes = computed(() => graphData.value?.entityTypes || [])
const relationTypes = computed(() => graphData.value?.relationTypes || [])

const filteredNodes = computed(() => {
  if (!graphData.value) return []
  if (selectedEntityTypes.value.size === 0) return graphData.value.nodes
  return graphData.value.nodes.filter((n) => selectedEntityTypes.value.has(n.type))
})

const filteredEdges = computed(() => {
  if (!graphData.value) return []
  const nodeIds = new Set(filteredNodes.value.map((n) => n.id))

  return graphData.value.edges.filter((e) => {
    const typeMatch = selectedRelationTypes.value.size === 0 || selectedRelationTypes.value.has(e.type)
    const nodesMatch = nodeIds.has(e.source) && nodeIds.has(e.target)
    return typeMatch && nodesMatch
  })
})

const nodeColorMap = computed(() => {
  const map = new Map<string, string>()
  for (const et of entityTypes.value) {
    map.set(et.type, et.color)
  }
  return map
})

// Methods
async function loadGraphData() {
  loading.value = true
  try {
    graphData.value = await getGraphData(mode.value)
    initializePositions()
    startSimulation()
  } catch (err) {
    console.error('Graph load error:', err)
  } finally {
    loading.value = false
  }
}

function initializePositions() {
  if (!graphData.value) return

  const width = 800
  const height = 600
  const nodes = graphData.value.nodes

  nodePositions.value = new Map()

  // Initialize in a larger circle with random jitter
  const centerX = width / 2
  const centerY = height / 2
  const radius = Math.min(width, height) * 0.35

  nodes.forEach((node, i) => {
    const angle = (2 * Math.PI * i) / nodes.length
    nodePositions.value.set(node.id, {
      x: centerX + radius * Math.cos(angle) + (Math.random() - 0.5) * 80,
      y: centerY + radius * Math.sin(angle) + (Math.random() - 0.5) * 80,
      vx: 0,
      vy: 0,
    })
  })
}

function startSimulation() {
  if (animationFrame) cancelAnimationFrame(animationFrame)

  let iterations = 0
  const maxIterations = 300

  function tick() {
    if (iterations >= maxIterations || !graphData.value) return

    const nodes = filteredNodes.value
    const edges = filteredEdges.value
    const alpha = 1 - iterations / maxIterations

    // Apply forces
    applyRepulsion(nodes, alpha)
    applyAttraction(edges, alpha)
    applyCenter(nodes, alpha)

    // Update positions
    for (const node of nodes) {
      const pos = nodePositions.value.get(node.id)
      if (pos) {
        pos.vx *= 0.85
        pos.vy *= 0.85
        pos.x += pos.vx
        pos.y += pos.vy

        // Keep in bounds (accounting for node size)
        pos.x = Math.max(60, Math.min(740, pos.x))
        pos.y = Math.max(40, Math.min(560, pos.y))
      }
    }

    iterations++
    animationFrame = requestAnimationFrame(tick)
  }

  tick()
}

function applyRepulsion(nodes: GraphNode[], alpha: number) {
  const strength = 2000 * alpha

  for (let i = 0; i < nodes.length; i++) {
    for (let j = i + 1; j < nodes.length; j++) {
      const posA = nodePositions.value.get(nodes[i].id)
      const posB = nodePositions.value.get(nodes[j].id)
      if (!posA || !posB) continue

      const dx = posB.x - posA.x
      const dy = posB.y - posA.y
      const dist = Math.sqrt(dx * dx + dy * dy) || 1

      // Apply repulsion to all nodes
      const force = strength / (dist * dist + 100)
      const fx = (dx / dist) * force
      const fy = (dy / dist) * force

      posA.vx -= fx
      posA.vy -= fy
      posB.vx += fx
      posB.vy += fy
    }
  }
}

function applyAttraction(edges: GraphEdge[], alpha: number) {
  const strength = 0.15 * alpha
  const targetDistance = 120

  for (const edge of edges) {
    const posA = nodePositions.value.get(edge.source)
    const posB = nodePositions.value.get(edge.target)
    if (!posA || !posB) continue

    const dx = posB.x - posA.x
    const dy = posB.y - posA.y
    const dist = Math.sqrt(dx * dx + dy * dy) || 1

    // Pull together when too far, push apart when too close
    const force = (dist - targetDistance) * strength
    const fx = (dx / dist) * force
    const fy = (dy / dist) * force

    posA.vx += fx
    posA.vy += fy
    posB.vx -= fx
    posB.vy -= fy
  }
}

function applyCenter(nodes: GraphNode[], alpha: number) {
  const strength = 0.02 * alpha
  const centerX = 400
  const centerY = 300

  for (const node of nodes) {
    const pos = nodePositions.value.get(node.id)
    if (!pos) continue

    pos.vx += (centerX - pos.x) * strength
    pos.vy += (centerY - pos.y) * strength
  }
}

function getNodePosition(id: string) {
  const pos = nodePositions.value.get(id)
  return pos ? { x: pos.x, y: pos.y } : { x: 400, y: 300 }
}

function toggleEntityType(type: string) {
  if (selectedEntityTypes.value.has(type)) {
    selectedEntityTypes.value.delete(type)
  } else {
    selectedEntityTypes.value.add(type)
  }
  selectedEntityTypes.value = new Set(selectedEntityTypes.value)
  startSimulation()
}

function toggleRelationType(type: string) {
  if (selectedRelationTypes.value.has(type)) {
    selectedRelationTypes.value.delete(type)
  } else {
    selectedRelationTypes.value.add(type)
  }
  selectedRelationTypes.value = new Set(selectedRelationTypes.value)
}

function selectNode(node: GraphNode) {
  selectedNode.value = selectedNode.value?.id === node.id ? null : node
}

function openNode(node: GraphNode) {
  if (mode.value === 'content') {
    router.push(`/entity/${node.type}/${node.id}`)
  }
}

function clearFilters() {
  selectedEntityTypes.value = new Set()
  selectedRelationTypes.value = new Set()
  startSimulation()
}

// Lifecycle
onMounted(() => {
  loadGraphData()
})

onBeforeUnmount(() => {
  if (animationFrame) cancelAnimationFrame(animationFrame)
})

watch(mode, () => {
  loadGraphData()
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
        <button class="btn btn-secondary" @click="clearFilters" v-if="selectedEntityTypes.size > 0 || selectedRelationTypes.size > 0">
          Clear Filters
        </button>
        <button class="btn btn-secondary" @click="loadGraphData" :disabled="loading">
          {{ loading ? 'Loading...' : 'Refresh' }}
        </button>
      </div>
    </header>

    <div v-if="loading" class="loading-state">
      <div class="spinner"></div>
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
              :class="{ active: selectedEntityTypes.size === 0 || selectedEntityTypes.has(et.type) }"
            >
              <span class="color-dot" :style="{ background: et.color }"></span>
              <span class="filter-label">{{ et.label }}</span>
              <span class="filter-count">{{ et.count }}</span>
              <input
                type="checkbox"
                :checked="selectedEntityTypes.has(et.type)"
                @change="toggleEntityType(et.type)"
              />
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
              :class="{ active: selectedRelationTypes.size === 0 || selectedRelationTypes.has(rt.type) }"
            >
              <span class="filter-label">{{ rt.label }}</span>
              <span class="filter-count">{{ rt.count }}</span>
              <input
                type="checkbox"
                :checked="selectedRelationTypes.has(rt.type)"
                @change="toggleRelationType(rt.type)"
              />
            </label>
          </div>
        </div>

        <!-- Node detail panel -->
        <div v-if="selectedNode" class="detail-panel">
          <h4>{{ selectedNode.title }}</h4>
          <p class="detail-meta">
            <span class="detail-type">{{ selectedNode.type }}</span>
            <span class="detail-id">{{ selectedNode.id }}</span>
          </p>
          <div class="detail-properties" v-if="Object.keys(selectedNode.properties).length > 0">
            <div v-for="(value, key) in selectedNode.properties" :key="key" class="detail-prop">
              <span class="prop-key">{{ key }}:</span>
              <span class="prop-value">{{ value }}</span>
            </div>
          </div>
          <button class="btn btn-primary btn-sm" @click="openNode(selectedNode)" v-if="mode === 'content'">
            Open Details
          </button>
        </div>
      </aside>

      <!-- SVG Canvas -->
      <div class="graph-canvas">
        <svg ref="svgRef" viewBox="0 0 800 600" preserveAspectRatio="xMidYMid meet">
          <defs>
            <marker
              id="arrowhead"
              markerWidth="8"
              markerHeight="6"
              refX="50"
              refY="3"
              orient="auto"
            >
              <polygon points="0 0, 8 3, 0 6" fill="#64748b" />
            </marker>
          </defs>
          <!-- Edges -->
          <g class="edges">
            <line
              v-for="(edge, i) in filteredEdges"
              :key="`edge-${i}`"
              :x1="getNodePosition(edge.source).x"
              :y1="getNodePosition(edge.source).y"
              :x2="getNodePosition(edge.target).x"
              :y2="getNodePosition(edge.target).y"
              class="edge"
              :class="{ dimmed: selectedRelationTypes.size > 0 && !selectedRelationTypes.has(edge.type) }"
              marker-end="url(#arrowhead)"
            />
          </g>

          <!-- Nodes -->
          <g class="nodes">
            <g
              v-for="node in filteredNodes"
              :key="node.id"
              class="node"
              :class="{ selected: selectedNode?.id === node.id }"
              :transform="`translate(${getNodePosition(node.id).x}, ${getNodePosition(node.id).y})`"
              @click="selectNode(node)"
              @dblclick="openNode(node)"
            >
              <rect
                x="-45"
                y="-20"
                width="90"
                height="40"
                rx="6"
                ry="6"
                :fill="nodeColorMap.get(node.type) || '#6366f1'"
              />
              <text dy="-4" text-anchor="middle" class="node-id">
                {{ node.id }}
              </text>
              <text dy="10" text-anchor="middle" class="node-title">
                {{ node.title.slice(0, 12) }}{{ node.title.length > 12 ? '...' : '' }}
              </text>
            </g>
          </g>
        </svg>
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

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.page-header h1 {
  margin: 0;
}

.header-actions {
  display: flex;
  gap: 12px;
  align-items: center;
}

.mode-select {
  padding: 8px 12px;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 6px;
  font-size: 14px;
}

.btn {
  padding: 8px 16px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  border: none;
  transition: all 0.15s;
}

.btn-sm {
  padding: 6px 12px;
  font-size: 13px;
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-secondary {
  background: #f1f5f9;
  color: #475569;
}

.btn-secondary:hover:not(:disabled) {
  background: #e2e8f0;
}

.btn-primary {
  background: var(--accent-color, #6366f1);
  color: white;
}

.btn-primary:hover:not(:disabled) {
  background: #4f46e5;
}

.loading-state {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 48px;
  color: #64748b;
}

.spinner {
  width: 24px;
  height: 24px;
  border: 3px solid var(--border-color);
  border-top-color: var(--accent-color);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.graph-container {
  flex: 1;
  display: flex;
  gap: 16px;
  min-height: 0;
}

.graph-sidebar {
  width: 240px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
  overflow-y: auto;
}

.filter-section {
  background: white;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 8px;
  padding: 12px;
}

.filter-section h4 {
  margin: 0 0 8px;
  font-size: 12px;
  font-weight: 600;
  color: #64748b;
  text-transform: uppercase;
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
  border-radius: 4px;
  cursor: pointer;
  font-size: 13px;
  transition: background 0.15s;
}

.filter-item:hover {
  background: #f8fafc;
}

.filter-item.active {
  background: #f1f5f9;
}

.filter-item input {
  margin-left: auto;
}

.color-dot {
  width: 12px;
  height: 12px;
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
  color: #94a3b8;
  font-size: 12px;
}

.detail-panel {
  background: white;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 8px;
  padding: 12px;
}

.detail-panel h4 {
  margin: 0 0 8px;
  font-size: 14px;
  font-weight: 600;
}

.detail-meta {
  display: flex;
  gap: 8px;
  margin: 0 0 12px;
  font-size: 12px;
}

.detail-type {
  background: #f1f5f9;
  padding: 2px 6px;
  border-radius: 4px;
  color: #64748b;
  text-transform: uppercase;
}

.detail-id {
  font-family: monospace;
  color: #64748b;
}

.detail-properties {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 12px;
}

.detail-prop {
  font-size: 12px;
  display: flex;
  gap: 4px;
}

.prop-key {
  color: #64748b;
}

.prop-value {
  color: #1e293b;
}

.graph-canvas {
  flex: 1;
  background: white;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 8px;
  overflow: hidden;
}

.graph-canvas svg {
  width: 100%;
  height: 100%;
}

.edge {
  stroke: #94a3b8;
  stroke-width: 2;
  opacity: 0.8;
}

.edge.dimmed {
  opacity: 0.3;
  stroke: #e2e8f0;
}

.node {
  cursor: pointer;
}

.node rect {
  stroke: rgba(0, 0, 0, 0.2);
  stroke-width: 1;
  transition: all 0.15s;
}

.node:hover rect {
  stroke-width: 2;
  filter: brightness(1.1);
}

.node.selected rect {
  stroke: #1e293b;
  stroke-width: 3;
}

.node-id {
  fill: rgba(255, 255, 255, 0.8);
  font-size: 8px;
  font-weight: 500;
  pointer-events: none;
}

.node-title {
  fill: white;
  font-size: 10px;
  font-weight: 600;
  pointer-events: none;
}
</style>
