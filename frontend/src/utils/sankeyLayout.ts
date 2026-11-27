/**
 * Pure layout math for the sankey view: column placement, crossing-minimised
 * ordering, node heights and link ribbons. No DOM, no store, no d3 (the
 * ganttLayout.ts precedent): the view renders the result with `v-for`.
 *
 * Every link runs from a lower column to a higher one. A link given the other
 * way round is flipped, and a link inside one column is dropped.
 */

export interface SankeyNodeIn {
  id: string
  type: string
  /** Minimum node value. Nodes without links use it (default 1) so they stay visible. */
  weight?: number
  /** Orders nodes of a fixed type inside their column (compared as text). */
  sortKey?: string
}

export interface SankeyLinkIn {
  source: string
  target: string
  /** Link weight; default 1. */
  value?: number
}

export interface SankeyOptions {
  /** Types per column, left to right. Types sharing a column stack top to bottom. */
  columns: string[][]
  /** Types whose order is pinned to `sortKey` (then id) instead of minimised. */
  fixedTypes?: string[]
  width: number
  height: number
  nodeWidth?: number
  nodePadding?: number
  /** Extra space above the second and later type of a column (room for a heading). */
  typeGap?: number
  marginTop?: number
  marginBottom?: number
  marginLeft?: number
  marginRight?: number
  /** Relaxation passes that move nodes towards their neighbours. */
  iterations?: number
}

interface SankeyNode extends SankeyNodeIn {
  col: number
  x0: number
  x1: number
  y0: number
  y1: number
  value: number
}

interface SankeyLink {
  source: string
  target: string
  value: number
  width: number
  /** Ribbon centre y at the source and at the target node. */
  sy: number
  ty: number
  x0: number
  x1: number
  /** SVG path for a stroke of `width`. */
  path: string
}

interface SankeyHeading {
  type: string
  col: number
  x: number
  y: number
}

export interface SankeyLayout {
  nodes: SankeyNode[]
  links: SankeyLink[]
  headings: SankeyHeading[]
  /** Link crossings of the chosen order. */
  crossings: number
}

interface Edge {
  source: string
  target: string
  value: number
}

/** orientLinks drops unknown ids and same-column links, and flips backward links. */
export function orientLinks(links: SankeyLinkIn[], colOf: Map<string, number>): Edge[] {
  const out: Edge[] = []
  for (const l of links) {
    const a = colOf.get(l.source)
    const b = colOf.get(l.target)
    if (a === undefined || b === undefined || a === b) continue
    const value = l.value ?? 1
    out.push(
      a < b
        ? { source: l.source, target: l.target, value }
        : { source: l.target, target: l.source, value }
    )
  }
  return out
}

/** inversions counts pairs i<j with seq[i] > seq[j] (Fenwick tree, O(n log n)). */
export function inversions(seq: number[]): number {
  const size = seq.reduce((m, v) => Math.max(m, v), 0) + 2
  const tree = new Array<number>(size + 1).fill(0)
  let n = 0
  for (let i = seq.length - 1; i >= 0; i--) {
    for (let k = seq[i]; k > 0; k -= k & -k) n += tree[k] // seen smaller values
    for (let k = seq[i] + 1; k <= size; k += k & -k) tree[k]++
  }
  return n
}

/** Orders hold the top-to-bottom id sequence of every column. */
type Orders = string[][]

function countCrossings(orders: Orders, edges: Edge[], colOf: Map<string, number>): number {
  const pos = new Map<string, number>()
  orders.forEach((ids) => ids.forEach((id, i) => pos.set(id, i)))
  const groups = new Map<string, Edge[]>()
  for (const e of edges) {
    const key = `${colOf.get(e.source)}>${colOf.get(e.target)}`
    const g = groups.get(key)
    if (g) g.push(e)
    else groups.set(key, [e])
  }
  let total = 0
  for (const g of groups.values()) {
    const sorted = [...g].sort(
      (a, b) => pos.get(a.source)! - pos.get(b.source)! || pos.get(a.target)! - pos.get(b.target)!
    )
    total += inversions(sorted.map((e) => pos.get(e.target)!))
  }
  return total
}

