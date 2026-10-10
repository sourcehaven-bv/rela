import type { ConfigSnapshot } from '@/api/configure'
import { isMap, type TreeValue } from '../tree'

/**
 * Turns a plain object into the wire tree: mappings become `$m` pairs in
 * key order, and a mapping inside a list gets `$i`, its index there, the way
 * the server sends it.
 */
export function toTree(value: unknown, index?: number): TreeValue {
  if (Array.isArray(value)) return value.map((item, i) => toTree(item, i))
  if (value !== null && typeof value === 'object') {
    if (isMap(value)) return value
    const map: { $m: [string, TreeValue][]; $i?: number } = {
      $m: Object.entries(value).map(([k, v]) => [k, toTree(v)]),
    }
    if (index !== undefined) map.$i = index
    return map
  }
  return value as TreeValue
}

export const schemaFixture = {
  types: {
    status: { values: ['open', 'doing', 'done'], default: 'open' },
    priority: { values: ['low', 'high'] },
  },
  entities: {
    ticket: {
      label: 'Ticket',
      label_plural: 'Tickets',
      id_prefix: 'TKT-',
      color: '#dbeafe',
      border_color: '#3b82f6',
      properties: {
        title: { type: 'string', required: true },
        status: { type: 'status' },
        effort: { type: 'number', description: 'Days of work' },
        checksum: { type: 'string', computed: 'sha(entity.title)' },
      },
    },
    feature: {
      label: 'Feature',
      properties: { title: { type: 'string', required: true }, priority: { type: 'priority' } },
    },
  },
  relations: {
    implements: {
      label: 'implements',
      from: ['ticket'],
      to: ['feature'],
      inverse: 'implementedBy',
      min_outgoing: 1,
    },
  },
  validations: [
    {
      name: 'ready_needs_effort',
      entity_type: 'ticket',
      when_condition: "entity.status == 'doing'",
      then_condition: 'entity.effort != nil',
      severity: 'error',
    },
    { name: 'scripted', entity_type: 'ticket', lua_file: 'scripts/check.lua', severity: 'warning' },
  ],
  automations: [
    {
      name: 'stamp_done',
      on: { entity: ['ticket'], property: 'status', becomes: 'done' },
      do: [{ set: 'effort', value: '0' }],
    },
    { name: 'scripted', on: { entity: ['ticket'], created: true }, do: [{ lua: 'print(1)' }] },
  ],
}

export const dataEntryFixture = {
  app: { name: 'Tracker' },
  styles: { status: { open: 'blue', done: 'green' } },
  forms: {
    new_ticket: {
      title: 'New ticket',
      entity_type: 'ticket',
      mode: 'create',
      fields: [
        { property: 'title' },
        { property: 'status', span: 6 },
        { property: 'effort', span: 6 },
      ],
    },
  },
  lists: {
    tickets: {
      title: 'Tickets',
      entity_type: 'ticket',
      columns: [{ property: 'title' }, { property: 'status', sortable: true }],
      sort: [{ property: 'status', direction: 'asc' }],
    },
  },
  kanbans: {
    board: {
      title: 'Board',
      entity_type: 'ticket',
      column_property: 'status',
      card: { fields: [{ property: 'effort' }] },
    },
  },
  dashboard: {
    cards: [
      { title: 'Open', query: 'type:ticket status:open', display: 'count' },
      { title: 'Secret', query: 'x', display: 'count', permission: 'admin' },
    ],
  },
  navigation: [
    {
      group: 'Work',
      items: [
        { label: 'Tickets', list: 'tickets' },
        { label: 'Board', kanban: 'board' },
      ],
    },
    { label: 'Admin', list: 'tickets', permission: 'admin' },
  ],
}

export function snapshot(version = 'v1'): ConfigSnapshot {
  return {
    version,
    schema: toTree(schemaFixture),
    data_entry: toTree(dataEntryFixture),
    editable: {
      'schema.yaml': [
        'entities.*.label',
        'entities.*.properties.*.type',
        'types.*.values.[]',
        'relations.*.inverse.id',
      ],
      'data-entry.yaml': ['nav.label', 'forms.*.fields.[].**', 'spaces.[].label'],
    },
  }
}
