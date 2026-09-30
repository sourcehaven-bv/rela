<script setup lang="ts">
import { computed, ref, type Component } from 'vue'
import type { NavItem, NavItemStatus, NavItemTone } from '../../types'
import RlIcon from '../common/RlIcon.vue'
import { resolveIcon } from '../common/resolveIcon'
import RlSidebarBadge from './RlSidebarBadge.vue'
import RlNavIcon from './RlNavIcon.vue'
import RlNavStatus from './RlNavStatus.vue'
import { useMessages } from '../../composables/useMessages'

const messages = useMessages()

const props = withDefaults(
  defineProps<{
    item: NavItem
    activeId?: string
    depth?: number
    /**
     * What to render each row as. A sidebar that navigates by URL wants a
     * real link: only an anchor gives the middle click, the modifier click,
     * the context menu and the status bar a user expects from navigation.
     *
     * Pass the router link component itself rather than a name, so this
     * library does not depend on a router, and pass its `to` or `href`
     * alongside — they fall through to the rendered element.
     *
     * Per item rather than per sidebar on purpose: one list can hold both a
     * link that goes somewhere and a control that runs an action, and each
     * should be the element it actually is.
     */
    as?: 'button' | 'a' | Component
    /**
     * The id of the item whose panel is open, if any. Only an item marked
     * `opensFlyout` reads it, and only to report its own state.
     */
    flyoutId?: string | null
  }>(),
  { depth: 0, as: 'button', flyoutId: null },
)

/*
 * The item's own choice wins over the list's default, so one sidebar can mix
 * links and buttons without the caller splitting it into two lists.
 */
const tag = computed(() => props.item.as ?? props.as)

/*
 * Only a real button takes `type`, and only a real button is a parent whose
 * children it expands; a link that also claimed `aria-expanded` would be
 * describing something it does not do.
 */
const isButton = computed(() => tag.value === 'button')

const emit = defineEmits<{ select: [item: NavItem] }>()

// Named so the template can recurse into itself.
defineOptions({ name: 'RlSidebarNavItem' })

const expanded = ref(props.item.expanded ?? false)

/*
 * A sidebar icon is the one name that comes from project config rather than
 * from this library's own code, so it is resolved through the allowlist rather
 * than cast. An unrecognised name draws the fallback; `none` draws nothing and
 * keeps the column.
 */
const icon = computed(() => resolveIcon(props.item.icon))

const hasChildren = computed(() => Boolean(props.item.children?.length))

/*
 * An item that opens a panel reports whether that panel is the open one, so
 * the row announces both that it opens something and whether it is open
 * now. `undefined` on every other row, which leaves `aria-expanded` to its
 * parent/child meaning or drops it entirely.
 */
const flyoutExpanded = computed(() =>
  props.item.opensFlyout ? props.flyoutId === props.item.id : undefined,
)

/*
 * A collapsed parent stands in for the statuses it is hiding, so a branch
 * with a failure inside does not look calm while it is shut. Without this a
 * user has to open every parent to find out whether anything is wrong, which
 * defeats an indicator meant to be read at a glance.
 *
 * The most severe wins, and the counts add up, because the row is answering
 * "is there anything in here" rather than reporting each child.
 */
const TONE_RANK: Record<NavItemTone, number> = {
  error: 4,
  warning: 3,
  new: 2,
  info: 1,
  success: 0,
}

function collectStatuses(items: NavItem[]): NavItemStatus[] {
  return items.flatMap((child) => [
    ...(child.status ? [child.status] : []),
    ...collectStatuses(child.children ?? []),
  ])
}

const rolledUpStatus = computed<NavItemStatus | undefined>(() => {
  /* An open parent shows nothing of its own: the children are saying it. */
  if (expanded.value || !hasChildren.value) return undefined

  const found = collectStatuses(props.item.children ?? [])
  if (!found.length) return undefined

  const worst = found.reduce((a, b) => (TONE_RANK[b.tone] > TONE_RANK[a.tone] ? b : a))

  /* One hidden status speaks for itself; there is nothing to summarise. */
  if (found.length === 1) return worst

  /*
   * Counted within the winning tone only. Adding an unread count to a failed
   * sync would produce a number meaning nothing: "7" across "4 new" and "3
   * errors" is not 7 of anything. Summing only what shares a tone keeps the
   * number answering one question.
   *
   * Dropped entirely unless every status of that tone carries a count, since
   * a partial sum under-reports and reads as exact.
   */
  const sameTone = found.filter((status) => status.tone === worst.tone)
  const counted = sameTone.every((status) => typeof status.count === 'number')
  const count = counted
    ? sameTone.reduce((sum, status) => sum + (status.count ?? 0), 0)
    : undefined

  /*
   * Labelled by how many rows are flagged rather than by repeating one
   * child's message, which would name a row the user cannot currently see.
   */
  return {
    tone: worst.tone,
    label: `${found.length} items need attention`,
    count: count || undefined,
  }
})

/* The item's own status wins; a parent only speaks for children it hides. */
const shownStatus = computed(() => props.item.status ?? rolledUpStatus.value)

/*
 * A button row toggles its own children, because pressing it has no other
 * meaning. A link row navigates, so the toggle moves to the caret beside it:
 * a row that both went somewhere and expanded in place would do two things
 * from one press, and the user could not ask for only one of them.
 */
const togglesOnPress = computed(() => hasChildren.value && isButton.value)

function onClick() {
  if (togglesOnPress.value) expanded.value = !expanded.value
  emit('select', props.item)
}

function onCaretClick() {
  expanded.value = !expanded.value
}
</script>