function mean(xs: number[]): number {
  return xs.reduce((a, b) => a + b, 0) / xs.length
}

/**
 * orderNodes returns the column sequences with few link crossings. Fixed
 * types keep their `sortKey` order; the rest are placed by alternating
 * barycentre sweeps, then improved by swapping adjacent neighbours while that
 * lowers the crossings.
 */
export function orderNodes(
  nodes: SankeyNodeIn[],
  edges: Edge[],
  opts: Pick<SankeyOptions, 'columns' | 'fixedTypes'>
): { orders: Orders; crossings: number } {
  const typeCol = new Map<string, number>()
  const typeRank = new Map<string, number>()
  opts.columns.forEach((types, c) =>
    types.forEach((t, r) => {
      typeCol.set(t, c)
      typeRank.set(t, r)
    })
  )
  const fixed = new Set(opts.fixedTypes ?? [])
  const byId = new Map(nodes.map((n) => [n.id, n]))
  const colOf = new Map(nodes.map((n) => [n.id, typeCol.get(n.type)!]))
  const neighbours = new Map<string, string[]>()
  for (const n of nodes) neighbours.set(n.id, [])
  for (const e of edges) {
    neighbours.get(e.source)!.push(e.target)
    neighbours.get(e.target)!.push(e.source)
  }
  const rank = (id: string) => typeRank.get(byId.get(id)!.type)!
  const isFixed = (id: string) => fixed.has(byId.get(id)!.type)
  const cmpFixed = (a: string, b: string) => {
    const ka = byId.get(a)!.sortKey ?? ''
    const kb = byId.get(b)!.sortKey ?? ''
    return ka < kb ? -1 : ka > kb ? 1 : a < b ? -1 : a > b ? 1 : 0
  }

  // Start with the input order inside each type, fixed types sorted.
  let orders: Orders = opts.columns.map(() => [])
  for (const n of nodes) orders[colOf.get(n.id)!].push(n.id)
  const settle = (ids: string[], want: Map<string, number>) =>
    ids.sort((a, b) => {
      const d = rank(a) - rank(b)
      if (d) return d
      if (isFixed(a)) return cmpFixed(a, b)
      return (want.get(a) ?? 0) - (want.get(b) ?? 0)
    })
  const noWant = new Map<string, number>()
  orders.forEach((ids) => settle(ids, noWant))

  const normPos = (o: Orders) => {
    const p = new Map<string, number>()
    o.forEach((ids) => ids.forEach((id, i) => p.set(id, (i + 0.5) / ids.length)))
    return p
  }

  let best = orders.map((ids) => [...ids])
  let fewest = countCrossings(best, edges, colOf)
  const sweep = (cols: number[]) => {
    for (const c of cols) {
      const p = normPos(orders)
      const want = new Map<string, number>()
      for (const id of orders[c]) {
        const ns = neighbours.get(id)!
        want.set(id, ns.length ? mean(ns.map((m) => p.get(m)!)) : p.get(id)!)
      }
      settle(orders[c], want)
    }
  }
  const forward = orders.map((_, c) => c)
  const backward = [...forward].reverse()
  for (let pass = 0; pass < 12; pass++) {
    sweep(pass % 2 ? backward : forward)
    const c = countCrossings(orders, edges, colOf)
    if (c < fewest) {
      fewest = c
      best = orders.map((ids) => [...ids])
    }
  }
  orders = best

  // Adjacent swaps of free nodes of one type, judged on the links of the pair only.
  const pos = new Map<string, number>()
  const refresh = () => orders.forEach((ids) => ids.forEach((id, i) => pos.set(id, i)))
  refresh()
  const swapGain = (a: string, b: string) => {
    // a is directly above b. Count crossings between their links with a above b, and with b above a.
    let before = 0
    let after = 0
    for (const u of neighbours.get(a)!) {
      for (const v of neighbours.get(b)!) {
        if (colOf.get(u) !== colOf.get(v) || u === v) continue
        if (pos.get(u)! > pos.get(v)!) before++
        else if (pos.get(u)! < pos.get(v)!) after++
      }
    }
    return after - before
  }
  for (let improved = true, guard = 0; improved && guard < 50; guard++) {
    improved = false
    for (const ids of orders) {
      for (let i = 0; i + 1 < ids.length; i++) {
        const a = ids[i]
        const b = ids[i + 1]
        if (isFixed(a) || isFixed(b) || rank(a) !== rank(b)) continue
        if (swapGain(a, b) < 0) {
          ids[i] = b
          ids[i + 1] = a
          pos.set(b, i)
          pos.set(a, i + 1)
          improved = true
        }
      }
    }
  }
  return { orders, crossings: countCrossings(orders, edges, colOf) }
}

