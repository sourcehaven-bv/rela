import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { SHELL_COMMAND_EVENT, useShellCommands } from './useShellCommands'
import { paletteOpen, shortcutsModalOpen } from './useKeyboardShortcuts'
import { useSpaceStore } from '@/stores/space'
import { useUIStore } from '@/stores/ui'

const push = vi.fn()
const back = vi.fn()
const forward = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push, back, forward }),
  useRoute: () => ({ path: '/' }),
}))

function send(command: string, arg?: string) {
  window.dispatchEvent(new CustomEvent(SHELL_COMMAND_EVENT, { detail: { command, arg } }))
}

describe('useShellCommands', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    push.mockReset()
    back.mockReset()
    forward.mockReset()
    paletteOpen.value = false
    shortcutsModalOpen.value = false
  })

  function host() {
    return mount(defineComponent({ setup: () => (useShellCommands(), () => h('div')) }))
  }

  it('runs the commands a shell sends', () => {
    const w = host()
    send('back')
    send('forward')
    send('palette')
    send('shortcuts')
    send('settings')
    expect(back).toHaveBeenCalledOnce()
    expect(forward).toHaveBeenCalledOnce()
    expect(paletteOpen.value).toBe(true)
    expect(shortcutsModalOpen.value).toBe(true)
    expect(push).toHaveBeenCalledWith('/settings')
    w.unmount()
  })

  it('toggles the sidebar', () => {
    const w = host()
    const ui = useUIStore()
    const before = ui.sidebarCollapsed
    send('toggle-sidebar')
    expect(ui.sidebarCollapsed).toBe(!before)
    w.unmount()
  })

  it('opens a space at its home and ignores an unknown one', () => {
    const w = host()
    const spaces = useSpaceStore()
    spaces.$patch({ spaces: [{ id: 'crm', label: 'CRM', home: '/kanban/pipeline' }] } as never)
    send('space', 'crm')
    send('space', 'nope')
    expect(push).toHaveBeenCalledTimes(1)
    expect(push).toHaveBeenCalledWith('/s/crm/kanban/pipeline')
    w.unmount()
  })

  it('navigates only to same-origin paths', () => {
    const w = host()
    send('navigate', '/entity/doc/DOC-1')
    send('navigate', '//evil.example/x')
    send('navigate', 'https://evil.example/')
    expect(push).toHaveBeenCalledTimes(1)
    expect(push).toHaveBeenCalledWith('/entity/doc/DOC-1')
    w.unmount()
  })

  it('stops listening once unmounted', () => {
    host().unmount()
    send('palette')
    expect(paletteOpen.value).toBe(false)
  })
})
