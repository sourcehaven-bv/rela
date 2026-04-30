import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import {
  useConfirm,
  useConfirmHost,
  withConfirmError,
  _resetConfirmForTest,
} from './useConfirm'

// Host harness mirrors what App.vue does — mounts useConfirmHost and exposes
// the state + handlers so tests can drive them like the modal would.
type HostExposed = ReturnType<typeof useConfirmHost>
function mountHost(): { wrapper: ReturnType<typeof mount>; host: HostExposed } {
  let host!: HostExposed
  const Host = defineComponent({
    setup(_, { expose }) {
      host = useConfirmHost()
      expose(host)
      return () => h('div')
    },
  })
  const wrapper = mount(Host)
  return { wrapper, host }
}

describe('useConfirm', () => {
  beforeEach(() => {
    _resetConfirmForTest()
  })

  describe('basic resolution', () => {
    it('resolves true when the host confirms', async () => {
      const { wrapper, host } = mountHost()
      const { confirm } = useConfirm()

      const promise = confirm({ title: 'Delete?', confirmLabel: 'Delete' })
      expect(host.state.open).toBe(true)
      await host.onConfirmEvent()
      expect(await promise).toBe(true)
      expect(host.state.open).toBe(false)
      wrapper.unmount()
    })

    it('resolves false when the host cancels', async () => {
      const { wrapper, host } = mountHost()
      const { confirm } = useConfirm()

      const promise = confirm({ title: 'Delete?' })
      host.onCancelEvent()
      expect(await promise).toBe(false)
      expect(host.state.open).toBe(false)
      wrapper.unmount()
    })

    it('mirrors options onto state', () => {
      const { wrapper, host } = mountHost()
      const { confirm } = useConfirm()

      void confirm({
        title: 'Run cmd?',
        message: 'This will delete files',
        confirmLabel: 'Run',
        cancelLabel: 'Abort',
        danger: true,
      })

      expect(host.state.title).toBe('Run cmd?')
      expect(host.state.message).toBe('This will delete files')
      expect(host.state.confirmLabel).toBe('Run')
      expect(host.state.cancelLabel).toBe('Abort')
      expect(host.state.danger).toBe(true)
      wrapper.unmount()
    })

    it('uses defaults for omitted labels', () => {
      const { wrapper, host } = mountHost()
      const { confirm } = useConfirm()

      void confirm({ title: 'X' })

      expect(host.state.confirmLabel).toBe('Confirm')
      expect(host.state.cancelLabel).toBe('Cancel')
      expect(host.state.message).toBe('')
      expect(host.state.danger).toBe(false)
      wrapper.unmount()
    })
  })

  describe('concurrent calls', () => {
    it('returns the in-flight decision to a second concurrent caller', async () => {
      const { wrapper, host } = mountHost()
      const { confirm } = useConfirm()

      const p1 = confirm({ title: 'First' })
      const p2 = confirm({ title: 'Second' })

      // The second call MUST NOT replace the first's options.
      expect(host.state.title).toBe('First')

      // Both callers observe the same user decision (single confirm event).
      await host.onConfirmEvent()
      expect(await p1).toBe(true)
      expect(await p2).toBe(true)
      wrapper.unmount()
    })

    it('allows a fresh confirm after the first one settles', async () => {
      const { wrapper, host } = mountHost()
      const { confirm } = useConfirm()

      const p1 = confirm({ title: 'A' })
      host.onCancelEvent()
      await p1

      const p2 = confirm({ title: 'B' })
      expect(host.state.title).toBe('B')
      expect(host.state.open).toBe(true)
      await host.onConfirmEvent()
      expect(await p2).toBe(true)
      wrapper.unmount()
    })

    it('user can cancel twice in a row and the modal stays operable', async () => {
      const { wrapper, host } = mountHost()
      const { confirm } = useConfirm()

      const p1 = confirm({ title: 'A' })
      host.onCancelEvent()
      expect(await p1).toBe(false)

      const p2 = confirm({ title: 'B' })
      host.onCancelEvent()
      expect(await p2).toBe(false)
      expect(host.state.open).toBe(false)
      wrapper.unmount()
    })
  })

  describe('onConfirm async action', () => {
    it('runs onConfirm with busy=true, resolves true on success', async () => {
      const { wrapper, host } = mountHost()
      const { confirm } = useConfirm()

      let busyDuringAction = false
      const action = vi.fn(async () => {
        busyDuringAction = host.state.busy
      })

      const promise = confirm({
        title: 'Delete?',
        onConfirm: action,
      })
      expect(host.state.busy).toBe(false)
      await host.onConfirmEvent()
      expect(action).toHaveBeenCalledTimes(1)
      expect(busyDuringAction).toBe(true)
      expect(await promise).toBe(true)
      expect(host.state.busy).toBe(false)
      expect(host.state.open).toBe(false)
      wrapper.unmount()
    })

    it('keeps the modal open with busy cleared when onConfirm throws', async () => {
      const { wrapper, host } = mountHost()
      const { confirm } = useConfirm()

      let resolved: boolean | null = null
      const promise = confirm({
        title: 'Delete?',
        onConfirm: async () => {
          throw new Error('boom')
        },
      })
      promise.then((v) => {
        resolved = v
      })

      await expect(host.onConfirmEvent()).rejects.toThrow('boom')
      // Promise still pending — caller should see the modal again and either
      // retry or cancel.
      expect(resolved).toBeNull()
      expect(host.state.open).toBe(true)
      expect(host.state.busy).toBe(false)

      // Cancel resolves the original promise to false.
      host.onCancelEvent()
      expect(await promise).toBe(false)
      wrapper.unmount()
    })

    it('cancel during busy is ignored', async () => {
      const { wrapper, host } = mountHost()
      const { confirm } = useConfirm()

      let resolveAction!: () => void
      const promise = confirm({
        title: 'Delete?',
        onConfirm: () =>
          new Promise<void>((r) => {
            resolveAction = r
          }),
      })
      const eventPromise = host.onConfirmEvent()
      expect(host.state.busy).toBe(true)

      // Cancel while busy: should be a no-op.
      host.onCancelEvent()
      expect(host.state.busy).toBe(true)
      expect(host.state.open).toBe(true)

      resolveAction()
      await eventPromise
      expect(await promise).toBe(true)
      wrapper.unmount()
    })
  })

  describe('host unmount', () => {
    it('resolves a pending promise to false on host unmount', async () => {
      const { wrapper } = mountHost()
      const { confirm } = useConfirm()

      const promise = confirm({ title: 'Pending' })
      wrapper.unmount()
      expect(await promise).toBe(false)
    })

    it('does nothing if there was no pending promise', () => {
      const { wrapper } = mountHost()
      expect(() => wrapper.unmount()).not.toThrow()
    })
  })

  describe('host-not-mounted safety', () => {
    it('resolves false and warns when no host is mounted', async () => {
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const { confirm } = useConfirm()

      const result = await confirm({ title: 'No host' })
      expect(result).toBe(false)
      expect(consoleSpy).toHaveBeenCalled()
      consoleSpy.mockRestore()
    })

    it('throws if a second host is mounted', () => {
      const { wrapper } = mountHost()
      // A second mount must fail — singleton invariant. Vue Test Utils logs
      // a missing-render warning when setup throws; suppress it to keep test
      // output clean.
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      expect(() => mountHost()).toThrow(/already mounted/i)
      warnSpy.mockRestore()
      wrapper.unmount()
    })

    it('allows remounting after the first host unmounts', () => {
      const { wrapper: w1 } = mountHost()
      w1.unmount()
      const { wrapper: w2 } = mountHost()
      w2.unmount()
    })
  })

  describe('withConfirmError', () => {
    it('runs the action and returns void on success', async () => {
      const { wrapper } = mountHost()
      const action = vi.fn().mockResolvedValue('ignored')
      const uiStore = { error: vi.fn() }

      const wrapped = withConfirmError(action, 'Failed', uiStore)
      await expect(wrapped()).resolves.toBeUndefined()
      expect(action).toHaveBeenCalledTimes(1)
      expect(uiStore.error).not.toHaveBeenCalled()
      wrapper.unmount()
    })

    it('toasts the message and rethrows when action throws', async () => {
      const { wrapper } = mountHost()
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const boom = new Error('boom')
      const action = vi.fn().mockRejectedValue(boom)
      const uiStore = { error: vi.fn() }

      const wrapped = withConfirmError(action, 'Failed', uiStore)
      await expect(wrapped()).rejects.toBe(boom)
      expect(uiStore.error).toHaveBeenCalledWith('Failed')
      expect(consoleSpy).toHaveBeenCalled()
      consoleSpy.mockRestore()
      wrapper.unmount()
    })
  })
})
