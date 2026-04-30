<script setup lang="ts">
/**
 * Property/value picker dropdown shared by SearchView and EntityList.
 *
 * Two-step flow: pick a property (or "type"), then pick a value (enum dropdown
 * for properties that declare `values`, free-text input otherwise). Emits
 * `apply(property, value)` once the user commits.
 *
 * Modes:
 * - `entityType` provided → list mode: only properties of that one type, no
 *   `type:` filter option.
 * - `entityType` omitted → search mode: properties unioned across all entity
 *   types (deduplicated by name) plus a synthetic `type` option.
 *
 * `lockedProperties` hides options the caller already covers (active ad-hoc
 * filters, statically-pinned config filters) so the menu can't produce a
 * duplicate.
 */
import { ref, computed, nextTick, onMounted, onBeforeUnmount, watch } from 'vue'
import { useSchemaStore } from '@/stores'
import type { EntityType, PropertyDef } from '@/types'

interface FilterOption {
  category: 'type' | 'property'
  property: string
  label: string
  propertyDef?: PropertyDef
}

const props = withDefaults(
  defineProps<{
    /**
     * `list`: only the properties of the bound `entityType` are offered.
     *   Required when this prop is set.
     * `search`: properties unioned across all entity types in the schema,
     *   plus a synthetic `type` option.
     *
     * Made explicit (rather than implicit-on-`entityType`) because the list
     * mode's `entityType` ref can transiently be undefined while the schema
     * store loads — falling through to the all-types union would offer
     * properties that don't exist on the actual list type, producing
     * silent zero-result pages once the user applies them.
     */
    mode: 'list' | 'search'
    entityType?: EntityType
    lockedProperties?: Set<string>
    buttonLabel?: string
    buttonHotkey?: string
  }>(),
  {
    lockedProperties: () => new Set<string>(),
    buttonLabel: '+ Filter',
    buttonHotkey: 'F',
  },
)

const emit = defineEmits<{
  apply: [property: string, value: string]
}>()

const schemaStore = useSchemaStore()

const open = ref(false)
const search = ref('')
const menuIndex = ref(0)
const selected = ref<FilterOption | null>(null)
const valueInput = ref('')

const containerRef = ref<HTMLDivElement | null>(null)
const searchInputRef = ref<HTMLInputElement | null>(null)

const allOptions = computed((): FilterOption[] => {
  const out: FilterOption[] = []
  const seen = new Set<string>()

  if (props.mode === 'search') {
    out.push({ category: 'type', property: 'type', label: 'Entity Type' })
    for (const [, typeDef] of schemaStore.entityTypes) {
      for (const [name, def] of Object.entries(typeDef.properties)) {
        if (seen.has(name)) continue
        seen.add(name)
        out.push({
          category: 'property',
          property: name,
          label: titleCase(name),
          propertyDef: def,
        })
      }
    }
  } else {
    // List mode. Without entityType (schema still loading, or list config
    // points at an unknown type) we render an empty list rather than fall
    // back to the all-types union — see the `mode` prop docstring above.
    if (!props.entityType) return []
    for (const [name, def] of Object.entries(props.entityType.properties)) {
      out.push({
        category: 'property',
        property: name,
        label: titleCase(name),
        propertyDef: def,
      })
    }
  }

  // Hide options already covered by active or static filters.
  return out.filter((o) => !props.lockedProperties.has(o.property))
})

const filteredOptions = computed(() => {
  if (!search.value) return allOptions.value
  const q = search.value.toLowerCase()
  return allOptions.value.filter(
    (o) => o.label.toLowerCase().includes(q) || o.property.toLowerCase().includes(q),
  )
})

const valueOptions = computed((): string[] => {
  if (!selected.value) return []

  if (selected.value.category === 'type') {
    const types: string[] = []
    for (const [name] of schemaStore.entityTypes) types.push(name)
    return types
  }

  // Property mode: prefer the propertyDef's own enum values, regardless of
  // mode (list mode's selected.value already comes from props.entityType,
  // search mode's from the cross-type seen-set).
  const propDef = selected.value.propertyDef
  if (propDef?.values?.length) return propDef.values

  // Search mode: a property may have enum values on OTHER entity types.
  // Union across all types so the picker offers them. Note that this
  // silently hides the fact that some types accept arbitrary strings for
  // the same property name — acceptable for v1, deferred to a follow-up
  // ticket if it surfaces as a UX problem.
  if (props.mode === 'search') {
    const all = new Set<string>()
    for (const [, typeDef] of schemaStore.entityTypes) {
      const p = typeDef.properties[selected.value.property]
      if (p?.values) p.values.forEach((v) => all.add(v))
    }
    return Array.from(all)
  }

  return []
})

function titleCase(str: string): string {
  return str.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
}

function toggle() {
  if (open.value) {
    close()
  } else {
    openMenu()
  }
}

function openMenu() {
  open.value = true
  search.value = ''
  selected.value = null
  valueInput.value = ''
  menuIndex.value = 0
  nextTick(() => searchInputRef.value?.focus())
}

function close() {
  open.value = false
  selected.value = null
  search.value = ''
  valueInput.value = ''
}

function pickProperty(option: FilterOption) {
  selected.value = option
  valueInput.value = ''
  menuIndex.value = 0
}

function commit(value: string) {
  if (!selected.value || !value) return
  emit('apply', selected.value.property, value)
  close()
}

