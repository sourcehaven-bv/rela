import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory, type Router } from 'vue-router'
import { setActivePinia, createPinia } from 'pinia'
import StatusBar from './StatusBar.vue'
import { useGitStore } from '@/stores'

// Mock the shortcutsModalOpen
vi.mock('@/composables/useKeyboardShortcuts', () => ({
  shortcutsModalOpen: { value: false },
}))

describe('StatusBar', () => {
  let router: Router
  let gitStore: ReturnType<typeof useGitStore>

  beforeEach(() => {
    setActivePinia(createPinia())

    // Use memory history with initial location to avoid timing issues
    router = createRouter({
      history: createMemoryHistory('/'),
      routes: [
        { path: '/', name: 'home', component: { template: '<div/>' } },
        { path: '/settings', name: 'settings', component: { template: '<div/>' } },
        { path: '/conflicts', name: 'conflicts', component: { template: '<div/>' } },
      ],
    })

    gitStore = useGitStore()
    // Mock fetchStatus to avoid timing issues
    vi.spyOn(gitStore, 'fetchStatus').mockResolvedValue()
  })

  async function mountStatusBar() {
    const wrapper = mount(StatusBar, {
      global: {
        plugins: [router],
      },
    })
    await router.isReady()
    return wrapper
  }

  describe('git status display', () => {
    it('shows git status when available', async () => {
      gitStore.status = {
        available: true,
        branch: 'main',
        local_changes: 3,
        remote_ahead: 0,
        syncing: false,
        conflict: false,
        conflict_files: [],
      }

      const wrapper = await mountStatusBar()
      await flushPromises()

      expect(wrapper.find('.git-branch').text()).toBe('main')
      expect(wrapper.find('.git-status-text').text()).toBe('3 changes')
    })

    it('hides git status when not available', async () => {
      gitStore.status = {
        available: false,
        branch: '',
        local_changes: 0,
        remote_ahead: 0,
        syncing: false,
        conflict: false,
        conflict_files: [],
      }

      const wrapper = await mountStatusBar()
      await flushPromises()

      expect(wrapper.find('.git-status').exists()).toBe(false)
    })

    it('shows synced status', async () => {
      gitStore.status = {
        available: true,
        branch: 'develop',
        local_changes: 0,
        remote_ahead: 0,
        syncing: false,
        conflict: false,
        conflict_files: [],
      }

      const wrapper = await mountStatusBar()
      await flushPromises()

      expect(wrapper.find('.git-status-text').text()).toBe('Synced')
      expect(wrapper.find('.git-status').classes()).toContain('synced')
    })

    it('shows changes status with correct class', async () => {
      gitStore.status = {
        available: true,
        branch: 'main',
        local_changes: 5,
        remote_ahead: 0,
        syncing: false,
        conflict: false,
        conflict_files: [],
      }

      const wrapper = await mountStatusBar()
      await flushPromises()

      expect(wrapper.find('.git-status').classes()).toContain('changes')
    })

    it('shows conflict status and link', async () => {
      gitStore.status = {
        available: true,
        branch: 'main',
        local_changes: 0,
        remote_ahead: 0,
        syncing: false,
        conflict: true,
        conflict_files: ['file1.md'],
      }

      const wrapper = await mountStatusBar()
      await flushPromises()

      expect(wrapper.find('.git-status-text').text()).toBe('Conflicts')
      expect(wrapper.find('.git-status').classes()).toContain('conflict')
      expect(wrapper.find('.status-warning').exists()).toBe(true)
      expect(wrapper.find('.status-warning').text()).toContain('Resolve Conflicts')
    })
  })

  describe('sync action', () => {
    it('calls sync when git status is clicked', async () => {
      gitStore.status = {
        available: true,
        branch: 'main',
        local_changes: 3,
        remote_ahead: 0,
        syncing: false,
        conflict: false,
        conflict_files: [],
      }
      const syncSpy = vi.spyOn(gitStore, 'sync').mockResolvedValue({
        success: true,
        conflict_files: [],
      })

      const wrapper = await mountStatusBar()
      await flushPromises()

      await wrapper.find('.git-status .status-item').trigger('click')
      await flushPromises()

      expect(syncSpy).toHaveBeenCalled()
    })

    it('navigates to conflicts on sync with conflicts', async () => {
      gitStore.status = {
        available: true,
        branch: 'main',
        local_changes: 3,
        remote_ahead: 0,
        syncing: false,
        conflict: false,
        conflict_files: [],
      }
      vi.spyOn(gitStore, 'sync').mockResolvedValue({
        success: true,
        conflict_files: ['file1.md', 'file2.md'],
      })
      const pushSpy = vi.spyOn(router, 'push')

      const wrapper = await mountStatusBar()
      await flushPromises()

      await wrapper.find('.git-status .status-item').trigger('click')
      await flushPromises()

      expect(pushSpy).toHaveBeenCalledWith('/conflicts')
    })
  })

  describe('navigation links', () => {
    it('renders settings link', async () => {
      const wrapper = await mountStatusBar()
      await flushPromises()

      // RouterLink renders with router-link-* classes, find by text content
      const items = wrapper.findAll('.status-item')
      const settingsLink = items.find((el) => el.text().includes('Settings'))
      expect(settingsLink).toBeDefined()
    })

    it('renders keyboard shortcuts button', async () => {
      const wrapper = await mountStatusBar()
      await flushPromises()

      const buttons = wrapper.findAll('.status-item')
      const shortcutsBtn = buttons.find((el) => el.text().includes('Shortcuts'))
      expect(shortcutsBtn).toBeDefined()
    })
  })

  describe('initial fetch', () => {
    it('fetches git status on mount', async () => {
      const fetchSpy = vi.spyOn(gitStore, 'fetchStatus').mockResolvedValue()

      await mountStatusBar()
      await flushPromises()

      expect(fetchSpy).toHaveBeenCalled()
    })
  })
})
