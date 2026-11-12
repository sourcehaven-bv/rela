<script setup lang="ts">
/**
 * One remote's conflict list (TKT-0FMYPEZ), paged on the server. Every entry
 * has passed the reader's row and field gate. A control shows only when the
 * entry's `_actions` allows it; the server checks the POST again.
 */
import { onBeforeUnmount, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import RlPagination from 'rela-components/components/data/RlPagination.vue'
import RlCallout from 'rela-components/components/feedback/RlCallout.vue'
import PageLayout from '@/components/common/PageLayout.vue'
import PageTitle from '@/components/common/PageTitle.vue'
import {
  dismissRemoteEntry,
  linkRemoteEntry,
  listRemoteConflicts,
  resolveRemoteEntry,
  type RemoteConflictPage,
  type RemoteEntry,
} from '@/api/remotes'
import { ApiError, getErrorMessage, shouldDropHeldContent } from '@/api/errors'
import { isCancelledFetch } from '@/composables/usePageData'
import { useConfirm } from '@/composables/useConfirm'
import { useSpaceStore } from '@/stores/space'
import { useUIStore } from '@/stores/ui'
import { formatInstant } from '@/components/flows/flowView'
import {
  causeLabel,
  entryHref,
  entryKey,
  fallbackLabel,
  formatValue,
  isPermissionRequired,
  reasonLabel,
} from '@/components/remotes/remoteView'

const props = defineProps<{ name: string }>()

const PER_PAGE = 25

const { t } = useI18n()
const space = useSpaceStore()
const uiStore = useUIStore()
const { confirm } = useConfirm()

const result = ref<RemoteConflictPage | null>(null)
const error = ref('')
const page = ref(1)
/** The entry and action in flight, as `key|action`. */
const busy = ref('')
/** The entry whose link form is open, and the address typed into it. */
const linking = ref('')
const address = ref('')
let controller: AbortController | null = null

function loadError(err: unknown): string {
  if (isPermissionRequired(err)) return t('remotes.needPermission')
  if (err instanceof ApiError && err.status === 404)
    return t('remotes.list.unknown', { name: props.name })
  return getErrorMessage(err, t('remotes.list.loadFailed'))
}

async function load() {
  controller?.abort()
  const ctl = new AbortController()
  controller = ctl
  try {
    const got = await listRemoteConflicts(props.name, page.value, PER_PAGE, ctl.signal)
    error.value = ''
    // An action that empties the last page leaves it past the end: step back
    // to the new last page, which the watch then loads.
    const last = pageCount(got.total)
    if (got.data.length === 0 && page.value > last) {
      page.value = last
      return
    }
    result.value = got
  } catch (err) {
    if (isCancelledFetch(err)) return
    if (shouldDropHeldContent(err)) result.value = null
    error.value = loadError(err)
  }
}

watch(() => [props.name, page.value], load, { immediate: true })
onBeforeUnmount(() => controller?.abort())

function pageCount(total: number): number {
  return Math.max(1, Math.ceil(total / PER_PAGE))
}

function isBusy(e: RemoteEntry, action: string): boolean {
  return busy.value === `${entryKey(e)}|${action}`
}

async function run(e: RemoteEntry, action: string, op: () => Promise<unknown>) {
  if (busy.value) return
  busy.value = `${entryKey(e)}|${action}`
  try {
    await op()
    uiStore.success(t('remotes.list.done'))
    linking.value = ''
    address.value = ''
    await load()
  } catch (err) {
    const busyRemote = err instanceof ApiError && err.status === 409 && err.code === 'remote_busy'
    uiStore.error(
      busyRemote ? t('remotes.list.busy') : getErrorMessage(err, t('remotes.list.actionFailed'))
    )
  } finally {
    busy.value = ''
  }
}

function key(e: RemoteEntry) {
  return { local: e.local, field: e.field }
}

async function keepLocal(e: RemoteEntry) {
  // Keeping the local side of an uncertain create sends the row again, which
  // makes a second copy when the first create did reach the remote.
  if (e.reason === 'uncertain') {
    const ok = await confirm({
      title: t('remotes.list.sendAgainTitle'),
      message: t('remotes.list.sendAgainMessage'),
      confirmLabel: t('remotes.list.sendAgain'),
      danger: true,
    })
    if (!ok) return
  }
  await run(e, 'local', () => resolveRemoteEntry(props.name, key(e), 'local'))
}

function takeRemote(e: RemoteEntry) {
  return run(e, 'remote', () => resolveRemoteEntry(props.name, key(e), 'remote'))
}

function dismiss(e: RemoteEntry) {
  return run(e, 'dismiss', () => dismissRemoteEntry(props.name, key(e)))
}

function openLink(e: RemoteEntry) {
  linking.value = linking.value === entryKey(e) ? '' : entryKey(e)
  address.value = ''
}

function link(e: RemoteEntry) {
  const target = address.value.trim()
  if (!target) return
  return run(e, 'link', () => linkRemoteEntry(props.name, e.local, target))
}
</script>

<template>
  <PageLayout :data-testid="result ? 'remote-conflicts-loaded' : 'remote-conflicts-loading'">
    <template #topbar>
      <PageTitle :title="t('remotes.list.title', { name })" />
    </template>

    <div class="remotes__back">
      <RouterLink :to="space.href('/remotes')">{{ t('remotes.list.back') }}</RouterLink>
    </div>

    <RlCallout
      v-if="error"
      tone="danger"
      class="remotes__error"
      data-testid="remote-conflicts-error"
    >
      {{ error }}
    </RlCallout>

    <template v-if="result">
      <p v-if="result.data.length === 0" class="remotes__muted">{{ t('remotes.list.empty') }}</p>
      <table v-else class="remotes__table">
        <thead>
          <tr>
            <th scope="col">{{ t('remotes.list.row') }}</th>
            <th scope="col">{{ t('remotes.list.field') }}</th>
            <th scope="col">{{ t('remotes.list.reasonCol') }}</th>
            <th scope="col">{{ t('remotes.list.remoteValue') }}</th>
            <th scope="col">{{ t('remotes.list.since') }}</th>
            <th scope="col">{{ t('remotes.list.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="e in result.data" :key="entryKey(e)" data-testid="remote-entry">
            <td>
              <RouterLink v-if="entryHref(e)" :to="space.href(entryHref(e))">{{
                e.local
              }}</RouterLink>
              <span v-else>{{ e.local }}</span>
              <div class="remotes__muted">{{ e.type }}</div>
              <div v-if="e.address" class="remotes__muted">{{ e.address }}</div>
            </td>
            <td>
              <template v-if="e.field">{{ e.field }}</template>
              <template v-else>
                {{ t('remotes.list.wholeRow') }}
                <div v-if="e.fields?.length" class="remotes__muted">{{ e.fields.join(', ') }}</div>
              </template>
            </td>
            <td>
              <div data-testid="remote-entry-reason">{{ reasonLabel(e.reason, t) }}</div>
              <div v-if="e.cause" class="remotes__muted">{{ causeLabel(e.cause, t) }}</div>
              <div v-if="e.status" class="remotes__muted">
                {{ t('remotes.list.status', { status: e.status }) }}
              </div>
              <div v-if="e.fallback" class="remotes__muted">{{ fallbackLabel(e.fallback, t) }}</div>
              <template v-if="e.superseded">
                <div class="remotes__muted">
                  {{
                    t(
                      e.superseded.winner === 'remote'
                        ? 'remotes.list.winner.remote'
                        : 'remotes.list.winner.local'
                    )
                  }}
                </div>
                <div
                  v-if="!e.superseded.lost_value_kept"
                  class="remotes__muted"
                  data-testid="remote-entry-not-kept"
                >
                  {{ t('remotes.list.lostNotKept') }}
                </div>
                <div v-else-if="e.superseded.local_value_omitted" class="remotes__muted">
                  {{ t('remotes.list.valueOmitted') }}
                </div>
                <div v-else class="remotes__muted remotes__value">
                  {{ t('remotes.list.lostKept', { value: formatValue(e.superseded.local_value) }) }}
                </div>
              </template>
            </td>
            <td class="remotes__value">
              <span v-if="e.remote_value_omitted" class="remotes__muted">{{
                t('remotes.list.valueOmitted')
              }}</span>
              <span v-else-if="e.remote_value === undefined" class="remotes__muted">{{
                t('remotes.list.noValue')
              }}</span>
              <span v-else>{{ formatValue(e.remote_value) }}</span>
            </td>
            <td>{{ formatInstant(e.since) }}</td>
            <td>
              <div class="remotes__actions">
                <RlButton
                  v-if="e._actions.resolve_local"
                  size="sm"
                  variant="secondary"
                  :loading="isBusy(e, 'local')"
                  :pending-label="t('remotes.list.keepingLocal')"
                  data-testid="remote-keep-local"
                  @click="keepLocal(e)"
                >
                  {{ t('remotes.list.keepLocal') }}
                </RlButton>
                <RlButton
                  v-if="e._actions.resolve_remote"
                  size="sm"
                  variant="secondary"
                  :loading="isBusy(e, 'remote')"
                  :pending-label="t('remotes.list.takingRemote')"
                  data-testid="remote-take-remote"
                  @click="takeRemote(e)"
                >
                  {{ t('remotes.list.takeRemote') }}
                </RlButton>
                <RlButton
                  v-if="e._actions.dismiss"
                  size="sm"
                  variant="secondary"
                  :loading="isBusy(e, 'dismiss')"
                  :pending-label="t('remotes.list.dismissing')"
                  data-testid="remote-dismiss"
                  @click="dismiss(e)"
                >
                  {{ t('remotes.list.dismiss') }}
                </RlButton>
                <RlButton
                  v-if="e._actions.link"
                  size="sm"
                  variant="secondary"
                  data-testid="remote-link-open"
                  @click="openLink(e)"
                >
                  {{ t('remotes.list.link') }}
                </RlButton>
              </div>
              <form
                v-if="e._actions.link && linking === entryKey(e)"
                class="remotes__link-form"
                @submit.prevent="link(e)"
              >
                <RlTextField
                  v-model="address"
                  :label="t('remotes.list.address')"
                  data-testid="remote-link-address"
                />
                <RlButton
                  size="sm"
                  type="submit"
                  :loading="isBusy(e, 'link')"
                  :pending-label="t('remotes.list.linking')"
                  data-testid="remote-link-submit"
                >
                  {{ t('remotes.list.link') }}
                </RlButton>
                <RlButton size="sm" variant="secondary" @click="openLink(e)">{{
                  t('remotes.list.cancel')
                }}</RlButton>
              </form>
            </td>
          </tr>
        </tbody>
      </table>
      <RlPagination
        v-if="pageCount(result.total) > 1"
        :page="page"
        :page-count="pageCount(result.total)"
        :summary="t('remotes.list.summary', { n: result.total })"
        @update:page="page = $event"
      />
    </template>
  </PageLayout>
</template>

<style scoped src="../components/remotes/remotes.css"></style>
