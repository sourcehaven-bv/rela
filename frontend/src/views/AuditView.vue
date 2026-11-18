<script setup lang="ts">
/**
 * The global audit log (TKT-X1ZFO04): every record, newest first, for a
 * holder of audit:read. The server serves the records whole; the permission
 * is the boundary. Without it the server answers 403 and this shows why.
 */
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlCallout from 'rela-components/components/feedback/RlCallout.vue'
import PageLayout from '@/components/common/PageLayout.vue'
import PageTitle from '@/components/common/PageTitle.vue'
import { getAuditLog, describeSubject, type AuditEntry } from '@/api/audit'
import { ApiError, getErrorMessage, shouldDropHeldContent } from '@/api/errors'
import { isCancelledFetch } from '@/composables/usePageData'
import { formatInstant } from '@/components/flows/flowView'

const { t } = useI18n()

const records = ref<AuditEntry[]>([])
const nextBefore = ref<number | undefined>()
const loaded = ref(false)
const forbidden = ref(false)
const loadingMore = ref(false)
const error = ref('')
let controller: AbortController | null = null

async function load(before?: number) {
  controller?.abort()
  const ctl = new AbortController()
  controller = ctl
  try {
    const page = await getAuditLog({ before }, ctl.signal)
    const got = page.records ?? []
    records.value = before ? [...records.value, ...got] : got
    nextBefore.value = page.next_before || undefined
    loaded.value = true
    forbidden.value = false
    error.value = ''
  } catch (err) {
    if (isCancelledFetch(err)) return
    if (shouldDropHeldContent(err)) {
      records.value = []
      nextBefore.value = undefined
      loaded.value = false
    }
    if (err instanceof ApiError && err.status === 403) {
      forbidden.value = true
      error.value = ''
      return
    }
    error.value = getErrorMessage(err)
  }
}

async function loadMore() {
  loadingMore.value = true
  try {
    await load(nextBefore.value)
  } finally {
    loadingMore.value = false
  }
}

onMounted(() => void load())
onBeforeUnmount(() => controller?.abort())
</script>

<template>
  <PageLayout :data-testid="loaded ? 'audit-loaded' : 'audit-loading'">
    <template #topbar>
      <PageTitle :title="t('audit.title')" />
    </template>

    <RlCallout v-if="forbidden" tone="info" data-testid="audit-forbidden">
      {{ t('audit.forbidden') }}
    </RlCallout>
    <RlCallout v-else-if="error" tone="danger">{{ error }}</RlCallout>

    <template v-if="loaded">
      <p v-if="records.length === 0" class="audit-empty">{{ t('audit.empty') }}</p>
      <table v-else class="audit-table">
        <thead>
          <tr>
            <th scope="col">{{ t('audit.col.time') }}</th>
            <th scope="col">{{ t('audit.col.user') }}</th>
            <th scope="col">{{ t('audit.col.op') }}</th>
            <th scope="col">{{ t('audit.col.subject') }}</th>
            <th scope="col">{{ t('audit.col.summary') }}</th>
            <th scope="col">{{ t('audit.col.trigger') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="e in records" :key="e.seq" data-testid="audit-row">
            <td>
              <time :datetime="e.record.time">{{ formatInstant(e.record.time) }}</time>
            </td>
            <td>{{ e.record.principal?.user || e.record.principal?.tool }}</td>
            <td>{{ e.record.op }}</td>
            <td>{{ describeSubject(e.record) }}</td>
            <td>{{ e.record.summary }}</td>
            <td>{{ e.record.triggered_by }}</td>
          </tr>
        </tbody>
      </table>
      <div v-if="nextBefore" class="audit-more">
        <RlButton
          variant="secondary"
          size="sm"
          :loading="loadingMore"
          :pending-label="t('common.loading')"
          @click="loadMore"
        >
          {{ t('audit.loadMore') }}
        </RlButton>
      </div>
    </template>
  </PageLayout>
</template>

<style scoped>
.audit-table {
  width: 100%;
  border-collapse: collapse;
  font-size: var(--font-size-sm);
}

.audit-table th,
.audit-table td {
  text-align: left;
  padding: 6px 8px;
  border-bottom: 1px solid var(--rl-color-border);
  vertical-align: top;
}

.audit-empty {
  color: var(--rl-color-text-muted);
}

.audit-more {
  margin-top: 12px;
}
</style>
