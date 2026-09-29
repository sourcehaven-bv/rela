<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import RlBackButton from 'rela-components/components/layout/RlBackButton.vue'
import { useSchemaStore } from '@/stores'
import { usePageStore } from '@/stores/pages'
import type { BackTarget } from '@/composables/useBackTarget'

const props = defineProps<{
  target: BackTarget
}>()

// Label resolution lives here (not in useBackTarget) so the composable
// stays generic — see TKT-JIEKC RR-RV4LA. When the hint carries a list id
// and schemaStore knows the list, we name it; otherwise the generic "Back"
// falls through. The arrow is the button's icon, so the label omits it.
const schemaStore = useSchemaStore()
const pageStore = usePageStore()
const label = computed(() => {
  const hint = props.target.labelHint
  if (hint?.kind === 'list') {
    const title = schemaStore.getList(hint.id)?.title
    if (title) return title
  }
  if (hint?.kind === 'page') {
    const title = pageStore.pages[hint.id]?.label
    if (title) return title
  }
  return 'Back'
})

</script>

<template>
  <!--
    RouterLink rather than an href: a real link keeps middle-click and
    "open in new tab" working, and letting the router render it avoids
    resolving the URL ourselves — which would make this component need a
    router that can resolve.
  -->
  <RlBackButton
    :as="RouterLink"
    :to="target.to"
    :label="label"
    data-testid="back-button"
  />
</template>
