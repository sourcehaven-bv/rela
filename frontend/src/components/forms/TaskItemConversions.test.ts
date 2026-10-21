// TKT-3IBLIP: the host side of a task-item conversion. The editor and the
// server are stubbed; these tests pin the flow between them: which offers
// count, when the dialog opens, what is sent, and what the editor is told.
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import TaskItemConversions, { taskItemConvertLabel } from './TaskItemConversions.vue'
import { useSchemaStore } from '@/stores/schema'
import { useUIStore } from '@/stores/ui'
import type { TaskItemConversionOffer } from '@/types'

vi.mock('@/api/taskItemConversions', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/api/taskItemConversions')>()),
  previewTaskItemConversion: vi.fn(),
  convertTaskItem: vi.fn(),
}))

import { convertTaskItem, previewTaskItemConversion } from '@/api/taskItemConversions'
import { ApiError } from '@/api/errors'

const preview = previewTaskItemConversion as ReturnType<typeof vi.fn>
const convert = convertTaskItem as ReturnType<typeof vi.fn>

const subtask: TaskItemConversionOffer = {
  name: 'subtask',
  label: 'Convert to task',
  to: 'task',
  item: 'link',
  allowed: true,
}
const blocker: TaskItemConversionOffer = {
  ...subtask,
  name: 'blocker',
  label: 'Convert to blocker',
}
const denied: TaskItemConversionOffer = { ...subtask, name: 'secret', allowed: false, reason: 'no' }

const item = { pos: 3, text: 'Write the migration' }

function editorStub(applied = true) {
  return {
    applyTaskItemResult: vi.fn().mockReturnValue(applied),
    markTaskItemBusy: vi.fn(() => 7),
    clearTaskItemBusy: vi.fn(),
  }
}

function mountHost(offers: TaskItemConversionOffer[], editor = editorStub()) {
  const wrapper = mount(TaskItemConversions, {
    props: { offers, source: { id: 'TKT-1', face: 'draft' }, editor: () => editor },
    attachTo: document.body,
  })
  return { wrapper, editor }
}

function start(w: ReturnType<typeof mountHost>['wrapper']) {
  ;(w.vm as unknown as { start: (t: typeof item) => void }).start(item)
}

function dialog(): HTMLElement | null {
  return document.querySelector('.task-item-dialog')
}

beforeEach(() => {
  setActivePinia(createPinia())
  preview.mockReset()
  convert.mockReset()
  document.body.innerHTML = ''
  useSchemaStore().entityTypes.set('task', {
    name: 'task',
    label: 'Task',
    properties: { owner: { type: 'string', label: 'Owner' } },
  } as never)
})

describe('taskItemConvertLabel', () => {
  it.each([
    ['none', undefined, ''],
    ['only denied', [denied], ''],
    ['one allowed', [subtask, denied], 'Convert to task'],
    ['several allowed', [subtask, blocker], 'Convert task item'],
  ])('%s', (_name, offers, want) => {
    expect(taskItemConvertLabel(offers)).toBe(want)
  })
})

