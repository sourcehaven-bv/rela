<script setup lang="ts">
import RlIconButton from '../common/RlIconButton.vue'

withDefaults(defineProps<{ shareLabel?: string; starred?: boolean }>(), {
  shareLabel: 'Share',
  starred: false,
})

defineEmits<{
  openNav: []
  share: []
  like: []
  toggleStar: []
  copyLink: []
  attach: []
  menu: []
}>()
</script>

<template>
  <header class="rl-doc-toolbar" aria-label="Document actions">
    <RlIconButton
      class="rl-doc-toolbar__nav-toggle"
      icon="menu"
      label="Open navigation"
      @click="$emit('openNav')"
    />

    <button type="button" class="rl-doc-toolbar__share" @click="$emit('share')">
      {{ shareLabel }}
    </button>

    <RlIconButton icon="thumbs-up" label="Like" @click="$emit('like')" />
    <RlIconButton
      icon="star"
      label="Star"
      :active="starred"
      :pressed="starred"
      @click="$emit('toggleStar')"
    />
    <RlIconButton icon="link" label="Copy link" @click="$emit('copyLink')" />
    <RlIconButton icon="paperclip" label="Attach file" @click="$emit('attach')" />
    <RlIconButton icon="ellipsis" label="More options" @click="$emit('menu')" />
  </header>
</template>

<style scoped>
.rl-doc-toolbar {
  display: flex;
  align-items: center;
  gap: var(--rl-space-1);
  padding: var(--rl-space-3) var(--rl-page-gutter-right) var(--rl-space-3) var(--rl-page-gutter-left);
  border-bottom: 1px solid var(--rl-color-border);
}

.rl-doc-toolbar__share {
  margin-left: auto;
  margin-right: var(--rl-space-2);
  padding: var(--rl-space-1) var(--rl-space-2);
  border: none;
  border-radius: var(--rl-radius-md);
  background: transparent;
  font-family: inherit;
  font-size: var(--rl-font-size-md);
  color: var(--rl-color-text);
  cursor: pointer;
}
.rl-doc-toolbar__share:hover { background: var(--rl-color-bg-hover); }

.rl-doc-toolbar__share:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 1px;
}
</style>
