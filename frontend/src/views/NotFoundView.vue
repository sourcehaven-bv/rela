<script setup lang="ts">
/**
 * The page for an address no route matches (BUG-UZGMHIK), with or without a
 * space prefix. Before it, such an address rendered an empty pane. The link
 * goes to the root, which opens the home of the current space.
 */
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import RlEmptyState from 'rela-components/components/feedback/RlEmptyState.vue'
import PageLayout from '@/components/common/PageLayout.vue'
import PageTitle from '@/components/common/PageTitle.vue'

const { t } = useI18n()
</script>

<template>
  <PageLayout data-testid="not-found">
    <template #topbar>
      <PageTitle :title="t('notFound.title')" />
    </template>
    <RlEmptyState
      :title="t('notFound.title')"
      :description="t('notFound.description')"
      icon="search"
    >
      <template #actions>
        <RouterLink to="/" data-testid="not-found-home">{{ t('notFound.home') }}</RouterLink>
      </template>
    </RlEmptyState>
  </PageLayout>
</template>
