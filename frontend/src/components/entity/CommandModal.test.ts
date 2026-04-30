import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import CommandModal from './CommandModal.vue'
import ConfirmModal from '@/components/ui/ConfirmModal.vue'
import { _resetModalStack } from '@/composables/modalStack'
import { useConfirmHost, _resetConfirmForTest } from '@/composables/useConfirm'
import type { Command } from '@/types'

// CommandModal exposes runCommand(cmd). When cmd.confirm is set, the global
// confirm modal must intercept; only on user confirm does the command fetch
// fire. These tests assemble the same wiring App.vue does.

describe('CommandModal confirm integration', () => {
  let originalFetch: typeof fetch
  let fetchSpy: ReturnType<typeof vi.fn>

  // Mount the same harness pattern App.vue uses: CommandModal next to a
  // ConfirmModal driven by the singleton confirm host.
  function mountWithHost() {
    const Host = defineComponent({
      setup() {
        const { state, onConfirmEvent, onCancelEvent } = useConfirmHost()
        return () => [
          h(CommandModal, { entityId: 'TKT-X' }),
          h(ConfirmModal, {
            open: state.open,
            title: state.title,
            message: state.message,
            confirmLabel: state.confirmLabel,
            cancelLabel: state.cancelLabel,
            busy: state.busy,
            danger: state.danger,
            onConfirm: () => { onConfirmEvent().catch(() => {}) },
            onCancel: () => { onCancelEvent() },
          }),
        ]
      },
    })
    const wrapper = mount(Host, { attachTo: document.body })
    const cmdVm = wrapper.findComponent(CommandModal).vm as unknown as {
      runCommand: (cmd: Command) => Promise<void>
    }
    return { wrapper, runCommand: (cmd: Command) => cmdVm.runCommand(cmd) }
  }

  function modalActionButtons(): HTMLButtonElement[] {
    return Array.from(
      document.querySelectorAll<HTMLButtonElement>('.modal-actions button')
    )
  }

  beforeEach(() => {
    _resetModalStack()
    _resetConfirmForTest()
    originalFetch = global.fetch
    // Resolve immediately with an empty body so runCommand finishes quickly
    // when it does fire.
    fetchSpy = vi.fn().mockResolvedValue(
      new Response(null, { status: 200 })
    )
    global.fetch = fetchSpy as never
  })

  afterEach(() => {
    document.body.innerHTML = ''
    _resetModalStack()
    _resetConfirmForTest()
    global.fetch = originalFetch
  })

  it('skips the confirm modal when cmd.confirm is empty', async () => {
    const { wrapper, runCommand } = mountWithHost()
    const cmd: Command = {
      id: 'noop',
      label: 'Run noop',
      context: 'entity',
    }
    await runCommand(cmd)
    await flushPromises()

    expect(fetchSpy).toHaveBeenCalledTimes(1)
    expect(fetchSpy.mock.calls[0]?.[0]).toContain('/api/command/noop')
    wrapper.unmount()
  })

  it('opens the confirm modal with cmd.label-derived title when cmd.confirm is set', async () => {
    const { wrapper, runCommand } = mountWithHost()
    const cmd: Command = {
      id: 'destroy',
      label: 'Destroy World',
      confirm: 'This will end civilization. Proceed?',
      context: 'entity',
    }
    void runCommand(cmd)
    await flushPromises()

    const modal = document.querySelector<HTMLElement>('.modal')
    expect(modal).not.toBeNull()
    expect(modal?.textContent).toContain('Destroy World?')
    expect(modal?.textContent).toContain('This will end civilization. Proceed?')

    // The confirm button label is the command label (matches the bulk-action
    // pattern at EntityList — see TKT-60E9G design review).
    const buttons = modalActionButtons()
    const confirmBtn = buttons[buttons.length - 1]
    expect(confirmBtn?.textContent?.trim()).toContain('Destroy World')
    wrapper.unmount()
  })

  it('runs the command after the user confirms', async () => {
    const { wrapper, runCommand } = mountWithHost()
    const cmd: Command = {
      id: 'destroy',
      label: 'Destroy World',
      confirm: 'Sure?',
      context: 'entity',
    }
    void runCommand(cmd)
    await flushPromises()

    expect(fetchSpy).not.toHaveBeenCalled()
    modalActionButtons()[1]!.click()
    await flushPromises()

    expect(fetchSpy).toHaveBeenCalledTimes(1)
    expect(fetchSpy.mock.calls[0]?.[0]).toContain('/api/command/destroy')
    wrapper.unmount()
  })

  it('does not run the command when the user cancels', async () => {
    const { wrapper, runCommand } = mountWithHost()
    const cmd: Command = {
      id: 'destroy',
      label: 'Destroy',
      confirm: 'Sure?',
      context: 'entity',
    }
    void runCommand(cmd)
    await flushPromises()

    modalActionButtons()[0]!.click()
    await flushPromises()

    expect(fetchSpy).not.toHaveBeenCalled()
    expect(document.querySelector('.modal-overlay')).toBeNull()
    wrapper.unmount()
  })
})
