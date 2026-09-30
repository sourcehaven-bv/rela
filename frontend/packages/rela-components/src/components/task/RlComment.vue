<script setup lang="ts">
import type { Comment } from '../../types'

defineProps<{ comment: Comment }>()
</script>

<template>
  <article class="rl-comment">
    <header class="rl-comment__header">
      <span class="rl-comment__author">{{ comment.author }}</span>
      <span class="rl-comment__timestamp">&middot; {{ comment.timestamp }}</span>
    </header>
    <p class="rl-comment__body">{{ comment.body }}</p>
  </article>
</template>

<style scoped>
.rl-comment + .rl-comment { margin-top: var(--rl-space-6); }

.rl-comment__header {
  display: flex;
  align-items: baseline;
  gap: var(--rl-space-2);
}

.rl-comment__author {
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text-muted);
}

.rl-comment__timestamp {
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text-subtle);
}

.rl-comment__body {
  margin: var(--rl-space-2) 0 0;
  font-size: var(--rl-font-size-sm);
  line-height: var(--rl-line-height-relaxed);
  color: var(--rl-color-text);
}
</style>
