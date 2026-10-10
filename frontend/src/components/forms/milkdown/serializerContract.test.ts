import { describe, it, expect } from 'vitest'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative, resolve } from 'node:path'
import { unified } from 'unified'
import remarkParse from 'remark-parse'
import remarkStringify from 'remark-stringify'
import remarkGfm from 'remark-gfm'
import { RELA_STRINGIFY_OPTIONS, isSemanticallyEqual, semanticShape } from './serializerContract'

/**
 * The processor under test. This must stay the same stack Milkdown runs:
 * remark-parse, remark-gfm, and remark-stringify with rela's options.
 */
const processor = unified()
  .use(remarkParse)
  .use(remarkGfm)
  .use(remarkStringify, RELA_STRINGIFY_OPTIONS)

function roundTrip(markdown: string): string {
  return String(processor.stringify(processor.parse(markdown)))
}

/**
 * Parses to the structural shape `isSemanticallyEqual` compares. The cast is
 * the one place the mdast union is narrowed; every node has `type`, and
 * `semanticShape` reads nothing else without going through `prop`.
 */
function parse(markdown: string): { type: string } {
  return processor.runSync(processor.parse(markdown)) as { type: string }
}

/**
 * Round-trips `markdown` twice, parsing each text once. The parse trees are
 * reused for the semantic comparison, which halves the parse work of
 * `roundTrip` plus `parse` on the corpus sweep. Stringify does not mutate
 * the tree, and the transform step runs only after it.
 */
function checkCorpusBody(markdown: string) {
  const srcTree = processor.parse(markdown)
  const once = String(processor.stringify(srcTree))
  const onceTree = processor.parse(once)
  const twice = String(processor.stringify(onceTree))
  const equal = isSemanticallyEqual(
    processor.runSync(srcTree) as { type: string },
    processor.runSync(onceTree) as { type: string }
  )
  return { once, twice, equal }
}

/** Strips YAML frontmatter, leaving the body the editor would receive. */
function body(raw: string): string {
  if (!raw.startsWith('---\n')) return raw
  const end = raw.indexOf('\n---\n', 4)
  if (end === -1) return raw
  return raw.slice(end + 5).replace(/^\n+/, '')
}

function walk(dir: string, out: string[] = []): string[] {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name)
    if (statSync(full).isDirectory()) walk(full, out)
    else if (name.endsWith('.md')) out.push(full)
  }
  return out
}

