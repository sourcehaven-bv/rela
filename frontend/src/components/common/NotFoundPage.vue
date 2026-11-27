<script setup lang="ts">
/**
 * The full not-found page: page layout, title and the not-found state. The
 * catch-all route and a list that the configuration does not have both
 * render it (BUG-UZGMHIK, BUG-JT865RH). It lives among the components so the
 * list need not import a view.
 */
import { useI18n } from 'vue-i18n'
import PageLayout from '@/components/common/PageLayout.vue'
import PageTitle from '@/components/common/PageTitle.vue'
import NotFoundState from '@/components/feedback/NotFoundState.vue'

const { t } = useI18n()
</script>

<template>
  <PageLayout data-testid="not-found">
    <template #topbar>
      <PageTitle :title="t('notFound.title')" />
    </template>
    <NotFoundState untitled />
  </PageLayout>
</template>
