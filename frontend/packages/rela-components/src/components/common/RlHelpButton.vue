<script setup lang="ts">
/**
 * Help affordance beside a field or heading: an icon button that opens its
 * explanation in a modal.
 *
 * A modal on every size rather than a popover on the desktop and a sheet on
 * touch. Help is read, not glanced at, and a popover that closes when the
 * pointer drifts is the wrong container for a paragraph the reader is halfway
 * through. One container also means one focus and Escape story, which
 * `RlModal` already owns.
 *
 * The content is a slot, so the app decides whether help is static copy or
 * fetched for a field. Fetching it here would make the library know where help
 * lives.
 */
import { computed, ref } from 'vue'
import RlIconButton from './RlIconButton.vue'
import RlModal from '../overlay/RlModal.vue'
import { useMessages } from '../../composables/useMessages'

const messages = useMessages()

const props = withDefaults(
  defineProps<{
    /** Names what the help is about, and titles the modal. */
    title?: string
    /**
     * The button's accessible name. Defaults from `title`, so a page of help
     * buttons does not present a row of identical "Show help" controls to a
     * screen-reader user listing them.
     */
    label?: string
    size?: 'sm' | 'md' | 'lg'
  }>(),
  { title: undefined, size: 'sm' },
)

const open = ref(false)
const titleText = computed(() => props.title ?? messages.chromeText({ key: 'help' }))
const accessibleName = () => props.label ?? `${messages.chromeText({ key: 'help' })}: ${titleText.value}`
</script>

<template>
  <RlIconButton
    icon="help"
    :label="accessibleName()"
    :size="14"
    :aria-expanded="open"
    @click="open = true"
  />

  <RlModal :open="open" :title="titleText" :size="size" @close="open = false">
    <slot />
  </RlModal>
</template>
