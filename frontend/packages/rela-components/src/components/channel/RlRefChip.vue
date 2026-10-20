<script setup lang="ts">
/**
 * An entity reference inside a channel message.
 *
 * A readable ref shows its id and title and links to the entity. A restricted
 * ref is one the server would not resolve for this user: it shows a fixed
 * placeholder and nothing else, because the id, the type and the title are
 * all things the user is not allowed to learn. The type has no `id` on that
 * branch, so the template cannot leak one by accident.
 *
 * The app owns navigation. Pass `href` for a real link (middle click and the
 * status bar keep working); `open` fires on a plain click so an app with a
 * router can call `preventDefault()` and navigate itself. Without `href` the
 * chip is a button that only emits `open`, and with `plain` it is text that
 * does nothing, for a place where opening the entity has no meaning.
 *
 * `href` must be an in-app path the app built from `value.id`, never text an
 * author wrote. As a second line of defence, an href that is not a same-page
 * relative path (starting with `/`, `#` or `?`, and not `//`) is ignored and
 * the chip falls back to the button, so a `javascript:` or `data:` URL can
 * never become a link.
 */
import { computed } from 'vue'
import type { ChannelReadableRef, ChannelRef } from './types'
import { useMessages } from '../../composables/useMessages'

const props = withDefaults(
  defineProps<{
    value: ChannelRef
    href?: string
    /** Render as text with no click behaviour. */
    plain?: boolean
  }>(),
  { plain: false },
)

const emit = defineEmits<{ open: [ref: ChannelReadableRef, event: MouseEvent] }>()

const messages = useMessages()

const readable = computed<ChannelReadableRef | null>(() =>
  props.value.restricted ? null : props.value,
)

/** The href when it is a same-page relative path; otherwise none. */
const safeHref = computed(() => {
  const h = props.href
  if (!h || h.startsWith('//')) return undefined
  return h.startsWith('/') || h.startsWith('#') || h.startsWith('?') ? h : undefined
})

const tag = computed(() => (props.plain ? 'span' : safeHref.value ? 'a' : 'button'))

function onClick(event: MouseEvent) {
  if (!readable.value) return
  // A modified click opens a new tab or window; that is the browser's.
  if (props.plain) return
  if (safeHref.value && (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey)) return
  emit('open', readable.value, event)
}
</script>

<template>
  <span
    v-if="!readable"
    class="rl-ref-chip rl-ref-chip--restricted"
    :title="messages.channelText({ key: 'restrictedRefTitle' })"
  >
    {{ messages.channelText({ key: 'restrictedRef' }) }}
  </span>
  <component
    :is="tag"
    v-else
    class="rl-ref-chip"
    :class="{ 'rl-ref-chip--plain': plain }"
    :href="tag === 'a' ? safeHref : undefined"
    :type="tag === 'button' ? 'button' : undefined"
    @click="onClick"
  >
    <span class="rl-ref-chip__id">{{ readable.id }}</span>
    <span v-if="readable.title" class="rl-ref-chip__title">{{ readable.title }}</span>
  </component>
</template>

<style scoped>
.rl-ref-chip {
  display: inline-flex;
  align-items: baseline;
  gap: var(--rl-space-1);
  max-width: 100%;
  padding: 0 var(--rl-space-2);
  border: 1px solid transparent;
  border-radius: var(--rl-radius-sm);
  background: var(--rl-color-bg-selected);
  color: var(--rl-color-info-fg);
  font: inherit;
  font-size: var(--rl-font-size-sm);
  font-weight: var(--rl-font-weight-medium);
  line-height: 20px;
  text-decoration: none;
  vertical-align: baseline;
  cursor: pointer;
}

.rl-ref-chip:hover { text-decoration: underline; }

.rl-ref-chip:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 1px;
}

.rl-ref-chip__id { white-space: nowrap; }

.rl-ref-chip__title {
  min-width: 0;
  overflow: hidden;
  color: var(--rl-color-text);
  font-weight: var(--rl-font-weight-normal);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rl-ref-chip--plain { cursor: default; }
.rl-ref-chip--plain:hover { text-decoration: none; }

.rl-ref-chip--restricted {
  border-color: var(--rl-color-border-strong);
  border-style: dashed;
  background: none;
  color: var(--rl-color-text-muted);
  font-weight: var(--rl-font-weight-normal);
  cursor: help;
}

.rl-ref-chip--restricted:hover { text-decoration: none; }
</style>
