import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useGitStore } from './git'
import * as api from '@/api'

vi.mock('@/api', () => ({
  getGitStatus: vi.fn(),
  syncGit: vi.fn(),
}))

describe('Git Store', () => {
  let store: ReturnType<typeof useGitStore>

  beforeEach(() => {
    store = useGitStore()
    vi.clearAllMocks()
  })

  describe('initial state', () => {
    it('starts with null status', () => {
      expect(store.status).toBeNull()
      expect(store.loading).toBe(false)
      expect(store.syncing).toBe(false)
      expect(store.lastError).toBeNull()
    })

    it('computes defaults correctly when no status', () => {
      expect(store.isAvailable).toBe(false)
      expect(store.branch).toBe('')
      expect(store.localChanges).toBe(0)
      expect(store.remoteAhead).toBe(0)
      expect(store.hasConflicts).toBe(false)
      expect(store.conflictFiles).toEqual([])
      expect(store.statusText).toBe('')
      expect(store.statusClass).toBe('')
    })
  })

  describe('fetchStatus', () => {
    it('fetches and stores git status', async () => {
      const mockStatus = {
        available: true,
        branch: 'main',
        local_changes: 3,
        remote_ahead: 0,
        syncing: false,
        conflict: false,
        conflict_files: [],
      }
      vi.mocked(api.getGitStatus).mockResolvedValue(mockStatus)

      await store.fetchStatus()

      expect(api.getGitStatus).toHaveBeenCalledTimes(1)
      expect(store.status).toEqual(mockStatus)
      expect(store.loading).toBe(false)
      expect(store.lastError).toBeNull()
    })

    it('sets loading state during fetch', async () => {
      let resolvePromise: (value: api.GitStatus) => void
      vi.mocked(api.getGitStatus).mockReturnValue(
        new Promise((resolve) => {
          resolvePromise = resolve
        })
      )

      const fetchPromise = store.fetchStatus()
      expect(store.loading).toBe(true)

      resolvePromise!({
        available: true,
        branch: 'main',
        local_changes: 0,
        remote_ahead: 0,
        syncing: false,
        conflict: false,
        conflict_files: [],
      })
      await fetchPromise

      expect(store.loading).toBe(false)
    })

    it('handles fetch errors', async () => {
      vi.mocked(api.getGitStatus).mockRejectedValue(new Error('Network error'))

      await store.fetchStatus()

      expect(store.lastError).toBe('Network error')
      expect(store.loading).toBe(false)
    })
  })

  describe('computed getters', () => {
    beforeEach(async () => {
      vi.mocked(api.getGitStatus).mockResolvedValue({
        available: true,
        branch: 'feature/test',
        local_changes: 5,
        remote_ahead: 2,
        syncing: false,
        conflict: false,
        conflict_files: [],
      })
      await store.fetchStatus()
    })

    it('computes isAvailable', () => {
      expect(store.isAvailable).toBe(true)
    })

    it('computes branch', () => {
      expect(store.branch).toBe('feature/test')
    })

    it('computes localChanges', () => {
      expect(store.localChanges).toBe(5)
    })

    it('computes remoteAhead', () => {
      expect(store.remoteAhead).toBe(2)
    })

    it('computes statusText for changes', () => {
      expect(store.statusText).toBe('5 changes')
    })

    it('computes statusClass for changes', () => {
      expect(store.statusClass).toBe('changes')
    })
  })

  describe('statusText variations', () => {
    it('shows "1 change" for single change', async () => {
      vi.mocked(api.getGitStatus).mockResolvedValue({
        available: true,
        branch: 'main',
        local_changes: 1,
        remote_ahead: 0,
        syncing: false,
        conflict: false,
        conflict_files: [],
      })
      await store.fetchStatus()

      expect(store.statusText).toBe('1 change')
    })

    it('shows "X behind" when remote ahead', async () => {
      vi.mocked(api.getGitStatus).mockResolvedValue({
        available: true,
        branch: 'main',
        local_changes: 0,
        remote_ahead: 3,
        syncing: false,
        conflict: false,
        conflict_files: [],
      })
      await store.fetchStatus()

      expect(store.statusText).toBe('3 behind')
    })

    it('shows "Synced" when up to date', async () => {
      vi.mocked(api.getGitStatus).mockResolvedValue({
        available: true,
        branch: 'main',
        local_changes: 0,
        remote_ahead: 0,
        syncing: false,
        conflict: false,
        conflict_files: [],
      })
      await store.fetchStatus()

      expect(store.statusText).toBe('Synced')
      expect(store.statusClass).toBe('synced')
    })

    it('shows "Conflicts" when in conflict', async () => {
      vi.mocked(api.getGitStatus).mockResolvedValue({
        available: true,
        branch: 'main',
        local_changes: 0,
        remote_ahead: 0,
        syncing: false,
        conflict: true,
        conflict_files: ['file1.md', 'file2.md'],
      })
      await store.fetchStatus()

      expect(store.statusText).toBe('Conflicts')
      expect(store.statusClass).toBe('conflict')
      expect(store.hasConflicts).toBe(true)
      expect(store.conflictFiles).toEqual(['file1.md', 'file2.md'])
    })

    it('shows "Syncing..." when syncing', async () => {
      vi.mocked(api.getGitStatus).mockResolvedValue({
        available: true,
        branch: 'main',
        local_changes: 0,
        remote_ahead: 0,
        syncing: false,
        conflict: false,
        conflict_files: [],
      })
      await store.fetchStatus()

      // Manually set syncing state
      store.syncing = true

      expect(store.statusText).toBe('Syncing...')
    })
  })

  describe('sync', () => {
    it('syncs and refreshes status', async () => {
      const mockSyncResponse = {
        success: true,
        conflict_files: [],
      }
      vi.mocked(api.syncGit).mockResolvedValue(mockSyncResponse)
      vi.mocked(api.getGitStatus).mockResolvedValue({
        available: true,
        branch: 'main',
        local_changes: 0,
        remote_ahead: 0,
        syncing: false,
        conflict: false,
        conflict_files: [],
      })

      const result = await store.sync()

      expect(api.syncGit).toHaveBeenCalledTimes(1)
      expect(api.getGitStatus).toHaveBeenCalledTimes(1)
      expect(result).toEqual(mockSyncResponse)
      expect(store.syncing).toBe(false)
    })

    it('sets syncing state during sync', async () => {
      let resolveSync: (value: api.GitSyncResponse) => void
      vi.mocked(api.syncGit).mockReturnValue(
        new Promise((resolve) => {
          resolveSync = resolve
        })
      )
      vi.mocked(api.getGitStatus).mockResolvedValue({
        available: true,
        branch: 'main',
        local_changes: 0,
        remote_ahead: 0,
        syncing: false,
        conflict: false,
        conflict_files: [],
      })

      const syncPromise = store.sync()
      expect(store.syncing).toBe(true)

      resolveSync!({ success: true, conflict_files: [] })
      await syncPromise

      expect(store.syncing).toBe(false)
    })

    it('handles sync errors', async () => {
      vi.mocked(api.syncGit).mockRejectedValue(new Error('Sync failed'))

      await expect(store.sync()).rejects.toThrow('Sync failed')

      expect(store.lastError).toBe('Sync failed')
      expect(store.syncing).toBe(false)
    })
  })
})
