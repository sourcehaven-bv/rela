import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, enableAutoUnmount } from '@vue/test-utils'

import FlowStepForm from './FlowStepForm.vue'
import type { FlowRun } from '@/api/flows'
import { useSchemaStore } from '@/stores'
import type { EntityType, FormConfig } from '@/types'
import { ApiError } from '@/api/errors'

const completeFlowStepMock = vi.fn()
vi.mock('@/api/flows', () => ({
  completeFlowStep: (...a: unknown[]) => completeFlowStepMock(...a),
}))
const getEntityMock = vi.fn()
const schema = () => useSchemaStore()
vi.mock('@/api/entities', () => ({
  getEntity: (...a: unknown[]) => getEntityMock(...a),
}))

// The review form: a checkbox, a reason only asked for when it is declined
// (and then required), a read-only name and a hidden note.
const reviewForm: FormConfig = {
  entity: 'person',
  fields: [
    { property: 'approved', label: 'Approve the hire', help: 'Tick when every check passed.' },
    {
      property: 'reason',
      widget: 'textarea',
      visible_when: 'form.approved != true',
      required_when: 'form.approved != true',
    },
    { property: 'name', readonly: true },
    { property: 'note', hidden: true },
  ],
}

function run(fields = ['approved', 'reason', 'name']): FlowRun {
  return {
    id: 'R1',
    flow: 'onboarding',
    flow_label: 'Onboarding',
    version: 'v1',
    subject: 'PER-1',
    state: 'review',
    state_label: 'Review',
    status: 'input_required',
    generation: 3,
    active: true,
    waiting: { you_hold: true, form: 'review', fields },
    log: [],
    _actions: { complete: true, cancel: false, retry: false },
  }
}

function mountForm(r = run()) {
  return mount(FlowStepForm, { props: { run: r, subjectType: 'person' } })
}

enableAutoUnmount(afterEach)

describe('FlowStepForm', () => {
  beforeEach(() => {
    completeFlowStepMock.mockReset()
    completeFlowStepMock.mockResolvedValue({
      written: [],
      moved: true,
      state: 'done',
      finished: true,
    })
    getEntityMock.mockReset()
    getEntityMock.mockResolvedValue({
      properties: { approved: false, reason: '', name: 'Ann', note: 'secret' },
    })
    const schema = useSchemaStore()
    schema.entityTypes.set('person', {
      label: 'Person',
      properties: {
        approved: { type: 'boolean', label: 'Approved' },
        reason: { type: 'string', label: 'Reason' },
        name: { type: 'string', label: 'Name' },
        note: { type: 'string', label: 'Note' },
      },
    } as unknown as EntityType)
    schema.forms.set('review', reviewForm)
  })

  it("renders the form's labels, help and widgets, and leaves hidden fields out", async () => {
    const wrapper = mountForm()
    await flushPromises()
    const text = wrapper.text()
    expect(text).toContain('Approve the hire')
    expect(text).toContain('Tick when every check passed.')
    expect(wrapper.find('textarea').exists()).toBe(true)
    expect(text).not.toContain('Note')
    expect(wrapper.find('input[type="checkbox"]').exists()).toBe(true)
  })

  it('shows a field only while its visible_when holds', async () => {
    const wrapper = mountForm()
    await flushPromises()
    expect(wrapper.find('textarea').exists()).toBe(true)
    await wrapper.find('input[type="checkbox"]').setValue(true)
    expect(wrapper.find('textarea').exists()).toBe(false)
  })

  it('refuses to send while a required_when field is empty', async () => {
    const wrapper = mountForm()
    await flushPromises()
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(completeFlowStepMock).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('required')

    await wrapper.find('textarea').setValue('No references')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(completeFlowStepMock).toHaveBeenCalledWith('R1', 3, { reason: 'No references' })
  })

  it('sends neither a field visible_when hides nor a read-only one', async () => {
    const wrapper = mountForm()
    await flushPromises()
    await wrapper.find('textarea').setValue('typed before approving')
    await wrapper.find('input[type="checkbox"]').setValue(true)
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(completeFlowStepMock).toHaveBeenCalledWith('R1', 3, { approved: true })
  })

  it('shows a field outside waiting.fields read-only', async () => {
    const wrapper = mountForm(run(['reason', 'name']))
    await flushPromises()
    expect(wrapper.find('input[type="checkbox"]').attributes('disabled')).toBeDefined()
  })

  it('shows why a step whose form cannot be used and offers no Complete', async () => {
    const r = run()
    r.waiting = { ...r.waiting!, form_problem: 'data-entry.yaml declares no form "review"' }
    const wrapper = mountForm(r)
    await flushPromises()
    expect(wrapper.find('[data-testid="flow-step-form-unusable"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="flow-complete"]').exists()).toBe(false)
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(completeFlowStepMock).not.toHaveBeenCalled()
  })

  it('offers no Complete when the schema store does not know the form', async () => {
    const r = run()
    r.waiting = { ...r.waiting!, form: 'gone' }
    const wrapper = mountForm(r)
    await flushPromises()
    expect(wrapper.find('[data-testid="flow-step-form-unusable"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="flow-complete"]').exists()).toBe(false)
  })

  it('sends no value the user left as it was', async () => {
    getEntityMock.mockResolvedValue({
      properties: { approved: false, reason: 'Already given', name: 'Ann' },
    })
    const wrapper = mountForm()
    await flushPromises()
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(completeFlowStepMock).toHaveBeenCalledWith('R1', 3, {})
  })

  it('starts an empty editable field at its default and sends it', async () => {
    schema().forms.set('review', {
      ...reviewForm,
      fields: (reviewForm.fields ?? []).map((f) =>
        f.property === 'reason' ? { ...f, default: 'No references' } : f
      ),
    })
    const wrapper = mountForm()
    await flushPromises()
    expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toBe('No references')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(completeFlowStepMock).toHaveBeenCalledWith('R1', 3, { reason: 'No references' })
  })

  it('shows why the values could not be loaded', async () => {
    getEntityMock.mockRejectedValue(new Error('boom'))
    const wrapper = mountForm()
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
  })

  it('reports a run that moved on as stale', async () => {
    completeFlowStepMock.mockRejectedValue(
      new ApiError('HTTP 409', { kind: 'http', status: 409, original: null })
    )
    const wrapper = mountForm()
    await flushPromises()
    await wrapper.find('textarea').setValue('No references')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(wrapper.emitted('stale')).toHaveLength(1)
    expect(wrapper.emitted('completed')).toBeUndefined()
  })
})