/** ribbonPath is a cubic Bézier between two x positions at the given centre heights. */
function ribbonPath(x0: number, sy: number, x1: number, ty: number): string {
  const xm = (x0 + x1) / 2
  return `M${x0},${sy}C${xm},${sy} ${xm},${ty} ${x1},${ty}`
}

/** layoutSankey places nodes and links. Nodes of unknown types are ignored. */
export function layoutSankey(
  nodesIn: SankeyNodeIn[],
  linksIn: SankeyLinkIn[],
  opts: SankeyOptions
): SankeyLayout {
  const nodeWidth = opts.nodeWidth ?? 12
  const pad = opts.nodePadding ?? 5
  const typeGap = Math.max(opts.typeGap ?? 28, pad)
  const mt = opts.marginTop ?? 34
  const mb = opts.marginBottom ?? 10
  const ml = opts.marginLeft ?? 0
  const mr = opts.marginRight ?? 0
  const iterations = opts.iterations ?? 32
  const ncols = opts.columns.length

  const typeCol = new Map<string, number>()
  opts.columns.forEach((types, c) => types.forEach((t) => typeCol.set(t, c)))
  const nodesKnown = nodesIn.filter((n) => typeCol.has(n.type))
  const colOf = new Map(nodesKnown.map((n) => [n.id, typeCol.get(n.type)!]))
  const edges = orientLinks(linksIn, colOf)

  const { orders, crossings } = orderNodes(nodesKnown, edges, opts)

  const byId = new Map<string, SankeyNode>()
  const inSum = new Map<string, number>()
  const outSum = new Map<string, number>()
  for (const e of edges) {
    outSum.set(e.source, (outSum.get(e.source) ?? 0) + e.value)
    inSum.set(e.target, (inSum.get(e.target) ?? 0) + e.value)
  }
  const colStep = ncols > 1 ? (opts.width - ml - mr - nodeWidth) / (ncols - 1) : 0
  for (const n of nodesKnown) {
    const col = colOf.get(n.id)!
    const linked = Math.max(inSum.get(n.id) ?? 0, outSum.get(n.id) ?? 0)
    const value = linked > 0 ? Math.max(linked, n.weight ?? 0) : (n.weight ?? 1)
    const x0 = ml + col * colStep
    byId.set(n.id, { ...n, col, x0, x1: x0 + nodeWidth, y0: 0, y1: 0, value })
  }

  const typeOfId = (id: string) => byId.get(id)!.type
  const gapAfter = (ids: string[], i: number) =>
    typeOfId(ids[i]) === typeOfId(ids[i + 1]) ? pad : typeGap
  const top = mt
  const bottom = opts.height - mb
  let scale = Infinity
  for (const ids of orders) {
    if (!ids.length) continue
    let gaps = 0
    for (let i = 0; i + 1 < ids.length; i++) gaps += gapAfter(ids, i)
    const sum = ids.reduce((s, id) => s + byId.get(id)!.value, 0)
    scale = Math.min(scale, Math.max(bottom - top - gaps, 1) / sum)
  }
  if (!Number.isFinite(scale)) scale = 1

  // Initial stack from the top.
  for (const ids of orders) {
    let y = top
    ids.forEach((id, i) => {
      const n = byId.get(id)!
      n.y0 = y
      n.y1 = y + n.value * scale
      y = n.y1 + (i + 1 < ids.length ? gapAfter(ids, i) : 0)
    })
  }

  const centre = (id: string) => (byId.get(id)!.y0 + byId.get(id)!.y1) / 2
  const collide = (ids: string[]) => {
    for (let i = 0; i < ids.length; i++) {
      const n = byId.get(ids[i])!
      if (i === 0) n.y0 = Math.max(n.y0, top)
      else n.y0 = Math.max(n.y0, byId.get(ids[i - 1])!.y1 + gapAfter(ids, i - 1))
      n.y1 = n.y0 + n.value * scale
    }
    for (let i = ids.length - 1; i >= 0; i--) {
      const n = byId.get(ids[i])!
      const h = n.y1 - n.y0
      const limit = i === ids.length - 1 ? bottom : byId.get(ids[i + 1])!.y0 - gapAfter(ids, i)
      if (n.y1 > limit) {
        n.y1 = limit
        n.y0 = limit - h
      }
    }
  }
  const links = new Map<string, Edge[]>() // by node, both directions
  for (const n of nodesKnown) links.set(n.id, [])
  for (const e of edges) {
    links.get(e.source)!.push(e)
    links.get(e.target)!.push(e)
  }
  const pull = (c: number, damp: number) => {
    for (const id of orders[c]) {
      const es = links.get(id)!
      if (!es.length) continue
      let w = 0
      let s = 0
      for (const e of es) {
        const other = e.source === id ? e.target : e.source
        w += e.value
        s += e.value * centre(other)
      }
      const n = byId.get(id)!
      const shift = (s / w - centre(id)) * damp
      n.y0 += shift
      n.y1 += shift
    }
    collide(orders[c])
  }
  for (let i = 0; i < iterations; i++) {
    const damp = Math.max(0.99 ** i, 0.25)
    for (let c = 1; c < ncols; c++) pull(c, damp)
    for (let c = ncols - 2; c >= 0; c--) pull(c, damp)
  }

  // Ribbons: stack each node's links by the height of the node at the other end.
  const outLinks = new Map<string, Edge[]>()
  const inLinks = new Map<string, Edge[]>()
  for (const n of nodesKnown) {
    outLinks.set(n.id, [])
    inLinks.set(n.id, [])
  }
  for (const e of edges) {
    outLinks.get(e.source)!.push(e)
    inLinks.get(e.target)!.push(e)
  }
  const sy = new Map<Edge, number>()
  const ty = new Map<Edge, number>()
  for (const n of byId.values()) {
    let y = n.y0
    for (const e of outLinks.get(n.id)!.sort((a, b) => centre(a.target) - centre(b.target))) {
      const w = e.value * scale
      sy.set(e, y + w / 2)
      y += w
    }
    y = n.y0
    for (const e of inLinks.get(n.id)!.sort((a, b) => centre(a.source) - centre(b.source))) {
      const w = e.value * scale
      ty.set(e, y + w / 2)
      y += w
    }
  }
  const outLinksResult: SankeyLink[] = edges.map((e) => {
    const s = byId.get(e.source)!
    const t = byId.get(e.target)!
    return {
      source: e.source,
      target: e.target,
      value: e.value,
      width: e.value * scale,
      sy: sy.get(e)!,
      ty: ty.get(e)!,
      x0: s.x1,
      x1: t.x0,
      path: ribbonPath(s.x1, sy.get(e)!, t.x0, ty.get(e)!),
    }
  })

  const headings: SankeyHeading[] = []
  opts.columns.forEach((types, col) => {
    types.forEach((type) => {
      const first = orders[col].find((id) => typeOfId(id) === type)
      const y = first ? byId.get(first)!.y0 - 8 : top - 8
      headings.push({ type, col, x: ml + col * colStep, y })
    })
  })

  return {
    nodes: orders.flat().map((id) => byId.get(id)!),
    links: outLinksResult,
    headings,
    crossings,
  }
}
