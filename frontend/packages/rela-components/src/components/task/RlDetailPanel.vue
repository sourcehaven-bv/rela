<script setup lang="ts">
/**
 * Right-hand detail pane with its own toolbar. Use `variant="page"` for the
 * full-width detail view and `variant="panel"` when it sits beside a list.
 *
 * The default slot is padded, so content put there needs nothing arranged
 * around it. A child that has to reach the panel's edges anyway, typically a
 * section rule or a banded row, takes the `rl-detail-panel-bleed` class and
 * gets the inline padding back on its own terms.
 *
 * The footer slot is not padded. It holds things that own their full width,
 * like `RlCommentComposer` with its border and background.
 */
import RlIconButton from '../common/RlIconButton.vue'

withDefaults(
  defineProps<{
    variant?: 'panel' | 'page'
    showNavigation?: boolean
    showExpand?: boolean
    showLink?: boolean
    showAttach?: boolean
    /** Adds an explicit dismiss control, so the panel reads as a layer. */
    showClose?: boolean
  }>(),
  {
    variant: 'panel',
    showNavigation: true,
    showExpand: true,
    showLink: false,
    showAttach: true,
    showClose: false,
  },
)

defineEmits<{
  previous: []
  next: []
  expand: []
  copyLink: []
  attach: []
  menu: []
  close: []
}>()
</script>

<template>
  <section class="rl-detail-panel" :class="`rl-detail-panel--${variant}`">
    <header class="rl-detail-panel__toolbar" aria-label="Task actions">
      <slot name="leading" />

      <template v-if="showNavigation">
        <RlIconButton
          class="rl-detail-panel__nav"
          icon="chevron-down"
          label="Previous"
          @click="$emit('previous')"
        />
        <RlIconButton
          class="rl-detail-panel__nav"
          icon="chevron-up"
          label="Next"
          @click="$emit('next')"
        />
      </template>

      <div class="rl-detail-panel__toolbar-end">
        <slot name="toolbar" />
        <RlIconButton v-if="showLink" icon="link" label="Copy link" @click="$emit('copyLink')" />
        <RlIconButton v-if="showAttach" icon="paperclip" label="Attach file" @click="$emit('attach')" />
        <RlIconButton v-if="showExpand" icon="maximize-2" label="Expand" @click="$emit('expand')" />
        <!-- A host with a real menu puts its trigger here, in the button's place. -->
        <slot name="menu">
          <RlIconButton icon="ellipsis" label="More options" @click="$emit('menu')" />
        </slot>
        <RlIconButton v-if="showClose" icon="x" label="Close panel" @click="$emit('close')" />
      </div>
    </header>

    <div class="rl-detail-panel__body">
      <slot />
    </div>

    <div v-if="$slots.footer" class="rl-detail-panel__footer">
      <slot name="footer" />
    </div>
  </section>
</template>

<style scoped>
.rl-detail-panel {
  display: flex;
  flex-direction: column;
  min-width: 0;
  height: 100%;
  background: var(--rl-color-bg);
}

.rl-detail-panel--panel {
  width: var(--rl-detail-panel-width);
  flex: none;
  border-left: 1px solid var(--rl-color-border);
}

/*
 * Tablet: the panel overlays the list at this width (see RlAppShell), so it
 * keeps a readable width rather than shrinking to share a row it no longer
 * shares.
 */
@media (max-width: 1079px) {
  .rl-detail-panel--panel {
    width: clamp(320px, 72vw, var(--rl-detail-panel-width));
  }
}

@media (max-width: 767px) {
  .rl-detail-panel--panel {
    width: 100%;
    border-left: none;
  }

  .rl-detail-panel__toolbar { padding: var(--rl-space-2) var(--rl-space-3); }

  /* Prev/next stepping is a pointer affordance; the back arrow replaces it. */
  .rl-detail-panel__nav { display: none; }

  .rl-detail-panel__body { padding-top: var(--rl-space-4); }
}


.rl-detail-panel--page { flex: 1; }

.rl-detail-panel__toolbar {
  display: flex;
  align-items: center;
  gap: var(--rl-space-1);
  padding: var(--rl-space-3) var(--rl-space-5);
  border-bottom: 1px solid var(--rl-color-border);
}

.rl-detail-panel__toolbar-end {
  display: flex;
  align-items: center;
  gap: var(--rl-space-1);
  margin-left: auto;
}

/*
 * The body pads itself, so anything dropped in the default slot is readable
 * without the consumer arranging it. The gutter tokens are the same ones the
 * page surfaces use, which is what keeps a panel's content aligned with a
 * full-width view of the same thing.
 */
.rl-detail-panel__body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: var(--rl-space-6) var(--rl-page-gutter-right) var(--rl-space-8) var(--rl-page-gutter-left);
}

/*
 * A section rule or a row that fills the panel has to cross the padding the
 * body just added, so it pulls back out by the same gutter and re-applies the
 * inline padding itself. Without this a divider stops at the text and reads
 * as part of the block above it rather than as a boundary between two.
 */
.rl-detail-panel__body :deep(.rl-detail-panel-bleed) {
  margin-inline: calc(-1 * var(--rl-page-gutter-left)) calc(-1 * var(--rl-page-gutter-right));
  padding-inline: var(--rl-page-gutter-left) var(--rl-page-gutter-right);
}

.rl-detail-panel__footer { flex: none; }
</style>
