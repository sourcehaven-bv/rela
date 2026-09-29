<script setup lang="ts">
/** Full-width labelled link pinned to the bottom of the sidebar. */
import RlIcon from '../common/RlIcon.vue'
import type { IconName } from '../common/icons'

withDefaults(defineProps<{ label: string; icon?: IconName }>(), { icon: 'help' })
defineEmits<{ click: [] }>()
</script>

<template>
  <!--
    Titled for the rail, where the label is hidden and the icon is all that
    is left to say which link this is.
  -->
  <button type="button" class="rl-sidebar-footer-link" :title="label" @click="$emit('click')">
    <RlIcon :name="icon" :size="16" />
    <!-- Wrapped rather than bare, so the rail has something to hide. -->
    <span class="rl-sidebar-footer-link__label">{{ label }}</span>
  </button>
</template>

<style scoped>
.rl-sidebar-footer-link {
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
  color: var(--rl-color-text-muted);
  cursor: pointer;
}

.rl-sidebar-footer-link:hover { background: var(--rl-color-bg-hover); }

.rl-sidebar-footer-link__label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rl-sidebar-footer-link:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: -2px;
}

/*
 * The rail, matching `RlSidebarNavItem`: the label is hidden visually rather
 * than removed, because it is the button's accessible name.
 */
@container rl-sidebar (max-width: 120px) {
  .rl-sidebar-footer-link {
    justify-content: center;
    gap: 0;
  }

  .rl-sidebar-footer-link__label {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }
}
</style>
