import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import EntityPreviewModal from './EntityPreviewModal.vue'
import { isAnyModalOpen, _resetModalStack } from '@/composables/modalStack'

/**
 * The preview moved onto the shared `RlModal`. EntityDetail is stubbed: this
 * covers the dialog wrapper, not the entity rendering it delegates to.
 */

type Props = InstanceType<typeof EntityPreviewModal>['$props']

function factory(props: Partial<Props> = {}) {
  // Annotated so `setProps` below keeps the component's prop types; an
  // inferred return from a generic factory widens them away.
  const wrapper: ReturnType<typeof mount<typeof EntityPreviewModal>> = mount(EntityPreviewModal, {
    props: {
      open: true,
      entityType: 'task',
      entityId: 'TASK-1',
      ...props,
    },
    global: {
      stubs: {
        EntityDetail: { template: '<div class="entity-detail-stub" />' },
        RouterLink: { props: ['to'], template: '<a><slot /></a>' },
      },
    },
    attachTo: document.body,
  })
  return wrapper
}

describe('EntityPreviewModal', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    _resetModalStack()
    document.body.innerHTML = ''
  })

  afterEach(() => {
    _resetModalStack()
    document.body.innerHTML = ''
  })

  it('renders nothing while closed', () => {
    factory({ open: false })

    expect(document.querySelector('[role="dialog"]')).toBeNull()
  })

  it('renders the entity inside a labelled dialog', () => {
    factory()

    const dialog = document.querySelector('[role="dialog"]')
    expect(dialog).not.toBeNull()
    expect(dialog?.getAttribute('aria-modal')).toBe('true')
    // The title names the dialog for assistive tech but is not drawn.
    expect(dialog?.getAttribute('aria-label')).toBe('Entity preview')
    expect(document.querySelector('.entity-detail-stub')).not.toBeNull()
  })

  /**
   * The preview is wider than RlModal's `lg`, which it gets through
   * `panelClass`. The scoped rule carrying the width is `:global`, so it only
   * applies if the class actually reaches the panel — that relationship is
   * what this pins, not the pixel value.
   */
  it('widens the panel through panelClass', () => {
    factory()

    const panel = document.querySelector('.rl-modal')
    expect(panel).not.toBeNull()
    expect(panel?.classList.contains('entity-preview-panel')).toBe(true)
  })

  it('registers with rela’s modal stack so shortcuts stand down', async () => {
    const wrapper = factory({ open: false })
    expect(isAnyModalOpen()).toBe(false)

    await wrapper.setProps({ open: true })
    expect(isAnyModalOpen()).toBe(true)

    await wrapper.setProps({ open: false })
    expect(isAnyModalOpen()).toBe(false)
  })

  it('offers Edit only when a form is configured', async () => {
    const wrapper = factory()
    expect(document.body.textContent).toContain('Open full page')
    expect(document.body.textContent).not.toContain('Edit')

    await wrapper.setProps({ editForm: 'task-form' })
    expect(document.body.textContent).toContain('Edit')
  })

  it('emits close when the modal asks to close', async () => {
    const wrapper = factory()

    const close = document.querySelector<HTMLButtonElement>('.rl-modal button')
    close?.click()
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('close')).toHaveLength(1)
  })
})