<template>
  <div class="rl-nav-item-wrapper">
    <component
      :is="tag"
      v-bind="item.attrs"
      :type="isButton ? 'button' : undefined"
      class="rl-nav-item"
      :class="{
        'rl-nav-item--active': activeId === item.id,
        'rl-nav-item--flyout-open': flyoutExpanded === true,
      }"
      :style="{ paddingLeft: `${8 + depth * 20}px` }"
      :aria-expanded="flyoutExpanded ?? (togglesOnPress ? expanded : undefined)"
      :aria-current="activeId === item.id && !isButton ? 'page' : undefined"
      @click="onClick"
    >
      <!--
        Its own control only where the row cannot carry the toggle itself: on
        a button row the press already does it, and a nested button would put
        a second tab stop on every parent for no gain.
      -->
      <button
        v-if="hasChildren && !isButton"
        type="button"
        class="rl-nav-item__caret rl-nav-item__caret--control"
        :aria-expanded="expanded"
        :aria-label="messages.toggleDisclosure({ label: item.label, open: expanded })"
        @click.prevent.stop="onCaretClick"
      >
        <RlIcon :name="expanded ? 'chevron-down' : 'chevron-right'" :size="12" />
      </button>
      <span v-else-if="hasChildren" class="rl-nav-item__caret">
        <RlIcon :name="expanded ? 'chevron-down' : 'chevron-right'" :size="12" />
      </span>
      <RlIcon v-else-if="icon.kind === 'icon'" :name="icon.name" :size="16" />
      <RlSidebarBadge v-else-if="item.initial" :initial="item.initial" />
      <!-- The author asked for no glyph; hold the column so labels stay aligned. -->
      <RlNavIcon v-else-if="icon.kind === 'none'" reserve :size="16" />
      <!--
        Neither icon nor badge: reserve the space so this item's label stays
        aligned with its siblings' instead of jumping left.
      -->
      <RlNavIcon v-else reserve :size="16" />

      <!--
        Titled, because the label is truncated and a workspace with many
        initiatives ends up with several that differ only past the ellipsis.
      -->
      <span class="rl-nav-item__label" :title="item.label">{{ item.label }}</span>

      <RlNavStatus v-if="shownStatus" :status="shownStatus" />
    </component>

    <div v-if="expanded && item.children?.length" class="rl-nav-item__children">
      <RlSidebarNavItem
        v-for="child in item.children"
        :key="child.id"
        :item="child"
        :active-id="activeId"
        :flyout-id="flyoutId"
        :depth="depth + 1"
        :as="as"
        @select="emit('select', $event)"
      />
    </div>
  </div>
</template>

<style scoped>
.rl-nav-item {
  /* Positions the rail's corner dot, which leaves the flow at that width. */
  position: relative;
  display: flex;
  align-items: center;
  gap: var(--rl-space-3);
  width: 100%;
  padding: 6px var(--rl-space-2);
  border: none;
  border-radius: var(--rl-radius-md);
  background: transparent;
  font-family: inherit;
  font-size: var(--rl-font-size-md);
  color: var(--rl-color-text);
  text-align: left;
  cursor: pointer;
  /* The row is styled as a row whether it renders as a button or a link. */
  text-decoration: none;
}

.rl-nav-item:hover { background: var(--rl-color-bg-hover); }
.rl-nav-item--active { background: var(--rl-color-bg-active); }

/*
 * The row whose panel is open, so the panel beside it has a visible origin.
 * Weaker than `--active`, which says where you are: a panel is something you
 * opened over the page, not a place you went.
 */
.rl-nav-item--flyout-open { background: var(--rl-color-bg-selected); }

.rl-nav-item__caret {
  display: flex;
  width: 16px;
  color: var(--rl-color-text-subtle);
}

/* The caret as its own control, where the row itself navigates instead. */
.rl-nav-item__caret--control {
  align-items: center;
  justify-content: center;
  flex: none;
  padding: 0;
  border: none;
  border-radius: var(--rl-radius-sm);
  background: none;
  color: inherit;
  cursor: pointer;
}

.rl-nav-item__caret--control:hover { color: var(--rl-color-text); }

.rl-nav-item__label {
  /*
   * Takes the slack, so a status marker is pushed to the end of the row
   * rather than sitting against the last word of a short label.
   */
  flex: 1;
  min-width: 0;
  /* Gives a status marker a baseline to sit on; see RlNavStatus. */
  align-self: baseline;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rl-nav-item:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: -2px;
}

/*
 * The rail. `RlNavStatus` and `RlWorkspaceSwitcher` already answer this
 * width; without the same rule here the row kept its full-width padding and
 * its label, so the nav overflowed its own rail and grew a scrollbar rather
 * than narrowing.
 *
 * `visually-hidden` rather than `display: none` on the label, for the reason
 * the switcher gives: the label is the row's accessible name, and dropping
 * it leaves a rail of controls that announce nothing. The `title` the
 * template already sets is what shows the word on hover.
 *
 * The left padding is overridden because the template sets it inline, per
 * depth, to indent nested rows. In the rail there is no room to indent and
 * nothing to indent under, since the caret goes too.
 */
@container rl-sidebar (max-width: 120px) {
  .rl-nav-item {
    justify-content: center;
    gap: 0;
    /* Beats the template's inline `padding-left`. */
    padding-left: var(--rl-space-2) !important;
    padding-right: var(--rl-space-2);
  }

  .rl-nav-item__label {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }

  /*
   * A disclosure caret with no labels to disclose is noise, and the children
   * are indented rows whose indent the rail has already dropped.
   */
  .rl-nav-item__caret { display: none; }
  .rl-nav-item__children { display: none; }
}
</style>
