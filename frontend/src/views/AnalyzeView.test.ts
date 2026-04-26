import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import AnalyzeView from './AnalyzeView.vue'
import { useScriptErrorStore } from '@/stores/scriptError'
import { useSchemaStore } from '@/stores/schema'
import type { AnalyzeIssue, AnalyzeResult } from '@/types'
import type { ScriptError } from '@/types/scriptError'

const routerPush = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: routerPush }),
  useRoute: () => ({ query: {}, path: '/analyze' }),
}))

const analyzeMock = vi.fn<[], Promise<AnalyzeResult>>()
vi.mock('@/api', () => ({
  analyze: () => analyzeMock(),
}))

vi.mock('@/composables/useBackTarget', () => ({
  useBackTarget: () => null,
}))

function makeScriptError(overrides: Partial<ScriptError> = {}): ScriptError {
  return {
    error: 'script_error',
    correlation_id: 'corr-1',
    script: { surface: 'action', path: 'validations/broken.lua' },
    lua: { message: 'attempt to index nil', line: 4 },
    ...overrides,
  }
}

function makeIssue(overrides: Partial<AnalyzeIssue> = {}): AnalyzeIssue {
  return {
    entityId: '',
    entityType: '',
    message: 'something',
    severity: 'error',
    checkType: 'Validations',
    ...overrides,
  }
}

function makeResult(issues: AnalyzeIssue[]): AnalyzeResult {
  const byCheck: Record<string, number> = {}
  for (const i of issues) {
    byCheck[i.checkType] = (byCheck[i.checkType] || 0) + 1
  }
  return {
    errors: issues.filter((i) => i.severity === 'error').length,
    warnings: issues.filter((i) => i.severity === 'warning').length,
    issues,
    byCheck,
  }
}

describe('AnalyzeView click discrimination', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    routerPush.mockReset()
    analyzeMock.mockReset()
    // Provide a minimal entity type so getEntityTypeLabel resolves.
    const schema = useSchemaStore()
    schema.entityTypes.set('note', { label: 'Note' } as never)
  })

  async function mountWith(issues: AnalyzeIssue[]) {
    analyzeMock.mockResolvedValue(makeResult(issues))
    const wrapper = mount(AnalyzeView, { attachTo: document.body })
    await flushPromises()
    return wrapper
  }

  it('opens ScriptErrorDialog (via store) when a script-error row is clicked', async () => {
    const scriptError = makeScriptError()
    const wrapper = await mountWith([
      makeIssue({
        title: 'broken-rule',
        message: 'Validation script failed: ...',
        scriptError,
      }),
    ])

    const store = useScriptErrorStore()
    expect(store.current).toBeNull()

    const row = wrapper.find('.issue-row')
    await row.trigger('click')

    expect(store.current).toEqual(scriptError)
    expect(routerPush).not.toHaveBeenCalled()
  })

  it('navigates to the entity when a regular violation row is clicked', async () => {
    const wrapper = await mountWith([
      makeIssue({
        entityId: 'note-2',
        entityType: 'note',
        message: 'priority must be normal',
        severity: 'error',
      }),
    ])

    const store = useScriptErrorStore()
    const row = wrapper.find('.issue-row')
    await row.trigger('click')

    expect(routerPush).toHaveBeenCalledWith('/entity/note/note-2')
    expect(store.current).toBeNull()
  })

  it('does nothing when a load-error row (no entity, no scriptError) is clicked', async () => {
    const wrapper = await mountWith([
      makeIssue({
        title: 'missing-script',
        message: 'Validation script load failed: file missing',
      }),
    ])

    const store = useScriptErrorStore()
    const row = wrapper.find('.issue-row')
    await row.trigger('click')

    expect(routerPush).not.toHaveBeenCalled()
    expect(store.current).toBeNull()
  })

  it('marks clickable rows with the .clickable class and inert rows without it', async () => {
    const wrapper = await mountWith([
      makeIssue({ scriptError: makeScriptError() }), // script-error: clickable
      makeIssue({ entityId: 'note-1', entityType: 'note' }), // entity: clickable
      makeIssue({ title: 'missing-script' }), // load-error: NOT clickable
    ])

    const rows = wrapper.findAll('.issue-row')
    expect(rows).toHaveLength(3)
    expect(rows[0].classes()).toContain('clickable')
    expect(rows[1].classes()).toContain('clickable')
    expect(rows[2].classes()).not.toContain('clickable')
  })

  it('renders an em-dash when entity cell or type cell is empty (preserves row separators)', async () => {
    const wrapper = await mountWith([
      makeIssue({
        title: 'broken-rule',
        message: 'Validation script failed',
        scriptError: makeScriptError(),
      }),
    ])

    const row = wrapper.find('.issue-row')
    // Entity cell falls back to em-dash element instead of empty <span>s.
    const empties = row.findAll('.entity-empty')
    expect(empties.length).toBeGreaterThanOrEqual(1)
    // The em-dash character should be present.
    expect(row.text()).toContain('—')
  })
})