function handleKeydown(e: KeyboardEvent) {
  if (!open.value) return
  const options = selected.value ? valueOptions.value : filteredOptions.value

  switch (e.key) {
    case 'ArrowDown':
      e.preventDefault()
      menuIndex.value = Math.min(options.length - 1, menuIndex.value + 1)
      break
    case 'ArrowUp':
      e.preventDefault()
      menuIndex.value = Math.max(0, menuIndex.value - 1)
      break
    case 'Enter':
      e.preventDefault()
      if (selected.value) {
        const v = valueOptions.value.length > 0
          ? valueOptions.value[menuIndex.value]
          : valueInput.value
        if (v) commit(v)
      } else {
        const o = filteredOptions.value[menuIndex.value]
        if (o) pickProperty(o)
      }
      break
    case 'Escape':
      e.preventDefault()
      if (selected.value) {
        selected.value = null
        search.value = ''
        menuIndex.value = 0
      } else {
        close()
      }
      break
    case 'Backspace':
      if (!search.value && selected.value) {
        selected.value = null
        menuIndex.value = 0
      }
      break
  }
}

function handleClickOutside(e: MouseEvent) {
  if (!containerRef.value) return
  if (!containerRef.value.contains(e.target as Node)) close()
}

// Reset highlight when option list changes (e.g., user typed in search box).
watch(filteredOptions, () => {
  menuIndex.value = 0
})

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})
onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
})

defineExpose({ open: openMenu, close })
</script>

<template>
  <div ref="containerRef" class="adhoc-filter-menu">
    <button class="btn btn-secondary filter-btn" type="button" @click.stop="toggle">
      {{ buttonLabel }}
      <kbd v-if="buttonHotkey">{{ buttonHotkey }}</kbd>
    </button>

    <div v-if="open" class="filter-menu" @click.stop @keydown="handleKeydown">
      <template v-if="!selected">
        <input
          ref="searchInputRef"
          v-model="search"
          type="text"
          placeholder="Search properties..."
          class="filter-search"
          @keydown="handleKeydown"
        />
        <div class="filter-options">
          <div
            v-for="(option, index) in filteredOptions"
            :key="option.property"
            class="filter-option"
            :class="{ highlighted: index === menuIndex }"
            @click="pickProperty(option)"
            @mouseenter="menuIndex = index"
          >
            <span class="option-category">{{ option.category }}</span>
            <span class="option-label">{{ option.label }}</span>
          </div>
          <div v-if="filteredOptions.length === 0" class="filter-empty">
            No matching properties
          </div>
        </div>
      </template>

      <template v-else>
        <div class="filter-header">
          <button class="back-btn" type="button" @click="selected = null">&larr;</button>
          <span>{{ selected.label }}</span>
        </div>

        <div v-if="valueOptions.length > 0" class="filter-options">
          <div
            v-for="(value, index) in valueOptions"
            :key="value"
            class="filter-option"
            :class="{ highlighted: index === menuIndex }"
            @click="commit(value)"
            @mouseenter="menuIndex = index"
          >
            {{ value }}
          </div>
        </div>

        <div v-else class="filter-text-input">
          <input
            v-model="valueInput"
            type="text"
            placeholder="Enter value..."
            class="filter-search"
            @keydown.enter="commit(valueInput)"
          />
          <button
            class="btn btn-primary btn-sm"
            type="button"
            :disabled="!valueInput"
            @click="commit(valueInput)"
          >
            Apply
          </button>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.adhoc-filter-menu {
  position: relative;
}

.filter-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.filter-btn kbd {
  display: inline-block;
  padding: 0.05rem 0.3rem;
  border: 1px solid var(--border-color);
  border-radius: 3px;
  background: var(--hover-bg);
  font-family: monospace;
  font-size: 11px;
  line-height: 1;
}

.filter-menu {
  position: absolute;
  top: 100%;
  /* The button typically sits at the right edge of the toolbar (after a
     wide search box), so anchoring the menu's right edge to the button
     keeps a 280px-wide dropdown inside the viewport. Anchoring left would
     push the menu past the right edge and force horizontal scroll. */
  right: 0;
  margin-top: 4px;
  min-width: 280px;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  z-index: 100;
  overflow: hidden;
}

.filter-search {
  width: 100%;
  padding: 10px 12px;
  border: none;
  border-bottom: 1px solid var(--border-color);
  font-size: 14px;
  outline: none;
  background: var(--input-bg);
  color: var(--text-color);
}

.filter-search:focus {
  background: var(--hover-bg);
}

.filter-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--border-color);
  background: var(--hover-bg);
  font-weight: 500;
  font-size: 14px;
}

.back-btn {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 16px;
  padding: 2px 6px;
  border-radius: 4px;
  color: var(--text-color);
}

.back-btn:hover {
  background: var(--border-color);
}

.filter-options {
  max-height: 240px;
  overflow-y: auto;
}

.filter-option {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  cursor: pointer;
  font-size: 14px;
  transition: background 0.1s;
}

.filter-option:hover,
.filter-option.highlighted {
  background: var(--hover-bg);
}

.option-category {
  font-size: 10px;
  text-transform: uppercase;
  color: var(--muted-text);
  background: var(--border-color);
  padding: 2px 6px;
  border-radius: 3px;
  font-weight: 500;
}

.option-label {
  flex: 1;
}

.filter-empty {
  padding: 16px;
  text-align: center;
  color: var(--muted-text);
  font-size: 14px;
}

.filter-text-input {
  display: flex;
  gap: 8px;
  padding: 12px;
}

.filter-text-input .filter-search {
  flex: 1;
  border: 1px solid var(--border-color);
  border-radius: 6px;
}

.btn-sm {
  padding: 6px 12px;
  font-size: 13px;
}

@media (max-width: 768px) {
  .filter-menu {
    min-width: 0;
    width: calc(100vw - 32px);
    max-width: 320px;
    left: auto;
    right: 0;
  }
}
</style>
