<script setup lang="ts">
import { computed } from 'vue'

// InaccessibleField renders the lock affordance when a property's value
// is unreadable -- most commonly because the entity is git-crypt
// encrypted and cannot be decrypted in the current context.
//
// Single owner of the affordance, so all view-side display modes
// (properties, cards, list) consume the same shape (RR-UD1F).
const props = defineProps<{
  reason?: string
}>()

const tooltip = computed(() => {
  if (props.reason === 'git-crypt') {
    return 'git-crypt encrypted (run `git-crypt unlock` to read)'
  }
  if (props.reason) {
    return `inaccessible (${props.reason})`
  }
  return 'inaccessible'
})
</script>

<template>
  <span class="property-inaccessible" :title="tooltip">🔒 inaccessible</span>
</template>

<style scoped>
.property-inaccessible {
  color: var(--muted-text);
  font-style: italic;
  cursor: help;
}
</style>
