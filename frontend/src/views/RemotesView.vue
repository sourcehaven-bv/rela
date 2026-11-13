<script setup lang="ts">
/**
 * The remotes page (TKT-0FMYPEZ): every remote with its run health and the
 * reader's gated conflict counts. Sync runs on the scheduler; the page shows
 * the next run and starts none.
 */
import { onBeforeUnmount, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import RlStatusPill from 'rela-components/components/common/RlStatusPill.vue'
import RlCallout from 'rela-components/components/feedback/RlCallout.vue'
import PageLayout from '@/components/common/PageLayout.vue'
import PageTitle from '@/components/common/PageTitle.vue'
import { listRemotes, type RemoteSummary } from '@/api/remotes'
import { getErrorMessage, shouldDropHeldContent } from '@/api/errors'
import { isCancelledFetch } from '@/composables/usePageData'
import { useSpaceStore } from '@/stores/space'
import { formatInstant } from '@/components/flows/flowView'
import {
  REMOTE_REASONS,
  isPermissionRequired,
  reasonLabel,
  runState,
} from '@/components/remotes/remoteView'

const { t } = useI18n()
const space = useSpaceStore()

const remotes = ref<RemoteSummary[] | null>(null)
const error = ref('')
let controller: AbortController | null = null

async function load() {
  controller?.abort()
  const ctl = new AbortController()
  controller = ctl
  try {
    remotes.value = await listRemotes(ctl.signal)
    error.value = ''
  } catch (err) {
    if (isCancelledFetch(err)) return
    if (shouldDropHeldContent(err)) remotes.value = null
    error.value = isPermissionRequired(err)
      ? t('remotes.needPermission')
      : getErrorMessage(err, t('remotes.loadFailed'))
  }
}

void load()
onBeforeUnmount(() => controller?.abort())

function conflictHref(name: string): string {
  return space.href(`/remotes/${encodeURIComponent(name)}`)
}

function reasonCounts(r: RemoteSummary): string {
  return REMOTE_REASONS.filter((k) => (r.conflicts.by_reason[k] ?? 0) > 0)
    .map((k) => `${reasonLabel(k, t)}: ${r.conflicts.by_reason[k]}`)
    .join(', ')
}
</script>

<template>
  <PageLayout :data-testid="remotes ? 'remotes-loaded' : 'remotes-loading'">
    <template #topbar>
      <PageTitle :title="t('remotes.title')" />
    </template>

    <RlCallout v-if="error" tone="danger" class="remotes__error" data-testid="remotes-error">
      {{ error }}
    </RlCallout>

    <template v-if="remotes">
      <p v-if="remotes.length === 0" class="remotes__muted">{{ t('remotes.empty') }}</p>
      <table v-else class="remotes__table">
        <thead>
          <tr>
            <th scope="col">{{ t('remotes.name') }}</th>
            <th scope="col">{{ t('remotes.status') }}</th>
            <th scope="col">{{ t('remotes.lastRun') }}</th>
            <th scope="col">{{ t('remotes.nextRun') }}</th>
            <th scope="col">{{ t('remotes.push') }}</th>
            <th scope="col">{{ t('remotes.conflictRule') }}</th>
            <th scope="col">{{ t('remotes.conflicts') }}</th>
            <th scope="col">{{ t('remotes.linked') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in remotes" :key="r.name" data-testid="remote-row">
            <td>
              <RouterLink :to="conflictHref(r.name)">{{ r.name }}</RouterLink>
              <div class="remotes__muted remotes__url">{{ r.url }}</div>
              <div v-if="r.credential_kind" class="remotes__muted">
                {{ t('remotes.credential') }}: {{ r.credential_kind }}
              </div>
            </td>
            <td>
              <RlStatusPill :label="runState(r, t).label" :color="runState(r, t).color" />
              <div v-if="r.next_retry" class="remotes__muted">
                {{ t('remotes.nextRetry', { at: formatInstant(r.next_retry) }) }}
              </div>
            </td>
            <td>{{ r.last_run ? formatInstant(r.last_run) : t('remotes.never') }}</td>
            <td>{{ r.next_run ? formatInstant(r.next_run) : t('remotes.noScheduledRun') }}</td>
            <td>{{ r.push ? t('remotes.on') : t('remotes.off') }}</td>
            <td>{{ r.conflict }}</td>
            <td data-testid="remote-conflict-total">
              <RouterLink v-if="r.conflicts.total > 0" :to="conflictHref(r.name)">
                {{ r.conflicts.total }}
              </RouterLink>
              <span v-else>{{ r.conflicts.total }}</span>
              <div v-if="r.conflicts.total > 0" class="remotes__muted">{{ reasonCounts(r) }}</div>
            </td>
            <td>{{ r.linked }}</td>
          </tr>
        </tbody>
      </table>
    </template>
  </PageLayout>
</template>

<style scoped src="../components/remotes/remotes.css"></style>
