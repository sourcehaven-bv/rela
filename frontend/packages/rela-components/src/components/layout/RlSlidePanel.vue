<script setup lang="ts">
/**
 * One panel in a slide-out stack: a column that slides in from the left edge
 * and sits above the page, with its own header and its own close control.
 *
 * This is the piece `RlSlidePanelStack` repeats. Use it directly only for a
 * single panel that never opens a second one; anything that stacks should go
 * through the stack, which owns the layering and the Escape order.
 *
 * It is not `RlDrawer`. A drawer is a place to finish a task and takes the
 * keyboard while it is open; this is a navigation column the user reads
 * alongside the page, so there is no scrim, no focus trap and no scroll lock.
 * Clicking the page behind it is a normal thing to do here.
 */
import { computed } from 'vue'
import RlHeading from '../common/RlHeading.vue'
import RlIconButton from '../common/RlIconButton.vue'

const props = withDefaults(
  defineProps<{
    title: string
    /**
     * How wide the panel is. `sm` is a list of names, `md` a list of rows
     * with metadata, `lg` a reading column.
     */
    size?: 'sm' | 'md' | 'lg'
    /** Hides the heading visually, leaving it for screen readers. */
    titleHidden?: boolean
    /** Whether the panel offers a close control. */
    closable?: boolean
    /**
     * Distance from the left edge of the page to this panel's own left edge.
     * The stack sets it so each panel clears the ones before it; on its own
     * the panel sits against the edge.
     */
    offset?: string
  }>(),
  { size: 'md', titleHidden: false, closable: true, offset: '0px' },
)

const emit = defineEmits<{ close: [] }>()

const style = computed(() => ({ left: props.offset }))
</script>

<template>
  <aside
    class="rl-slide-panel"
    :class="[`rl-slide-panel--${size}`]"
    :style="style"
    role="complementary"
    :aria-label="title"
    tabindex="-1"
  >
    <header class="rl-slide-panel__header">
      <RlHeading v-if="!titleHidden" :level="2" size="md" class="rl-slide-panel__title">
        {{ title }}
      </RlHeading>
      <span v-else class="rl-visually-hidden">{{ title }}</span>

      <div v-if="$slots.actions" class="rl-slide-panel__actions">
        <slot name="actions" />
      </div>

      <RlIconButton
        v-if="closable"
        icon="x"
        label="Close panel"
        class="rl-slide-panel__close"
        @click="emit('close')"
      />
    </header>

    <div class="rl-slide-panel__body"><slot /></div>

    <footer v-if="$slots.footer" class="rl-slide-panel__footer">
      <slot name="footer" />
    </footer>
  </aside>
</template>

<style scoped>
.rl-slide-panel {
  position: absolute;
  top: 0;
  bottom: 0;
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  background: var(--rl-color-bg-raised);
  /*
   * The edge faces the page on the right only. There is no scrim here, so
   * unlike a drawer this border is doing the separating in both themes and
   * the shadow only deepens it.
   */
  border-right: 1px solid var(--rl-color-border);
  box-shadow: var(--rl-shadow-lg);
  /* Above the page, below anything the overlay stack owns. */
  z-index: var(--rl-z-sticky-raised);
}

/*
 * The panel takes focus when the stack is managing it, but it is a region
 * rather than a control, so it draws no ring: the ring would claim the whole
 * column is the thing you are about to act on.
 */
.rl-slide-panel:focus { outline: none; }

.rl-slide-panel--sm { width: var(--rl-slide-panel-width-sm); }
.rl-slide-panel--md { width: var(--rl-slide-panel-width-md); }
.rl-slide-panel--lg { width: var(--rl-slide-panel-width-lg); }

/*
 * A panel cannot be wider than what is left of the page beside the ones it
 * is stacked on, which `left` already measures. Without this the third panel
 * of three would run off a narrow window and take its close button with it.
 */
.rl-slide-panel { max-width: 100%; }

.rl-slide-panel__header {
  display: flex;
  align-items: center;
  gap: var(--rl-space-3);
  flex: none;
  padding: calc(var(--rl-space-5) + var(--rl-safe-inset-top))
           var(--rl-space-5) var(--rl-space-4);
}

.rl-slide-panel__title {
  min-width: 0;
  /* A long title shortens rather than pushing the close control off. */
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rl-slide-panel__actions {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  margin-left: auto;
}

/* Still last when there are no actions to push it over. */
.rl-slide-panel__actions + .rl-slide-panel__close { margin-left: 0; }
.rl-slide-panel__close { margin-left: auto; }

.rl-slide-panel__body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

/*
 * A section heading dropped straight into the body is aligned with the rows
 * under it, so the panel keeps one left edge. `RlSectionHeading` carries no
 * padding of its own because a board column and a table section each place it
 * differently, which leaves the panel to place it too.
 *
 * Scoped to a direct child so a heading inside a consumer's own padded
 * wrapper is left alone rather than indented twice.
 */
.rl-slide-panel__body > :deep(.rl-section-heading),
.rl-slide-panel__body > * > :deep(.rl-section-heading) {
  padding: var(--rl-space-4) var(--rl-space-5) var(--rl-space-2);
}

.rl-slide-panel__body:last-child {
  padding-bottom: var(--rl-safe-inset-bottom);
}

.rl-slide-panel__footer {
  flex: none;
  padding: var(--rl-space-4) var(--rl-space-5)
           calc(var(--rl-space-4) + var(--rl-safe-inset-bottom));
  border-top: 1px solid var(--rl-color-border);
}

/*
 * The panel stands against the left edge of the page when nothing is to its
 * left, so it clears the notch there. Panels further along the stack are
 * already clear of it.
 */
.rl-slide-panel__header,
.rl-slide-panel__footer {
  padding-left: calc(var(--rl-space-5) + var(--rl-safe-inset-left));
}
</style>
