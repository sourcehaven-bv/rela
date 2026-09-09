<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { apiUrl } from '@/api/base'

/**
 * Switches between projects open in the desktop shell.
 *
 * The list comes from the shell rather than from the data-entry API: which
 * projects are mounted is a property of the host, not of any one project. On
 * rela-server the endpoint is absent, so the fetch fails and the switcher
 * simply does not render — which is also what happens with a single project
 * open, since there is nothing to switch to.
 */
interface ProjectSummary {
  id: string
  name: string
  root: string
  href: string
  active: boolean
}

const projects = ref<ProjectSummary[]>([])
const open = ref(false)

const current = computed(() => projects.value.find((p) => p.active) ?? null)
// One project is not a choice, and no projects means we are not in the shell.
const show = computed(() => projects.value.length > 1)

onMounted(async () => {
  // Only the desktop shell mounts several projects, and only it injects the
  // Wails runtime. Checking first keeps rela-server from issuing a request
  // per page load for an endpoint it does not serve — which also costs a
  // round trip that `networkidle` waits on.
  if (!('wails' in window)) return

  try {
    const res = await fetch(apiUrl('/api/v1/_projects'))
    if (!res.ok) return // shell without the endpoint yet
    projects.value = await res.json()
  } catch {
    // Stay hidden rather than surface a failure the user cannot act on.
  }
})

// A full navigation, not router.push: each project is a different history
// base, so the router of this page cannot address another project's routes.
function switchTo(p: ProjectSummary) {
  open.value = false
  if (!p.active) window.location.href = p.href
}
</script>

<template>
  <div v-if="show" class="project-switcher">
    <button
      class="switcher-btn"
      :aria-expanded="open"
      aria-haspopup="listbox"
      @click="open = !open"
    >
      <span class="switcher-name">{{ current?.name ?? 'Projects' }}</span>
      <span class="switcher-caret" aria-hidden="true">▾</span>
    </button>

    <ul v-if="open" class="switcher-menu" role="listbox">
      <li v-for="p in projects" :key="p.id" role="option" :aria-selected="p.active">
        <button class="switcher-item" :class="{ active: p.active }" @click="switchTo(p)">
          <span class="item-name">{{ p.name }}</span>
          <span class="item-root">{{ p.root }}</span>
        </button>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.project-switcher {
  position: relative;
}

.switcher-btn {
  display: flex;
  align-items: center;
  gap: 4px;
  width: 100%;
  padding: 4px 8px;
  border: 1px solid var(--border);
  border-radius: var(--radius, 6px);
  background: var(--bg-card);
  color: var(--text);
  font-size: 12px;
  cursor: pointer;
}

.switcher-btn:hover {
  border-color: var(--primary);
}

.switcher-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.switcher-caret {
  margin-left: auto;
  opacity: 0.6;
}

.switcher-menu {
  position: absolute;
  z-index: 20;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  margin: 0;
  padding: 4px;
  list-style: none;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius, 6px);
  box-shadow: var(--shadow, 0 2px 8px rgba(0, 0, 0, 0.15));
}

.switcher-item {
  display: flex;
  flex-direction: column;
  width: 100%;
  padding: 6px 8px;
  border: none;
  border-radius: 4px;
  background: none;
  color: var(--text);
  text-align: left;
  cursor: pointer;
}

.switcher-item:hover {
  background: var(--primary-light);
}

.switcher-item.active {
  font-weight: 600;
}

.item-name {
  font-size: 13px;
}

.item-root {
  font-size: 11px;
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