describe('serializer contract', () => {
  describe('option choices', () => {
    it('writes bullets as - and keeps one-space indent', () => {
      expect(roundTrip('* a\n* b\n')).toBe('- a\n- b\n')
      expect(roundTrip('- a\n  - nested\n')).toBe('- a\n  - nested\n')
    })

    it('writes strong as ** and emphasis as _', () => {
      expect(roundTrip('__bold__ and *italic*\n')).toBe('**bold** and _italic_\n')
    })

    it('writes thematic breaks as ---', () => {
      expect(roundTrip('***\n')).toBe('---\n')
    })

    it('keeps code fences fenced with their language', () => {
      expect(roundTrip('```go\nx := 1\n```\n')).toBe('```go\nx := 1\n```\n')
    })

    it('writes ATX headings without a closing hash run', () => {
      expect(roundTrip('## Title ##\n')).toBe('## Title\n')
      expect(roundTrip('Title\n=====\n')).toBe('# Title\n')
    })

    it('preserves an entity ref code span verbatim', () => {
      expect(roundTrip('See `TKT-U2R7GU` for detail.\n')).toBe('See `TKT-U2R7GU` for detail.\n')
    })

    it('preserves GFM tables, task lists and strikethrough', () => {
      const table = '| a | b |\n| - | - |\n| 1 | 2 |\n'
      expect(parse(roundTrip(table))).toEqual(expect.objectContaining({ type: 'root' }))
      expect(isSemanticallyEqual(parse(table), parse(roundTrip(table)))).toBe(true)
      expect(roundTrip('- [ ] todo\n- [x] done\n')).toBe('- [ ] todo\n- [x] done\n')
      expect(roundTrip('~~gone~~\n')).toBe('~~gone~~\n')
    })
  })

  describe('semantic comparison', () => {
    it('ignores a newline folded inside an inline code span', () => {
      const a = parse('`foo\nbar`\n')
      const b = parse('`foo bar`\n')
      expect(isSemanticallyEqual(a, b)).toBe(true)
    })

    it('does not ignore a changed link target', () => {
      expect(isSemanticallyEqual(parse('[x](/a)'), parse('[x](/b)'))).toBe(false)
    })

    it('does not ignore a changed heading depth', () => {
      expect(isSemanticallyEqual(parse('# t'), parse('## t'))).toBe(false)
    })

    it('does not ignore dropped text', () => {
      expect(isSemanticallyEqual(parse('a b'), parse('a'))).toBe(false)
    })

    it('does not ignore a changed code language', () => {
      expect(isSemanticallyEqual(parse('```go\nx\n```'), parse('```js\nx\n```'))).toBe(false)
    })

    it('excludes source positions from the shape', () => {
      expect(JSON.stringify(semanticShape(parse('# hi')))).not.toContain('position')
    })
  })

  /**
   * The Phase 0 gate. Every entity body in the repository must survive a
   * round-trip with its meaning intact, and a second round-trip must be a
   * fixed point. Byte-level churn is reported but does not fail: the
   * write-back guard suppresses it at save time.
   */
  describe('corpus round-trip', () => {
    // Resolved from the repo root rather than the test file, so a moved test
    // fails loudly on the corpus-size assertion instead of silently scanning
    // an empty set.
    const repoRoot = resolve(process.cwd(), '..')
    const roots = [join(repoRoot, 'tickets/entities'), join(repoRoot, 'docs-project')]
    const all = roots.flatMap((r) => walk(r))

    /**
     * The full sweep takes over a minute, which is too slow for the normal
     * unit-test run. By default every Nth file is checked, which still covers
     * several hundred bodies of every entity type. Set RELA_FULL_CORPUS=1 to
     * sweep all of them; CI runs that variant.
     */
    const FULL = process.env.RELA_FULL_CORPUS === '1'
    const STRIDE = 8
    const files = FULL ? all : all.filter((_, i) => i % STRIDE === 0)

    it('finds the corpus', () => {
      expect(all.length).toBeGreaterThan(500)
      expect(files.length).toBeGreaterThan(100)
    })

    /**
     * One test per directory under each root, split into chunks of at most
     * CHUNK files. The corpus grows with every ticket, and a single test over
     * all of it outgrew its timeout (178 s of 180 s on CI). Chunks keep each
     * test far below the cap however large one type grows.
     */
    const CHUNK = 250
    const byDir = new Map<string, string[]>()
    for (const file of files) {
      const root = roots.find((r) => file.startsWith(r + '/'))
      if (!root) throw new Error(`corpus file outside the roots: ${file}`)
      const dir = relative(repoRoot, join(root, relative(root, file).split('/')[0]))
      const list = byDir.get(dir)
      if (list) list.push(file)
      else byDir.set(dir, [file])
    }
    const groups: [string, string[]][] = []
    for (const [dir, list] of byDir) {
      const n = Math.ceil(list.length / CHUNK)
      for (let i = 0; i < n; i++) {
        const name = n > 1 ? `${dir} [${i + 1}/${n}]` : dir
        groups.push([name, list.slice(i * CHUNK, (i + 1) * CHUNK)])
      }
    }

    it.each(groups)(
      'preserves meaning and converges for every entity body in %s',
      (group, groupFiles) => {
        const semanticDrift: string[] = []
        const notIdempotent: string[] = []
        let byteChurn = 0

        for (const file of groupFiles) {
          const src = body(readFileSync(file, 'utf8'))
          if (!src.trim()) continue

          const { once, twice, equal } = checkCorpusBody(src)

          if (!equal) semanticDrift.push(file)
          if (once !== twice) notIdempotent.push(file)
          if (once !== src) byteChurn++
        }

        // Reported for visibility; byte churn is handled by the write-back guard.
        console.log(
          `corpus ${group}: ${groupFiles.length} files, ${byteChurn} with byte churn, ` +
            `${semanticDrift.length} semantic drift, ${notIdempotent.length} non-idempotent`
        )

        expect(semanticDrift).toEqual([])
        expect(notIdempotent).toEqual([])
      },
      120_000
    )

    // A sampled pass is not the gate. RELA_STRINGIFY_OPTIONS decides how
    // every stored file gets rewritten, so a green run over one eighth of
    // them should not read as clearance to change it.
    if (!FULL) {
      it('warns that the sweep is sampled', () => {
        console.warn(
          `corpus: SAMPLED 1-in-${STRIDE} of ${all.length}. This is NOT the full gate — run ` +
            `RELA_FULL_CORPUS=1 npm run test:run before changing ` +
            `RELA_STRINGIFY_OPTIONS or semanticShape. CI runs the full sweep.`
        )
      })
    }
  })
})

describe('semantic comparison treats fenced code as significant', () => {
  // Indentation inside a fenced block IS the content. Collapsing it would let
  // the write-back guard classify a reindented Python snippet as suppressible
  // churn and write the corruption back to the file.
  it('reports a reindented code block as different', () => {
    const a = parse('```python\nif x:\n    return 1\n```\n')
    const b = parse('```python\nif x:\nreturn 1\n```\n')
    expect(isSemanticallyEqual(a, b)).toBe(false)
  })

  it('reports a reindented yaml block as different', () => {
    const a = parse('```yaml\nroot:\n  child: 1\n```\n')
    const b = parse('```yaml\nroot:\nchild: 1\n```\n')
    expect(isSemanticallyEqual(a, b)).toBe(false)
  })

  it('reports raw html with changed whitespace as different', () => {
    const a = parse('<div>\n  <span>x</span>\n</div>\n')
    const b = parse('<div>\n<span>x</span>\n</div>\n')
    expect(isSemanticallyEqual(a, b)).toBe(false)
  })

  // The case the normalization exists for: remark folds a newline inside an
  // inline code span to a space, which changes bytes but not meaning.
  it('still ignores a folded newline inside an inline code span', () => {
    const a = parse('a `one\ntwo` b\n')
    const b = parse('a `one two` b\n')
    expect(isSemanticallyEqual(a, b)).toBe(true)
  })

  it('still ignores paragraph re-wrapping', () => {
    const a = parse('one two\nthree four\n')
    const b = parse('one two three four\n')
    expect(isSemanticallyEqual(a, b)).toBe(true)
  })
})
