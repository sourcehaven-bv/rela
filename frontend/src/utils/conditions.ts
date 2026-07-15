/**
 * conditions — a small, self-contained boolean-expression engine for
 * config-driven conditions in the data-entry SPA (form `visible_when` /
 * `required_when`, and later ACL button enable/disable and view visibility).
 *
 * # Why hand-written
 *
 * The only comparable engine, Go's `internal/predicate`, is Go-only: it
 * consumes gopher-lua's parser, builds a typed IR, and type-checks at compile
 * time. Porting it would mean a Lua parser in the SPA and a second
 * implementation to keep in sync. This grammar has no statements and no
 * scoping, so a hand-written Pratt parser + tree-walk evaluator is small and
 * has no dependencies.
 *
 * # Grammar (Lua 5.1 precedence, low → high)
 *
 *   1. `or`
 *   2. `and`
 *   3. comparison: `==` `!=` `<` `<=` `>` `>=` `=~`   (non-associative)
 *   4. `not`  (unary prefix)
 *   5. primary: literal | reference | call | `( expr )`
 *
 * `and`/`or` are left-associative; comparison does not chain (`a < b < c` is a
 * parse error). Unary `not` binds tighter than comparison, so `not a == b`
 * parses as `(not a) == b` — matching Lua and `internal/predicate`.
 *
 * Literals: single-quoted strings (`\'` and `\\` escapes), numbers
 * (int/float/exp), `true`, `false`, `nil`. References are single-level dotted
 * identifiers rooted at a binding namespace: `form.<field>`, `entity.<field>`,
 * `current_user.<field>`. Function calls (`name(arg, …)`) parse but are
 * rejected at eval time — the host-function registry is deferred to the ACL
 * consumer (see the ticket).
 *
 * # Congruence with `internal/predicate` — with asterisks
 *
 * Surface and precedence match predicate so authors see one condition
 * language and a future server-evaluated path stays a drop-in. Two deliberate
 * divergences: `!=` is the canonical not-equal here (predicate uses `~=`,
 * which is accepted as a silent alias), and this engine is **permissive**
 * where predicate is strict — cross-type comparisons coerce here (see
 * {@link compareEq}) instead of being compile errors, and boolean positions
 * coerce truthiness instead of requiring a bool.
 *
 * # Fail-safe
 *
 * Nothing throws at eval time. A per-node eval error (bad reference, rejected
 * call, invalid regex) coerces to `false` *locally* and evaluation continues,
 * so one broken reference does not sink an otherwise-valid `or`. An expression
 * that fails to *parse* compiles to a constant-`false` program plus a
 * `console.warn` — the caller still gets a usable `Program`.
 */

/**
 * Maximum number of AST nodes a single expression may produce. This bounds
 * BOTH nesting and flat operator chains (a long `a or b or c …` builds a deep
 * left spine that `evalNode` recurses over), so it — not a nesting-only depth
 * counter — is what keeps eval-time recursion from overflowing the stack.
 * Comfortably above any hand-authored condition; far below a stack-blowing size.
 */
const MAX_NODES = 500

/** Maximum length of a `=~` regex pattern, to bound ReDoS exposure. */
const MAX_REGEX_LENGTH = 200

/** A compiled, reusable condition. Immutable; safe to cache and re-eval. */
export interface Program {
  /** The original source expression, for diagnostics. */
  readonly source: string
  /** Evaluate against a set of named bindings; never throws. */
  eval(bindings: Bindings): boolean
}

/**
 * Binding namespaces. Each key (`form`, `entity`, `current_user`) maps to a
 * flat record of field values. Missing namespaces and fields read as `nil`.
 */
export type Bindings = Record<string, Record<string, unknown> | undefined>

// ---------------------------------------------------------------------------
// AST
// ---------------------------------------------------------------------------

type Node =
  | { kind: 'lit'; value: string | number | boolean | null }
  | { kind: 'ref'; ns: string; field: string }
  | { kind: 'call'; name: string; args: Node[] }
  | { kind: 'not'; expr: Node }
  | { kind: 'logical'; op: 'and' | 'or'; left: Node; right: Node }
  | { kind: 'compare'; op: CompareOp; left: Node; right: Node }

type CompareOp = '==' | '!=' | '<' | '<=' | '>' | '>=' | '=~'

// ---------------------------------------------------------------------------
// Tokenizer
// ---------------------------------------------------------------------------

type TokenType =
  'ident' | 'number' | 'string' | 'op' | 'lparen' | 'rparen' | 'comma' | 'dot' | 'eof'

