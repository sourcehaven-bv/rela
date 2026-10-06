/**
 * The configuration files as the Configure API sends them: plain JSON trees.
 *
 * A YAML mapping arrives as `{"$m": [[key, value], ...]}` so its key order
 * survives (a plain JSON object cannot keep it). A mapping that sits in a
 * list also carries `"$i"`, its index in that list when the file was read.
 * The server matches an edited item to its old self by that index, so it
 * must travel back untouched; a new item has none.
 *
 * The helpers here edit trees IN PLACE. The draft store clones a tree before
 * it hands it to an edit, so a screen never mutates the copy the server sent.
 */

export type TreeScalar = string | number | boolean | null

export interface TreeMap {
  $m: [string, TreeValue][]
  $i?: number
}

export type TreeValue = TreeScalar | TreeMap | TreeValue[]

/** Path segments: a key in a mapping, or an index in a list. */
export type TreePath = (string | number)[]

export function isMap(value: unknown): value is TreeMap {
  return (
    typeof value === 'object' &&
    value !== null &&
    !Array.isArray(value) &&
    Array.isArray((value as TreeMap).$m)
  )
}

export function isList(value: unknown): value is TreeValue[] {
  return Array.isArray(value)
}

/** A new mapping from an object, in the object's key order. */
export function newMap(fields: Record<string, TreeValue | undefined> = {}): TreeMap {
  const pairs: [string, TreeValue][] = []
  for (const [key, value] of Object.entries(fields)) {
    if (value !== undefined) pairs.push([key, value])
  }
  return { $m: pairs }
}

export function keys(map: TreeValue | undefined): string[] {
  return isMap(map) ? map.$m.map(([key]) => key) : []
}

export function entries(map: TreeValue | undefined): [string, TreeValue][] {
  return isMap(map) ? map.$m : []
}

export function get(map: TreeValue | undefined, key: string): TreeValue | undefined {
  if (!isMap(map)) return undefined
  const pair = map.$m.find(([k]) => k === key)
  return pair ? pair[1] : undefined
}

export function has(map: TreeValue | undefined, key: string): boolean {
  return isMap(map) && map.$m.some(([k]) => k === key)
}

/** Replaces the value under key, or appends the key when it is absent. */
export function set(map: TreeMap, key: string, value: TreeValue): void {
  const pair = map.$m.find(([k]) => k === key)
  if (pair) pair[1] = value
  else map.$m.push([key, value])
}

/**
 * Sets key when value says something, removes it when it does not. Used for
 * optional settings: an empty text or an unticked box leaves no key behind.
 */
export function setOrRemove(map: TreeMap, key: string, value: TreeValue | undefined): void {
  if (value === undefined || value === '' || value === false || value === null) remove(map, key)
  else set(map, key, value)
}

export function remove(map: TreeMap, key: string): void {
  const index = map.$m.findIndex(([k]) => k === key)
  if (index >= 0) map.$m.splice(index, 1)
}

/** Renames a key where it stands, so the order is kept. */
export function renameKey(map: TreeMap, from: string, to: string): void {
  if (from === to) return
  const pair = map.$m.find(([k]) => k === from)
  if (!pair) return
  if (has(map, to)) throw new Error(`"${to}" already exists`)
  pair[0] = to
}

/** Moves a key to a new position among its siblings. */
export function moveKey(map: TreeMap, key: string, toIndex: number): void {
  const from = map.$m.findIndex(([k]) => k === key)
  if (from < 0) return
  const [pair] = map.$m.splice(from, 1)
  map.$m.splice(Math.max(0, Math.min(toIndex, map.$m.length)), 0, pair)
}

/** The mapping under key, created empty when it is absent or not a mapping. */
export function ensureMap(map: TreeMap, key: string): TreeMap {
  const value = get(map, key)
  if (isMap(value)) return value
  const created = newMap()
  set(map, key, created)
  return created
}

