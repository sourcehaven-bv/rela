import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { getGitStatus, syncGit, type GitStatus, type GitSyncResponse } from '@/api'
import { getErrorMessage } from '@/api/errors'
import { isCancelledFetch } from '@/composables/usePageData'
import { t } from '@/i18n'

export const useGitStore = defineStore('git', () => {
  // State
  const status = ref<GitStatus | null>(null)
  const loading = ref(false)
  const syncing = ref(false)
  const lastError = ref<string | null>(null)

  // Getters
  const isAvailable = computed(() => status.value?.available ?? false)
  const branch = computed(() => status.value?.branch ?? '')
  const localChanges = computed(() => status.value?.local_changes ?? 0)
  const remoteAhead = computed(() => status.value?.remote_ahead ?? 0)
  const hasConflicts = computed(() => status.value?.conflict ?? false)
  const conflictFiles = computed(() => status.value?.conflict_files ?? [])
  const canSync = computed(() => status.value?.can_sync ?? false)

  const statusText = computed(() => {
    if (!isAvailable.value) return ''
    if (syncing.value) return t('git.syncing')
    if (hasConflicts.value) return t('git.conflicts')
    if (localChanges.value > 0) return t('git.changes', { n: localChanges.value }, localChanges.value)
    if (remoteAhead.value > 0) return t('git.behind', { n: remoteAhead.value })
    return t('git.synced')
  })

  const statusClass = computed(() => {
    if (!isAvailable.value) return ''
    if (hasConflicts.value) return 'conflict'
    if (localChanges.value > 0 || remoteAhead.value > 0) return 'changes'
    return 'synced'
  })

  // Actions
  async function fetchStatus() {
    loading.value = true
    lastError.value = null
    try {
      status.value = await getGitStatus()
    } catch (err) {
      // Suppress cancellation errors from rapid navigation in Firefox
      // (see BUG-6C3V and src/composables/usePageData.ts).
      if (isCancelledFetch(err)) return
      console.error('Failed to fetch git status:', err)
      lastError.value = getErrorMessage(err, t('git.fetchFailed'))
    } finally {
      loading.value = false
    }
  }

  async function sync(): Promise<GitSyncResponse> {
    syncing.value = true
    lastError.value = null
    try {
      const result = await syncGit()
      // Refresh status after sync
      await fetchStatus()
      return result
    } catch (err) {
      console.error('Failed to sync:', err)
      lastError.value = getErrorMessage(err, t('git.syncFailed'))
      throw err
    } finally {
      syncing.value = false
    }
  }

  return {
    // State
    status,
    loading,
    syncing,
    lastError,

    // Getters
    isAvailable,
    branch,
    localChanges,
    remoteAhead,
    hasConflicts,
    conflictFiles,
    canSync,
    statusText,
    statusClass,

    // Actions
    fetchStatus,
    sync,
  }
})
