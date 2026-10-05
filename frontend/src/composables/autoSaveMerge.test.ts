import { describe, it, expect } from 'vitest'
import { mergeProperty, mergeRelations, mergeText } from './autoSaveMerge'

const eq = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b)

describe('mergeText', () => {
  it.each([
    ['identical edits', 'a\nb', 'a\nB', 'a\nB', { ok: true, merged: 'a\nB' }],
    ['only ours changed', 'a\nb', 'A\nb', 'a\nb', { ok: true, merged: 'A\nb' }],
    ['only theirs changed', 'a\nb', 'a\nb', 'a\nB', { ok: true, merged: 'a\nB' }],
    ['different lines', 'a\nb\nc\n', 'A\nb\nc\n', 'a\nb\nC\n', { ok: true, merged: 'A\nb\nC\n' }],
    [
      'insertions at both ends',
      'm\n',
      'top\nm\n',
      'm\nbottom\n',
      { ok: true, merged: 'top\nm\nbottom\n' },
    ],
    [
      'same line, different edits',
      'a\nb\n',
      'A1\nb\n',
      'A2\nb\n',
      { ok: false, oursInConflicts: 'A1\nb\n' },
    ],
  ])('%s', (_name, base, ours, theirs, want) => {
    expect(mergeText(base, ours, theirs)).toEqual(want)
  })

  it('keeps their clean hunks and our side of a conflict', () => {
    const r = mergeText('a\nb\nc\n', 'A1\nb\nc\n', 'A2\nb\nC\n')
    expect(r).toEqual({ ok: false, oursInConflicts: 'A1\nb\nC\n' })
  })

  it('never emits conflict markers', () => {
    const r = mergeText('x\n', 'y\n', 'z\n')
    expect(r.ok).toBe(false)
    expect(JSON.stringify(r)).not.toContain('<<<<<<<')
  })
})

describe('mergeProperty', () => {
  it.each([
    ['theirs unchanged', 'a', 'b', 'a', 'write'],
    ['theirs already ours', 'a', 'b', 'b', 'same'],
    ['both changed', 'a', 'b', 'c', 'conflict'],
    ['both unset', 'a', undefined, undefined, 'same'],
    ['they set, we unset', 'a', undefined, 'c', 'conflict'],
    ['filled empty field on both sides', undefined, 'b', 'c', 'conflict'],
  ])('%s', (_name, base, ours, theirs, want) => {
    expect(mergeProperty(base, ours, theirs, eq).kind).toBe(want)
  })
})

describe('mergeRelations', () => {
  const typeOf = (id: string) => (id.startsWith('F') ? 'feature' : undefined)
  const dataOf = (r: ReturnType<typeof mergeRelations>, key: string) => {
    const entry = r?.[key]
    return entry && 'data' in entry ? entry.data : undefined
  }
  const ids = (r: ReturnType<typeof mergeRelations>, key: string) =>
    dataOf(r, key)
      ?.map((e) => e.id)
      .sort()

  it('passes a delta entry through unchanged', () => {
    const delta = { add: [{ type: 'feature', id: 'F9' }], remove: [{ id: 'F1' }] }
    const r = mergeRelations(
      { implements: ['F1'] },
      { implements: delta },
      { implements: ['F1', 'F2'] },
      () => false,
      typeOf
    )
    expect(r?.implements).toBe(delta)
  })

  it('keeps their additions and ours', () => {
    const r = mergeRelations(
      { implements: ['F1'] },
      {
        implements: {
          data: [
            { type: 'feature', id: 'F1' },
            { type: 'feature', id: 'F2' },
          ],
        },
      },
      { implements: ['F1', 'F3'] },
      () => false,
      typeOf
    )
    expect(ids(r, 'implements')).toEqual(['F1', 'F2', 'F3'])
  })

  it('applies our removal and theirs', () => {
    const r = mergeRelations(
      { implements: ['F1', 'F2', 'F3'] },
      {
        implements: {
          data: [
            { type: 'feature', id: 'F2' },
            { type: 'feature', id: 'F3' },
          ],
        },
      },
      { implements: ['F1', 'F2'] },
      () => false,
      typeOf
    )
    expect(ids(r, 'implements')).toEqual(['F2'])
  })

  it('keeps our entry, with its meta, for an edge both sides hold', () => {
    const mine = { type: 'feature', id: 'F1', meta: { weight: 2 } }
    const r = mergeRelations(
      { implements: ['F1'] },
      { implements: { data: [mine] } },
      { implements: ['F1'] },
      () => false,
      typeOf
    )
    expect(dataOf(r, 'implements')).toEqual([mine])
  })

  it('refuses when a target type is unknown rather than drop the edge', () => {
    const r = mergeRelations(
      {},
      { implements: { data: [] } },
      { implements: ['X-9'] },
      () => false,
      typeOf
    )
    expect(r).toBeNull()
  })

  it('passes incoming keys through unchanged', () => {
    const body = { implemented_by: { data: [{ type: 'ticket', id: 'T-1' }] } }
    const r = mergeRelations({}, body, {}, (k) => k === 'implemented_by', typeOf)
    expect(r).toEqual(body)
  })
})
