import { describe, it, expect } from 'vitest'
import {
  getEditFormId,
  listHeaderMarkdown,
  listFooterMarkdown,
  type FormConfig,
  type ListConfig,
} from './config'

const list = (over: Partial<ListConfig>): ListConfig => ({
  entity: 'risico',
  columns: [],
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

  describe('listHeaderMarkdown', () => {
    it('returns header when set', () => {
      expect(listHeaderMarkdown(list({ header: '# Hi' }))).toBe('# Hi')
    })

    it('falls back to description (legacy alias) when header is unset', () => {
      expect(listHeaderMarkdown(list({ description: 'legacy' }))).toBe('legacy')
    })

    it('prefers header over description when both are set', () => {
      expect(listHeaderMarkdown(list({ header: 'new', description: 'legacy' }))).toBe('new')
    })

    it('ignores an empty header and falls back to description', () => {
      expect(listHeaderMarkdown(list({ header: '', description: 'legacy' }))).toBe('legacy')
    })

    it('treats a whitespace-only header as unset and falls back to description', () => {
      expect(listHeaderMarkdown(list({ header: '   \n', description: 'legacy' }))).toBe('legacy')
    })

    it('returns empty string when both are whitespace-only', () => {
      expect(listHeaderMarkdown(list({ header: '  ', description: '\t' }))).toBe('')
    })

    it('returns empty string when neither is set', () => {
      expect(listHeaderMarkdown(list({}))).toBe('')
    })

    it('returns empty string for an undefined list', () => {
      expect(listHeaderMarkdown(undefined)).toBe('')
    })
  })

  describe('listFooterMarkdown', () => {
    it('returns footer when set', () => {
      expect(listFooterMarkdown(list({ footer: 'bye' }))).toBe('bye')
    })

    it('returns empty string when unset', () => {
      expect(listFooterMarkdown(list({}))).toBe('')
    })

    it('returns empty string for an undefined list', () => {
      expect(listFooterMarkdown(undefined)).toBe('')
    })
  })
})
