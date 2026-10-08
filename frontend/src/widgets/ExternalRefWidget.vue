<script setup lang="ts">
// Renders an `external_ref` property (TKT-SM20FG). Read-only in both modes:
// the server refuses an external ref from the data-entry surface, so this
// widget never emits `update:modelValue` and the form never sends the value.
import { computed } from 'vue'
import type { WidgetProps } from './types'
import { externalRefHref, parseExternalRef } from './externalRef'

const props = defineProps<WidgetProps>()

const extRef = computed(() => parseExternalRef(props.modelValue))
const href = computed(() => externalRefHref(extRef.value))
const system = computed(() => props.propertyDef?.system)
</script>

<template>
  <span :id="id" class="display-value external-ref" :aria-describedby="describedBy">
    <template v-if="extRef">
      <a v-if="href" :href="href" target="_blank" rel="noopener noreferrer">{{ extRef.id }}</a>
      <span v-else>{{ extRef.id }}</span>
    </template>
    <span v-else-if="mode === 'display'">-</span>
    <span v-if="mode === 'edit'" class="external-ref-note">
      {{ system ? `Set by the ${system} sync` : 'Set by a sync' }}
    </span>
  </span>
</template>

<style scoped>
.external-ref-note {
  margin-left: var(--rl-space-2, 0.5rem);
  color: var(--rl-text-muted, inherit);
  font-size: 0.875em;
}
</style>
