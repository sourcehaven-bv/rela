<script setup lang="ts">
/**
 * The claim badge: who holds an advisory lease on an entity (TKT-4YPG4Q).
 *
 * Every part of the lease is text the holder or their tool chose, `status`
 * most of all (up to 140 characters of free text). It is rendered through
 * interpolation only, in the text and in the `title` attribute, so markup in
 * it shows as the characters typed. Never switch this to v-html.
 *
 * The visible text is "Claimed by {user} ({label})", with the tool standing
 * in when the claim carries no label. It truncates to fit a title cell or a
 * card; the title attribute holds the full text and the status for a mouse
 * user. A title is not announced reliably and cannot be reached by keyboard
 * or touch, so the status is also in a visually hidden span that screen
 * readers read with the badge.
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { EntityLease } from '@/types'

const props = defineProps<{
  lease?: EntityLease
}>()

const { t } = useI18n()

const text = computed(() => {
  const l = props.lease
  if (!l) return ''
  const who = l.label || l.tool
  return who
    ? t('shell.lease.claimedByWho', { user: l.user, who })
    : t('shell.lease.claimedBy', { user: l.user })
})

const title = computed(() => {
  const status = props.lease?.status
  return status ? `${text.value}\n${status}` : text.value
})
</script>

<template>
  <span v-if="lease" class="lease-badge" :title="title"
    ><span class="lease-badge__text">{{ text }}</span
    ><span v-if="lease.status" class="rl-visually-hidden">{{ t('shell.lease.status', { status: lease.status }) }}</span></span
  >
</template>

<style scoped>
.lease-badge {
  position: relative;
  display: inline-block;
  max-width: 16rem;
  overflow: hidden;
  padding: 0.05rem 0.35rem;
  margin-left: 0.35rem;
  border: 1px solid color-mix(in srgb, var(--rl-color-status-blue) 60%, transparent);
  border-radius: 4px;
  background: color-mix(in srgb, var(--rl-color-status-blue) 14%, transparent);
  color: var(--rl-color-text);
  font-size: 0.72rem;
  font-weight: normal;
  line-height: 1.5;
  text-overflow: ellipsis;
  vertical-align: middle;
  white-space: nowrap;
}
</style>
