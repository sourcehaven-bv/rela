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

const analyzeMock = vi.fn<() => Promise<AnalyzeResult>>()
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

  it('opens ScriptErrorDialog (via store) when the message cell of a script-error row is clicked', async () => {
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

    // The message cell — not the row — owns the "why did this fire" reveal.
    await wrapper.find('.message-toggle').trigger('click')

    expect(store.current).toEqual(scriptError)
    expect(routerPush).not.toHaveBeenCalled()
  })

  it('navigates to the entity when the entity-title cell is clicked (message cell does not navigate)', async () => {
    const wrapper = await mountWith([
      makeIssue({
        entityId: 'note-2',
        entityType: 'note',
        message: 'priority must be normal',
        severity: 'error',
      }),
    ])

    const store = useScriptErrorStore()
    await wrapper.find('.entity-title').trigger('click')

    expect(routerPush).toHaveBeenCalledWith('/entity/note/note-2')
    expect(store.current).toBeNull()

    // A plain violation (no detail, no scriptError) has no interactive
    // message affordance, so nothing there navigates.
    expect(wrapper.find('.message-toggle').exists()).toBe(false)
  })

  it('does nothing when a load-error row (no entity, no scriptError, no detail) is clicked', async () => {
    const wrapper = await mountWith([
      makeIssue({
        title: 'missing-script',
        message: 'Validation script load failed: file missing',
      }),
    ])

    const store = useScriptErrorStore()
    // Neither cell is interactive on a load-error row.
    expect(wrapper.find('.entity-title').exists()).toBe(false)
    expect(wrapper.find('.message-toggle').exists()).toBe(false)
    await wrapper.find('.issue-row').trigger('click')

    expect(routerPush).not.toHaveBeenCalled()
    expect(store.current).toBeNull()
  })

  it('marks the entity title clickable only when the issue has an entity', async () => {
    const wrapper = await mountWith([
      makeIssue({ scriptError: makeScriptError() }), // script-error: no entity
      makeIssue({ entityId: 'note-1', entityType: 'note' }), // entity: navigable
      makeIssue({ title: 'missing-script' }), // load-error: no entity
    ])

    const rows = wrapper.findAll('.issue-row')
    expect(rows).toHaveLength(3)
    // Only the row with a real entity exposes a clickable title.
    expect(rows[0].find('.entity-title').exists()).toBe(false)
    expect(rows[1].find('.entity-title').classes()).toContain('clickable')
    expect(rows[2].find('.entity-title').exists()).toBe(false)
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

// GH#785: every warning counted in the summary badge must be visible on
// the page. Before the fix, the Duplicates and ID Gaps sections were
// summed into the badge but never rendered, producing a count > visible
// rows. These tests pin the rendering of those new cards.
describe('AnalyzeView section rendering (GH#785)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    routerPush.mockReset()
    analyzeMock.mockReset()
    const schema = useSchemaStore()
    schema.entityTypes.set('note', { label: 'Note' } as never)
  })

  async function mountWith(issues: AnalyzeIssue[]) {
    analyzeMock.mockResolvedValue(makeResult(issues))
    const wrapper = mount(AnalyzeView, { attachTo: document.body })
    await flushPromises()
    return wrapper
  }

  // Match by .check-title's leading text rather than substring-includes
  // on the whole card; the latter is brittle against future card-copy
  // changes ("Orphans card mentions 'duplicated entities'" would
  // collide with a substring of 'Duplicates').
  function findCard(wrapper: ReturnType<typeof mount>, label: string) {
    return wrapper
      .findAll('.check-card')
      .find((c) => c.find('.check-title').text().trim().startsWith(label))
  }

  it('renders a Duplicates card with clickable rows showing duplicate messages', async () => {
    const duplicateMessage = 'Duplicate title (shared by note-1, note-2)'
    const wrapper = await mountWith([
      makeIssue({
        entityId: 'note-1',
        entityType: 'note',
        message: duplicateMessage,
        severity: 'warning',
        checkType: 'Duplicates',
      }),
      makeIssue({
        entityId: 'note-2',
        entityType: 'note',
        message: duplicateMessage,
        severity: 'warning',
        checkType: 'Duplicates',
      }),
    ])

    const duplicates = findCard(wrapper, 'Duplicates')
    expect(duplicates).toBeTruthy()
    expect(duplicates!.find('.check-count').text()).toBe('2')

    const rows = duplicates!.findAll('.issue-row')
    expect(rows).toHaveLength(2)
    // Rows carry the duplicate message — pins the message-cell rendering,
    // not just the row count.
    expect(rows[0].text()).toContain(duplicateMessage)
    // Duplicates rows have real entities, so their title must be clickable.
    expect(rows[0].find('.entity-title').classes()).toContain('clickable')
  })

  it('renders an ID Gaps card with inert rows for each missing ID', async () => {
    const gapMessage = 'Missing ID: TKT-005'
    const wrapper = await mountWith([
      makeIssue({
        entityId: '',
        entityType: '',
        message: gapMessage,
        severity: 'warning',
        checkType: 'ID Gaps',
      }),
    ])

    const gaps = findCard(wrapper, 'ID Gaps')
    expect(gaps).toBeTruthy()
    expect(gaps!.find('.check-count').text()).toBe('1')

    const rows = gaps!.findAll('.issue-row')
    expect(rows).toHaveLength(1)
    // Gap rows have no entity, so there's no clickable title.
    expect(rows[0].find('.entity-title').exists()).toBe(false)
    expect(rows[0].text()).toContain(gapMessage)
  })

  it('summary badge total equals sum of visible card counts', async () => {
    const issues = [
      makeIssue({ entityId: 'n-1', entityType: 'note', severity: 'error', checkType: 'Properties' }),
      makeIssue({ entityId: 'n-2', entityType: 'note', severity: 'error', checkType: 'Cardinality' }),
      makeIssue({ entityId: 'n-3', entityType: 'note', severity: 'warning', checkType: 'Validations' }),
      makeIssue({ entityId: 'n-4', entityType: 'note', severity: 'warning', checkType: 'Orphans' }),
      makeIssue({ entityId: 'n-5', entityType: 'note', severity: 'warning', checkType: 'Duplicates' }),
      makeIssue({ entityId: '', entityType: '', severity: 'warning', checkType: 'ID Gaps' }),
    ]
    const wrapper = await mountWith(issues)

    // The invariant from GH#785: badge total == sum of per-card counts.
    // Derive both sides from the rendered DOM so the test fails the same
    // way the user-visible bug fails.
    const badgeErrors = Number(wrapper.find('.badge.error').text().trim().split(/\s+/)[0])
    const badgeWarnings = Number(wrapper.find('.badge.warning').text().trim().split(/\s+/)[0])
    const cardSum = wrapper
      .findAll('.check-card .check-count')
      .map((el) => Number(el.text()))
      .reduce((a, b) => a + b, 0)

    expect(badgeErrors + badgeWarnings).toBe(cardSum)
    // Sanity: must equal the seeded count (one issue per check type).
    expect(cardSum).toBe(issues.length)
  })
})

