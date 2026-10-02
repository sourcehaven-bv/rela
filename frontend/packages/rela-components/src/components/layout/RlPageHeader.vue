<script setup lang="ts">
import type { StatusColor } from '../../types'
import RlIconButton from '../common/RlIconButton.vue'
import RlStatusPill from '../common/RlStatusPill.vue'
import RlHeading from '../common/RlHeading.vue'

withDefaults(
  defineProps<{
    title: string
    statusLabel?: string
    statusColor?: StatusColor
    starred?: boolean
    showStar?: boolean
    showMenu?: boolean
    /** Shows the drawer trigger on compact viewports. */
    showNavToggle?: boolean
  }>(),
  {
    statusColor: 'green',
    starred: false,
    showStar: true,
    showMenu: true,
    showNavToggle: true,
  },
)

const emit = defineEmits<{ toggleStar: []; menu: []; openNav: [] }>()
</script>

<template>
  <header class="rl-page-header">
    <div class="rl-page-header__title-row">
      <RlIconButton
        v-if="showNavToggle"
        class="rl-page-header__nav-toggle"
        icon="menu"
        label="Open navigation"
        @click="emit('openNav')"
      />

      <RlHeading :level="1" size="xl" truncate class="rl-page-header__title">{{ title }}</RlHeading>

      <RlIconButton
        v-if="showStar"
        icon="star"
        label="Star"
        :active="starred"
        :pressed="starred"
        @click="emit('toggleStar')"
      />

      <!-- A host with a real menu puts its trigger here, in the button's place. -->
      <slot name="menu">
        <RlIconButton
          v-if="showMenu"
          icon="ellipsis"
          label="More options"
          @click="emit('menu')"
        />
      </slot>

      <!-- A host that draws its own status marker puts it here, in the pill's place. -->
      <slot name="status">
        <RlStatusPill v-if="statusLabel" :label="statusLabel" :color="statusColor" />
      </slot>

      <div class="rl-page-header__actions">
        <slot name="actions" />
      </div>
    </div>

    <div v-if="$slots.tabs || $slots.tools" class="rl-page-header__bar">
      <slot name="tabs" />
      <div class="rl-page-header__tools">
        <slot name="tools" />
      </div>
    </div>
  </header>
</template>

<style scoped>
.rl-page-header {
  padding: var(--rl-space-5) var(--rl-page-gutter-right) 0 var(--rl-page-gutter-left);
  border-bottom: 1px solid var(--rl-color-border);
  background: var(--rl-color-bg);
}

.rl-page-header__title-row {
  display: flex;
  align-items: center;
  gap: var(--rl-space-3);
  min-width: 0;
}

/* The title shrinks (and so truncates) before the chrome beside it does, but
 * it does not claim the free space: the star, menu and status pill sit
 * directly after it. */
.rl-page-header__title { min-width: 0; }

.rl-page-header__actions {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  margin-left: auto;
}

.rl-page-header__bar {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: var(--rl-space-3);
  margin-top: var(--rl-space-4);
}

.rl-page-header__tools {
  display: flex;
  align-items: center;
  gap: var(--rl-space-1);
  padding-bottom: var(--rl-space-2);
  /*
   * Seats the tools right whether or not the bar has tabs beside them. The
   * `space-between` above only separates two children. A header with no view
   * tabs, which is a list screen with a single view, would otherwise leave
   * its only child flush left, putting Search at the opposite end of the bar
   * from where the same control sits on a screen that does have tabs.
   */
  margin-left: auto;
}

/*
 * The drawer trigger only exists where the sidebar is hidden. The shell sets
 * --rl-nav-toggle-display on the medium breakpoint when the panel is open.
 */
.rl-page-header__nav-toggle { display: var(--rl-nav-toggle-display, none); }

@media (max-width: 767px) {
  .rl-page-header {
    padding-top: var(--rl-space-3);
  }

  .rl-page-header__nav-toggle { display: flex; }

  .rl-page-header__title {
    font-size: var(--rl-font-size-lg);
    /* Let the title claim the row; secondary chrome moves out of its way. */
    flex: 1;
  }

  /* Star and overflow menu are reachable from the "..." menu on mobile. */
  .rl-page-header__title-row > .rl-icon-button:not(.rl-page-header__nav-toggle) {
    display: none;
  }

  .rl-page-header__title-row { gap: var(--rl-space-2); }

  /* Tabs scroll horizontally; tools drop to avoid crushing them. */
  .rl-page-header__bar {
    margin-top: var(--rl-space-3);
    overflow-x: auto;
    scrollbar-width: none;
  }
  .rl-page-header__bar::-webkit-scrollbar { display: none; }

  .rl-page-header__tools { display: none; }
}
</style>
