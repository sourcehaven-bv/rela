<script setup lang="ts">
/**
 * Who else has this entity open (TKT-MHXH87): an overlapping row of avatars
 * with a "+N" for the rest.
 *
 * Built from RlAvatar, in the shape of RlAvatarGroup, rather than from the
 * group itself: the group derives its overflow from the people it is given
 * and has no per-avatar title, while presence needs both the server's `more`
 * (viewers past its cap, whom the client never sees) and a `user (tool)`
 * title on each avatar. An agent (an MCP client) carries a sparkles badge,
 * after the realtime avatar mockup in rela-components' fixtures.
 *
 * Nothing renders when nobody else is viewing.
 */
import { computed } from 'vue'
import RlAvatar from 'rela-components/components/data/RlAvatar.vue'
import RlIcon from 'rela-components/components/common/RlIcon.vue'
import type { PresenceViewer } from '@/types'

const props = withDefaults(
  defineProps<{
    viewers: PresenceViewer[]
    /** Viewers past the server's cap; added to the "+N". */
    more?: number
    /** Avatars shown before the rest fold into "+N". */
    max?: number
  }>(),
  { more: 0, max: 5 }
)

const shown = computed(() => props.viewers.slice(0, props.max))
const overflow = computed(() => Math.max(0, props.viewers.length - props.max) + props.more)

function viewerTitle(v: PresenceViewer): string {
  return v.tool ? `${v.user} (${v.tool})` : v.user
}

const description = computed(() => {
  const names = props.viewers.map((v) => v.user).join(', ')
  if (!props.more) return `Also viewing: ${names}`
  return names
    ? `Also viewing: ${names} and ${props.more} more`
    : `Also viewing: ${props.more} people`
})
</script>

<template>
  <span
    v-if="viewers.length || more"
    class="presence-avatars"
    role="img"
    :aria-label="description"
    data-testid="presence-avatars"
  >
    <span
      v-for="v in shown"
      :key="`${v.user}/${v.tool}`"
      class="presence-avatar"
      :title="viewerTitle(v)"
    >
      <RlAvatar :name="v.user" size="sm" decorative />
      <span v-if="v.tool === 'mcp'" class="presence-avatar__agent">
        <RlIcon name="sparkles" :size="9" aria-hidden="true" />
      </span>
    </span>
    <span v-if="overflow" class="presence-avatars__more" aria-hidden="true">+{{ overflow }}</span>
  </span>
</template>

<style scoped>
.presence-avatars {
  display: inline-flex;
  align-items: center;
  flex: none;
}

.presence-avatar {
  position: relative;
  display: inline-flex;
  flex: none;
}

/* Each avatar overlaps the one before it and carries a ring to stay distinct. */
.presence-avatar:not(:first-child),
.presence-avatars__more {
  margin-left: -6px;
}

.presence-avatars :deep(.rl-avatar) {
  box-shadow: 0 0 0 2px var(--rl-color-bg);
}

.presence-avatar__agent {
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

.presence-avatars__more {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: none;
  min-width: 24px;
  height: 24px;
  padding: 0 4px;
  border-radius: var(--rl-radius-pill);
  background: var(--rl-color-bg-active);
  color: var(--rl-color-text);
  font-size: 10px;
  font-weight: var(--rl-font-weight-medium);
  box-shadow: 0 0 0 2px var(--rl-color-bg);
}
</style>
