import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, RouterLinkStub, enableAutoUnmount } from '@vue/test-utils'

import FlowRunCard from './FlowRunCard.vue'
import { ApiError } from '@/api/errors'
import type { FlowRun } from '@/api/flows'
import { useSchemaStore, useUIStore } from '@/stores'
import type { EntityType, FormConfig } from '@/types'

const getFlowRunMock = vi.fn()
const completeFlowStepMock = vi.fn()
const cancelFlowRunMock = vi.fn()
const retryFlowRunMock = vi.fn()
vi.mock('@/api/flows', () => ({
  getFlowRun: (...a: unknown[]) => getFlowRunMock(...a),
  completeFlowStep: (...a: unknown[]) => completeFlowStepMock(...a),
  cancelFlowRun: (...a: unknown[]) => cancelFlowRunMock(...a),
  retryFlowRun: (...a: unknown[]) => retryFlowRunMock(...a),
}))
const getEntityMock = vi.fn()
vi.mock('@/api/entities', () => ({
  getEntity: (...a: unknown[]) => getEntityMock(...a),
}))
const confirmMock = vi.fn()
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: confirmMock }),
}))
vi.mock('@/utils/markdown', () => ({ renderMermaidDiagrams: vi.fn() }))

function run(over: Partial<FlowRun> = {}): FlowRun {
  return {
    id: 'R1',
    flow: 'onboarding',
    flow_label: 'Onboarding',
    version: 'v1',
    subject: 'PER-1',
    state: 'collect',
    state_label: 'Collect documents',
    status: 'input_required',
    generation: 3,
    active: true,
    waiting: {
      roles: ['hr'],
      you_hold: true,
      form: 'collect_documents',
      fields: ['documents'],
      timers: [{ name: 'stalled', label: 'Stalled', to: 'escalated', due: '2026-02-01T10:00:00Z' }],
      branches: [{ to: 'review', guard: 'entity.documents == true', holds: false }],
    },
    log: [{ at: '2026-01-01T10:00:00Z', state: 'collect', via: 'start' }],
    _actions: { complete: true, manage: true, cancel: false, retry: false },
    ...over,
  }
}

function mountCard(r: FlowRun) {
  return mount(FlowRunCard, {
    props: { run: r, subjectType: 'person' },
    global: { stubs: { RouterLink: RouterLinkStub } },
  })
}

enableAutoUnmount(afterEach)

