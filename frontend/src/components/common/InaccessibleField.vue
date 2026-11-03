<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

// InaccessibleField renders the lock affordance when a property's value
// is unreadable -- most commonly because the entity is git-crypt
// encrypted and cannot be decrypted in the current context.
//
// Single owner of the affordance, so all view-side display modes
// (properties, cards, list) consume the same shape (RR-UD1F).
const props = defineProps<{
  reason?: string
}>()

const { t } = useI18n()

const tooltip = computed(() => {
  if (props.reason === 'git-crypt') {
    return t('shell.inaccessible.gitCrypt')
  }
  if (props.reason) {
    return t('shell.inaccessible.reason', { reason: props.reason })
  }
  return t('shell.inaccessible.label')
})
</script>

<template>
  <span class="property-inaccessible" :title="tooltip">🔒 {{ t('shell.inaccessible.label') }}</span>
</template>

<style scoped>
.property-inaccessible {
  color: var(--rl-color-text-muted);
  font-style: italic;
  cursor: help;
}
</style>
