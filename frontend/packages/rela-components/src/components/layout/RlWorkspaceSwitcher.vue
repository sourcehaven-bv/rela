<script setup lang="ts">
/**
 * Workspace name and logo at the top of the sidebar; opens the switcher.
 *
 * The button itself, not the menu. Attribute fallthrough lands on the
 * `<button>`, so this is a working `RlMenu` or `RlPopover` trigger: spread the
 * slot's `attrs` onto it and the ARIA state comes with it. `triggerAttrs.types`
 * checks that shape on every build.
 */
import RlIcon from '../common/RlIcon.vue'

withDefaults(
  defineProps<{
    name: string
    /**
     * Whether the control opens something. Off drops the chevron and the
     * hover fill, leaving the name as a plain label.
     *
     * For the case where there is nothing to switch to: one workspace, or
     * one space. A chevron that opens a menu listing only where you already
     * are is a control that does nothing, and the header has to read right
     * both ways — so the affordance follows whether a choice exists rather
     * than being painted unconditionally.
     */
    menu?: boolean
  }>(),
  { menu: true },
)
defineEmits<{ click: [] }>()
</script>

<template>
  <button
    type="button"
    class="rl-workspace-switcher"
    :class="{ 'rl-workspace-switcher--static': !menu }"
    @click="$emit('click')"
  >
    <span class="rl-workspace-switcher__logo">
      <slot name="logo">
        <!-- A stand-in mark for a workspace that brought no logo. -->
        <RlIcon name="apps" :size="18" />
      </slot>
    </span>
    <!--
      Both the name and the chevron go in the rail, where there is no room for
      either and the logo alone identifies the workspace. Hidden rather than
      unmounted: the button keeps its accessible name, so a rail user still
      hears which workspace this is.
    -->
    <span class="rl-workspace-switcher__name">{{ name }}</span>
    <RlIcon v-if="menu" name="chevron-down" :size="12" class="rl-workspace-switcher__chevron" />
  </button>
</template>

<style scoped>
.rl-workspace-switcher {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  flex: 1;
  min-width: 0;
  padding: var(--rl-space-1);
  border: none;
  border-radius: var(--rl-radius-md);
  background: transparent;
  font-family: inherit;
  font-size: var(--rl-font-size-md);
  font-weight: var(--rl-font-weight-semibold);
  color: var(--rl-color-text);
  cursor: pointer;
}

.rl-workspace-switcher:hover { background: var(--rl-color-bg-hover); }

/*
 * Nothing to open, so nothing that invites a press. Still a button, because
 * the consumer may hang a tooltip or a rename on it, and a hover fill is the
 * only part that promises a menu.
 */
.rl-workspace-switcher--static { cursor: default; }
.rl-workspace-switcher--static:hover { background: transparent; }

.rl-workspace-switcher__logo {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  flex: none;
  color: var(--rl-color-text);
}

.rl-workspace-switcher__name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rl-workspace-switcher:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: -2px;
}

/*
 * The rail keeps the logo and drops the words, matching `RlNavStatus` at the
 * same width. A container query rather than a prop for the reason stated
 * there: the sidebar has no collapsed state of its own, so the app rebinds
 * `--rl-sidebar-width` and each part notices the width itself.
 *
 * `visually-hidden` rather than `display: none` on the name: the button's
 * accessible name is its text, and removing it leaves a rail control that
 * announces nothing.
 */
@container rl-sidebar (max-width: 120px) {
  .rl-workspace-switcher { flex: none; justify-content: center; }

  .rl-workspace-switcher__name {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }

  .rl-workspace-switcher__chevron { display: none; }
}
</style>
