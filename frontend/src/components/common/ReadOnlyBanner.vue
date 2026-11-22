<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { workspaceReadOnly } from '@/api/workspaceReadOnly'

/**
 * Says that the workspace is read-only, while it is (rela#2511). The create
 * buttons are hidden then, so without this a user would not know why.
 */
const { t } = useI18n()
</script>

<template>
  <div
    v-if="workspaceReadOnly"
    class="read-only-banner"
    role="status"
    data-testid="read-only-banner"
  >
    {{ t('shell.access.readOnlyBanner') }}
  </div>
</template>

<style scoped>
.read-only-banner {
  margin-bottom: var(--space-md);
  padding: var(--space-sm) var(--space-md);
  font-size: var(--font-size-sm);
  color: var(--rl-color-text);
  background: color-mix(in srgb, var(--rl-color-status-amber) 12%, transparent);
  border: 1px solid color-mix(in srgb, var(--rl-color-status-amber) 60%, transparent);
  border-radius: var(--radius-md);
}
</style>