describe('TaskItemConversions', () => {
  it('converts straight away when one recipe needs nothing more', async () => {
    preview.mockResolvedValue({ target_type: 'task', required: [] })
    convert.mockResolvedValue({
      definition: 'subtask',
      entity_id: 'TASK-7',
      entity_type: 'task',
      title: 'Write the migration',
      item: 'remove',
    })
    const { wrapper, editor } = mountHost([subtask, denied])
    start(wrapper)
    await flushPromises()
    expect(dialog()).toBeNull()
    const sent = { source_id: 'TKT-1', source_face: 'draft', text: 'Write the migration' }
    expect(preview).toHaveBeenCalledWith('subtask', sent)
    expect(convert).toHaveBeenCalledWith('subtask', sent)
    expect(editor.applyTaskItemResult).toHaveBeenCalledWith(7, 'Write the migration', 'remove', {
      id: 'TASK-7',
      title: 'Write the migration',
      entityType: 'task',
    })
    expect(editor.markTaskItemBusy).toHaveBeenCalledWith(item)
    expect(editor.clearTaskItemBusy).toHaveBeenCalledWith(7)
  })

  it('asks for required fields, then sends them', async () => {
    preview.mockResolvedValue({ target_type: 'task', required: ['owner'] })
    convert.mockResolvedValue({
      definition: 'subtask',
      entity_id: 'TASK-7',
      entity_type: 'task',
      title: '',
      item: 'link',
    })
    const { wrapper, editor } = mountHost([subtask])
    start(wrapper)
    await flushPromises()
    expect(convert).not.toHaveBeenCalled()
    const input = dialog()?.querySelector('input') as HTMLInputElement
    expect(dialog()?.textContent).toContain('Owner')
    input.value = 'bob'
    input.dispatchEvent(new Event('input'))
    dialog()?.querySelector('form')?.dispatchEvent(new Event('submit'))
    await flushPromises()
    expect(convert).toHaveBeenCalledWith('subtask', {
      source_id: 'TKT-1',
      source_face: 'draft',
      text: 'Write the migration',
      properties: { owner: 'bob' },
    })
    expect(editor.applyTaskItemResult).toHaveBeenCalled()
    expect(dialog()).toBeNull()
  })

  it('offers a choice when several recipes are allowed', async () => {
    preview.mockResolvedValue({ target_type: 'task', required: ['owner'] })
    const { wrapper } = mountHost([subtask, blocker, denied])
    start(wrapper)
    await flushPromises()
    const choices = Array.from(dialog()?.querySelectorAll('.task-item-choices button') ?? [])
    const label = (b: Element) => b.querySelector('.rl-button__label')?.textContent?.trim()
    expect(choices.map(label)).toEqual(['Convert to task', 'Convert to blocker'])
    ;(choices[1] as HTMLButtonElement).click()
    await flushPromises()
    expect(preview).toHaveBeenCalledWith('blocker', expect.anything())
  })

  it('shows a refusal in the dialog and keeps it open', async () => {
    preview.mockResolvedValue({ target_type: 'task', required: ['owner'] })
    convert.mockRejectedValue(new Error('Not permitted'))
    const { wrapper, editor } = mountHost([subtask])
    start(wrapper)
    await flushPromises()
    dialog()?.querySelector('form')?.dispatchEvent(new Event('submit'))
    await flushPromises()
    expect(dialog()?.querySelector('[role="alert"]')?.textContent).toContain('Not permitted')
    expect(editor.applyTaskItemResult).not.toHaveBeenCalled()
  })

  it('reports a refused preflight as a toast when no dialog is open', async () => {
    preview.mockRejectedValue(new Error('Not permitted'))
    const { wrapper } = mountHost([subtask])
    const ui = useUIStore()
    const error = vi.spyOn(ui, 'error')
    start(wrapper)
    await flushPromises()
    expect(error).toHaveBeenCalledWith(expect.stringContaining('Not permitted'))
  })

  it('warns when the item changed before the result arrived', async () => {
    preview.mockResolvedValue({ target_type: 'task', required: [] })
    convert.mockResolvedValue({
      definition: 'subtask',
      entity_id: 'TASK-7',
      entity_type: 'task',
      title: '',
      item: 'link',
    })
    const { wrapper } = mountHost([subtask], editorStub(false))
    const ui = useUIStore()
    const warning = vi.spyOn(ui, 'warning')
    start(wrapper)
    await flushPromises()
    expect(warning).toHaveBeenCalledWith(expect.stringContaining('TASK-7'))
  })

  it('reports the warnings of a conversion that succeeded', async () => {
    preview.mockResolvedValue({ target_type: 'task', required: [] })
    convert.mockResolvedValue({
      definition: 'subtask',
      entity_id: 'TASK-7',
      entity_type: 'task',
      title: '',
      item: 'link',
      warnings: [{ code: 'follow_up_failed', path: '', detail: 'x' }],
    })
    const { wrapper } = mountHost([subtask])
    const ui = useUIStore()
    const warning = vi.spyOn(ui, 'warning')
    const success = vi.spyOn(ui, 'success')
    start(wrapper)
    await flushPromises()
    expect(warning).toHaveBeenCalledWith(expect.stringContaining('follow_up_failed'))
    expect(success).not.toHaveBeenCalled()
  })

  it('holds the host edit open while a request runs', async () => {
    let finish: (v: unknown) => void = () => {}
    preview.mockReturnValue(new Promise((resolve) => (finish = resolve)))
    const { wrapper } = mountHost([subtask])
    const vm = wrapper.vm as unknown as { panels: () => Element[] }
    start(wrapper)
    await flushPromises()
    // No dialog is open yet, but a press anywhere must not end the edit.
    expect(dialog()).toBeNull()
    expect(vm.panels()).toEqual([document.documentElement])
    finish({ target_type: 'task', required: ['owner'] })
    await flushPromises()
    expect(vm.panels()[0].contains(dialog())).toBe(true)
    expect(vm.panels()).not.toContain(document.documentElement)
  })

  it('creates nothing when the dialog is dismissed during the preflight', async () => {
    let finish: (v: unknown) => void = () => {}
    preview.mockReturnValue(new Promise((resolve) => (finish = resolve)))
    const { wrapper, editor } = mountHost([subtask, blocker])
    start(wrapper)
    await flushPromises()
    ;(dialog()?.querySelector('.task-item-choices button') as HTMLButtonElement).click()
    await flushPromises()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await flushPromises()
    expect(dialog()).toBeNull()
    finish({ target_type: 'task', required: [] })
    await flushPromises()
    expect(convert).not.toHaveBeenCalled()
    expect(editor.clearTaskItemBusy).toHaveBeenCalledWith(7)
  })

  it('sends an untouched required boolean as false', async () => {
    useSchemaStore().entityTypes.set('task', {
      name: 'task',
      label: 'Task',
      properties: { urgent: { type: 'boolean', label: 'Urgent' } },
    } as never)
    preview.mockResolvedValue({ target_type: 'task', required: ['urgent'] })
    convert.mockResolvedValue({ entity_id: 'TASK-7', entity_type: 'task', title: '', item: 'link' })
    const { wrapper } = mountHost([subtask])
    start(wrapper)
    await flushPromises()
    dialog()?.querySelector('form')?.dispatchEvent(new Event('submit'))
    await flushPromises()
    expect(convert).toHaveBeenCalledWith(
      'subtask',
      expect.objectContaining({ properties: { urgent: false } })
    )
  })

  it('sends a list enum as the values ticked', async () => {
    useSchemaStore().entityTypes.set('task', {
      name: 'task',
      label: 'Task',
      properties: { tags: { type: 'enum', list: true, label: 'Tags', values: ['a', 'b', 'c'] } },
    } as never)
    preview.mockResolvedValue({ target_type: 'task', required: ['tags'] })
    convert.mockResolvedValue({ entity_id: 'TASK-7', entity_type: 'task', title: '', item: 'link' })
    const { wrapper } = mountHost([subtask])
    start(wrapper)
    await flushPromises()
    const boxes = Array.from(
      dialog()?.querySelectorAll<HTMLInputElement>('input[type="checkbox"]') ?? []
    )
    expect(boxes).toHaveLength(3)
    for (const box of [boxes[0], boxes[2]]) {
      box.checked = true
      box.dispatchEvent(new Event('change'))
    }
    await flushPromises()
    dialog()?.querySelector('form')?.dispatchEvent(new Event('submit'))
    await flushPromises()
    expect(convert).toHaveBeenCalledWith(
      'subtask',
      expect.objectContaining({ properties: { tags: ['a', 'c'] } })
    )
  })

  it('asks for a datetime with a date and time input', async () => {
    useSchemaStore().entityTypes.set('task', {
      name: 'task',
      label: 'Task',
      properties: { due: { type: 'datetime', label: 'Due' } },
    } as never)
    preview.mockResolvedValue({ target_type: 'task', required: ['due'] })
    const { wrapper } = mountHost([subtask])
    start(wrapper)
    await flushPromises()
    expect(dialog()?.querySelector('input[type="datetime-local"]')).not.toBeNull()
    expect(dialog()?.textContent).toContain('Due')
  })

  it('links the item to an entity created without its relation', async () => {
    preview.mockResolvedValue({ target_type: 'task', required: [] })
    convert.mockRejectedValue(
      new ApiError('Conversion incomplete', {
        kind: 'http',
        status: 500,
        problem: {
          type: 'https://rela.dev/errors/conversion_incomplete',
          title: 'Conversion incomplete',
          status: 500,
          entity_id: 'TASK-9',
        },
        original: null,
      })
    )
    const { wrapper, editor } = mountHost([subtask])
    const error = vi.spyOn(useUIStore(), 'error')
    start(wrapper)
    await flushPromises()
    expect(editor.applyTaskItemResult).toHaveBeenCalledWith(7, 'Write the migration', 'link', {
      id: 'TASK-9',
      title: '',
      entityType: 'task',
    })
    expect(error).toHaveBeenCalledWith(
      "Created TASK-9, but could not link it to this item's source. Link it by hand."
    )
    expect(editor.clearTaskItemBusy).toHaveBeenCalledWith(7)
  })

  it('names its dialog for the host that ends an edit on outside presses', async () => {
    preview.mockResolvedValue({ target_type: 'task', required: ['owner'] })
    const { wrapper } = mountHost([subtask])
    const vm = wrapper.vm as unknown as { panels: () => Element[] }
    expect(vm.panels()).toEqual([])
    start(wrapper)
    await flushPromises()
    const panels = vm.panels()
    expect(panels).toHaveLength(1)
    expect(panels[0].contains(dialog())).toBe(true)
  })
})