/** The list under key, created empty when it is absent or not a list. */
export function ensureList(map: TreeMap, key: string): TreeValue[] {
  const value = get(map, key)
  if (isList(value)) return value
  const created: TreeValue[] = []
  set(map, key, created)
  return created
}

export function getIn(root: TreeValue | undefined, path: TreePath): TreeValue | undefined {
  let node: TreeValue | undefined = root
  for (const segment of path) {
    if (typeof segment === 'number') {
      node = isList(node) ? node[segment] : undefined
    } else {
      node = get(node, segment)
    }
    if (node === undefined) return undefined
  }
  return node
}

/** Moves an item in a list to a new index. */
export function moveItem<T>(list: T[], item: T, toIndex: number): void {
  const from = list.indexOf(item)
  if (from < 0) return
  list.splice(from, 1)
  list.splice(Math.max(0, Math.min(toIndex, list.length)), 0, item)
}

/**
 * Where the item at `position` of a list of `length` items ends up when
 * `moveItem` moves the item at `from` to `to`.
 */
export function positionAfterMove(
  position: number,
  from: number,
  to: number,
  length: number
): number {
  const positions = Array.from({ length }, (_, i) => i)
  if (from >= 0 && from < length) moveItem(positions, from, to)
  return positions.indexOf(position)
}

export function clone<T extends TreeValue | undefined>(value: T): T {
  return value === undefined ? value : (JSON.parse(JSON.stringify(value)) as T)
}

export function equal(a: TreeValue | undefined, b: TreeValue | undefined): boolean {
  return JSON.stringify(a ?? null) === JSON.stringify(b ?? null)
}

// --- Reading scalars ---

/** A scalar as text; undefined for a mapping, a list or nothing. */
export function str(value: TreeValue | undefined): string | undefined {
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  return undefined
}

export function bool(value: TreeValue | undefined): boolean {
  return value === true || value === 'true'
}

export function num(value: TreeValue | undefined): number | undefined {
  if (typeof value === 'number') return value
  if (typeof value === 'string' && value.trim() !== '' && !Number.isNaN(Number(value))) {
    return Number(value)
  }
  return undefined
}

/**
 * A list of names. The files accept a single name where a list is meant
 * (`entity: ticket` as well as `entity: [ticket]`), so both read the same.
 */
export function strList(value: TreeValue | undefined): string[] {
  if (isList(value)) return value.map(str).filter((s): s is string => s !== undefined)
  const single = str(value)
  return single === undefined || single === '' ? [] : [single]
}

/** The mapping items of a list, skipping anything else. */
export function mapItems(value: TreeValue | undefined): TreeMap[] {
  return isList(value) ? value.filter(isMap) : []
}

/**
 * The mapping at a path, creating empty mappings for missing keys along the
 * way. A list index descends into the item that is there, keeping its `$i`,
 * but never creates one: an item that is not there is not made up. A key
 * followed by an index must already hold a list.
 */
export function ensurePath(root: TreeMap, path: TreePath): TreeMap | undefined {
  let node: TreeValue = root
  for (let i = 0; i < path.length; i++) {
    const segment = path[i]
    if (typeof segment === 'number') {
      const item: TreeValue | undefined = isList(node) ? node[segment] : undefined
      if (!isMap(item)) return undefined
      node = item
      continue
    }
    if (!isMap(node)) return undefined
    if (typeof path[i + 1] === 'number') {
      const list = get(node, segment)
      if (!isList(list)) return undefined
      node = list
    } else {
      node = ensureMap(node, segment)
    }
  }
  return isMap(node) ? node : undefined
}

/** The mapping at a path, if there is one. */
export function mapAt(root: TreeValue | undefined, path: TreePath): TreeMap | undefined {
  const node = getIn(root, path)
  return isMap(node) ? node : undefined
}

/** The list at a path, if there is one. */
export function listAt(root: TreeValue | undefined, path: TreePath): TreeValue[] | undefined {
  const node = getIn(root, path)
  return isList(node) ? node : undefined
}