describe('FlowRunCard', () => {
  beforeEach(() => {
    getFlowRunMock.mockReset()
    completeFlowStepMock.mockReset()
    cancelFlowRunMock.mockReset()
    retryFlowRunMock.mockReset()
    getEntityMock.mockReset()
    confirmMock.mockReset()
    getEntityMock.mockResolvedValue({ properties: { documents: false, name: 'Ann' } })
    useSchemaStore().entityTypes.set('person', {
      label: 'Person',
      properties: {
        documents: { type: 'boolean', label: 'Documents' },
        name: { type: 'string' },
      },
    } as unknown as EntityType)
    const form: FormConfig = { entity: 'person', fields: [{ property: 'documents' }] }
    useSchemaStore().forms.set('collect_documents', form)
  })

  it('links the title to the run page only for a reader who manages the run', async () => {
    const mountLinked = (r: FlowRun) =>
      mount(FlowRunCard, {
        props: { run: r, subjectType: 'person', linkTitle: true },
        global: { stubs: { RouterLink: RouterLinkStub } },
      })
    const managed = mountLinked(run())
    await flushPromises()
    expect(managed.findComponent(RouterLinkStub).props('to')).toBe('/flows/runs/R1')

    const other = mountLinked(
      run({ _actions: { complete: true, manage: false, cancel: false, retry: false } })
    )
    await flushPromises()
    expect(other.find('.flow-run-card__title').text()).toBe('Onboarding')
    expect(other.findComponent(RouterLinkStub).exists()).toBe(false)
    expect(other.find('[data-testid="flow-run-status"]').exists()).toBe(true)
  })

  it('leaves the flow label out when the page already names the flow', async () => {
    const wrapper = mount(FlowRunCard, {
      props: { run: run(), subjectType: 'person', hideTitle: true },
      global: { stubs: { RouterLink: RouterLinkStub } },
    })
    await flushPromises()
    expect(wrapper.find('.flow-run-card__title').exists()).toBe(false)
    expect(mountCard(run()).find('.flow-run-card__title').exists()).toBe(true)
  })

  it('says why the run waits: roles, branches with their verdict and timers', async () => {
    const wrapper = mountCard(run())
    await flushPromises()
    expect(wrapper.find('[data-testid="flow-waiting-roles"]').text()).toContain('hr')
    const branch = wrapper.find('[data-testid="flow-branch"]')
    expect(branch.text()).toContain('review')
    expect(branch.text()).toContain('entity.documents == true')
    expect(wrapper.findAll('[data-testid="flow-timer"]')).toHaveLength(1)
  })

  it('says where a job step is, and why it failed', async () => {
    const waiting = (job: NonNullable<NonNullable<FlowRun['waiting']>['pending']>['job']) => ({
      you_hold: false,
      pending: { failures: 0, job },
    })
    let wrapper = mountCard(run({ waiting: waiting({ phase: 'started', failed: 1, attempts: 3 }) }))
    await flushPromises()
    let line = wrapper.find('[data-testid="flow-waiting-job"]')
    expect(line.text()).toContain('attempt 2 of 3')
    expect(line.attributes('role')).toBeUndefined()
    expect(wrapper.find('[data-testid="flow-waiting-pending"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('Waits for no one')

    wrapper = mountCard(
      run({ waiting: waiting({ phase: 'failed', failed: 1, attempts: 3, fail_kind: 'permanent' }) })
    )
    await flushPromises()
    line = wrapper.find('[data-testid="flow-waiting-job"]')
    expect(line.text()).toContain('another attempt cannot fix the error')
    expect(line.text()).toContain('after 1 failed attempt:')
    expect(line.attributes('role')).toBe('alert')
    // A failed step does not wait for anyone: it waits for a retry.
    expect(wrapper.text()).not.toContain('Waits for no one')

    wrapper = mountCard(run({ waiting: waiting({ phase: 'failed', failed: 3, attempts: 3 }) }))
    await flushPromises()
    expect(wrapper.find('[data-testid="flow-waiting-job"]').text()).toContain(
      'after 3 failed attempts:'
    )
  })

  it('shows the step form only when complete is allowed', async () => {
    const wrapper = mountCard(run({ _actions: { complete: false, cancel: false, retry: false } }))
    await flushPromises()
    expect(wrapper.find('[data-testid="flow-step-form"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="flow-cancel"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="flow-retry"]').exists()).toBe(false)
  })

  it('completes the step with the generation and only the changed properties', async () => {
    completeFlowStepMock.mockResolvedValue({
      written: ['documents'],
      moved: true,
      state: 'review',
      finished: false,
    })
    getFlowRunMock.mockResolvedValue(run({ generation: 4, state: 'review', state_label: 'Review' }))
    const wrapper = mountCard(run())
    await flushPromises()

    expect(getEntityMock).toHaveBeenCalledWith('person', 'PER-1')
    const box = wrapper.find('[data-testid="flow-step-form"] input[type="checkbox"]')
    await box.setValue(true)
    await wrapper.find('[data-testid="flow-step-form"]').trigger('submit')
    await flushPromises()

    expect(completeFlowStepMock).toHaveBeenCalledWith('R1', 3, { documents: true })
    expect(getFlowRunMock).toHaveBeenCalledWith('R1', expect.any(AbortSignal))
    expect(wrapper.find('[data-testid="flow-run-state"]').text()).toBe('Review')
    expect(wrapper.emitted('changed')).toHaveLength(1)
  })

  it('warns when no condition of the next step holds', async () => {
    completeFlowStepMock.mockResolvedValue({
      written: ['documents'],
      moved: false,
      state: 'collect',
      finished: false,
      problem: 'blocked',
    })
    getFlowRunMock.mockResolvedValue(run())
    const wrapper = mountCard(run())
    await flushPromises()
    expect(wrapper.find('[data-testid="flow-step-blocked"]').exists()).toBe(false)

    await wrapper.find('[data-testid="flow-step-form"]').trigger('submit')
    await flushPromises()

    // The form says it where the reader is, not in a toast.
    expect(useUIStore().toasts).toHaveLength(0)
    expect(wrapper.find('[data-testid="flow-step-blocked"]').text()).toContain(
      'no condition of the next step holds'
    )
    // The fields stay on screen, and the run's reload does not read them again.
    expect(wrapper.findAll('[data-testid="flow-step-field"]')).toHaveLength(1)
    expect(getEntityMock).toHaveBeenCalledTimes(1)
  })

  it('marks the fields the condition reads as required, and keeps feedback at the fields', async () => {
    const card = () =>
      run({
        waiting: {
          roles: ['hr'],
          you_hold: true,
          form: 'docs_and_name',
          fields: ['documents', 'name'],
          branches: [
            {
              to: 'review',
              guard: 'entity.documents == true',
              holds: false,
              fields: ['documents'],
            },
          ],
        },
      })
    completeFlowStepMock.mockResolvedValue({
      written: ['name'],
      moved: false,
      state: 'collect',
      finished: false,
      problem: 'blocked',
      warnings: [{ field: 'name', message: 'Write capitals only.' }],
    })
    const form: FormConfig = {
      entity: 'person',
      fields: [{ property: 'documents' }, { property: 'name' }],
    }
    useSchemaStore().forms.set('docs_and_name', form)
    // The run as read after the step: a fresh object.
    getFlowRunMock.mockResolvedValue(card())
    // Attached, so focus can move.
    const wrapper = mount(FlowRunCard, {
      props: { run: card(), subjectType: 'person' },
      global: { stubs: { RouterLink: RouterLinkStub } },
      attachTo: document.body,
    })
    await flushPromises()

    const [docs, name] = wrapper.findAll('[data-testid="flow-step-field"]')
    expect(docs.find('.required').exists()).toBe(true)
    expect(name.find('.required').exists()).toBe(false)
    // The browser does not hold a marked field back: the server decides.
    expect(wrapper.find('form').attributes('novalidate')).toBeDefined()

    await name.find('input').setValue('Bo')
    await wrapper.find('[data-testid="flow-step-form"]').trigger('submit')
    await flushPromises()

    // A pattern warning shows at its field, and the blocked notice points
    // at the marked fields; the first flagged field gets focus.
    let after = wrapper.findAll('[data-testid="flow-step-field"]')
    expect(after[1].text()).toContain('Write capitals only.')
    expect(after[0].text()).not.toContain('Write capitals only.')
    expect(wrapper.find('[data-testid="flow-step-blocked"]').text()).toContain('marked *')
    expect(after[0].element.contains(document.activeElement)).toBe(true)
    expect(useUIStore().toasts).toHaveLength(0)

    // Editing the field takes its warning away.
    await after[1].find('input').setValue('BO')
    expect(wrapper.text()).not.toContain('Write capitals only.')

    // A refused write: a rule shows at the first field of the form it
    // reads, a failure without a field of the form above the button.
    completeFlowStepMock.mockRejectedValue(
      new ApiError('HTTP 422', {
        kind: 'http',
        status: 422,
        problem: {
          type: 'about:blank',
          title: 'Unprocessable',
          status: 422,
          errors: [
            {
              source: { pointer: '/properties' },
              code: 'rule',
              fields: ['stage', 'documents'],
              message: 'Check the documents first.',
              detail: 'validation rule "r" is not satisfied: Check the documents first.',
            },
            { source: { pointer: '/properties' }, code: 'rule', detail: 'Something else.' },
          ],
        },
        original: null,
      })
    )
    await after[0].find('input').setValue(true)
    // An expected refusal is not an application error (BUG-UZGMHIK).
    const logged = vi.spyOn(console, 'error')
    await wrapper.find('[data-testid="flow-step-form"]').trigger('submit')
    await flushPromises()
    expect(logged).not.toHaveBeenCalled()
    logged.mockRestore()
    after = wrapper.findAll('[data-testid="flow-step-field"]')
    expect(after[0].text()).toContain('Check the documents first.')
    expect(after[0].text()).not.toContain('validation rule')
    expect(wrapper.find('[data-testid="flow-step-refused"]').text()).toBe('Something else.')
    expect(wrapper.find('[data-testid="flow-step-blocked"]').exists()).toBe(false)
    expect(useUIStore().toasts).toHaveLength(0)
  })

  it('says at each empty required field that it is required when the step is blocked', async () => {
    getEntityMock.mockResolvedValue({ properties: { name: 'Ann', terms: false } })
    const card = () =>
      run({
        waiting: {
          roles: ['hr'],
          you_hold: true,
          form: 'terms_form',
          fields: ['country', 'terms', 'name'],
          branches: [
            {
              to: 'review',
              guard: 'entity.terms == true and entity.country ~= nil and entity.name ~= nil',
              holds: false,
              fields: ['country', 'terms', 'name'],
            },
          ],
        },
      })
    useSchemaStore().entityTypes.set('person', {
      label: 'Person',
      properties: {
        country: { type: 'string', label: 'Country' },
        terms: { type: 'boolean', label: 'Terms accepted' },
        name: { type: 'string', label: 'Name' },
      },
    } as unknown as EntityType)
    const form: FormConfig = {
      entity: 'person',
      fields: [{ property: 'country' }, { property: 'terms' }, { property: 'name' }],
    }
    useSchemaStore().forms.set('terms_form', form)
    completeFlowStepMock.mockResolvedValue({
      written: [],
      moved: false,
      state: 'collect',
      finished: false,
      problem: 'blocked',
    })
    getFlowRunMock.mockResolvedValue(card())
    const wrapper = mountCard(card())
    await flushPromises()

    await wrapper.find('[data-testid="flow-step-form"]').trigger('submit')
    await flushPromises()

    const [country, terms, name] = wrapper.findAll('[data-testid="flow-step-field"]')
    expect(country.text()).toContain('This field is required')
    // An unticked checkbox the condition reads is empty too.
    expect(terms.text()).toContain('This field is required')
    expect(name.text()).not.toContain('This field is required')

    // Filling the field takes its message away.
    await country.find('input').setValue('NL')
    expect(wrapper.findAll('[data-testid="flow-step-field"]')[0].text()).not.toContain(
      'This field is required'
    )
  })

  it("says the flow's own text to a reader of a failed step who does not manage it", async () => {
    const failed = (failed_message?: string) =>
      run({
        failed_message,
        waiting: {
          you_hold: false,
          pending: { failures: 0, job: { phase: 'failed', failed: 3, attempts: 3 } },
        },
        _actions: { complete: false, manage: false, cancel: false, retry: false },
      })
    let wrapper = mountCard(failed('Mail support@example.com.'))
    await flushPromises()
    expect(wrapper.find('[data-testid="flow-waiting-plain"]').text()).toBe(
      'Mail support@example.com.'
    )

    wrapper = mountCard(failed())
    await flushPromises()
    const line = wrapper.find('[data-testid="flow-waiting-plain"]').text()
    expect(line).toContain('Something went wrong at this step')
    expect(line).not.toContain('retry')
  })

  it('shows the last job error and the retry state to a manager', async () => {
    const wrapper = mountCard(
      run({
        waiting: {
          you_hold: false,
          pending: {
            failures: 0,
            next_attempt: '2026-02-01T10:00:00Z',
            job: { phase: 'waiting', failed: 1, attempts: 5, error: 'webhook answered 503' },
          },
        },
      })
    )
    await flushPromises()
    expect(wrapper.find('[data-testid="flow-waiting-job"]').text()).toContain(
      'is retried: attempt 2 of 5'
    )
    expect(wrapper.find('[data-testid="flow-waiting-job-error"]').text()).toContain(
      'webhook answered 503'
    )
    // A step that retries does not claim to wait for no one.
    expect(wrapper.text()).not.toContain('Waits for no one')
  })

  it('reloads the run when the step answers that the run moved on', async () => {
    completeFlowStepMock.mockRejectedValue(
      new ApiError('HTTP 409', { kind: 'http', status: 409, original: null })
    )
    getFlowRunMock.mockResolvedValue(
      run({ generation: 5, state: 'escalated', state_label: 'Escalated' })
    )
    const wrapper = mountCard(run())
    await flushPromises()

    await wrapper.find('[data-testid="flow-step-form"]').trigger('submit')
    await flushPromises()

    expect(getFlowRunMock).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-testid="flow-run-state"]').text()).toBe('Escalated')
    expect(wrapper.emitted('changed')).toHaveLength(1)
  })

  it('cancels after confirmation, and retries, when the actions allow it', async () => {
    confirmMock.mockResolvedValue(true)
    cancelFlowRunMock.mockResolvedValue(undefined)
    retryFlowRunMock.mockResolvedValue(undefined)
    getFlowRunMock.mockResolvedValue(
      run({ active: false, status: 'cancelled', waiting: undefined })
    )
    const wrapper = mountCard(
      run({ _actions: { complete: false, manage: true, cancel: true, retry: true } })
    )
    await flushPromises()

    await wrapper.find('[data-testid="flow-retry"]').trigger('click')
    await flushPromises()
    expect(retryFlowRunMock).toHaveBeenCalledWith('R1')

    // The reload answered a cancelled run without actions; mount again to cancel.
    const again = mountCard(
      run({ _actions: { complete: false, manage: true, cancel: true, retry: false } })
    )
    await flushPromises()
    await again.find('[data-testid="flow-cancel"]').trigger('click')
    await flushPromises()
    expect(confirmMock).toHaveBeenCalled()
    expect(cancelFlowRunMock).toHaveBeenCalledWith('R1')
  })

  it('does not cancel when the confirmation is declined', async () => {
    confirmMock.mockResolvedValue(false)
    const wrapper = mountCard(
      run({ _actions: { complete: false, manage: true, cancel: true, retry: false } })
    )
    await flushPromises()
    await wrapper.find('[data-testid="flow-cancel"]').trigger('click')
    await flushPromises()
    expect(cancelFlowRunMock).not.toHaveBeenCalled()
  })

  it('shows each parallel branch with its own step, completed at the branch generation', async () => {
    completeFlowStepMock.mockResolvedValue({
      written: ['documents'],
      moved: true,
      state: 'joined',
      finished: false,
    })
    getFlowRunMock.mockResolvedValue(run({ state: 'joined', waiting: undefined }))
    const forked = run({
      state: 'joined',
      state_label: 'Joined',
      generation: 0,
      waiting: { you_hold: false },
      _actions: { complete: false, manage: true, cancel: false, retry: false },
      parallel: [
        {
          branch: 'it',
          state: 'it',
          state_label: 'IT',
          status: 'input_required',
          generation: 2,
          waiting: {
            roles: ['hr'],
            you_hold: true,
            form: 'collect_documents',
            fields: ['documents'],
          },
          complete: true,
        },
        {
          branch: 'legal',
          state: 'joined',
          state_label: 'Joined',
          status: 'working',
          generation: 3,
          arrived: true,
          complete: false,
        },
      ],
    })
    const wrapper = mountCard(forked)
    await flushPromises()

    // The join's own reason is not shown: the branches say why the run waits.
    expect(wrapper.find('[data-testid="flow-waiting-roles"]').text()).toContain('hr')
    const branches = wrapper.findAll('[data-testid="flow-run-branch"]')
    expect(branches).toHaveLength(2)
    expect(branches[1].find('[data-testid="flow-branch-arrived"]').exists()).toBe(true)
    expect(branches[1].find('[data-testid="flow-step-form"]').exists()).toBe(false)

    const form = branches[0].find('[data-testid="flow-step-form"]')
    await form.find('input[type="checkbox"]').setValue(true)
    await form.trigger('submit')
    await flushPromises()
    expect(completeFlowStepMock).toHaveBeenCalledWith('R1', 2, { documents: true })
  })

  it('reads the detail of an active run that came without its timeline', async () => {
    getFlowRunMock.mockResolvedValue(run())
    mountCard(run({ log: undefined }))
    await flushPromises()
    expect(getFlowRunMock).toHaveBeenCalledWith('R1', expect.any(AbortSignal))
  })

  it('shows a manager the timeline and diagram of a finished run, with no controls', async () => {
    const wrapper = mountCard(
      run({
        active: false,
        state: 'finished',
        status: 'completed',
        waiting: undefined,
        diagram: 'stateDiagram-v2\n  class finished current',
        log: [
          { at: '2026-01-01T10:00:00Z', state: 'collect', via: 'start' },
          { at: '2026-01-02T10:00:00Z', state: 'finished', from: 'collect', via: 'change' },
        ],
        _actions: { complete: false, manage: true, cancel: false, retry: false },
      })
    )
    await flushPromises()
    expect(getFlowRunMock).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Timeline')
    expect(wrapper.text()).toContain('Diagram')
    expect(wrapper.find('[data-testid="flow-cancel"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="flow-retry"]').exists()).toBe(false)
  })
})

describe('FlowRunCard for a reader who does not manage the flow', () => {
  const doer = { complete: true, manage: false, cancel: false, retry: false }

  beforeEach(() => {
    getFlowRunMock.mockReset()
    getEntityMock.mockReset()
    getEntityMock.mockResolvedValue({ properties: {} })
    useSchemaStore().entityTypes.set('account', {
      label: 'Account',
      properties: {
        company_name: { type: 'string', label: 'Company' },
        vat_number: { type: 'string' },
      },
    } as unknown as EntityType)
    const form: FormConfig = {
      entity: 'account',
      fields: [{ property: 'company_name' }, { property: 'vat_number' }],
    }
    useSchemaStore().forms.set('signup', form)
  })

  function mountDoer(r: FlowRun) {
    return mount(FlowRunCard, {
      props: { run: r, subjectType: 'account' },
      global: { stubs: { RouterLink: RouterLinkStub } },
    })
  }

  it('shows the step form and none of how the flow works', async () => {
    const wrapper = mountDoer(
      run({
        _actions: doer,
        waiting: { ...run().waiting!, form: 'signup', fields: ['company_name', 'vat_number'] },
      })
    )
    await flushPromises()
    expect(wrapper.find('[data-testid="flow-step-form"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="flow-waiting"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('entity.documents == true')
    expect(wrapper.text()).not.toContain('hr')
    expect(wrapper.find('[data-testid="flow-branch"]').exists()).toBe(false)
    expect(wrapper.find('details').exists()).toBe(false)
  })

  it('labels each field from the schema, or with its name, one per row', async () => {
    const wrapper = mountDoer(
      run({
        _actions: doer,
        waiting: { you_hold: true, form: 'signup', fields: ['company_name', 'vat_number'] },
      })
    )
    await flushPromises()
    const fields = wrapper.find('.flow-step-form__fields')
    expect(fields.classes()).not.toContain('properties-list')
    const labels = fields.findAll('label').map((l) => l.text().replace(/\*/g, '').trim())
    expect(labels).toEqual(['Company', 'vat_number'])
  })

  it('says in one line whom the step waits for, or that it is processed or stuck', async () => {
    const cases: [NonNullable<FlowRun['waiting']>, string, string | undefined][] = [
      [{ roles: ['staff'], you_hold: false }, 'waiting for someone else', undefined],
      // The reader holds the role but has no form here: nothing to do.
      [{ roles: ['staff'], you_hold: true }, 'This step is waiting.', undefined],
      // A timer and no role.
      [
        { you_hold: false, timers: [{ name: 'reminder', to: 'done' }] },
        'This step is waiting.',
        undefined,
      ],
      [{ you_hold: false, pending: { failures: 0 } }, 'being processed', undefined],
      [
        {
          you_hold: false,
          pending: { failures: 3, job: { phase: 'failed', failed: 3, attempts: 3 } },
        },
        'Something went wrong at this step',
        'alert',
      ],
    ]
    for (const [waiting, text, role] of cases) {
      getFlowRunMock.mockResolvedValue(run({ _actions: { ...doer, complete: false }, waiting }))
      const wrapper = mountDoer(run({ _actions: { ...doer, complete: false }, waiting }))
      await flushPromises()
      const line = wrapper.find('[data-testid="flow-waiting-plain"]')
      expect(line.text()).toContain(text)
      expect(line.attributes('role')).toBe(role)
      expect(wrapper.find('[data-testid="flow-waiting-roles"]').exists()).toBe(false)
      expect(wrapper.find('[data-testid="flow-waiting-job"]').exists()).toBe(false)
      expect(wrapper.find('[data-testid="flow-timer"]').exists()).toBe(false)
    }
  })
})