// The Entity cell renders two stacked lines: the entity's display title
// on top and the ID below. The backend supplies the title via
// AnalyzeIssue.title (DisplayTitle from the metamodel); the renderer must
// honour that field and fall back to the ID only when title is absent.
describe('AnalyzeView entity title rendering', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    routerPush.mockReset()
    analyzeMock.mockReset()
    const schema = useSchemaStore()
    schema.entityTypes.set('note', { label: 'Note' } as never)
  })

  async function mountWith(issues: AnalyzeIssue[]) {
    analyzeMock.mockResolvedValue(makeResult(issues))
    const wrapper = mount(AnalyzeView, { attachTo: document.body })
    await flushPromises()
    return wrapper
  }

  it('renders the backend-supplied title on the title line and ID below', async () => {
    const wrapper = await mountWith([
      makeIssue({
        entityId: 'note-2',
        entityType: 'note',
        title: 'My Important Note',
        message: 'priority must be normal',
        severity: 'error',
        checkType: 'Properties',
      }),
    ])

    const row = wrapper.find('.issue-row')
    expect(row.find('.entity-title').text()).toBe('My Important Note')
    expect(row.find('.entity-id').text()).toBe('note-2')
  })

  it('falls back to the entityId on the title line when title is empty', async () => {
    const wrapper = await mountWith([
      makeIssue({
        entityId: 'note-2',
        entityType: 'note',
        title: '',
        message: 'priority must be normal',
        severity: 'error',
        checkType: 'Properties',
      }),
    ])

    const row = wrapper.find('.issue-row')
    // Title line shows the raw ID (not a cosmetic transform of it).
    expect(row.find('.entity-title').text()).toBe('note-2')
    expect(row.find('.entity-id').text()).toBe('note-2')
  })

  it('falls back to the entityId on the title line when title is omitted', async () => {
    const wrapper = await mountWith([
      makeIssue({
        entityId: 'note-2',
        entityType: 'note',
        message: 'priority must be normal',
        severity: 'error',
        checkType: 'Properties',
      }),
    ])

    const row = wrapper.find('.issue-row')
    expect(row.find('.entity-title').text()).toBe('note-2')
    expect(row.find('.entity-id').text()).toBe('note-2')
  })

  it('falls back to the entityId on the title line when title is whitespace-only', async () => {
    const wrapper = await mountWith([
      makeIssue({
        entityId: 'note-2',
        entityType: 'note',
        title: '   ',
        message: 'priority must be normal',
        severity: 'error',
        checkType: 'Properties',
      }),
    ])

    const row = wrapper.find('.issue-row')
    expect(row.find('.entity-title').text()).toBe('note-2')
  })
})

