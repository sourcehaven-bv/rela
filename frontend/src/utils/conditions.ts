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
 * # Two responsibilities, one throw boundary
 *
 * The engine does exactly two things, and the split mirrors the platform's own
 * (`new RegExp('[')` / `JSON.parse('{')` throw at construction; only matching
 * is lenient):
 *
 * - {@link parse} — **throws** {@link ConditionError} on a *statically* broken
 *   expression: syntax error, comparison chaining, bare/unknown namespace,
 *   over-budget size, or an invalid/oversized *literal* `=~` pattern. These are
 *   config bugs; they fail loud.
 * - {@link Program.eval} — **never throws.** A per-node *data-dependent* failure
 *   (a reference that resolves to nil in a bad way, a rejected function call, a
 *   *dynamic* `=~` pattern that only resolves at eval) coerces to `false`
 *   locally and evaluation continues, so one broken leaf does not sink an
 *   otherwise-valid `or`.
 *
 * Note the `=~` asymmetry, which is deliberate and matches JS: a *literal*
 * pattern is validated at parse (throws), a *binding-sourced* pattern is only
 * knowable at eval (fail-safe false + warn).
 *
 * The engine ships **no leniency wrapper and no one-shot helper**. Deciding what
 * a broken or unmet condition *means* — hide a branch, surface an inline error,
 * refuse to render — is the calling layer's policy, made where the error can be
 * surfaced. A caller that must not throw during render wraps {@link parse} in
 * its own try/catch and picks the fallback; the engine does not pick for it.
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
      const op = normalizeCompareOp(tok.value)
      // A `=~` whose pattern is a string LITERAL is statically knowable, so
      // validate it now: an invalid or oversized literal regex is a config
      // bug and throws at parse (like JS rejecting `/[/` at parse time). A
      // pattern that comes from a binding can only be checked at eval, where
      // it stays fail-safe (see compareRegex).
      if (op === '=~' && right.kind === 'lit' && typeof right.value === 'string') {
        validateRegexLiteral(right.value, tok.pos)
      }
      return this.node({ kind: 'compare', op, left, right })
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
  // Read the property BEFORE the own-property check. When `scope` is a Vue
  // reactive object, the read is what registers the dependency — including for
  // a key that is currently absent (undefined). Checking hasOwnProperty first
  // would short-circuit without ever reading, so a condition that references a
  // not-yet-set field would never re-evaluate when that field is later set.
  const raw = (scope as Record<string, unknown>)[field]
  // Own-property only: an inherited value (e.g. from a poisoned prototype) is
  // treated as absent, preserving the prototype-pollution guarantee.
  if (!Object.prototype.hasOwnProperty.call(scope, field)) return NIL
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

/**
 * Validate a `=~` pattern known at PARSE time (a string literal). Throws
 * {@link ConditionError} on an oversized or syntactically-invalid pattern so
 * the config bug surfaces loudly at parse, consistent with every other static
 * error. The length cap bounds ReDoS exposure: JS's backtracking RegExp engine
 * has no match timeout, so a pathological pattern (e.g. `(a+)+$`) could hang
 * the thread — the cap is a coarse ceiling on that.
 */
function validateRegexLiteral(pattern: string, pos: number): void {
  if (pattern.length > MAX_REGEX_LENGTH) {
    throw new ConditionError(`regex pattern too long (>${MAX_REGEX_LENGTH} chars) at ${pos}`)
  }
  try {
    new RegExp(pattern)
  } catch (err) {
    throw new ConditionError(
      `invalid regex ${JSON.stringify(pattern)} at ${pos}: ${err instanceof Error ? err.message : err}`
    )
  }
}

function compareRegex(value: Value, pattern: Value): boolean {
  if (pattern === NIL || value === NIL) return false
  const src = String(pattern)
  // Reached with a DYNAMIC pattern (from a binding) — literals were already
  // validated at parse. A dynamic pattern can only be checked now, so it stays
  // fail-safe: oversized or invalid → false + warn, never throw. (Same length
  // cap as validateRegexLiteral; see that function for the ReDoS rationale.)
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

// Memoize by source string. A successful parse caches its Program; a failed
// parse caches the ConditionError so a repeated bad string re-throws cheaply
// rather than re-tokenizing. This engine has exactly two responsibilities —
// parse (throws on static errors) and eval (never throws). It deliberately
// ships NO leniency wrapper and NO one-shot helper: deciding what a broken or
// unmet condition *means* (hide a branch? surface an inline error? refuse to
// render?) is the calling layer's policy, made where the error can actually be
// surfaced. A library that swallowed it would take that decision away.
const cache = new Map<string, Program | ConditionError>()

/**
 * Parse an expression into a reusable {@link Program}, or **throw**
 * {@link ConditionError} if it is statically malformed — syntax error,
 * comparison chaining, a bare/unknown namespace, an over-budget expression, or
 * an invalid/oversized *literal* `=~` pattern. Results (success and failure)
 * are memoized by source string.
 *
 * This mirrors the platform's own split: `new RegExp('[')` and `JSON.parse('{')`
 * throw at construction; only *evaluation* is fail-safe. Callers that must not
 * throw during render (e.g. a Vue computed) wrap this in their own try/catch and
 * choose the fallback — the engine does not choose for them.
 */
export function parse(source: string): Program {
  const cached = cache.get(source)
  if (cached instanceof ConditionError) throw cached
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
          // Unexpected error: eval must never throw, so stay fail-safe.
          console.warn(`[conditions] eval error in ${JSON.stringify(source)}:`, err)
          return false
        }
      },
    }
  } catch (err) {
    const condErr =
      err instanceof ConditionError
        ? err
        : new ConditionError(err instanceof Error ? err.message : String(err))
    cache.set(source, condErr)
    throw condErr
  }

  cache.set(source, program)
  return program
}

/** Test-only: clear the memoization cache. */
export function _clearCache(): void {
  cache.clear()
}
