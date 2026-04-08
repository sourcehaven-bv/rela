import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, ref } from 'vue'
import {
  registerModal,
  unregisterModal,
  isAnyModalOpen,
  useModalStack,
  _resetModalStack,
} from './modalStack'

describe('modalStack', () => {
  beforeEach(() => {
    _resetModalStack()
  })

  describe('manual register/unregister', () => {
    it('is empty by default', () => {
      expect(isAnyModalOpen()).toBe(false)
    })

    it('reports open after register', () => {
      const id = Symbol('test')
      registerModal(id)
      expect(isAnyModalOpen()).toBe(true)
    })

    it('reports closed after unregister', () => {
      const id = Symbol('test')
      registerModal(id)
      unregisterModal(id)
      expect(isAnyModalOpen()).toBe(false)
    })

    it('tracks multiple modals independently', () => {
      const a = Symbol('a')
      const b = Symbol('b')
      registerModal(a)
      registerModal(b)
      expect(isAnyModalOpen()).toBe(true)
      unregisterModal(a)
      expect(isAnyModalOpen()).toBe(true)
      unregisterModal(b)
      expect(isAnyModalOpen()).toBe(false)
    })

    it('unregistering an unknown id is a no-op', () => {
      expect(() => unregisterModal(Symbol('never-registered'))).not.toThrow()
      expect(isAnyModalOpen()).toBe(false)
    })
  })

  describe('useModalStack', () => {
    function createHarness(open: ReturnType<typeof ref<boolean>>) {
      return defineComponent({
        setup() {
          useModalStack(open as ReturnType<typeof ref<boolean>> & { value: boolean })
        },
        template: '<div />',
      })
    }

    it('registers on mount when open=true', () => {
      const open = ref(true)
      const wrapper = mount(createHarness(open))
      expect(isAnyModalOpen()).toBe(true)
      wrapper.unmount()
    })

    it('does not register on mount when open=false', () => {
      const open = ref(false)
      const wrapper = mount(createHarness(open))
      expect(isAnyModalOpen()).toBe(false)
      wrapper.unmount()
    })

    it('registers when open flips to true', async () => {
      const open = ref(false)
      const wrapper = mount(createHarness(open))
      expect(isAnyModalOpen()).toBe(false)
      open.value = true
      await wrapper.vm.$nextTick()
      expect(isAnyModalOpen()).toBe(true)
      wrapper.unmount()
    })

    it('unregisters when open flips to false', async () => {
      const open = ref(true)
      const wrapper = mount(createHarness(open))
      expect(isAnyModalOpen()).toBe(true)
      open.value = false
      await wrapper.vm.$nextTick()
      expect(isAnyModalOpen()).toBe(false)
      wrapper.unmount()
    })

    it('unregisters on unmount even when still open', () => {
      const open = ref(true)
      const wrapper = mount(createHarness(open))
      expect(isAnyModalOpen()).toBe(true)
      wrapper.unmount()
      expect(isAnyModalOpen()).toBe(false)
    })

    it('multiple instances track independently', () => {
      const openA = ref(true)
      const openB = ref(true)
      const wrapperA = mount(createHarness(openA))
      const wrapperB = mount(createHarness(openB))
      expect(isAnyModalOpen()).toBe(true)
      wrapperA.unmount()
      expect(isAnyModalOpen()).toBe(true)
      wrapperB.unmount()
      expect(isAnyModalOpen()).toBe(false)
    })
  })
})
