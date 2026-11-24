<script setup lang="ts">
/**
 * The not-found message with a way back to the home page. The not-found page
 * shows it, and so do an entity page, a list and a form whose address names
 * nothing, so none of them shows a raw server error with a Retry that cannot
 * help.
 *
 * The not-found page names the page "Page not found" in its own title, so it
 * passes `untitled` and the state shows only the description and the link.
 */
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import RlEmptyState from 'rela-components/components/feedback/RlEmptyState.vue'

defineProps<{ untitled?: boolean }>()

const { t } = useI18n()
</script>

<template>
  <RlEmptyState
    :title="untitled ? undefined : t('notFound.title')"
    :description="t('notFound.description')"
    icon="search"
    data-testid="not-found-state"
  >
    <template #actions>
      <RouterLink to="/" data-testid="not-found-home">{{ t('notFound.home') }}</RouterLink>
    </template>
  </RlEmptyState>
</template>
