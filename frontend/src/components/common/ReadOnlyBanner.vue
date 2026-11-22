<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { workspaceReadOnly } from '@/api/workspaceReadOnly'
import { bootMeta } from '@/i18n/bootMeta'

/**
 * Says that the workspace is read-only, while it is (rela#2511). The create
 * buttons are hidden then, so without this a user would not know why. The app
 * puts it in the shell's full-width `banner` slot, so it is a strip with a
 * rule below rather than a card.
 *
 * `account.read_only_text` replaces the default text. The server writes it
 * into the shell (`bootMeta`), like the suspended and archived page texts.
 */
const { t } = useI18n()
const text = bootMeta('rela-read-only-text')
</script>

<template>
  <div
    v-if="workspaceReadOnly"
    class="read-only-banner"
    role="status"
    data-testid="read-only-banner"
  >
    {{ text || t('shell.access.readOnlyBanner') }}
  </div>
</template>

<style scoped>
.read-only-banner {
  padding: var(--space-sm) var(--space-md);
  font-size: var(--font-size-sm);
  color: var(--rl-color-text);
  background: color-mix(in srgb, var(--rl-color-status-amber) 12%, var(--rl-color-bg));
  border-bottom: 1px solid color-mix(in srgb, var(--rl-color-status-amber) 60%, transparent);
}
</style>
