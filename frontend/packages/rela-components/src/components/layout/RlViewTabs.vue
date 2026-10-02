<script setup lang="ts">
/**
 * The row of views above a page.
 *
 * ## A tab that is also a URL
 *
 * A tab renders as a button by default, because switching a view is usually
 * a change of state within one page. Where a view has its own URL, give the
 * tab `as` and `attrs` and it renders as a link instead: only an anchor gives
 * the middle click, the modifier click, the context menu and the status bar a
 * user expects from navigation.
 *
 * This follows `RlSidebarNavItem`, including passing a router link as the
 * component itself rather than as a name, so this library does not take a
 * dependency on a router.
 *
 *     { id: 'notes', label: 'Notes', as: 'a', attrs: { href: '/p/x/notes' } }
 *     { id: 'notes', label: 'Notes', as: RouterLink, attrs: { to: '/p/x/notes' } }
 *
 * Per tab rather than per row, so a row can mix a view that is a URL with one
 * that is not.
 *
 * `update:modelValue` still fires either way. A link already navigates, so a
 * caller driving the active tab from the route can ignore the event; one that
 * wants to switch without waiting for the route can use it.
 */
import type { Component } from 'vue'
import RlIcon from '../common/RlIcon.vue'
import type { IconName } from '../common/icons'

export interface ViewTab {
  id: string
  label: string
  icon?: IconName
  /**
   * How the detail panel sits against this view. A view that needs its full
   * width declares `overlay` here rather than the shell special-casing it.
   */
  panelMode?: 'inline' | 'overlay'
  /**
   * What this tab renders as. A view with its own URL should be a real link;
   * see the note above.
   *
   * Carried as the component itself, never as a name to look up, for the
   * reason `NavItem.as` gives.
   */
  as?: 'button' | 'a' | Component
  /**
   * Extra attributes for the rendered element, such as `to` for a router
   * link or `href` and `target` for a plain anchor.
   */
  attrs?: Record<string, unknown>
}

const props = withDefaults(
  defineProps<{ tabs: ViewTab[]; modelValue: string; showAdd?: boolean }>(),
  { showAdd: true },
)

const emit = defineEmits<{ 'update:modelValue': [id: string]; add: [] }>()

/*
 * Only a real button takes `type`. An anchor given one would be claiming
 * something it is not, the same distinction `RlSidebarNavItem` draws.
 */
function tagOf(tab: ViewTab) {
  return tab.as ?? 'button'
}

function isButton(tab: ViewTab) {
  return tagOf(tab) === 'button'
}

/**
 * Arrow-key roving focus, as expected of a tablist.
 *
 * The tab is focused as well as selected, because with links the two come
 * apart: selecting alone would leave focus on the tab the user arrowed away
 * from, and the next arrow press would move from there instead of from the
 * tab they just reached.
 */
function move(event: KeyboardEvent, delta: number) {
  const index = props.tabs.findIndex((t) => t.id === props.modelValue)
  if (index === -1) return
  const next = props.tabs[(index + delta + props.tabs.length) % props.tabs.length]
  emit('update:modelValue', next.id)

  const list = (event.currentTarget as HTMLElement).parentElement
  const target = list?.querySelector<HTMLElement>(`[data-tab-id="${CSS.escape(next.id)}"]`)
  target?.focus()
}
</script>

<template>
  <div class="rl-view-tabs">
    <!-- Only tabs may live inside a tablist, so "Add" sits outside it. -->
    <div class="rl-view-tabs__list" role="tablist" aria-label="View">
      <!--
        `data-tab-id` is how the arrow keys find the tab to focus. Needed
        because a tab may be a router link, whose rendered element this
        component never holds a ref to.
      -->
      <component
        :is="tagOf(tab)"
        v-for="tab in tabs"
        :key="tab.id"
        v-bind="tab.attrs"
        :type="isButton(tab) ? 'button' : undefined"
        role="tab"
        class="rl-view-tab"
        :class="{ 'rl-view-tab--active': tab.id === modelValue }"
        :data-tab-id="tab.id"
        :aria-selected="tab.id === modelValue"
        :tabindex="tab.id === modelValue ? undefined : -1"
        @click="emit('update:modelValue', tab.id)"
        @keydown.left.prevent="move($event, -1)"
        @keydown.right.prevent="move($event, 1)"
      >
        <RlIcon v-if="tab.icon" :name="tab.icon" :size="15" />
        {{ tab.label }}
      </component>
    </div>

    <button v-if="showAdd" type="button" class="rl-view-tab rl-view-tab--add" @click="emit('add')">
      <RlIcon name="plus" :size="15" />
      Add
    </button>
  </div>
</template>

<style scoped>
.rl-view-tabs,
.rl-view-tabs__list {
  display: flex;
  align-items: center;
  gap: var(--rl-space-1);
}

.rl-view-tab {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-2);
  padding: var(--rl-space-2) var(--rl-space-2) var(--rl-space-3);
  border: none;
  border-bottom: 2px solid transparent;
  background: transparent;
  font-family: inherit;
  font-size: var(--rl-font-size-md);
  color: var(--rl-color-text-muted);
  cursor: pointer;
  /* The tab is styled as a tab whether it renders as a button or a link. */
  text-decoration: none;
}

.rl-view-tab:hover { color: var(--rl-color-text); }

.rl-view-tab--active {
  color: var(--rl-color-text);
  border-bottom-color: var(--rl-color-text);
}

.rl-view-tab--add { color: var(--rl-color-text-subtle); }

.rl-view-tab:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: -2px;
}
</style>
