import { describe, expect, it } from 'vitest'
import {
  inversions,
  layoutSankey,
  orderNodes,
  orientLinks,
  type SankeyNodeIn,
} from './sankeyLayout'

const opts = { columns: [['a'], ['b'], ['c']], width: 600, height: 400 }

function node(id: string, type: string, sortKey?: string): SankeyNodeIn {
  return { id, type, sortKey }
}

describe('inversions', () => {
  it.each([
    [[], 0],
    [[1, 2, 3], 0],
    [[3, 2, 1], 3],
    [[2, 1, 2, 1], 3],
  ])('%j -> %i', (seq, want) => {
    expect(inversions(seq)).toBe(want)
  })
})

describe('orientLinks', () => {
  it('flips backward links and drops same-column and unknown ones', () => {
    const col = new Map([
      ['x', 0],
      ['y', 1],
      ['z', 1],
    ])
    const out = orientLinks(
      [
        { source: 'y', target: 'x' },
        { source: 'y', target: 'z' },
        { source: 'x', target: 'nope' },
      ],
      col
    )
    expect(out).toEqual([{ source: 'x', target: 'y', value: 1 }])
  })
})

describe('orderNodes', () => {
  it('removes a crossing between two columns', () => {
    const nodes = [node('a1', 'a', '1'), node('a2', 'a', '2'), node('b1', 'b'), node('b2', 'b')]
    const edges = [
      { source: 'a1', target: 'b2', value: 1 },
      { source: 'a2', target: 'b1', value: 1 },
    ]
    const { orders, crossings } = orderNodes(nodes, edges, {
      columns: [['a'], ['b']],
      fixedTypes: ['a'],
    })
    expect(crossings).toBe(0)
    expect(orders[0]).toEqual(['a1', 'a2'])
    expect(orders[1]).toEqual(['b2', 'b1'])
  })

  it('keeps fixed types in sortKey order', () => {
    const nodes = [node('n3', 'a', 'c'), node('n1', 'a', 'a'), node('n2', 'a', 'b'), node('m', 'b')]
    const { orders } = orderNodes(nodes, [{ source: 'n2', target: 'm', value: 1 }], {
      columns: [['a'], ['b']],
      fixedTypes: ['a'],
    })
    expect(orders[0]).toEqual(['n1', 'n2', 'n3'])
  })

  it('never ends worse than the input order on a random graph', () => {
    let seed = 7
    const rnd = () => (seed = (seed * 48271) % 2147483647) / 2147483647
    const nodes: SankeyNodeIn[] = []
    for (const [t, n] of [
      ['a', 20],
      ['b', 30],
      ['c', 20],
    ] as const) {
      for (let i = 0; i < n; i++) nodes.push(node(`${t}${i}`, t))
    }
    const edges = []
    for (let i = 0; i < 80; i++) {
      edges.push({
        source: `a${Math.floor(rnd() * 20)}`,
        target: `b${Math.floor(rnd() * 30)}`,
        value: 1,
      })
      edges.push({
        source: `b${Math.floor(rnd() * 30)}`,
        target: `c${Math.floor(rnd() * 20)}`,
        value: 1,
      })
    }
    const base = orderNodes(nodes, edges, {
      columns: [['a'], ['b'], ['c']],
      fixedTypes: ['a', 'b', 'c'],
    })
    const free = orderNodes(nodes, edges, { columns: [['a'], ['b'], ['c']] })
    expect(free.crossings).toBeLessThan(base.crossings)
  })
})

describe('layoutSankey', () => {
  const nodes = [
    node('a1', 'a'),
    node('a2', 'a'),
    node('b1', 'b'),
    node('c1', 'c'),
    node('lone', 'c'),
  ]
  const links = [
    { source: 'a1', target: 'b1' },
    { source: 'a2', target: 'b1' },
    { source: 'b1', target: 'c1' },
  ]

  it('places columns left to right inside the width', () => {
    const l = layoutSankey(nodes, links, { ...opts, marginLeft: 10, marginRight: 20 })
    const x = (id: string) => l.nodes.find((n) => n.id === id)!
    expect(x('a1').x0).toBe(10)
    expect(x('c1').x1).toBe(580)
    expect(x('b1').x0).toBeGreaterThan(x('a1').x1)
  })

  it('keeps nodes inside the height without overlap', () => {
    const l = layoutSankey(nodes, links, opts)
    for (const n of l.nodes) {
      expect(n.y0).toBeGreaterThanOrEqual(34)
      expect(n.y1).toBeLessThanOrEqual(390.0001)
    }
    const col = l.nodes.filter((n) => n.col === 2).sort((p, q) => p.y0 - q.y0)
    expect(col[1].y0).toBeGreaterThanOrEqual(col[0].y1)
  })

  it('gives a node without links a visible height', () => {
    const l = layoutSankey(nodes, links, opts)
    const lone = l.nodes.find((n) => n.id === 'lone')!
    expect(lone.y1 - lone.y0).toBeGreaterThan(0)
  })

  it('stacks ribbons inside the node they leave', () => {
    const l = layoutSankey(nodes, links, opts)
    const b1 = l.nodes.find((n) => n.id === 'b1')!
    for (const k of l.links.filter((k) => k.target === 'b1')) {
      expect(k.ty - k.width / 2).toBeGreaterThanOrEqual(b1.y0 - 1e-6)
      expect(k.ty + k.width / 2).toBeLessThanOrEqual(b1.y1 + 1e-6)
    }
    expect(l.links[0].path).toMatch(/^M[\d.]+,[\d.]+C/)
  })

  it('adds a heading gap when two types share a column', () => {
    const l = layoutSankey(
      [node('a1', 'a'), node('d1', 'd'), node('b1', 'b')],
      [{ source: 'a1', target: 'b1' }],
      { columns: [['a', 'd'], ['b']], width: 400, height: 300, typeGap: 30 }
    )
    const a = l.nodes.find((n) => n.id === 'a1')!
    const d = l.nodes.find((n) => n.id === 'd1')!
    expect(d.y0 - a.y1).toBeGreaterThanOrEqual(30 - 1e-6)
    expect(l.headings.map((h) => h.type)).toEqual(['a', 'd', 'b'])
  })

  it('ignores nodes of types outside the columns', () => {
    const l = layoutSankey([...nodes, node('x', 'zzz')], links, opts)
    expect(l.nodes.find((n) => n.id === 'x')).toBeUndefined()
  })

  it('handles an empty graph', () => {
    const l = layoutSankey([], [], opts)
    expect(l.nodes).toEqual([])
    expect(l.links).toEqual([])
  })

  it('keeps every node of a long column inside the drawing area', () => {
    const many = Array.from({ length: 600 }, (_, i) => node(`a${i}`, 'a'))
    const height = 600 * 17 + 28 + 80 // what SankeyView asks for
    const l = layoutSankey(many, [], { columns: [['a']], width: 600, height })
    const ys = l.nodes.map((n) => n.y0)
    expect(Math.min(...ys)).toBeGreaterThanOrEqual(0)
    expect(Math.max(...l.nodes.map((n) => n.y1))).toBeLessThanOrEqual(height)
  })

  it('sorts a node without a sort key before keyed ones', () => {
    const l = layoutSankey(
      [node('CTL-3', 'a'), node('CTL-1', 'a', '1.1'), node('CTL-2', 'a', '1.2')],
      [],
      { columns: [['a']], width: 600, height: 400, fixedTypes: ['a'] }
    )
    expect(l.nodes.sort((x, y) => x.y0 - y.y0).map((n) => n.id)).toEqual([
      'CTL-3',
      'CTL-1',
      'CTL-2',
    ])
  })
})
