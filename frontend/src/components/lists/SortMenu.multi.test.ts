import { describe, it, expect, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import SortMenu from './SortMenu.vue'
import type { SortSpec } from '@/types'

// A multi-key sort built with shift-click on list column headers shows every
// key in the menu, not only the first.

const OPTIONS = [
  { property: 'title', label: 'Title' },
  { property: 'due', label: 'Due date' },
  { property: 'status', label: 'Status' },
]

const MULTI: SortSpec[] = [
  { property: 'status', direction: 'asc' },
  { property: 'due', direction: 'desc' },
]

function mountMenu(modelValue: SortSpec[]) {
  return mount(SortMenu, { props: { options: OPTIONS, modelValue }, attachTo: document.body })
}

async function openMenu(wrapper: ReturnType<typeof mountMenu>) {
  await wrapper.find('[data-testid="sort-menu-button"]').trigger('click')
  await flushPromises()
}

function keyItem(property: string): HTMLElement {
  const found = document.querySelector<HTMLElement>(`[data-sort-key="${property}"]`)
  if (!found) throw new Error(`no sort key ${property}`)
  return found
}

afterEach(() => {
  document.body.innerHTML = ''
})

describe('SortMenu with several keys', () => {
  it('says on the trigger that more keys follow the first', () => {
    const wrapper = mountMenu(MULTI)
    expect(wrapper.find('[data-testid="sort-menu-button"]').text()).toContain('Sort: Status +1')
    wrapper.unmount()
  })

  it('lists every key with its direction', async () => {
    const wrapper = mountMenu(MULTI)
    await openMenu(wrapper)

    expect(keyItem('status').textContent).toContain('Status (ascending)')
    expect(keyItem('due').textContent).toContain('Due date (descending)')
    wrapper.unmount()
  })

  it('lists no key section for a single key', async () => {
    const wrapper = mountMenu([MULTI[0]])
    await openMenu(wrapper)

    expect(document.querySelector('[data-sort-key]')).toBeNull()
    wrapper.unmount()
  })

  it('choosing an ascending key flips it to descending and keeps the others', async () => {
    const wrapper = mountMenu(MULTI)
    await openMenu(wrapper)
    keyItem('status').click()

    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([
      [
        { property: 'status', direction: 'desc' },
        { property: 'due', direction: 'desc' },
      ],
    ])
    wrapper.unmount()
  })

  it('choosing a descending key removes it', async () => {
    const wrapper = mountMenu(MULTI)
    await openMenu(wrapper)
    keyItem('due').click()

    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([[{ property: 'status', direction: 'asc' }]])
    wrapper.unmount()
  })

  it('the direction rows keep the later keys', async () => {
    const wrapper = mountMenu(MULTI)
    await openMenu(wrapper)
    const descending = Array.from(document.querySelectorAll<HTMLElement>('[data-testid="sort-desc"]'))[0]
    descending.click()

    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([
      [
        { property: 'status', direction: 'desc' },
        { property: 'due', direction: 'desc' },
      ],
    ])
    wrapper.unmount()
  })

  it('choosing a property replaces the whole sort', async () => {
    const wrapper = mountMenu(MULTI)
    await openMenu(wrapper)
    document.querySelector<HTMLElement>('[data-sort-property="title"]')?.click()

    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([[{ property: 'title', direction: 'asc' }]])
    wrapper.unmount()
  })
})
