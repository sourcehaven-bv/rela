<script setup lang="ts">
/**
 * The page for an address no route matches (BUG-UZGMHIK), with or without a
 * space prefix, and for a list or form that the configuration does not have.
 * Before it, such an address rendered an empty pane. The link goes to the
 * root, which opens the home of the current space.
 */
import { useI18n } from 'vue-i18n'
import PageLayout from '@/components/common/PageLayout.vue'
import PageTitle from '@/components/common/PageTitle.vue'
import NotFoundState from '@/components/common/NotFoundState.vue'

const { t } = useI18n()
</script>

<template>
  <PageLayout data-testid="not-found">
    <template #topbar>
      <PageTitle :title="t('notFound.title')" />
    </template>
    <NotFoundState />
  </PageLayout>
</template>
