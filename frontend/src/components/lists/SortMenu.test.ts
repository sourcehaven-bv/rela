import { describe, it, expect, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import SortMenu from './SortMenu.vue'
import type { SortSpec } from '@/types'

const OPTIONS = [
  { property: 'title', label: 'Title' },
  { property: 'due', label: 'Due date' },
]

function mountMenu(modelValue: SortSpec[] = []) {
  return mount(SortMenu, { props: { options: OPTIONS, modelValue }, attachTo: document.body })
}

async function openMenu(wrapper: ReturnType<typeof mountMenu>) {
  await wrapper.find('[data-testid="sort-menu-button"]').trigger('click')
  await flushPromises()
}

function item(label: string): HTMLElement {
  const found = Array.from(document.querySelectorAll<HTMLElement>('[role="menuitem"]')).find((el) =>
    el.textContent?.includes(label),
  )
  if (!found) throw new Error(`no item ${label}`)
  return found
}

afterEach(() => {
  document.body.innerHTML = ''
})

describe('SortMenu', () => {
  it('renders nothing without options', () => {
    const wrapper = mount(SortMenu, { props: { options: [], modelValue: [] } })
    expect(wrapper.find('[data-testid="sort-menu-button"]').exists()).toBe(false)
  })

  it('names the active sort on the trigger', () => {
    const wrapper = mountMenu([{ property: 'due', direction: 'desc' }])
    expect(wrapper.find('[data-testid="sort-menu-button"]').text()).toContain('Sort: Due date')
    wrapper.unmount()
  })

  it('choosing a property sorts it ascending', async () => {
    const wrapper = mountMenu()
    await openMenu(wrapper)
    item('Due date').click()

    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([[{ property: 'due', direction: 'asc' }]])
    wrapper.unmount()
  })

  it('re-choosing the active property keeps its direction', async () => {
    const wrapper = mountMenu([{ property: 'due', direction: 'desc' }])
    await openMenu(wrapper)
    item('Due date').click()

    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([[{ property: 'due', direction: 'desc' }]])
    wrapper.unmount()
  })

  it('the direction rows change only the direction', async () => {
    const wrapper = mountMenu([{ property: 'title', direction: 'asc' }])
    await openMenu(wrapper)
    item('Descending').click()

    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([[{ property: 'title', direction: 'desc' }]])
    wrapper.unmount()
  })

  it('the direction rows are disabled until a property is chosen', async () => {
    const wrapper = mountMenu()
    await openMenu(wrapper)

    expect(item('Ascending').hasAttribute('disabled')).toBe(true)
    expect(item('Descending').hasAttribute('disabled')).toBe(true)
    wrapper.unmount()
  })

  it('Default order clears the sort', async () => {
    const wrapper = mountMenu([{ property: 'title', direction: 'asc' }])
    await openMenu(wrapper)
    item('Default order').click()

    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([[]])
    wrapper.unmount()
  })

  it('opens from the keyboard and Escape closes it', async () => {
    const wrapper = mountMenu()
    await wrapper.find('[data-testid="sort-menu-button"]').trigger('keydown', { key: 'ArrowDown' })
    await flushPromises()

    const menu = document.querySelector('[role="menu"]') as HTMLElement
    expect(menu).not.toBeNull()
    menu.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await flushPromises()

    expect(document.querySelector('[role="menu"]')).toBeNull()
    wrapper.unmount()
  })
})
