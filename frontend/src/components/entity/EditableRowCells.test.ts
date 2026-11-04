// Unit tests for EditableRowCells (TKT-4Q2EIVZ): which cells edit, and that
// an edit saves the row through its own autosave.

import { describe, it, expect, beforeEach, afterEach, vi, type Mock } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount, flushPromises } from '@vue/test-utils'
import { useEntitiesStore } from '@/stores/entities'
import { useSchemaStore } from '@/stores'
import EditableRowCells from './EditableRowCells.vue'
import InlinePropertyValue from '@/components/forms/InlinePropertyValue.vue'
import type { ViewColumn, ViewRow } from '@/api/views'
import type { Entity } from '@/types'

const LINE_TYPE = {
  name: 'line',
  properties: {
    description: { type: 'string', label: 'Description' },
    quantity: { type: 'integer', label: 'Quantity' },
    price: { type: 'decimal', label: 'Price' },
  },
}

const COLUMNS: ViewColumn[] = [
  { property: 'description', render: 'input' },
  { property: 'quantity', render: 'input' },
  { property: 'price' },
]

function makeRow(overrides: Partial<ViewRow> = {}): ViewRow {
  return {
    entityId: 'LINE-1',
    entityType: 'line',
    cells: [{ values: ['Widget'] }, { values: ['2'] }, { values: ['9.50'] }],
    _props: { description: 'Widget', quantity: 2, price: '9.50' },
    _fields: {
      description: { writable: true },
      quantity: { writable: false },
      price: { writable: true },
    },
    ...overrides,
  }
}

function mountCells(row: ViewRow, onPropertyApplied: Mock = vi.fn()) {
  return mount(EditableRowCells, {
    props: { row, columns: COLUMNS, onPropertyApplied, onError: vi.fn() },
    slots: {
      cell: `<template #cell="{ cell, index }"><span class="ro">{{ index }}:{{ cell.values[0] }}</span></template>`,
      actions: `<template #actions="{ status }"><td class="actions">{{ status }}</td></template>`,
    },
  })
}

describe('EditableRowCells', () => {
  let updateMock: Mock

  beforeEach(() => {
    setActivePinia(createPinia())
    useSchemaStore().entityTypes.set('line', LINE_TYPE as never)
    updateMock = vi.spyOn(useEntitiesStore(), 'update').mockResolvedValue({
      id: 'LINE-1',
      type: 'line',
      properties: {},
    } as Entity) as unknown as Mock
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'Date'] })
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('edits only an input column the verdict allows; the rest go through the cell slot', () => {
    const wrapper = mountCells(makeRow())
    const inputs = wrapper.findAllComponents(InlinePropertyValue)
    expect(inputs.map((c) => c.props('property'))).toEqual(['description'])
    // quantity is an input column the reader may not write; price is not an
    // input column at all.
    expect(wrapper.findAll('.ro').map((e) => e.text())).toEqual(['1:2', '2:9.50'])
    expect(wrapper.find('.actions').text()).toBe('idle')
  })

  it('reads the typed value from _props, not the cell string', () => {
    const wrapper = mountCells(makeRow({ _props: { description: 'Typed' } }))
    expect(wrapper.findComponent(InlinePropertyValue).props('value')).toBe('Typed')
  })

  it('marks numeric input columns', () => {
    const row = makeRow({ _fields: { quantity: { writable: true } } })
    const wrapper = mountCells(row)
    const tds = wrapper.findAll('td').filter((td) => !td.classes('actions'))
    expect(tds.map((td) => td.classes('data-table-cell--numeric'))).toEqual([false, true, false])
  })

  it('saves an edit on the row address and reports it to the host', async () => {
    // The host hears the value the server confirms.
    updateMock.mockResolvedValue({
      id: 'LINE-1',
      type: 'line',
      properties: { description: 'Gadget' },
    } as Entity)
    const onApplied = vi.fn()
    const wrapper = mountCells(makeRow({ _self: 'LINE-1@draft' }), onApplied)
    wrapper.findComponent(InlinePropertyValue).vm.$emit('update', 'Gadget')
    await vi.advanceTimersByTimeAsync(900)
    await flushPromises()
    expect(updateMock).toHaveBeenCalledTimes(1)
    expect(updateMock.mock.calls[0][0]).toBe('line')
    expect(updateMock.mock.calls[0][1]).toBe('LINE-1@draft')
    expect(updateMock.mock.calls[0][2]).toEqual({ properties: { description: 'Gadget' } })
    expect(onApplied).toHaveBeenCalledWith('description', 'Gadget', {
      type: 'line',
      id: 'LINE-1@draft',
    })
  })
})
