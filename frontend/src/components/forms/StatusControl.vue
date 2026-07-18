<script setup lang="ts">
/**
 * StatusControl — a machine-aware status picker (TKT-3G93B8).
 *
 * Unlike a plain enum <select> (which lists every value), this control shows
 * ONLY the transitions the server resolved as performable for this principal on
 * this entity, sourced from the entity's `_transitions` wire map. Each option is
 * named by its MOVE (the action label, e.g. "Start progress"), falling back to
 * the target value's display label and then the raw value. Selecting a move
 * commits it as an atomic single-field change (the parent routes `update`
 * through the same field-save path any widget uses).
 *
 * No client-side ACL (dataentry/CLAUDE.md): the options are exactly what the
 * server said are allowed; this component never evaluates guards or predicates.
 * The write is re-enforced server-side (attempt-and-recover), so a stale verdict
 * simply surfaces the existing structured-error toast.
 */
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'
import type { TransitionOption } from '@/types'
import { useSchemaStore } from '@/stores/schema'
import Badge from '@/components/common/Badge.vue'

const props = defineProps<{
  // Current value of the state-machine field (the entity's present state).
  modelValue: string
  // The property's wire binding, used for label + badge-style lookup.
  property: string
  entityType?: string
  // The resolved outgoing transitions for this field (from entity._transitions).
  // The server sends every declared out-edge with an `allowed` flag; this
  // control renders only the allowed ones as selectable moves.
  transitions: TransitionOption[]
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const schemaStore = useSchemaStore()

// Show only performable moves — the "only allowed" contract. A self-loop (to ==
// current) is never offered; the server already excludes it, but guard here too.
const allowedMoves = computed(() =>
  props.transitions
    .filter((t) => t.allowed && t.to !== props.modelValue)
    .slice()
    .sort((a, b) => moveLabel(a).localeCompare(moveLabel(b)))
)

const hasMoves = computed(() => allowedMoves.value.length > 0)

// A move's display text: its action label, else the target value's state label,
// else the raw target value. Keeps transitions reading as verbs, not states.
function moveLabel(t: TransitionOption): string {
  if (t.label && t.label.trim() !== '') return t.label
  return schemaStore.getEnumLabel(t.to, props.property, props.entityType) ?? t.to
}

const open = ref(false)
const containerRef = ref<HTMLElement | null>(null)

function toggle() {
  if (props.disabled || !hasMoves.value) return
  open.value = !open.value
}

function selectMove(t: TransitionOption) {
  open.value = false
  if (t.to === props.modelValue) return
  emit('update:modelValue', t.to)
}

function handleClickOutside(e: MouseEvent) {
  if (!containerRef.value) return
  if (!containerRef.value.contains(e.target as Node)) open.value = false
}

onMounted(() => document.addEventListener('click', handleClickOutside))
onBeforeUnmount(() => document.removeEventListener('click', handleClickOutside))
</script>

<template>
  <div ref="containerRef" class="status-control">
    <button
      type="button"
      class="status-trigger"
      :class="{ 'is-static': disabled || !hasMoves }"
      :disabled="disabled || !hasMoves"
      :aria-haspopup="hasMoves ? 'menu' : undefined"
      :aria-expanded="open"
      @click.stop="toggle"
    >
      <Badge :value="modelValue" :property="property" />
      <span v-if="hasMoves" class="status-caret" aria-hidden="true">&#9662;</span>
    </button>

    <ul v-if="open && hasMoves" class="status-menu" role="menu">
      <li v-for="move in allowedMoves" :key="move.to" role="none">
        <button type="button" class="status-move" role="menuitem" @click.stop="selectMove(move)">
          <span class="status-move-arrow" aria-hidden="true">&rarr;</span>
          <span class="status-move-label">{{ moveLabel(move) }}</span>
        </button>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.status-control {
  position: relative;
  display: inline-block;
}

.status-trigger {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--input-bg);
  color: var(--text-color);
  cursor: pointer;
  font-size: 14px;
  transition: all 0.15s;
}

.status-trigger:hover:not(:disabled) {
  border-color: var(--accent-color, #6366f1);
}

.status-trigger.is-static {
  cursor: default;
  border-color: transparent;
  background: transparent;
  padding-left: 0;
}

.status-trigger:disabled {
  cursor: not-allowed;
}

.status-caret {
  font-size: 10px;
  color: var(--muted-text);
}

.status-menu {
  position: absolute;
  z-index: 20;
  margin: 4px 0 0;
  padding: 4px;
  list-style: none;
  min-width: 180px;
  background: var(--input-bg);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.12);
}

.status-move {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 8px 10px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--text-color);
  cursor: pointer;
  font-size: 14px;
  text-align: left;
}

.status-move:hover {
  background: var(--hover-bg);
}

.status-move-arrow {
  color: var(--muted-text);
}
</style>
