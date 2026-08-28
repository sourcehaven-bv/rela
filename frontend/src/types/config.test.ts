import { describe, it, expect } from 'vitest'
import {
  getEditFormId,
  viewHeaderMarkdown,
  viewFooterMarkdown,
  type FormConfig,
  type ListConfig,
  type KanbanConfig,
} from './config'

const list = (over: Partial<ListConfig>): ListConfig => ({
  entity: 'risico',
  columns: [],
  ...over,
})

// Lists opt into the legacy `description` → header alias; kanban does not.
const ALIAS = { allowDescriptionAlias: true }

const kanban = (over: Partial<KanbanConfig>): KanbanConfig => ({
  entity: 'ticket',
  column_property: 'status',
  card: { title: 'title' },
  ...over,
})

describe('config', () => {
  describe('getEditFormId', () => {
    it('returns edit form when mode is edit', () => {
      const schemaStore = {
        forms: new Map<string, FormConfig>([
          ['task-create', { entity: 'task', mode: 'create' }],
          ['task-edit', { entity: 'task', mode: 'edit' }],
        ]),
      }

      expect(getEditFormId(schemaStore, 'task')).toBe('task-edit')
    })

    it('falls back to any form for entity type when no edit mode', () => {
      const schemaStore = {
        forms: new Map<string, FormConfig>([
          ['task-form', { entity: 'task' }],
          ['bug-form', { entity: 'bug' }],
        ]),
      }

      expect(getEditFormId(schemaStore, 'task')).toBe('task-form')
    })

    it('returns undefined when no form matches entity type', () => {
      const schemaStore = {
        forms: new Map<string, FormConfig>([
          ['bug-form', { entity: 'bug', mode: 'edit' }],
        ]),
      }

      expect(getEditFormId(schemaStore, 'task')).toBeUndefined()
    })

    it('returns undefined for empty forms map', () => {
      const schemaStore = {
        forms: new Map<string, FormConfig>(),
      }

      expect(getEditFormId(schemaStore, 'task')).toBeUndefined()
    })

    it('prefers edit mode over other modes', () => {
      const schemaStore = {
        forms: new Map<string, FormConfig>([
          ['task-view', { entity: 'task', mode: 'view' }],
          ['task-edit', { entity: 'task', mode: 'edit' }],
          ['task-create', { entity: 'task', mode: 'create' }],
        ]),
      }

      expect(getEditFormId(schemaStore, 'task')).toBe('task-edit')
    })
  })

  describe('viewHeaderMarkdown', () => {
    it('returns header when set', () => {
      expect(viewHeaderMarkdown(list({ header: '# Hi' }))).toBe('# Hi')
    })

    it('falls back to description (legacy alias) when header is unset', () => {
      expect(viewHeaderMarkdown(list({ description: 'legacy' }), ALIAS)).toBe('legacy')
    })

    it('prefers header over description when both are set', () => {
      expect(viewHeaderMarkdown(list({ header: 'new', description: 'legacy' }), ALIAS)).toBe('new')
    })

    it('ignores an empty header and falls back to description', () => {
      expect(viewHeaderMarkdown(list({ header: '', description: 'legacy' }), ALIAS)).toBe('legacy')
    })

    it('treats a whitespace-only header as unset and falls back to description', () => {
      expect(viewHeaderMarkdown(list({ header: '   \n', description: 'legacy' }), ALIAS)).toBe('legacy')
    })

    it('returns empty string when both are whitespace-only', () => {
      expect(viewHeaderMarkdown(list({ header: '  ', description: '\t' }), ALIAS)).toBe('')
    })

    it('ignores description when the alias is not opted into', () => {
      // The alias is a per-call-site opt-in, NOT a consequence of the config
      // type: types erase at runtime, so an object carrying `description` would
      // otherwise leak it regardless of which config interface it claims to be.
      expect(viewHeaderMarkdown(list({ description: 'legacy' }))).toBe('')
    })

    it('ignores description on a kanban-shaped object even if one is present', () => {
      // Guards the RR-GNWJFO drift scenario: a future Go `Kanban.Description`
      // field reaching the SPA must NOT silently switch on the list alias.
      const boardWithStrayDescription = {
        entity: 'ticket',
        column_property: 'status',
        description: 'must not render',
      }
      expect(viewHeaderMarkdown(boardWithStrayDescription)).toBe('')
    })

    it('returns empty string when neither is set', () => {
      expect(viewHeaderMarkdown(list({}))).toBe('')
    })

    it('returns empty string for an undefined list', () => {
      expect(viewHeaderMarkdown(undefined)).toBe('')
    })

    it('resolves a kanban header', () => {
      expect(viewHeaderMarkdown(kanban({ header: '# Board' }))).toBe('# Board')
    })

    it('treats a whitespace-only kanban header as unset', () => {
      expect(viewHeaderMarkdown(kanban({ header: '  \n' }))).toBe('')
    })

    it('returns empty string for a kanban with no header', () => {
      // Kanban never opts into the alias, so there is no fallback to find.
      expect(viewHeaderMarkdown(kanban({}))).toBe('')
    })
  })

  describe('viewFooterMarkdown', () => {
    it('returns footer when set', () => {
      expect(viewFooterMarkdown(list({ footer: 'bye' }))).toBe('bye')
    })

    it('returns empty string when unset', () => {
      expect(viewFooterMarkdown(list({}))).toBe('')
    })

    it('returns empty string for an undefined list', () => {
      expect(viewFooterMarkdown(undefined)).toBe('')
    })

    it('resolves a kanban footer', () => {
      expect(viewFooterMarkdown(kanban({ footer: 'see the runbook' }))).toBe('see the runbook')
    })

    it('treats a whitespace-only kanban footer as unset', () => {
      expect(viewFooterMarkdown(kanban({ footer: '\t' }))).toBe('')
    })
  })
})
