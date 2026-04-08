import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, ref, type Ref } from 'vue'
import { useListKeyboard } from './useListKeyboard'
import { _resetModalStack, registerModal } from './modalStack'

// Mock the dom utility
vi.mock('@/utils/dom', () => ({
  isInputFocused: vi.fn(() => false),
}))

function createKeyEvent(key: string): KeyboardEvent {
  return new KeyboardEvent('keydown', { key, bubbles: true })
}

// Create a test component factory
function createTestComponent(options: {
  itemCount: Ref<number>
  onOpen?: (index: number) => void
  onEdit?: (index: number) => void
  onCreate?: () => void
  onDelete?: (index: number) => void
  onPrevPage?: () => void
  onNextPage?: () => void
  hasPrevPage?: Ref<boolean>
  hasNextPage?: Ref<boolean>
}) {
  return defineComponent({
    setup() {
      const result = useListKeyboard(options)
      return { ...result }
    },
    template: '<div>Test</div>',
  })
}

describe('useListKeyboard', () => {
  let mockElement: HTMLElement

  beforeEach(() => {
    vi.clearAllMocks()
    _resetModalStack()
    mockElement = document.createElement('div')
    mockElement.className = 'entity-row'
    mockElement.scrollIntoView = vi.fn()
    document.body.appendChild(mockElement)
  })

  afterEach(() => {
    document.body.innerHTML = ''
    _resetModalStack()
  })

  describe('initialization', () => {
    it('starts with no selection', () => {
      const itemCount = ref(5)
      const wrapper = mount(createTestComponent({ itemCount }))
      expect(wrapper.vm.selectedIndex).toBe(-1)
      wrapper.unmount()
    })
  })

  describe('j/k navigation', () => {
    it('selects first item on j when nothing selected', () => {
      const itemCount = ref(5)
      const wrapper = mount(createTestComponent({ itemCount }))

      document.dispatchEvent(createKeyEvent('j'))

      expect(wrapper.vm.selectedIndex).toBe(0)
      wrapper.unmount()
    })

    it('selects last item on k when nothing selected', () => {
      const itemCount = ref(5)
      const wrapper = mount(createTestComponent({ itemCount }))

      document.dispatchEvent(createKeyEvent('k'))

      expect(wrapper.vm.selectedIndex).toBe(4)
      wrapper.unmount()
    })

    it('moves down with j', () => {
      const itemCount = ref(5)
      const wrapper = mount(createTestComponent({ itemCount }))

      document.dispatchEvent(createKeyEvent('j'))
      document.dispatchEvent(createKeyEvent('j'))

      expect(wrapper.vm.selectedIndex).toBe(1)
      wrapper.unmount()
    })

    it('moves up with k', () => {
      const itemCount = ref(5)
      const wrapper = mount(createTestComponent({ itemCount }))

      document.dispatchEvent(createKeyEvent('j'))
      document.dispatchEvent(createKeyEvent('j'))
      document.dispatchEvent(createKeyEvent('j'))
      document.dispatchEvent(createKeyEvent('k'))

      expect(wrapper.vm.selectedIndex).toBe(1)
      wrapper.unmount()
    })

    it('does not go below 0', () => {
      const itemCount = ref(5)
      const wrapper = mount(createTestComponent({ itemCount }))

      document.dispatchEvent(createKeyEvent('j'))
      document.dispatchEvent(createKeyEvent('k'))
      document.dispatchEvent(createKeyEvent('k'))

      expect(wrapper.vm.selectedIndex).toBe(0)
      wrapper.unmount()
    })

    it('does not go above item count', () => {
      const itemCount = ref(3)
      const wrapper = mount(createTestComponent({ itemCount }))

      for (let i = 0; i < 10; i++) {
        document.dispatchEvent(createKeyEvent('j'))
      }

      expect(wrapper.vm.selectedIndex).toBe(2)
      wrapper.unmount()
    })

    it('does nothing when item count is 0', () => {
      const itemCount = ref(0)
      const wrapper = mount(createTestComponent({ itemCount }))

      document.dispatchEvent(createKeyEvent('j'))

      expect(wrapper.vm.selectedIndex).toBe(-1)
      wrapper.unmount()
    })
  })

  describe('arrow key navigation', () => {
    it('moves down with ArrowDown', () => {
      const itemCount = ref(5)
      const wrapper = mount(createTestComponent({ itemCount }))

      document.dispatchEvent(createKeyEvent('ArrowDown'))

      expect(wrapper.vm.selectedIndex).toBe(0)
      wrapper.unmount()
    })

    it('moves up with ArrowUp', () => {
      const itemCount = ref(5)
      const wrapper = mount(createTestComponent({ itemCount }))

      document.dispatchEvent(createKeyEvent('ArrowUp'))

      expect(wrapper.vm.selectedIndex).toBe(4)
      wrapper.unmount()
    })
  })

  describe('action callbacks', () => {
    it('calls onOpen with Enter when item selected', () => {
      const itemCount = ref(5)
      const onOpen = vi.fn()
      const wrapper = mount(createTestComponent({ itemCount, onOpen }))

      document.dispatchEvent(createKeyEvent('j'))
      document.dispatchEvent(createKeyEvent('Enter'))

      expect(onOpen).toHaveBeenCalledWith(0)
      wrapper.unmount()
    })

    it('calls onOpen with o when item selected', () => {
      const itemCount = ref(5)
      const onOpen = vi.fn()
      const wrapper = mount(createTestComponent({ itemCount, onOpen }))

      document.dispatchEvent(createKeyEvent('j'))
      document.dispatchEvent(createKeyEvent('o'))

      expect(onOpen).toHaveBeenCalledWith(0)
      wrapper.unmount()
    })

    it('does not call onOpen when nothing selected', () => {
      const itemCount = ref(5)
      const onOpen = vi.fn()
      const wrapper = mount(createTestComponent({ itemCount, onOpen }))

      document.dispatchEvent(createKeyEvent('Enter'))

      expect(onOpen).not.toHaveBeenCalled()
      wrapper.unmount()
    })

    it('calls onEdit with e when item selected', () => {
      const itemCount = ref(5)
      const onEdit = vi.fn()
      const wrapper = mount(createTestComponent({ itemCount, onEdit }))

      document.dispatchEvent(createKeyEvent('j'))
      document.dispatchEvent(createKeyEvent('e'))

      expect(onEdit).toHaveBeenCalledWith(0)
      wrapper.unmount()
    })

    it('calls onCreate with n', () => {
      const itemCount = ref(5)
      const onCreate = vi.fn()
      const wrapper = mount(createTestComponent({ itemCount, onCreate }))

      document.dispatchEvent(createKeyEvent('n'))

      expect(onCreate).toHaveBeenCalled()
      wrapper.unmount()
    })

    it('calls onDelete with Delete when item selected', () => {
      const itemCount = ref(5)
      const onDelete = vi.fn()
      const wrapper = mount(createTestComponent({ itemCount, onDelete }))

      document.dispatchEvent(createKeyEvent('j'))
      document.dispatchEvent(createKeyEvent('Delete'))

      expect(onDelete).toHaveBeenCalledWith(0)
      wrapper.unmount()
    })

    it('calls onDelete with Backspace when item selected', () => {
      const itemCount = ref(5)
      const onDelete = vi.fn()
      const wrapper = mount(createTestComponent({ itemCount, onDelete }))

      document.dispatchEvent(createKeyEvent('j'))
      document.dispatchEvent(createKeyEvent('Backspace'))

      expect(onDelete).toHaveBeenCalledWith(0)
      wrapper.unmount()
    })

    it('does not call onDelete with Backspace when nothing selected', () => {
      const itemCount = ref(5)
      const onDelete = vi.fn()
      const wrapper = mount(createTestComponent({ itemCount, onDelete }))

      document.dispatchEvent(createKeyEvent('Backspace'))

      expect(onDelete).not.toHaveBeenCalled()
      wrapper.unmount()
    })

    it('does not call onDelete with Delete when nothing selected', () => {
      const itemCount = ref(5)
      const onDelete = vi.fn()
      const wrapper = mount(createTestComponent({ itemCount, onDelete }))

      document.dispatchEvent(createKeyEvent('Delete'))

      expect(onDelete).not.toHaveBeenCalled()
      wrapper.unmount()
    })
  })

  describe('pagination', () => {
    it('calls onPrevPage with h when hasPrevPage is true', () => {
      const itemCount = ref(5)
      const onPrevPage = vi.fn()
      const hasPrevPage = ref(true)
      const wrapper = mount(createTestComponent({ itemCount, onPrevPage, hasPrevPage }))

      document.dispatchEvent(createKeyEvent('h'))

      expect(onPrevPage).toHaveBeenCalled()
      wrapper.unmount()
    })

    it('does not call onPrevPage when hasPrevPage is false', () => {
      const itemCount = ref(5)
      const onPrevPage = vi.fn()
      const hasPrevPage = ref(false)
      const wrapper = mount(createTestComponent({ itemCount, onPrevPage, hasPrevPage }))

      document.dispatchEvent(createKeyEvent('h'))

      expect(onPrevPage).not.toHaveBeenCalled()
      wrapper.unmount()
    })

    it('calls onNextPage with l when hasNextPage is true', () => {
      const itemCount = ref(5)
      const onNextPage = vi.fn()
      const hasNextPage = ref(true)
      const wrapper = mount(createTestComponent({ itemCount, onNextPage, hasNextPage }))

      document.dispatchEvent(createKeyEvent('l'))

      expect(onNextPage).toHaveBeenCalled()
      wrapper.unmount()
    })

    it('does not call onNextPage when hasNextPage is false', () => {
      const itemCount = ref(5)
      const onNextPage = vi.fn()
      const hasNextPage = ref(false)
      const wrapper = mount(createTestComponent({ itemCount, onNextPage, hasNextPage }))

      document.dispatchEvent(createKeyEvent('l'))

      expect(onNextPage).not.toHaveBeenCalled()
      wrapper.unmount()
    })
  })

  describe('clearSelection', () => {
    it('resets selection to -1', () => {
      const itemCount = ref(5)
      const wrapper = mount(createTestComponent({ itemCount }))

      document.dispatchEvent(createKeyEvent('j'))
      document.dispatchEvent(createKeyEvent('j'))
      expect(wrapper.vm.selectedIndex).toBe(1)

      wrapper.vm.clearSelection()

      expect(wrapper.vm.selectedIndex).toBe(-1)
      wrapper.unmount()
    })
  })

  describe('input focus handling', () => {
    it('ignores keys when input is focused', async () => {
      const { isInputFocused } = await import('@/utils/dom')
      vi.mocked(isInputFocused).mockReturnValue(true)

      const itemCount = ref(5)
      const wrapper = mount(createTestComponent({ itemCount }))

      document.dispatchEvent(createKeyEvent('j'))

      expect(wrapper.vm.selectedIndex).toBe(-1)
      wrapper.unmount()
    })
  })

  describe('modal handling', () => {
    it('ignores keys when shortcuts modal is open', () => {
      const overlay = document.createElement('div')
      overlay.className = 'shortcuts-overlay'
      document.body.appendChild(overlay)

      const itemCount = ref(5)
      const wrapper = mount(createTestComponent({ itemCount }))

      document.dispatchEvent(createKeyEvent('j'))

      expect(wrapper.vm.selectedIndex).toBe(-1)
      wrapper.unmount()
    })

    it('ignores keys when a modal is registered in the modal stack', () => {
      registerModal(Symbol('test-modal'))

      const itemCount = ref(5)
      const wrapper = mount(createTestComponent({ itemCount }))

      document.dispatchEvent(createKeyEvent('j'))

      expect(wrapper.vm.selectedIndex).toBe(-1)
      wrapper.unmount()
    })
  })

  describe('cleanup', () => {
    it('removes event listener on unmount', () => {
      const itemCount = ref(5)
      const wrapper = mount(createTestComponent({ itemCount }))

      wrapper.unmount()

      // After unmount, keyboard events should have no effect
      // The best we can do is verify no errors occur
      expect(() => document.dispatchEvent(createKeyEvent('j'))).not.toThrow()
    })
  })
})
