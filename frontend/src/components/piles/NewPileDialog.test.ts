import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import { reactive } from 'vue'
import { useSchemaStore } from '@/stores'
import { resetPilesState } from '@/composables/usePiles'
import NewPileDialog from './NewPileDialog.vue'

const api = vi.hoisted(() => ({
  listPiles: vi.fn(),
  createPile: vi.fn(),
  updatePile: vi.fn(),
}))
vi.mock('@/api/piles', async (orig) => ({
  ...(await orig<typeof import('@/api/piles')>()),
  ...api,
}))
vi.mock('@/composables/useEvents', () => ({ useEvents: () => ({ on: () => {}, off: () => {} }) }))
const route = reactive({ path: '/', query: {} })
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => ({ push: vi.fn() }) }))

const created = {
  id: 'PIL-NEW00001',
  name: 'Friday',
  icon: 'star',
  count: 2,
  created: '',
  updated: '',
  items: [],
}

let pinia: Pinia
const mounted: VueWrapper[] = []

async function mountDialog(props: InstanceType<typeof NewPileDialog>['$props'] = {}) {
  useSchemaStore().setPiles({ piles_available: true, piles: null })
  const wrapper = mount(NewPileDialog, {
    props,
    global: { plugins: [pinia, PiniaColada] },
    attachTo: document.body,
  })
  mounted.push(wrapper)
  await flushPromises()
  return wrapper
}

function nameInput(): HTMLInputElement {
  return document.body.querySelector(
    '[data-testid="pile-name"] input, input[data-testid="pile-name"]'
  ) as HTMLInputElement
}

function submitButton(): HTMLButtonElement {
  return document.body.querySelector('[data-testid="pile-submit"]') as HTMLButtonElement
}

async function typeName(value: string) {
  const input = nameInput()
  input.value = value
  input.dispatchEvent(new Event('input'))
  await flushPromises()
}

beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  Object.values(api).forEach((fn) => fn.mockReset())
  api.listPiles.mockResolvedValue({ piles: [], icons: ['layers', 'star', 'flag'] })
  resetPilesState()
})

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount())
  document.body.innerHTML = ''
})

describe('NewPileDialog', () => {
  it('cannot be submitted without a name', async () => {
    await mountDialog()
    expect(submitButton().disabled).toBe(true)
    await typeName('   ')
    expect(submitButton().disabled).toBe(true)
  })

  it('refuses a name over 80 characters', async () => {
    await mountDialog()
    await typeName('x'.repeat(81))
    expect(submitButton().disabled).toBe(true)
    expect(document.body.textContent).toContain('Use at most 80 characters')

    await typeName('x'.repeat(80))
    expect(submitButton().disabled).toBe(false)
    expect(document.body.textContent).not.toContain('Use at most 80 characters')
  })

  it('offers the server’s icons', async () => {
    await mountDialog()
    const radios = document.body.querySelectorAll('[role="radio"]')
    expect([...radios].map((r) => r.getAttribute('aria-label') ?? r.textContent?.trim())).toEqual([
      'Layers',
      'Star',
      'Flag',
    ])
  })

  it('creates the pile with the selected items on Enter, then closes', async () => {
    api.createPile.mockResolvedValue(created)
    const wrapper = await mountDialog({ items: ['TKT-1', 'POL-1@draft'] })
    expect(document.body.textContent).toContain('The 2 selected items go on the new pile.')

    await typeName('  Friday  ')
    nameInput().form!.dispatchEvent(new Event('submit', { cancelable: true }))
    await flushPromises()

    expect(api.createPile).toHaveBeenCalledWith(
      { name: 'Friday', icon: 'layers', items: ['TKT-1', 'POL-1@draft'] },
      undefined
    )
    expect(wrapper.emitted('saved')?.[0]).toEqual([created])
    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('stays open when the server refuses the name', async () => {
    const { ApiError } = await import('@/api/errors')
    api.createPile.mockRejectedValue(
      new ApiError('taken', {
        kind: 'http',
        status: 409,
        problem: { type: 'https://rela.dev/errors/pile_name_taken', title: 'taken', status: 409 },
        original: null,
      })
    )
    const wrapper = await mountDialog()
    await typeName('Friday')
    nameInput().form!.dispatchEvent(new Event('submit', { cancelable: true }))
    await flushPromises()

    expect(wrapper.emitted('close')).toBeFalsy()
  })

  it('renames an existing pile', async () => {
    api.updatePile.mockResolvedValue({ ...created, name: 'Monday' })
    const wrapper = await mountDialog({ pile: created })
    expect(nameInput().value).toBe('Friday')

    await typeName('Monday')
    nameInput().form!.dispatchEvent(new Event('submit', { cancelable: true }))
    await flushPromises()

    expect(api.updatePile).toHaveBeenCalledWith(
      created.id,
      { name: 'Monday', icon: 'star' },
      undefined
    )
    expect(api.createPile).not.toHaveBeenCalled()
    expect(wrapper.emitted('close')).toBeTruthy()
  })
})
