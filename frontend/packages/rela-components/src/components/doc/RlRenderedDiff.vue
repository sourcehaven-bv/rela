<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useMessages } from '../../composables/useMessages'
import { collapseUnchanged, diffBlocks, type DiffBlock } from './renderedDiff'

/**
 * A readable diff of two markdown documents: both sides are rendered, changed
 * blocks are tinted, and changed words inside a block are highlighted. Long
 * runs of unchanged blocks fold into an expander, and a stepper walks the
 * changes one at a time.
 */
const { before, after, context = 1 } = defineProps<{
  before: string
  after: string
  /** Unchanged blocks kept visible on each side of a change. */
  context?: number
}>()

const messages = useMessages()
const root = ref<HTMLElement | null>(null)

const blocks = computed(() => diffBlocks(before, after))
const segments = computed(() => collapseUnchanged(blocks.value, context))
const expanded = ref(new Set<number>())

/** Adjacent changed blocks (a removal next to its addition) are one change. */
const groupOf = computed(() => {
  const map = new Map<DiffBlock, number>()
  let group = -1
  let inRun = false
  for (const b of blocks.value) {
    if (b.op === 'equal') inRun = false
    else {
      if (!inRun) group++
      inRun = true
      map.set(b, group)
    }
  }
  return map
})
const total = computed(() => new Set(groupOf.value.values()).size)
const current = ref(0)
const stepped = ref(false)
watch([blocks, () => context], () => {
  current.value = 0
  stepped.value = false
  expanded.value = new Set()
})

function expand(i: number) {
  expanded.value = new Set(expanded.value).add(i)
}
function step(delta: number) {
  if (!total.value) return
  stepped.value = true
  current.value = (current.value + delta + total.value) % total.value
  root.value?.querySelector(`[data-group="${current.value}"]`)?.scrollIntoView({ block: 'center', behavior: 'smooth' })
}
</script>

<template>
  <div ref="root" class="rl-rd">
    <p v-if="!total" class="rl-rd__empty">{{ messages.diffNoChanges }}</p>
    <template v-else>
      <div class="rl-rd__bar" role="group" :aria-label="messages.diffChangeCount({ count: total })">
        <span class="rl-rd__count">{{ messages.diffChangeCount({ count: total }) }}</span>
        <span class="rl-rd__pos" aria-live="polite">{{ messages.diffPosition({ current: current + 1, total }) }}</span>
        <button type="button" class="rl-rd__step" :aria-label="messages.diffPrevious" @click="step(-1)">↑</button>
        <button type="button" class="rl-rd__step" :aria-label="messages.diffNext" @click="step(1)">↓</button>
      </div>
      <template v-for="(s, i) in segments" :key="i">
        <!-- eslint-disable vue/no-v-html -->
        <div
          v-if="s.kind === 'block'"
          class="rl-rd__block"
          :class="{ 'rl-rd__block--current': stepped && groupOf.get(s.block) === current }"
          :data-op="s.block.op"
          :data-group="groupOf.get(s.block)"
          v-html="s.block.html"
        />
        <template v-else-if="expanded.has(i)">
          <div v-for="(b, j) in s.blocks" :key="j" class="rl-rd__block" data-op="equal" v-html="b.html" />
        </template>
        <!-- eslint-enable vue/no-v-html -->
        <button v-else type="button" class="rl-rd__fold" @click="expand(i)">
          {{ messages.diffUnchangedBlocks({ count: s.blocks.length }) }}
        </button>
      </template>
    </template>
  </div>
</template>

<style scoped>
.rl-rd {
  color: var(--rl-color-text, inherit);
  line-height: 1.6;
}
.rl-rd__empty {
  color: var(--rl-color-text-muted, #6b7280);
  font-style: italic;
}
.rl-rd__block {
  position: relative;
  margin: 0 0 var(--rl-space-2, 8px);
  padding: 2px var(--rl-space-3, 12px);
  border-left: 3px solid transparent;
  border-radius: var(--rl-radius-sm, 4px);
}
.rl-rd__block :deep(> :first-child) {
  margin-top: 0;
}
.rl-rd__block :deep(> :last-child) {
  margin-bottom: 0;
}
.rl-rd__block[data-op='add'] {
  border-left-color: var(--rl-color-status-green, #28c772);
  background: var(--rl-color-success-bg, #e8f8ee);
}
.rl-rd__block[data-op='del'] {
  border-left-color: var(--rl-color-status-red, #f74f4d);
  background: var(--rl-color-danger-bg, #fdecec);
  text-decoration: line-through;
  opacity: 0.85;
}
.rl-rd__block[data-op='change'] {
  background: color-mix(in srgb, var(--rl-color-status-amber, #e0a100) 9%, transparent);
}
.rl-rd__block--current {
  outline: 2px solid var(--rl-color-focus-ring, #4f46e5);
  outline-offset: 1px;
}
.rl-rd__bar {
  position: sticky;
  top: 0;
  z-index: 1;
  display: flex;
  align-items: center;
  gap: var(--rl-space-2, 8px);
  margin-bottom: var(--rl-space-2, 8px);
  padding: var(--rl-space-1, 4px) var(--rl-space-2, 8px);
  border: 1px solid var(--rl-color-border, #d1d5db);
  border-radius: var(--rl-radius-sm, 4px);
  background: var(--rl-color-surface, #fff);
  font-size: var(--rl-font-size-sm, 13px);
}
.rl-rd__count {
  font-weight: var(--rl-font-weight-medium, 500);
}
.rl-rd__pos {
  margin-right: auto;
  color: var(--rl-color-text-muted, #6b7280);
}
.rl-rd__step {
  width: 28px;
  height: 28px;
  border: 1px solid var(--rl-color-border, #d1d5db);
  border-radius: var(--rl-radius-sm, 4px);
  background: transparent;
  color: inherit;
  font: inherit;
  cursor: pointer;
}
.rl-rd__step:hover {
  background: var(--rl-color-surface-hover, rgba(0, 0, 0, 0.04));
}
.rl-rd__block :deep(ins.rl-rd__ins) {
  text-decoration: none;
  background: var(--rl-color-success-bg, #e8f8ee);
  color: var(--rl-color-success-fg, #14803c);
  border-radius: 2px;
  padding: 0 2px;
}
.rl-rd__block :deep(del.rl-rd__del) {
  background: var(--rl-color-danger-bg, #fdecec);
  color: var(--rl-color-danger, #c62828);
  text-decoration: line-through;
  border-radius: 2px;
  padding: 0 2px;
}
.rl-rd__fold {
  display: block;
  width: 100%;
  margin: 0 0 var(--rl-space-2, 8px);
  padding: var(--rl-space-1, 4px);
  border: 1px dashed var(--rl-color-border, #d1d5db);
  border-radius: var(--rl-radius-sm, 4px);
  background: transparent;
  color: var(--rl-color-text-muted, #6b7280);
  font: inherit;
  font-size: var(--rl-font-size-sm, 13px);
  cursor: pointer;
}
.rl-rd__fold:hover {
  background: var(--rl-color-surface-hover, rgba(0, 0, 0, 0.04));
}
</style>
