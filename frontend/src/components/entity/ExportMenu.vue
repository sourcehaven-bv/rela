<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { getTransforms, type TransformInfo } from '@/api/transforms'

const props = defineProps<{
  /**
   * Given a transform name, return the export URL to navigate to. The parent
   * builds it (entity vs. list, plus any current list filter/sort params) so
   * the menu stays agnostic about the export target.
   */
  urlFor: (transform: string) => string
}>()

const transforms = ref<TransformInfo[]>([])
const open = ref(false)
const rootRef = ref<HTMLElement | null>(null)
let abort: AbortController | null = null

onMounted(async () => {
  abort = new AbortController()
  try {
    transforms.value = await getTransforms(abort.signal)
  } catch (err) {
    // A missing/empty registry simply hides the menu; don't disrupt the page.
    console.error('Failed to load export transforms:', err)
    transforms.value = []
  }
  document.addEventListener('click', onDocClick)
})

onBeforeUnmount(() => {
  abort?.abort()
  document.removeEventListener('click', onDocClick)
})

function onDocClick(e: MouseEvent) {
  if (rootRef.value && !rootRef.value.contains(e.target as Node)) {
    open.value = false
  }
}

function toggle() {
  open.value = !open.value
}

function exportAs(t: TransformInfo) {
  open.value = false
  // Navigate to the hardened forced-download endpoint. Using a real nav (not
  // fetch) lets the browser's download machinery handle Content-Disposition.
  window.location.href = props.urlFor(t.name)
}
</script>

<template>
  <div v-if="transforms.length" ref="rootRef" class="export-menu">
    <button
      class="btn btn-secondary"
      :aria-expanded="open"
      aria-haspopup="menu"
      @click="toggle"
    >
      Export ▾
    </button>
    <ul v-if="open" class="export-menu-list" role="menu">
      <li v-for="t in transforms" :key="t.name" role="none">
        <button role="menuitem" class="export-menu-item" @click="exportAs(t)">
          {{ t.name }}
        </button>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.export-menu {
  position: relative;
  display: inline-block;
}

.export-menu-list {
  position: absolute;
  right: 0;
  top: calc(100% + 4px);
  z-index: 20;
  min-width: 8rem;
  margin: 0;
  padding: 0.25rem;
  list-style: none;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.12);
}

.export-menu-item {
  display: block;
  width: 100%;
  padding: 0.4rem 0.6rem;
  text-align: left;
  background: none;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  color: var(--text-color);
  font-size: 0.9rem;
  text-transform: uppercase;
}

.export-menu-item:hover,
.export-menu-item:focus {
  background: var(--hover-bg);
}
</style>