interface Token {
  type: TokenType
  value: string
  pos: number
}

/** Identifier-segment allowlist, mirroring utils/filters.ts PROPERTY_NAME_RE. */
const IDENT_RE = /[a-zA-Z_][a-zA-Z0-9_]*/y

/** Segments that must never be used to index a binding object. */
const FORBIDDEN_KEYS = new Set(['__proto__', 'constructor', 'prototype'])

export class ConditionError extends Error {}

function tokenize(src: string): Token[] {
  const tokens: Token[] = []
  let i = 0
  const n = src.length

  const twoCharOps = new Set(['==', '!=', '<=', '>=', '=~', '~='])

  while (i < n) {
    const c = src[i]

    if (c === ' ' || c === '\t' || c === '\n' || c === '\r') {
      i++
      continue
    }

    if (c === '(') {
      tokens.push({ type: 'lparen', value: c, pos: i++ })
      continue
    }
    if (c === ')') {
      tokens.push({ type: 'rparen', value: c, pos: i++ })
      continue
    }
    if (c === ',') {
      tokens.push({ type: 'comma', value: c, pos: i++ })
      continue
    }
    if (c === '.') {
      tokens.push({ type: 'dot', value: c, pos: i++ })
      continue
    }

    // String literal: single-quoted with \' and \\ escapes.
    if (c === "'") {
      const start = i
      i++
      let out = ''
      let closed = false
      while (i < n) {
        const ch = src[i]
        if (ch === '\\') {
          const next = src[i + 1]
          if (next === "'" || next === '\\') {
            out += next
            i += 2
            continue
          }
          // Unknown escape: keep the backslash literally.
          out += ch
          i++
          continue
        }
        if (ch === "'") {
          i++
          closed = true
          break
        }
        out += ch
        i++
      }
      if (!closed) {
        throw new ConditionError(`unterminated string literal at ${start}`)
      }
      tokens.push({ type: 'string', value: out, pos: start })
      continue
    }

    // Number literal: int / float / exponent (no leading sign; unary minus is
    // not in the grammar).
    if (c >= '0' && c <= '9') {
      const start = i
      const numRe = /[0-9]+(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?/y
      numRe.lastIndex = i
      const m = numRe.exec(src)
      if (!m || m.index !== i) {
        throw new ConditionError(`malformed number at ${start}`)
      }
      i += m[0].length
      tokens.push({ type: 'number', value: m[0], pos: start })
      continue
    }

    // Two-character operators before single-char ones.
    const two = src.slice(i, i + 2)
    if (twoCharOps.has(two)) {
      tokens.push({ type: 'op', value: two, pos: i })
      i += 2
      continue
    }
    if (c === '<' || c === '>' || c === '=') {
      tokens.push({ type: 'op', value: c, pos: i++ })
      continue
    }

    // Identifier (also matches keywords and/or/not/true/false/nil).
    if (/[a-zA-Z_]/.test(c)) {
      IDENT_RE.lastIndex = i
      const m = IDENT_RE.exec(src)
      if (!m || m.index !== i) {
        throw new ConditionError(`invalid identifier at ${i}`)
      }
      i += m[0].length
      tokens.push({ type: 'ident', value: m[0], pos: m.index })
      continue
    }

    throw new ConditionError(`unexpected character ${JSON.stringify(c)} at ${i}`)
  }

  tokens.push({ type: 'eof', value: '', pos: n })
  return tokens
}

// ---------------------------------------------------------------------------
// Parser (Pratt / recursive descent)
// ---------------------------------------------------------------------------

const KEYWORDS = new Set(['and', 'or', 'not', 'true', 'false', 'nil'])
const NAMESPACES = new Set(['form', 'entity', 'current_user'])
const COMPARE_OPS = new Set(['==', '!=', '~=', '=~', '<', '<=', '>', '>='])

class Parser {
  private pos = 0
  private nodes = 0

  constructor(private readonly tokens: Token[]) {}

  /**
   * Record one AST node and enforce the total-node budget. Called for every
   * node the parser emits so that both deep nesting and long flat operator
   * chains are bounded uniformly — this is the guard that prevents eval-time
   * stack overflow, since `evalNode` recurses over the built tree.
   */
  private node<T extends Node>(n: T): T {
    if (++this.nodes > MAX_NODES) {
      throw new ConditionError(`expression too complex (>${MAX_NODES} nodes)`)
    }
    return n
  }

  parse(): Node {
    const node = this.parseOr()
    this.expect('eof')
    return node
  }

  private peek(): Token {
    return this.tokens[this.pos]
  }

  private next(): Token {
    return this.tokens[this.pos++]
  }

  private expect(type: TokenType): Token {
    const tok = this.peek()
    if (tok.type !== type) {
      throw new ConditionError(`expected ${type} but found ${describeToken(tok)} at ${tok.pos}`)
    }
    return this.next()
  }

  private isKeyword(word: string): boolean {
    const tok = this.peek()
    return tok.type === 'ident' && tok.value === word
  }

  // or := and ('or' and)*
  private parseOr(): Node {
    let left = this.parseAnd()
    while (this.isKeyword('or')) {
      this.next()
      const right = this.parseAnd()
      left = this.node({ kind: 'logical', op: 'or', left, right })
    }
    return left
  }

  // and := compare ('and' compare)*
  private parseAnd(): Node {
    let left = this.parseCompare()
    while (this.isKeyword('and')) {
      this.next()
      const right = this.parseCompare()
      left = this.node({ kind: 'logical', op: 'and', left, right })
    }
    return left
  }

  // compare := not (COMPARE_OP not)?   — non-associative (no chaining)
  private parseCompare(): Node {
    const left = this.parseNot()
    const tok = this.peek()
    if (tok.type === 'op' && COMPARE_OPS.has(tok.value)) {
      this.next()
      const right = this.parseNot()
      // A second comparison operator here would be chaining — reject it.
      const after = this.peek()
      if (after.type === 'op' && COMPARE_OPS.has(after.value)) {
        throw new ConditionError(`comparison operators do not chain at ${after.pos}`)
      }
      return this.node({ kind: 'compare', op: normalizeCompareOp(tok.value), left, right })
    }
    return left
  }

  // not := 'not' not | primary
  private parseNot(): Node {
    if (this.isKeyword('not')) {
      this.next()
      const expr = this.parseNot()
      return this.node({ kind: 'not', expr })
    }
    return this.parsePrimary()
  }

  private parsePrimary(): Node {
    const tok = this.peek()

    if (tok.type === 'lparen') {
      // A parenthesized group adds no node of its own — it returns the inner
      // expression directly — so it does not consume the node budget twice.
      this.next()
      const inner = this.parseOr()
      this.expect('rparen')
      return inner
    }

    if (tok.type === 'string') {
      this.next()
      return this.node({ kind: 'lit', value: tok.value })
    }

    if (tok.type === 'number') {
      this.next()
      return this.node({ kind: 'lit', value: Number(tok.value) })
    }

    if (tok.type === 'ident') {
      return this.parseIdentExpr()
    }

    throw new ConditionError(`unexpected ${describeToken(tok)} at ${tok.pos}`)
  }

  private parseIdentExpr(): Node {
    const ident = this.next() // ident

    // Keyword literals.
    if (ident.value === 'true') return this.node({ kind: 'lit', value: true })
    if (ident.value === 'false') return this.node({ kind: 'lit', value: false })
    if (ident.value === 'nil') return this.node({ kind: 'lit', value: null })
    if (KEYWORDS.has(ident.value)) {
      throw new ConditionError(`unexpected keyword '${ident.value}' at ${ident.pos}`)
    }

    // Function call: name(args)
    if (this.peek().type === 'lparen') {
      this.next()
      const args: Node[] = []
      if (this.peek().type !== 'rparen') {
        args.push(this.parseOr())
        while (this.peek().type === 'comma') {
          this.next()
          args.push(this.parseOr())
        }
      }
      this.expect('rparen')
      return this.node({ kind: 'call', name: ident.value, args })
    }

    // Dotted reference: namespace.field
    if (this.peek().type === 'dot') {
      this.next()
      const field = this.expect('ident')
      if (!NAMESPACES.has(ident.value)) {
        throw new ConditionError(
          `unknown namespace '${ident.value}' (expected form/entity/current_user) at ${ident.pos}`
        )
      }
      if (FORBIDDEN_KEYS.has(field.value)) {
        throw new ConditionError(`forbidden field name '${field.value}' at ${field.pos}`)
      }
      // A second dot would be a multi-level path — not supported.
      if (this.peek().type === 'dot') {
        throw new ConditionError(`nested field access is not supported at ${this.peek().pos}`)
      }
      return this.node({ kind: 'ref', ns: ident.value, field: field.value })
    }

    throw new ConditionError(
      `bare identifier '${ident.value}' — use a namespace (form./entity./current_user.) at ${ident.pos}`
    )
  }
}

function describeToken(tok: Token): string {
  if (tok.type === 'eof') return 'end of input'
  return `'${tok.value}'`
}

function normalizeCompareOp(op: string): CompareOp {
  return op === '~=' ? '!=' : (op as CompareOp)
}

// ---------------------------------------------------------------------------
// Evaluator
// ---------------------------------------------------------------------------

/** Sentinel thrown internally and caught per-node to coerce to false. */
class EvalFail extends Error {}

const NIL = Symbol('nil')
type Value = string | number | boolean | typeof NIL

function evalNode(node: Node, bindings: Bindings): Value {
  switch (node.kind) {
    case 'lit':
      return node.value === null ? NIL : node.value

    case 'ref':
      return resolveRef(node.ns, node.field, bindings)

    case 'call':
      // Registry deferred: no host functions exist yet.
      throw new EvalFail(`no such function: ${node.name}`)

    case 'not':
      return !truthy(evalNode(node.expr, bindings))

    case 'logical': {
      // Short-circuit on the truthiness of the (fail-safe) left operand.
      const left = truthyFailSafe(node.left, bindings)
      if (node.op === 'and') {
        return left ? truthyFailSafe(node.right, bindings) : false
      }
      return left ? true : truthyFailSafe(node.right, bindings)
    }

    case 'compare':
      return compare(node.op, node.left, node.right, bindings)
  }
}

/** Evaluate a node's truthiness, coercing any per-node error to false. */
function truthyFailSafe(node: Node, bindings: Bindings): boolean {
  try {
    return truthy(evalNode(node, bindings))
  } catch (err) {
    if (err instanceof EvalFail) return false
    throw err
  }
}

function resolveRef(ns: string, field: string, bindings: Bindings): Value {
  // Belt-and-suspenders: the parser already rejects __proto__/constructor/
  // prototype field names, so this guard is unreachable from a compiled
  // expression. It stays because resolveRef could be reached from a
  // hand-constructed AST in a future caller, and the cost is one Set lookup.
  if (FORBIDDEN_KEYS.has(field)) throw new EvalFail(`forbidden field: ${field}`)
  const scope = bindings[ns]
  if (scope == null || typeof scope !== 'object') return NIL
  if (!Object.prototype.hasOwnProperty.call(scope, field)) return NIL
  const raw = (scope as Record<string, unknown>)[field]
  return normalizeValue(raw)
}

/** Coerce an arbitrary bound value into the engine's Value domain. */
function normalizeValue(raw: unknown): Value {
  if (raw === null || raw === undefined) return NIL
  if (typeof raw === 'boolean' || typeof raw === 'number' || typeof raw === 'string') {
    return raw
  }
  // Arrays/objects have no meaningful comparison in this grammar. Map them to
  // NIL (which only equals nil) rather than their string form — stringifying
  // would make `form.tags == '1,2'` accidentally match `[1, 2]`, a baffling
  // coincidence. A non-scalar simply never equals a literal.
  return NIL
}

/** Truthiness: nil/false/''/0 are falsy; everything else truthy. */
function truthy(v: Value): boolean {
  if (v === NIL) return false
  if (typeof v === 'boolean') return v
  if (typeof v === 'number') return v !== 0 && !Number.isNaN(v)
  return v !== ''
}

function compare(op: CompareOp, leftN: Node, rightN: Node, bindings: Bindings): boolean {
  const left = evalNode(leftN, bindings)
  const right = evalNode(rightN, bindings)

  switch (op) {
    case '==':
      return compareEq(left, right)
    case '!=':
      return !compareEq(left, right)
    case '=~':
      return compareRegex(left, right)
    case '<':
    case '<=':
    case '>':
    case '>=':
      return compareOrdered(op, left, right)
  }
}

/**
 * Permissive equality. Differs from predicate's strict Lua equality by design
 * (see the module docstring). One side is typically a literal, the other a
 * resolved binding value that may be a string even when a bool/number is meant.
 */
function compareEq(a: Value, b: Value): boolean {
  // nil only equals nil.
  if (a === NIL || b === NIL) return a === b

  // Order the pair as (literal-ish, value) is irrelevant; coerce by the type of
  // either operand that is a bool or number.
  if (typeof a === 'boolean' || typeof b === 'boolean') {
    const ab = toBool(a)
    const bb = toBool(b)
    return ab !== undefined && bb !== undefined && ab === bb
  }
  if (typeof a === 'number' || typeof b === 'number') {
    const an = toNumber(a)
    const bn = toNumber(b)
    return an !== undefined && bn !== undefined && an === bn
  }
  // Both strings: byte-for-byte.
  return String(a) === String(b)
}

function compareOrdered(op: CompareOp, a: Value, b: Value): boolean {
  if (a === NIL || b === NIL) return false
  const an = toNumber(a)
  const bn = toNumber(b)
  let cmp: number
  if (an !== undefined && bn !== undefined) {
    cmp = an < bn ? -1 : an > bn ? 1 : 0
  } else {
    const as = String(a)
    const bs = String(b)
    cmp = as < bs ? -1 : as > bs ? 1 : 0
  }
  switch (op) {
    case '<':
      return cmp < 0
    case '<=':
      return cmp <= 0
    case '>':
      return cmp > 0
    case '>=':
      return cmp >= 0
    default:
      return false
  }
}

function compareRegex(value: Value, pattern: Value): boolean {
  if (pattern === NIL || value === NIL) return false
  const src = String(pattern)
  // Cap the pattern length. JS's backtracking RegExp engine has no match
  // timeout, so a pathological pattern (e.g. `(a+)+$`) can hang the render
  // thread indefinitely — a length cap is a coarse but effective ceiling on
  // that exposure. `=~` patterns are expected to be trusted config, not
  // free-form user input; this is defence-in-depth, not the primary boundary.
  if (src.length > MAX_REGEX_LENGTH) {
    console.warn(`[conditions] regex pattern too long (>${MAX_REGEX_LENGTH} chars); rejected`)
    return false
  }
  let re: RegExp
  try {
    re = new RegExp(src)
  } catch {
    console.warn(`[conditions] invalid regex ${JSON.stringify(src)}`)
    return false
  }
  return re.test(String(value))
}

function toBool(v: Value): boolean | undefined {
  if (typeof v === 'boolean') return v
  if (typeof v === 'string') {
    const s = v.toLowerCase()
    if (s === 'true') return true
    if (s === 'false') return false
  }
  return undefined
}

/**
 * Strict decimal numeral: optional sign, digits, optional fraction, optional
 * exponent. Deliberately narrower than JS `Number()`, which also accepts hex
 * (`0x10`), binary (`0b1`), `Infinity`, and surrounding whitespace — none of
 * which a config author means by "a numeric string", and all of which would
 * diverge from the Go `internal/predicate` path this grammar tracks.
 */
const DECIMAL_RE = /^[+-]?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?$/

function toNumber(v: Value): number | undefined {
  if (typeof v === 'number') return Number.isFinite(v) ? v : undefined
  if (typeof v === 'string' && DECIMAL_RE.test(v)) {
    const n = Number(v)
    return Number.isFinite(n) ? n : undefined
  }
  return undefined
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

const cache = new Map<string, Program>()

/** A program that always evaluates to false — used when parsing fails. */
function constFalse(source: string): Program {
  return { source, eval: () => false }
}

/**
 * Compile an expression into a reusable {@link Program}. Compilation never
 * throws: a parse error logs a warning and returns a constant-false program so
 * callers can treat every expression uniformly. Results are memoized by source
 * string.
 */
export function compile(source: string): Program {
  const cached = cache.get(source)
  if (cached) return cached

  let program: Program
  try {
    const ast = new Parser(tokenize(source)).parse()
    program = {
      source,
      eval(bindings: Bindings): boolean {
        try {
          return truthy(evalNode(ast, bindings))
        } catch (err) {
          if (err instanceof EvalFail) return false
          // Unexpected error: stay fail-safe rather than crash a render.
          console.warn(`[conditions] eval error in ${JSON.stringify(source)}:`, err)
          return false
        }
      },
    }
  } catch (err) {
    const reason = err instanceof Error ? err.message : String(err)
    console.warn(`[conditions] parse error in ${JSON.stringify(source)}: ${reason}`)
    program = constFalse(source)
  }

  cache.set(source, program)
  return program
}

/**
 * Convenience one-shot: compile (memoized) and evaluate. An empty or
 * whitespace-only expression is treated as "no condition" and returns true.
 */
export function evaluate(source: string | undefined | null, bindings: Bindings): boolean {
  if (source == null || source.trim() === '') return true
  return compile(source).eval(bindings)
}

/** Test-only: clear the compile cache. */
export function _clearCache(): void {
  cache.clear()
}
