import { describe, it, expect, vi, beforeEach } from 'vitest'
import { compile, evaluate, _clearCache, type Bindings } from './conditions'

beforeEach(() => {
  _clearCache()
})

function evalWith(expr: string, bindings: Bindings): boolean {
  return compile(expr).eval(bindings)
}

describe('conditions', () => {
  describe('empty / no-condition', () => {
    it('treats empty/whitespace/undefined as "always true"', () => {
      expect(evaluate('', {})).toBe(true)
      expect(evaluate('   ', {})).toBe(true)
      expect(evaluate(undefined, {})).toBe(true)
      expect(evaluate(null, {})).toBe(true)
    })
  })

  describe('references & literals', () => {
    it('resolves form/entity/current_user namespaces', () => {
      const b: Bindings = {
        form: { kind: 'note' },
        entity: { status: 'open' },
        current_user: { name: 'jeroen' },
      }
      expect(evalWith("form.kind == 'note'", b)).toBe(true)
      expect(evalWith("entity.status == 'open'", b)).toBe(true)
      expect(evalWith("current_user.name == 'jeroen'", b)).toBe(true)
    })

    it('unset field and missing namespace read as nil', () => {
      expect(evalWith('form.absent == nil', { form: {} })).toBe(true)
      expect(evalWith('form.absent == nil', {})).toBe(true)
      expect(evalWith('form.absent != nil', { form: { absent: 'x' } })).toBe(true)
    })

    it('parses boolean, number, string and nil literals', () => {
      expect(evalWith('true', {})).toBe(true)
      expect(evalWith('false', {})).toBe(false)
      expect(evalWith("form.x == 'a b c'", { form: { x: 'a b c' } })).toBe(true)
      expect(evalWith('form.n == 1.5e1', { form: { n: 15 } })).toBe(true)
    })

    it('handles escaped quotes and backslashes in strings', () => {
      expect(evalWith("form.x == 'it\\'s'", { form: { x: "it's" } })).toBe(true)
      expect(evalWith("form.x == 'a\\\\b'", { form: { x: 'a\\b' } })).toBe(true)
    })
  })

  describe('coercion & equality table (RR-9IQBT)', () => {
    it('bool literal matches real boolean and string true/false', () => {
      expect(evalWith('form.f == true', { form: { f: true } })).toBe(true)
      expect(evalWith('form.f == true', { form: { f: 'true' } })).toBe(true)
      expect(evalWith('form.f == true', { form: { f: 'TRUE' } })).toBe(true)
      expect(evalWith('form.f == true', { form: { f: false } })).toBe(false)
      expect(evalWith('form.f == true', { form: { f: 'on' } })).toBe(false)
      expect(evalWith('form.f != true', { form: { f: false } })).toBe(true)
    })

    it('number literal matches number and numeric string', () => {
      expect(evalWith('form.c == 3', { form: { c: 3 } })).toBe(true)
      expect(evalWith('form.c == 3', { form: { c: '3' } })).toBe(true)
      expect(evalWith('form.c == 3', { form: { c: '3.0' } })).toBe(true)
      expect(evalWith('form.c == 3', { form: { c: 'three' } })).toBe(false)
      expect(evalWith('form.c == 3', { form: {} })).toBe(false)
    })

    it('string literal compares byte-for-byte', () => {
      expect(evalWith("form.k == 'note'", { form: { k: 'note' } })).toBe(true)
      expect(evalWith("form.k == 'note'", { form: { k: 'Note' } })).toBe(false)
    })

    it('ordered comparisons: numeric then lexicographic fallback', () => {
      expect(evalWith('form.n < 10', { form: { n: 5 } })).toBe(true)
      expect(evalWith('form.n < 10', { form: { n: '5' } })).toBe(true)
      expect(evalWith('form.n >= 10', { form: { n: 10 } })).toBe(true)
      expect(evalWith("form.d < '2025-02-01'", { form: { d: '2025-01-15' } })).toBe(true)
      expect(evalWith("form.d < '2025-02-01'", { form: { d: '2025-03-15' } })).toBe(false)
    })

    it('ordered comparison against nil is false, never throws', () => {
      expect(evalWith('form.missing < 10', { form: {} })).toBe(false)
      expect(evalWith('form.missing > 10', { form: {} })).toBe(false)
    })

    it('=~ regex matches, invalid regex is false + warns (RR-9IQBT)', () => {
      expect(evalWith("form.name =~ '^foo'", { form: { name: 'foobar' } })).toBe(true)
      expect(evalWith("form.name =~ '^foo'", { form: { name: 'barfoo' } })).toBe(false)
      const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
      expect(evalWith("form.name =~ '('", { form: { name: 'x' } })).toBe(false)
      expect(warn).toHaveBeenCalled()
      warn.mockRestore()
    })
  })

  describe('operators, precedence & associativity (RR-8VZSP)', () => {
    it('a == b and c == d or e groups as ((a==b) and (c==d)) or e', () => {
      // e true dominates via or, regardless of the and clause.
      const b: Bindings = { form: { a: 1, b: 2, c: 1, d: 1, e: true } }
      expect(evalWith('form.a == form.b and form.c == form.d or form.e', b)).toBe(true)
      // e false, and-clause false -> overall false
      expect(
        evalWith('form.a == form.b and form.c == form.d or form.e', {
          form: { a: 1, b: 2, c: 1, d: 1, e: false },
        })
      ).toBe(false)
      // e false, and-clause true -> overall true
      expect(
        evalWith('form.a == form.b and form.c == form.d or form.e', {
          form: { a: 1, b: 1, c: 1, d: 1, e: false },
        })
      ).toBe(true)
    })

    it('not binds tighter than comparison: not a == b is (not a) == b', () => {
      // not(form.flag) yields a boolean; compared to form.expected.
      // form.flag = false -> (not false)=true ; expected true -> equal.
      expect(
        evalWith('not form.flag == form.expected', {
          form: { flag: false, expected: true },
        })
      ).toBe(true)
      // form.flag = true -> (not true)=false ; expected true -> not equal.
      expect(
        evalWith('not form.flag == form.expected', {
          form: { flag: true, expected: true },
        })
      ).toBe(false)
    })

    it('parentheses override precedence', () => {
      expect(
        evalWith('form.a == 1 and (form.b == 2 or form.c == 3)', {
          form: { a: 1, b: 9, c: 3 },
        })
      ).toBe(true)
      expect(
        evalWith('form.a == 1 and (form.b == 2 or form.c == 3)', {
          form: { a: 1, b: 9, c: 9 },
        })
      ).toBe(false)
    })

    it('accepts ~= as an alias for != (RR-YTKIC)', () => {
      expect(evalWith("form.k ~= 'note'", { form: { k: 'book' } })).toBe(true)
      expect(evalWith("form.k ~= 'note'", { form: { k: 'note' } })).toBe(false)
    })

    it('cross-field OR', () => {
      const expr = 'form.has_processors == true or form.is_joint_controller == true'
      expect(evalWith(expr, { form: { has_processors: true, is_joint_controller: false } })).toBe(
        true
      )
      expect(evalWith(expr, { form: { has_processors: false, is_joint_controller: true } })).toBe(
        true
      )
      expect(evalWith(expr, { form: { has_processors: false, is_joint_controller: false } })).toBe(
        false
      )
    })
  })

  describe('short-circuit & fail-safe (RR-TNMRC)', () => {
    it('or: an erroring left operand coerces to false, right still decides', () => {
      const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
      // unknown_fn(...) errors -> false; right side true -> overall true
      expect(evalWith('missing_fn() or form.ok == true', { form: { ok: true } })).toBe(true)
      warn.mockRestore()
    })

    it('and: an erroring left operand short-circuits to false', () => {
      const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
      expect(evalWith('missing_fn() and form.ok == true', { form: { ok: true } })).toBe(false)
      warn.mockRestore()
    })

    it('and short-circuits: false left does not require right', () => {
      expect(evalWith('false and form.x == 1', { form: {} })).toBe(false)
    })

    it('or short-circuits: true left does not require right', () => {
      expect(evalWith('true or form.x == 1', { form: {} })).toBe(true)
    })
  })

  describe('deferred host functions (RR-P6GVE)', () => {
    it('function-call syntax parses but eval rejects the call (false)', () => {
      // Parses cleanly (no parse-error warning), evaluates to false.
      const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
      expect(evalWith("has_role('editor')", {})).toBe(false)
      // No parse-error warning should have fired (it's an eval-time rejection).
      expect(warn).not.toHaveBeenCalledWith(expect.stringContaining('parse error'))
      warn.mockRestore()
    })
  })

  describe('prototype-pollution guard (RR + security)', () => {
    it('rejects __proto__/constructor/prototype field references at parse time', () => {
      const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
      // These fail to parse -> constant-false program + warning.
      expect(evalWith('form.__proto__ == 1', { form: {} })).toBe(false)
      expect(evalWith('form.constructor == 1', { form: {} })).toBe(false)
      expect(evalWith('form.prototype == 1', { form: {} })).toBe(false)
      expect(warn).toHaveBeenCalled()
      warn.mockRestore()
    })

    it('does not read inherited properties, only own', () => {
      // A binding whose prototype has a poisoned key must not resolve it.
      const poisoned = Object.create({ inherited: 'boom' }) as Record<string, unknown>
      expect(evalWith("form.inherited == 'boom'", { form: poisoned })).toBe(false)
      expect(evalWith('form.inherited == nil', { form: poisoned })).toBe(true)
    })
  })

  describe('parse errors are fail-safe (RR-8GRLD)', () => {
    it('malformed expression compiles to constant-false + warns', () => {
      const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
      expect(evalWith('form. == ==', {})).toBe(false)
      expect(evalWith('> > >', {})).toBe(false)
      expect(evalWith('form.a < form.b < form.c', {})).toBe(false) // chaining
      expect(evalWith("form.x == 'unterminated", {})).toBe(false)
      expect(evalWith('bareword', {})).toBe(false)
      expect(evalWith('unknown_ns.field == 1', {})).toBe(false)
      expect(warn).toHaveBeenCalled()
      warn.mockRestore()
    })

    it('rejects deeply nested expressions (node budget)', () => {
      const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const deep = 'not '.repeat(1000) + 'true'
      expect(evalWith(deep, {})).toBe(false)
      expect(warn).toHaveBeenCalled()
      warn.mockRestore()
    })

    it('rejects a long flat and/or chain BEFORE eval can overflow (RR-P3HL8)', () => {
      // A flat chain builds a deep left AST spine that evalNode recurses over;
      // the node budget must reject it at parse time rather than let a
      // legitimately-true expression silently return false at eval.
      const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const clauses = Array.from({ length: 5000 }, (_, i) => `form.x == ${i}`)
      const longChain = clauses.join(' or ')
      // x matches the last clause, so a working evaluator would say true; the
      // budget rejects it first -> constant-false program.
      expect(evalWith(longChain, { form: { x: 4999 } })).toBe(false)
      expect(warn).toHaveBeenCalledWith(expect.stringContaining('too complex'))
      warn.mockRestore()
    })

    it('a chain within the node budget still evaluates correctly', () => {
      const clauses = Array.from({ length: 20 }, (_, i) => `form.x == ${i}`)
      expect(evalWith(clauses.join(' or '), { form: { x: 19 } })).toBe(true)
      expect(evalWith(clauses.join(' or '), { form: { x: 99 } })).toBe(false)
    })

    it('deeply parenthesized single value stays within budget (parens are free)', () => {
      // Parens no longer consume the node budget, so wrapping is cheap.
      const wrapped = '('.repeat(100) + 'true' + ')'.repeat(100)
      expect(evalWith(wrapped, {})).toBe(true)
    })
  })

  describe('ReDoS guard on =~ (RR-IROUO)', () => {
    it('rejects an over-long regex pattern instead of running it', () => {
      const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
      // A pattern sourced from a binding, longer than the cap.
      const evil = '(a+)+' + 'a'.repeat(300)
      const t0 = performance.now()
      expect(evalWith('form.v =~ form.pat', { form: { v: 'x', pat: evil } })).toBe(false)
      const elapsed = performance.now() - t0
      // Must be rejected on length, not executed — so it returns effectively
      // instantly rather than backtracking.
      expect(elapsed).toBeLessThan(100)
      expect(warn).toHaveBeenCalledWith(expect.stringContaining('too long'))
      warn.mockRestore()
    })

    it('a normal-length pattern still matches', () => {
      expect(evalWith('form.v =~ form.pat', { form: { v: 'foobar', pat: '^foo' } })).toBe(true)
    })

    it('=~ against a nil value or nil pattern is false', () => {
      expect(evalWith('form.missing =~ form.pat', { form: { pat: '.*' } })).toBe(false)
      expect(evalWith('form.v =~ form.missing', { form: { v: 'x' } })).toBe(false)
    })
  })

  describe('numeric-string coercion is strict decimal (RR-ATHC2)', () => {
    it('accepts plain decimal, fraction, exponent, sign', () => {
      expect(evalWith('form.n == 3', { form: { n: '3' } })).toBe(true)
      expect(evalWith('form.n == 3', { form: { n: '3.0' } })).toBe(true)
      expect(evalWith('form.n == 1000', { form: { n: '1e3' } })).toBe(true)
      expect(evalWith('form.n == 3', { form: { n: '+3' } })).toBe(true)
    })

    it('rejects hex / binary / whitespace / Infinity strings', () => {
      // These strings must NOT coerce to the decimal literal.
      expect(evalWith('form.n == 16', { form: { n: '0x10' } })).toBe(false)
      expect(evalWith('form.n == 5', { form: { n: '0b101' } })).toBe(false)
      expect(evalWith('form.n == 5', { form: { n: '  5  ' } })).toBe(false)
      // 'Infinity' does not coerce to a number: equality with a numeric
      // literal is false (it never becomes ±∞). (Ordered comparison would fall
      // back to lexicographic string compare, which is the documented
      // numeric-else-string rule — not asserted here.)
      expect(evalWith('form.n == 1000000', { form: { n: 'Infinity' } })).toBe(false)
    })
  })

  describe('non-scalar bound values (RR-KR035)', () => {
    it('arrays and objects read as nil, never matching a literal', () => {
      expect(evalWith("form.tags == '1,2'", { form: { tags: [1, 2] } })).toBe(false)
      expect(evalWith('form.tags == nil', { form: { tags: [1, 2] } })).toBe(true)
      expect(evalWith('form.obj == nil', { form: { obj: { a: 1 } } })).toBe(true)
    })
  })

  describe('memoization (RR-7VKNB)', () => {
    it('compile returns the same Program instance for the same source', () => {
      const a = compile("form.x == 'y'")
      const b = compile("form.x == 'y'")
      expect(a).toBe(b)
    })
  })
})
