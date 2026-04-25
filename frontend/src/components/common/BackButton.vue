<script setup lang="ts">
import { computed } from 'vue'
import { useSchemaStore } from '@/stores'
import type { BackTarget } from '@/composables/useBackTarget'

const props = defineProps<{
  target: BackTarget
}>()

// Label resolution lives here (not in useBackTarget) so the composable
// stays generic — see TKT-JIEKC RR-RV4LA. When the hint carries a list id
// and schemaStore knows the list, we render "← <list title>"; otherwise
// the generic "← Back" falls through.
const schemaStore = useSchemaStore()
const label = computed(() => {
  const hint = props.target.labelHint
  if (hint?.kind === 'list') {
    const title = schemaStore.getList(hint.id)?.title
    if (title) return `← ${title}`
  }
  return '← Back'
})
</script>

<template>
  <router-link
    :to="target.to"
    class="scope-nav-btn"
    data-testid="back-button"
  >
    {{ label }}
  </router-link>
</template>
