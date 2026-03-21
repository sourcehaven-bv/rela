import { describe, it, expect } from 'vitest'
import { getEditFormId, type FormConfig } from './config'

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
})
