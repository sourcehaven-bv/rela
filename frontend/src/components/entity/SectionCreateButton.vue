<script setup lang="ts">
/**
 * The "create related entity" affordance (TKT-R4BMJM).
 *
 * One component for every section display mode. `EntityDetail.vue` renders six
 * branches (`properties` / `content` / `cards` / `list` / `table` / `nested`)
 * and this sits in the shared section header above all of them, so the button
 * exists once rather than six times — the branch structure is FEAT-KQ45P's to
 * rework, and copying a button into each arm would make that harder.
 *
 * Renders nothing unless the server sent an affordance with at least one
 * target. Presence in `targets` IS the permission answer (see
 * `ViewSectionCreate`), so there is no permission logic here.
 *
 * One target renders a direct button; several render a menu, because a
 * homogeneous relation — the common case — should not cost a click to
 * disambiguate something with one option.
 */
import { computed, ref, onBeforeUnmount } from 'vue'
import type { ViewSectionCreate, ViewSectionCreateTarget } from '@/api/views'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlIcon from 'rela-components/components/common/RlIcon.vue'

const props = defineProps<{
  create?: ViewSectionCreate
  /** Label for the collapsed menu; defaults to a generic "New". */
  menuLabel?: string
}>()

const emit = defineEmits<{
  select: [create: ViewSectionCreate, target: ViewSectionCreateTarget]
}>()

const open = ref(false)
const root = ref<HTMLElement | null>(null)

const targets = computed(() => props.create?.targets ?? [])
const single = computed(() => (targets.value.length === 1 ? targets.value[0] : null))

function choose(target: ViewSectionCreateTarget) {
  open.value = false
  if (props.create) emit('select', props.create, target)
}

function toggle() {
  open.value = !open.value
  // Bound while open only: a document listener left attached would swallow the
  // next click for every closed menu on the page.
  if (open.value) {
    document.addEventListener('click', onDocumentClick, true)
  } else {
    document.removeEventListener('click', onDocumentClick, true)
  }
}

function onDocumentClick(e: MouseEvent) {
  if (root.value && !root.value.contains(e.target as Node)) {
    open.value = false
    document.removeEventListener('click', onDocumentClick, true)
  }
}

onBeforeUnmount(() => document.removeEventListener('click', onDocumentClick, true))
</script>

<template>
  <div v-if="targets.length > 0" ref="root" class="section-create">
    <RlButton v-if="single" variant="secondary" size="sm" icon="plus" @click="choose(single)">
      {{ single.label }}
    </RlButton>

    <template v-else>
      <RlButton
        variant="secondary"
        size="sm"
        icon="plus"
        :aria-expanded="open"
        aria-haspopup="menu"
        @click="toggle"
      >
        {{ menuLabel || 'New' }}
        <template #trailing>
          <RlIcon name="chevron-down" :size="14" aria-hidden="true" />
        </template>
      </RlButton>
      <div v-if="open" class="create-menu" role="menu">
        <button
          v-for="target in targets"
          :key="target.entityType"
          type="button"
          class="create-menu-item"
          role="menuitem"
          @click="choose(target)"
        >
          {{ target.label }}
        </button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.section-create {
  position: relative;
  display: inline-block;
}

.create-menu {
  position: absolute;
  right: 0;
  z-index: 10;
  min-width: 160px;
  margin-top: var(--space-xs);
  padding: var(--space-xs);
  background: var(--rl-color-bg-raised);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-md);
}

.create-menu-item {
  display: block;
  width: 100%;
  padding: var(--space-xs) var(--space-sm);
  font-size: var(--font-size-sm);
  color: var(--rl-color-text);
  text-align: left;
  background: none;
  border: none;
  border-radius: var(--radius-sm);
  cursor: pointer;
}

.create-menu-item:hover {
  background: var(--rl-color-bg-hover);
}

.create-menu-item:focus-visible {
  outline: none;
  box-shadow:
    0 0 0 2px var(--rl-color-bg),
    0 0 0 4px var(--rl-color-focus);
}
</style>
