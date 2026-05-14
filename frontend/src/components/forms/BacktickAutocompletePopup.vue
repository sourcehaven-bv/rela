<script setup lang="ts">
/**
 * Anchored, non-focus-stealing popup used by the markdown editor's
 * inline backtick-triggered entity-reference autocomplete (TKT-2RCP).
 *
 * The popup is positioned at the trigger backtick's character coords
 * (provided by `useBacktickAutocomplete` via CodeMirror's `charCoords`)
 * and renders one of two lists depending on the session phase:
 *
 *   - phase = 'prefix'  → list of project ID prefixes (TKT-, FEAT-, …)
 *   - phase = 'id'      → list of entities of the resolved type
 *
 * It is purely presentational: clicks emit `pick(idx)` and hover-over
 * emits `hover(idx)`; the actual state machine + insertion lives in
 * the composable.
 *
 * Focus stays in the editor — the wrapper element's mousedown handler
 * preventDefaults so a click on a result row doesn't take focus from
 * CodeMirror's textarea.
 *
 * z-index sits at 10000, the same level as `EntityPickerModal`'s
 * overlay, so the popup stays above EasyMDE's fullscreen layer (9999).
 */
import { computed } from 'vue'
import type {
  Phase,
  PrefixItem,
  ReadonlySessionState,
} from '@/composables/useBacktickAutocomplete'

const props = defineProps<{
  state: ReadonlySessionState
  /** Bounding rect of the editor container so we can convert window-
   *  relative anchor coords into element-relative ones. */
  editorRect: DOMRect | null
}>()

const emit = defineEmits<{
  pick: [index: number]
  hover: [index: number]
}>()

const visible = computed(
  () => props.state.phase === 'prefix' || props.state.phase === 'id',
)

const positionStyle = computed(() => {
  const a = props.state.anchor
  if (!a || !props.editorRect) return { display: 'none' }
  return {
    left: `${a.left - props.editorRect.left}px`,
    top: `${a.bottom - props.editorRect.top + 2}px`,
  }
})

// The popup's state is exposed as DeepReadonly which doesn't compose
// cleanly with `Entity`'s mutable nested types (e.g. `relations: Record<
// string, string[]>`). The popup doesn't mutate either shape — it just
// reads through them — so we use a small structural type alias instead
// of a full `Entity | PrefixItem` union to keep the type-check happy.
interface DisplayPrefix {
  prefix: string
  type: string
  label: string
  isManual: boolean
}
interface DisplayEntity {
  id: string
  type: string
  _title?: string
  properties?: Record<string, unknown>
}
type RowItem = DisplayPrefix | DisplayEntity

const items = computed<readonly RowItem[]>(() => {
  if (props.state.phase === 'prefix') return props.state.prefixItems as readonly DisplayPrefix[]
  if (props.state.phase === 'id') return props.state.entityItems as readonly DisplayEntity[]
  return []
})

function isPrefixItem(item: RowItem): item is DisplayPrefix {
  return typeof (item as DisplayPrefix).prefix === 'string'
}

function entityLabel(e: DisplayEntity): string {
  if (typeof e._title === 'string' && e._title !== '') return e._title
  const t = e.properties?.title
  if (typeof t === 'string' && t !== '') return t
  return e.id
}

function hintText(phase: Phase, resolved: PrefixItem | null): string {
  if (phase === 'prefix') {
    return 'Select an entity type — keep typing or use ↑↓ + Enter'
  }
  if (phase === 'id' && resolved) {
    return resolved.isManual
      ? `${resolved.label} entities`
      : `Entities matching ${resolved.prefix}`
  }
  return ''
}

function onWrapperMousedown(e: MouseEvent): void {
  // Keep CodeMirror's textarea focused — clicks on the popup must not
  // shift document focus. Mousedown fires before focus moves.
  e.preventDefault()
}
</script>

<template>
  <div
    v-if="visible"
    class="backtick-popup"
    :style="positionStyle"
    role="listbox"
    aria-label="Entity reference suggestions"
    @mousedown="onWrapperMousedown"
  >
    <div class="backtick-popup-hint">
      {{ hintText(state.phase, state.resolvedPrefix) }}
    </div>
    <div v-if="state.errorMsg" class="backtick-popup-error">{{ state.errorMsg }}</div>
    <ul v-if="items.length > 0" class="backtick-popup-list">
      <li
        v-for="(item, idx) in items"
        :key="isPrefixItem(item) ? `p-${item.type}-${item.prefix}` : item.id"
        class="backtick-popup-option"
        :class="{ active: idx === state.highlightedIndex }"
        role="option"
        :aria-selected="idx === state.highlightedIndex"
        @mouseenter="emit('hover', idx)"
        @click="emit('pick', idx)"
      >
        <template v-if="isPrefixItem(item)">
          <span class="backtick-popup-badge">{{ item.label }}</span>
          <span class="backtick-popup-title">
            {{ item.isManual ? '(manual)' : `${item.prefix}*` }}
          </span>
        </template>
        <template v-else>
          <span class="backtick-popup-badge">{{ item.type }}</span>
          <span class="backtick-popup-title">{{ entityLabel(item) }}</span>
          <span class="backtick-popup-id">{{ item.id }}</span>
        </template>
      </li>
    </ul>
    <div
      v-else-if="state.phase === 'id' && !state.loading && !state.errorMsg"
      class="backtick-popup-hint"
    >
      No matches
    </div>
  </div>
</template>

<style scoped>
/* z-index 10000 mirrors EntityPickerModal so the popup sits above
   EasyMDE's fullscreen layer (9999). See TKT-I5NO's RR-WMG2. */
.backtick-popup {
  position: absolute;
  z-index: 10000;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
  min-width: 320px;
  max-width: 420px;
  max-height: 260px;
  overflow-y: auto;
  font-size: 13px;
  animation: backtick-popup-in 100ms ease-out;
}

@keyframes backtick-popup-in {
  from {
    opacity: 0;
    transform: translateY(-2px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.backtick-popup-hint {
  padding: 8px 12px;
  font-size: 12px;
  color: var(--muted-text);
  border-bottom: 1px solid var(--border-color);
}

.backtick-popup-error {
  padding: 8px 12px;
  font-size: 12px;
  color: var(--error-color);
}

.backtick-popup-list {
  list-style: none;
  margin: 0;
  padding: 4px 0;
}

.backtick-popup-option {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 12px;
  cursor: pointer;
}

.backtick-popup-option.active {
  background: var(--hover-bg);
}

.backtick-popup-badge {
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  padding: 2px 6px;
  border-radius: 4px;
  background: var(--hover-bg);
  color: var(--muted-text);
  flex-shrink: 0;
  min-width: 50px;
  text-align: center;
  font-weight: 500;
}

.backtick-popup-option.active .backtick-popup-badge {
  background: var(--card-bg);
}

.backtick-popup-title {
  flex: 1;
  color: var(--text-color);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.backtick-popup-id {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  color: var(--muted-text);
  flex-shrink: 0;
}
</style>
