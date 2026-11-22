<script setup lang="ts">
/**
 * The page for an address no route matches (BUG-UZGMHIK), with or without a
 * space prefix. Before it, such an address rendered an empty pane. The link
 * goes to the root, which opens the home of the current space.
 */
import NotFoundPage from '@/components/common/NotFoundPage.vue'
</script>

<template>
  <NotFoundPage />
</template>
