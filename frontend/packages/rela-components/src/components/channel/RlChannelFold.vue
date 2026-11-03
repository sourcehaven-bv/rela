<script setup lang="ts">
/**
 * The fold line of a run of collapsed messages in a channel log: one button
 * that says how many messages it stands for and of which types, such as
 * `3 messages: Claim, Progress`.
 *
 * The text is the button's name and does not change with the state;
 * `aria-expanded` carries it. `controls` is the id of the list that holds
 * the run's messages, set only while it is expanded. A click emits `toggle`
 * and leaves focus on the button; the parent shows the messages below it.
 */
import { computed } from 'vue'
import RlIcon from '../common/RlIcon.vue'
import { useMessages } from '../../composables/useMessages'

const props = withDefaults(
  defineProps<{
    /** How many messages the run shows when expanded. */
    count: number
    /** The type labels of those messages, in first-seen order. */
    labels: string[]
    expanded: boolean
    /** The id of the expanded run's list; ignored while collapsed. */
    controls?: string
  }>(),
  { controls: undefined },
)

const emit = defineEmits<{ toggle: [] }>()

const strings = useMessages()
const text = computed(() => strings.channelFold({ count: props.count, labels: props.labels }))
</script>

<template>
  <button
    type="button"
    class="rl-channel-fold"
    :aria-expanded="expanded"
    :aria-controls="expanded ? controls : undefined"
    @click="emit('toggle')"
  >
    <RlIcon :name="expanded ? 'chevron-down' : 'chevron-right'" :size="14" />
    <span class="rl-channel-fold__text">{{ text }}</span>
  </button>
</template>

<style scoped>
.rl-channel-fold {
  display: flex;
  align-items: center;
  gap: var(--rl-space-1);
  max-width: 100%;
  /* The only way to expand a run, so it meets the 24 px target (WCAG 2.5.8). */
  min-block-size: 24px;
  margin: var(--rl-space-1) var(--rl-space-5);
  padding: 2px var(--rl-space-2);
  border: 1px dashed var(--rl-color-border-strong);
  border-radius: var(--rl-radius-md);
  background: transparent;
  color: var(--rl-color-text-muted);
  font: inherit;
  font-size: var(--rl-font-size-xs);
  text-align: start;
  cursor: pointer;
}

.rl-channel-fold:hover {
  color: var(--rl-color-text);
}

.rl-channel-fold:focus-visible {
  outline: var(--rl-focus-ring-width) solid var(--rl-color-focus);
  outline-offset: 1px;
}

.rl-channel-fold__text {
  min-width: 0;
  overflow-wrap: anywhere;
}
</style>