// TKT-IL499B: content required-headers violations carry a `detail` list
// of missing headers. Clicking the message cell expands a full-width
// detail row listing them; clicking again collapses it.
describe('AnalyzeView missing-header detail', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    routerPush.mockReset()
    analyzeMock.mockReset()
    const schema = useSchemaStore()
    schema.entityTypes.set('ncr', { label: 'NCR' } as never)
  })

  async function mountWith(issues: AnalyzeIssue[]) {
    analyzeMock.mockResolvedValue(makeResult(issues))
    const wrapper = mount(AnalyzeView, { attachTo: document.body })
    await flushPromises()
    return wrapper
  }

  function detailIssue() {
    return makeIssue({
      entityId: 'NCR-001',
      entityType: 'ncr',
      title: 'Verklaring van Toepasselijkheid',
      message: 'NCR body moet standaardkoppen bevatten',
      severity: 'error',
      checkType: 'Validations',
      detail: ['## Beschrijving', '## Oorzaak'],
    })
  }

  it('toggles the detail row listing the missing headers on message click', async () => {
    const wrapper = await mountWith([detailIssue()])

    // Collapsed by default: no detail row.
    expect(wrapper.find('.issue-detail-row').exists()).toBe(false)

    const toggle = wrapper.find('.message-toggle')
    expect(toggle.attributes('aria-expanded')).toBe('false')

    await toggle.trigger('click')
    const detail = wrapper.find('.issue-detail-row')
    expect(detail.exists()).toBe(true)
    const items = detail.findAll('.detail-list li').map((li) => li.text())
    expect(items).toEqual(['## Beschrijving', '## Oorzaak'])
    expect(wrapper.find('.message-toggle').attributes('aria-expanded')).toBe('true')
    // Expanding detail must not navigate.
    expect(routerPush).not.toHaveBeenCalled()

    // Clicking again collapses.
    await wrapper.find('.message-toggle').trigger('click')
    expect(wrapper.find('.issue-detail-row').exists()).toBe(false)
  })

  it('is keyboard-operable: Enter on the message cell expands the detail', async () => {
    const wrapper = await mountWith([detailIssue()])
    await wrapper.find('.message-toggle').trigger('keydown.enter')
    expect(wrapper.find('.issue-detail-row').exists()).toBe(true)
  })

  it('renders two same-message rows on one entity as distinct, independently-expandable rows', async () => {
    // Two content rules sharing a Description, both violated by NCR-001,
    // produce identical entityId+message rows. They must not collide on
    // the row key (else Vue merges them and expand cross-links).
    const sharedMessage = 'NCR body moet standaardkoppen bevatten'
    const wrapper = await mountWith([
      makeIssue({
        entityId: 'NCR-001',
        entityType: 'ncr',
        message: sharedMessage,
        severity: 'error',
        checkType: 'Validations',
        detail: ['## Oorzaak'],
      }),
      makeIssue({
        entityId: 'NCR-001',
        entityType: 'ncr',
        message: sharedMessage,
        severity: 'error',
        checkType: 'Validations',
        detail: ['## Corrigerende maatregel'],
      }),
    ])

    // Scope to the table layout: IssuesTable also renders a mobile card
    // list (hidden via CSS on desktop) that duplicates the toggles.
    const toggles = wrapper.findAll('.issues-table .message-toggle')
    expect(toggles).toHaveLength(2)

    // Expanding the first must not expand the second.
    await toggles[0].trigger('click')
    const detailRows = wrapper.findAll('.issue-detail-row')
    expect(detailRows).toHaveLength(1)
    expect(detailRows[0].text()).toContain('## Oorzaak')
    expect(detailRows[0].text()).not.toContain('## Corrigerende maatregel')
  })

  it('shows no expand affordance when the issue has no detail', async () => {
    const wrapper = await mountWith([
      makeIssue({
        entityId: 'NCR-002',
        entityType: 'ncr',
        message: 'some other rule',
        severity: 'error',
        checkType: 'Validations',
      }),
    ])
    expect(wrapper.find('.message-toggle').exists()).toBe(false)
    expect(wrapper.find('.disclosure').exists()).toBe(false)
    expect(wrapper.find('.issue-detail-row').exists()).toBe(false)
  })
})
