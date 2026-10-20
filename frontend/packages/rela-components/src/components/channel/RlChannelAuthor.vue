<script setup lang="ts">
/**
 * Who wrote a channel message: an avatar, the name, and a badge when the
 * author is not a person typing in the app.
 *
 * A system message (posted by the server, such as a trigger's) gets a gear
 * instead of initials, so it never looks like a colleague wrote it. An agent
 * keeps the person avatar with a sparkles badge, since an agent acts for
 * someone and reads as a member of the channel.
 */
import RlAvatar from '../data/RlAvatar.vue'
import RlIcon from '../common/RlIcon.vue'
import RlTag from '../common/RlTag.vue'
import type { ChannelAuthorView } from './types'

withDefaults(
  defineProps<{
    author: ChannelAuthorView
    /** Hide the avatar, for a compact line. */
    compact?: boolean
  }>(),
  { compact: false },
)
</script>

<template>
  <span class="rl-channel-author">
    <template v-if="!compact">
      <span v-if="author.kind === 'system'" class="rl-channel-author__system" aria-hidden="true">
        <RlIcon name="cog" :size="14" />
      </span>
      <span v-else class="rl-channel-author__avatar">
        <RlAvatar :name="author.name" size="sm" decorative />
        <span v-if="author.kind === 'agent'" class="rl-channel-author__agent" aria-hidden="true">
          <RlIcon name="sparkles" :size="9" />
        </span>
      </span>
    </template>
    <span class="rl-channel-author__name">{{ author.name }}</span>
    <RlTag v-if="author.kind === 'system'" label="system" color="grey" />
    <RlTag v-else-if="author.kind === 'agent'" label="agent" color="purple" />
  </span>
</template>

<style scoped>
.rl-channel-author {
  display: inline-flex;
  align-items: center;
  gap: var(--rl-space-2);
  min-width: 0;
}

.rl-channel-author__avatar {
  position: relative;
  display: inline-flex;
  flex: none;
}

.rl-channel-author__system {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 24px;
  height: 24px;
  border-radius: var(--rl-radius-pill);
  background: var(--rl-color-bg-hover);
  color: var(--rl-color-text-muted);
}

.rl-channel-author__agent {
  position: absolute;
  right: -3px;
  bottom: -3px;
  display: inline-flex;
  padding: 1px;
  border-radius: var(--rl-radius-pill);
  background: var(--rl-tag-purple-bg);
  color: var(--rl-tag-purple-fg);
  box-shadow: 0 0 0 2px var(--rl-color-bg);
}

.rl-channel-author__name {
  overflow: hidden;
  font-weight: var(--rl-font-weight-semibold);
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
