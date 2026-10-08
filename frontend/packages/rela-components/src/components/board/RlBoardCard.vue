<script setup lang="ts" generic="T extends CollectionItem">
/**
 * Wraps whatever the `card` slot rendered and makes it draggable.
 *
 * It wraps rather than replaces, because what a card looks like is the
 * consumer's and a board that rendered its own card would take that back.
 * The wrapper contributes three things the card cannot know about: that it
 * can be picked up, that it is currently being dragged, and a keyboard way
 * to move it for people who do not drag.
 *
 * The keyboard path is a grab-and-place, not a continuous drag. Enter or
 * Space on a focused card picks it up and the board shows where it can go;
 * the arrow keys choose a column, Enter drops it there, Escape puts it back.
 * A held-down drag driven by arrow keys would have to invent a pointer
 * position, and every key press would fire a drag event against a cursor
 * that is not where the user is looking.
 */
import { computed, onMounted, onUpdated, ref, toRef } from 'vue'
import type { CollectionItem } from '../../types'
import { useDraggableCard, useDropTargetCard } from '../../composables/useBoardDnd'
import { useMessages } from '../../composables/useMessages'

const props = withDefaults(
  defineProps<{
    item: T
    sectionId: string
    laneId?: string
    /** Whether this card may be picked up at all. */
    draggable?: boolean
    /** Whether this card is the one the keyboard has picked up. */
    grabbed?: boolean
    /** Names the column the card is in, for the grab announcement. */
    sectionTitle?: string
    /**
     * Whether another card can be dropped before or after this one. See
     * `reorder` on `RlBoard`.
     */
    reorder?: boolean
  }>(),
  { draggable: false, grabbed: false, reorder: false },
)

const emit = defineEmits<{ grab: [item: T]; release: [] }>()

const messages = useMessages()

const element = ref<HTMLElement>()

/*
 * Links and images drag themselves.
 *
 * An `<a>` or an `<img>` is natively draggable, so a pointer press that lands
 * on one starts the browser's own drag of that element: the card's drag never
 * begins, no column lights up, and the drop never fires. The card still looks
 * draggable, which is what makes this worth handling here rather than in a
 * note — the failure is silent, and every consumer whose card is a link would
 * have to rediscover it.
 *
 * Marking them `draggable="false"` hands the gesture back to the wrapper. An
 * element that already carries the attribute is left alone: an author who set
 * it meant it.
 */
function releaseNativeDrags() {
  if (!props.draggable || !element.value) return
  for (const child of element.value.querySelectorAll('a, img')) {
    if (!child.hasAttribute('draggable')) child.setAttribute('draggable', 'false')
  }
}

/* The slot's content is the consumer's and can change without the card doing. */
onMounted(releaseNativeDrags)
onUpdated(releaseNativeDrags)

const { dragging } = useDraggableCard({
  element,
  enabled: toRef(() => props.draggable),
  data: computed(() => ({
    itemId: props.item.id,
    fromSectionId: props.sectionId,
    fromLaneId: props.laneId,
  })),
})

const { edge } = useDropTargetCard({
  element,
  enabled: toRef(() => props.reorder),
  itemId: toRef(() => props.item.id),
})

function onKeydown(event: KeyboardEvent) {
  if (!props.draggable) return
  if (event.key !== 'Enter' && event.key !== ' ') return

  /*
   * A card is usually a link or holds a button, and both answer Enter and
   * Space themselves. Only the modifier-free press on the card's own box is
   * a grab, so a click-through still opens the task.
   */
  if (event.target !== event.currentTarget) return

  /*
   * Only picking a card up is the card's business. Once it is held, every
   * key belongs to the board, which is what knows the columns: Enter drops
   * it, the arrows aim it, Escape puts it back.
   *
   * The grab is stopped from bubbling because the board answers Enter too,
   * and one press that both picked a card up and dropped it would leave the
   * board exactly as it was.
   */
  if (props.grabbed) return

  event.preventDefault()
  event.stopPropagation()
  emit('grab', props.item)
}
</script>

<template>
  <div
    ref="element"
    class="rl-board-card"
    :class="{
      'rl-board-card--draggable': draggable,
      'rl-board-card--dragging': dragging,
      'rl-board-card--grabbed': grabbed,
      [`rl-board-card--drop-${edge}`]: edge !== null,
    }"
    :tabindex="draggable ? 0 : undefined"
    :role="draggable ? 'button' : undefined"
    :aria-roledescription="draggable ? 'Draggable card' : undefined"
    :aria-grabbed="draggable ? grabbed : undefined"
    :aria-label="
      draggable
        ? grabbed
          ? messages.cardGrabbed({ title: item.title })
          : messages.cardAtRest({ title: item.title, section: sectionTitle })
        : undefined
    "
    @keydown="onKeydown"
  >
    <slot />
  </div>
</template>

<style scoped>
.rl-board-card {
  border-radius: var(--rl-radius-lg);
}

.rl-board-card--draggable { cursor: grab; }

/*
 * The card under the cursor while its own copy is being dragged. Left in
 * place rather than removed, so the column does not close up and every card
 * below it jump a slot the moment the drag starts.
 */
.rl-board-card--dragging {
  opacity: 0.4;
  cursor: grabbing;
}

/*
 * Where a held card will land: a line just above or below this card, in
 * the gap between cards, so it reads as "between these two" rather than as
 * a mark on either. It sits within the 2px the column's scroll area keeps
 * around its cards: any further out, and the line above the first card or
 * below the last is clipped.
 */
.rl-board-card { position: relative; }
.rl-board-card--drop-top::before,
.rl-board-card--drop-bottom::before {
  content: '';
  position: absolute;
  inset-inline: 0;
  height: 2px;
  border-radius: 1px;
  background: var(--rl-color-accent);
  pointer-events: none;
}
.rl-board-card--drop-top::before { top: -2px; }
.rl-board-card--drop-bottom::before { bottom: -2px; }

/* The keyboard equivalent of holding a card. */
.rl-board-card--grabbed {
  outline: 2px solid var(--rl-color-accent);
  outline-offset: 2px;
}

.rl-board-card:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 2px;
}

.rl-board-card {
  transition: opacity var(--rl-duration-fast) var(--rl-ease), transform var(--rl-duration-fast) var(--rl-ease);
}

@media (prefers-reduced-motion: reduce) {
  .rl-board-card { transition: none; }
}
</style>
