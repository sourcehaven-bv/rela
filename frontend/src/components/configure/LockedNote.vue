<script setup lang="ts">
import RlCallout from 'rela-components/components/feedback/RlCallout.vue'

/**
 * Says that part of what a screen shows cannot be changed here. Scripts,
 * commands and access rules run with the server's authority, so they are
 * changed in the project's files by someone who can run code there, never in
 * the browser.
 */
withDefaults(defineProps<{ title?: string }>(), { title: 'Shown, not editable here' })
</script>

<template>
  <RlCallout tone="info" :title="title" data-testid="config-locked">
    <slot>
      This runs a script or is part of access control. Change it in the project's files.
    </slot>
  </RlCallout>
</template>
