<script setup lang="ts">
/**
 * Back link to the place the reader came from.
 *
 * `label` names that place: "Back to Sprint 4" tells them where they are going,
 * where a bare "Back" only says they will leave. Resolving what the previous
 * place is called needs the router and the app's own titles, so the app passes
 * the finished label and this draws it.
 *
 * Renders an anchor when given `href`, a button otherwise, and whatever `as`
 * names when the app has its own link component. A real link is what makes
 * middle-click and "open in new tab" work, so an app with URLs should render
 * one even while handling `navigate` itself.
 */
import { computed, type Component } from 'vue'
import RlIcon from '../common/RlIcon.vue'

const props = withDefaults(
  defineProps<{
    /** What to call the destination, without the arrow. */
    label?: string
    /**
     * The destination URL. Omit for a history-based back step.
     *
     * For a native anchor. A link component passed through `as` resolves its
     * own URL from its own prop, usually `to`, so this is not passed on to one
     * and setting both has no meaning.
     */
    href?: string
    /**
     * What to render. A router-based app passes its link component here
     * rather than resolving the URL itself: `router.resolve(to).href` makes
     * the calling component depend on a router that can resolve, which a
     * partially mocked router in a test cannot.
     *
     * Pass the component itself rather than a name, so this library does not
     * depend on a router, and pass its `to` alongside — it falls through to
     * the rendered element.
     *
     * Left unset, `href` still picks the element: an anchor when there is a
     * URL, a button when there is not.
     */
    as?: 'button' | 'a' | Component
  }>(),
  { label: 'Back' },
)

/*
 * An explicit `as` wins; otherwise a URL means a link and its absence means a
 * button. Anything that is not the native button is treated as a link, so it
 * takes no `type` and its activation is the browser's to handle.
 */
const tag = computed(() => props.as ?? (props.href ? 'a' : 'button'))

const isButton = computed(() => tag.value === 'button')

/*
 * The element-specific attributes, as an object so an unused one is absent
 * rather than present and undefined. The difference matters: a link component
 * passed `href: undefined` takes it as an explicit value and it beats the
 * `href` the component resolved from its own `to`, which strips the URL from
 * the anchor and costs it the middle click and "open in new tab" that were the
 * reason to render a link. Vue's fallthrough would lose that collision, but a
 * binding written here does not, so the key has to be omitted outright.
 *
 * Only a native element takes either attribute: a link component owns its own
 * `href`, and `type` is meaningless on anything but a button.
 */
const elementAttrs = computed(() => {
  if (isButton.value) return { type: 'button' as const }
  return tag.value === 'a' ? { href: props.href } : {}
})

const emit = defineEmits<{ navigate: [event: MouseEvent] }>()

/*
 * Let the browser handle a modified click on a real link: intercepting
 * cmd-click to run an in-app navigation is how a link loses the ability to open
 * in a new tab.
 */
function onClick(event: MouseEvent) {
  if (event.metaKey || event.ctrlKey || event.shiftKey || event.button !== 0) return
  emit('navigate', event)
}
</script>

<template>
  <component
    :is="tag"
    v-bind="elementAttrs"
    class="rl-back-button"
    @click="onClick"
  >
    <!-- The arrow is decorative: the label already says this goes back. -->
    <RlIcon name="arrow-left" :size="14" aria-hidden="true" />
    <span>{{ label }}</span>
  </component>
</template>

<style scoped>
.rl-back-button {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-1);
  padding: var(--rl-space-1) 0;
  border: none;
  background: none;
  font-family: inherit;
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text-muted);
  text-decoration: none;
  cursor: pointer;
}

.rl-back-button:hover { color: var(--rl-color-text); }

.rl-back-button:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 2px;
  border-radius: var(--rl-radius-sm);
}
</style>
