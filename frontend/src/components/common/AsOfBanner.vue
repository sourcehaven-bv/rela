<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import RlButton from 'rela-components/components/common/RlButton.vue'
import { intlLocale } from '@/i18n/locale'
import { useAsOf } from '@/composables/useAsOf'

/**
 * Says, above every page, that the page shows a past state and takes no
 * changes, with the way back to the present (TKT-6KLUT3U).
 */
const { t } = useI18n()
const { asOf, setAsOf } = useAsOf()

const when = computed(() =>
  asOf.value
    ? new Date(asOf.value).toLocaleString(intlLocale(), { dateStyle: 'long', timeStyle: 'short' })
    : ''
)
</script>

<template>
  <div v-if="asOf" class="as-of-banner" role="status" data-testid="as-of-banner">
    <span class="as-of-banner__note">{{ t('asOf.banner', { when }) }}</span>
    <RlButton size="sm" variant="secondary" @click="setAsOf(undefined)">{{
      t('asOf.backToNow')
    }}</RlButton>
  </div>
</template>

<style scoped>
.as-of-banner {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-xs) var(--space-md);
  margin-bottom: var(--space-md);
  padding: var(--space-sm) var(--space-md);
  background: color-mix(in srgb, var(--rl-color-status-amber) 12%, transparent);
  border: 1px solid color-mix(in srgb, var(--rl-color-status-amber) 60%, transparent);
  border-radius: var(--radius-md);
}

.as-of-banner__note {
  flex: 1 1 24rem;
  font-size: var(--font-size-sm);
  color: var(--rl-color-text);
}
</style>
