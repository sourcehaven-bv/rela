import { describe, expect, it } from 'vitest'
import {
  clone,
  ensurePath,
  equal,
  get,
  getIn,
  keys,
  listAt,
  mapAt,
  moveItem,
  moveKey,
  newMap,
  positionAfterMove,
  renameKey,
  set,
  setOrRemove,
  strList,
} from '../tree'
import { toTree } from './fixture'

describe('tree helpers', () => {
  it('keeps key order on set, rename and move', () => {
    const map = newMap({ a: 1, b: 2, c: 3 })
    set(map, 'b', 20)
    set(map, 'd', 4)
    expect(keys(map)).toEqual(['a', 'b', 'c', 'd'])
    expect(get(map, 'b')).toBe(20)

    renameKey(map, 'b', 'bee')
    expect(keys(map)).toEqual(['a', 'bee', 'c', 'd'])
    expect(get(map, 'bee')).toBe(20)

    moveKey(map, 'd', 0)
    expect(keys(map)).toEqual(['d', 'a', 'bee', 'c'])
  })

  it('removes a setting set to nothing', () => {
    const map = newMap({ a: 'x', b: true })
    for (const empty of [undefined, '', false, null]) {
      const copy = clone(map)
      setOrRemove(copy, 'a', empty)
      expect(keys(copy)).toEqual(['b'])
    }
    setOrRemove(map, 'a', 'y')
    expect(get(map, 'a')).toBe('y')
  })

  it('reads paths through mappings and lists', () => {
    const tree = toTree({
      forms: { f: { fields: [{ property: 'title' }, { property: 'status' }] } },
    })
    expect(get(getIn(tree, ['forms', 'f', 'fields', 1]), 'property')).toBe('status')
    expect(getIn(tree, ['forms', 'missing', 'fields'])).toBeUndefined()
    expect(listAt(tree, ['forms', 'f', 'fields'])).toHaveLength(2)
    expect(mapAt(tree, ['forms', 'f', 'fields'])).toBeUndefined()
  })

  it('creates mappings along a path, but never list items', () => {
    const root = newMap()
    const leaf = ensurePath(root, ['styles', 'status'])
    expect(leaf).toBeDefined()
    expect(getIn(root, ['styles', 'status'])).toBe(leaf)

    const withList = toTree({ items: [] })
    expect(ensurePath(withList as ReturnType<typeof newMap>, ['items', 0])).toBeUndefined()
  })

  it('descends into an existing list item without creating one', () => {
    const root = toTree({ rules: [{ name: 'a' }, { name: 'b', on: { x: 1 } }] }) as ReturnType<
      typeof newMap
    >
    const item = ensurePath(root, ['rules', 1])
    expect(item).toBe(getIn(root, ['rules', 1]))
    expect(item?.$i).toBe(1)

    const nested = ensurePath(root, ['rules', 0, 'on'])
    expect(nested).toBeDefined()
    expect(getIn(root, ['rules', 0, 'on'])).toBe(nested)
    expect(getIn(root, ['rules', 0])).toMatchObject({ $i: 0 })

    expect(ensurePath(root, ['rules', 2])).toBeUndefined()
    expect(listAt(root, ['rules'])).toHaveLength(2)
    // A key followed by an index never turns a missing list into a mapping.
    expect(ensurePath(root, ['other', 0])).toBeUndefined()
    expect(getIn(root, ['other'])).toBeUndefined()
  })

  it('follows an item through a move', () => {
    expect(positionAfterMove(2, 2, 0, 3)).toBe(0)
    expect(positionAfterMove(0, 2, 0, 3)).toBe(1)
    expect(positionAfterMove(1, 0, 2, 3)).toBe(0)
    expect(positionAfterMove(1, 2, 2, 3)).toBe(1)
  })

  it('moves a list item by identity', () => {
    const list = ['a', 'b', 'c']
    moveItem(list, 'c', 0)
    expect(list).toEqual(['c', 'a', 'b'])
  })

  it('compares trees by value and clones deeply', () => {
    const tree = toTree({ a: [{ b: 1 }] })
    const copy = clone(tree)
    expect(equal(tree, copy)).toBe(true)
    const item = getIn(copy, ['a', 0])
    if (item && typeof item === 'object' && !Array.isArray(item)) set(item, 'b', 2)
    expect(equal(tree, copy)).toBe(false)
    expect(get(getIn(tree, ['a', 0]), 'b')).toBe(1)
  })

  it('carries the list index a mapping was read at', () => {
    const tree = toTree({ cards: [{ title: 'one' }, { title: 'two' }] })
    expect(getIn(tree, ['cards', 1])).toMatchObject({ $i: 1 })
    expect(strList(toTree(['x', 1, 'y']))).toEqual(['x', '1', 'y'])
  })
})
