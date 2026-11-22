<script setup lang="ts">
/**
 * The not-found message with a way back to the home page. The not-found page
 * shows it, and so do an entity page, a list and a form whose address names
 * nothing, so none of them shows a raw server error with a Retry that cannot
 * help.
 */
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import RlEmptyState from 'rela-components/components/feedback/RlEmptyState.vue'

const { t } = useI18n()
</script>

<template>
  <RlEmptyState
    :title="t('notFound.title')"
    :description="t('notFound.description')"
    icon="search"
    data-testid="not-found-state"
  >
    <template #actions>
      <RouterLink to="/" data-testid="not-found-home">{{ t('notFound.home') }}</RouterLink>
    </template>
  </RlEmptyState>
</template>
