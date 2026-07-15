import { describe, it, expect, vi, beforeEach } from 'vitest'
import { parse, ConditionError, _clearCache, type Bindings } from './conditions'

beforeEach(() => {
  _clearCache()
})

// parse() throws on static errors; eval() never throws. evalWith exercises the
// happy path (parse succeeds, then eval).
function evalWith(expr: string, bindings: Bindings): boolean {
  return parse(expr).eval(bindings)
}

describe('conditions', () => {
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
    })

    it('a set field is not nil', () => {
      expect(evalWith('form.present != nil', { form: { present: 'x' } })).toBe(true)
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

    it('=~ regex matches a literal pattern (RR-9IQBT)', () => {
      expect(evalWith("form.name =~ '^foo'", { form: { name: 'foobar' } })).toBe(true)
      expect(evalWith("form.name =~ '^foo'", { form: { name: 'barfoo' } })).toBe(false)
      // Invalid-literal-regex behavior is covered in the ReDoS block (it throws
      // at parse, since a literal pattern is statically knowable).
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
    it('function-call syntax PARSES but eval rejects the call (false)', () => {
      // A call is not a static error (the registry may exist later), so parse
      // succeeds; there is simply no function to call yet, so eval is false.
      const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const prog = parse("has_role('editor')") // does not throw
      expect(prog.eval({})).toBe(false)
      warn.mockRestore()
    })
  })

  describe('prototype-pollution guard (RR + security)', () => {
    it('throws on __proto__/constructor/prototype field references at parse', () => {
      // A forbidden field name is a static error — reject loudly at parse.
      expect(() => parse('form.__proto__ == 1')).toThrow(ConditionError)
      expect(() => parse('form.constructor == 1')).toThrow(ConditionError)
      expect(() => parse('form.prototype == 1')).toThrow(ConditionError)
    })

    it('does not read inherited properties, only own', () => {
      // A binding whose prototype has a poisoned key must not resolve it.
      const poisoned = Object.create({ inherited: 'boom' }) as Record<string, unknown>
      expect(evalWith("form.inherited == 'boom'", { form: poisoned })).toBe(false)
      expect(evalWith('form.inherited == nil', { form: poisoned })).toBe(true)
    })
  })

  describe('parse errors throw ConditionError (RR-8GRLD)', () => {
    it('throws on malformed / static errors', () => {
      // Static config bugs fail loud at parse — not swallowed to a silent false.
      expect(() => parse('form. == ==')).toThrow(ConditionError)
      expect(() => parse('> > >')).toThrow(ConditionError)
      expect(() => parse('form.a < form.b < form.c')).toThrow(ConditionError) // chaining
      expect(() => parse("form.x == 'unterminated")).toThrow(ConditionError)
      expect(() => parse('bareword')).toThrow(ConditionError)
      expect(() => parse('unknown_ns.field == 1')).toThrow(ConditionError)
      expect(() => parse('')).toThrow(ConditionError) // empty is not "true" — caller's policy
    })

    it('eval never throws — a valid parse with a nil reference is false', () => {
      // The eval side stays fail-safe: an unknown-but-syntactically-valid field
      // resolves to nil, never throws.
      expect(evalWith('form.typo_field == true', { form: {} })).toBe(false)
    })

    it('rejects deeply nested expressions at parse (node budget)', () => {
      const deep = 'not '.repeat(1000) + 'true'
      expect(() => parse(deep)).toThrow(/too complex/)
    })

    it('rejects a long flat and/or chain at parse, before eval can overflow (RR-P3HL8)', () => {
      // A flat chain builds a deep left AST spine that evalNode recurses over;
      // the node budget rejects it at parse rather than risk an eval-time
      // stack overflow.
      const clauses = Array.from({ length: 5000 }, (_, i) => `form.x == ${i}`)
      expect(() => parse(clauses.join(' or '))).toThrow(/too complex/)
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
    it('a LITERAL invalid/oversized pattern throws at parse (statically knowable)', () => {
      expect(() => parse("form.v =~ '('")).toThrow(/invalid regex/) // bad syntax
      const evil = "'" + '(a+)+' + 'a'.repeat(300) + "'"
      expect(() => parse(`form.v =~ ${evil}`)).toThrow(/too long/) // oversized
    })

    it('a DYNAMIC (binding-sourced) oversized pattern is fail-safe at eval, not executed', () => {
      const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const evilPat = '(a+)+' + 'a'.repeat(300)
      const prog = parse('form.v =~ form.pat') // parses fine — pattern unknown yet
      const t0 = performance.now()
      expect(prog.eval({ form: { v: 'x', pat: evilPat } })).toBe(false)
      expect(performance.now() - t0).toBeLessThan(100) // rejected on length, not run
      expect(warn).toHaveBeenCalledWith(expect.stringContaining('too long'))
      warn.mockRestore()
    })

    it('a normal literal pattern still matches', () => {
      expect(evalWith("form.v =~ '^foo'", { form: { v: 'foobar' } })).toBe(true)
    })

    it('a dynamic normal-length pattern still matches', () => {
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
    it('parse returns the same Program instance for the same source', () => {
      const a = parse("form.x == 'y'")
      const b = parse("form.x == 'y'")
      expect(a).toBe(b)
    })

    it('a repeated bad source re-throws (failure is memoized too)', () => {
      expect(() => parse('bad ==')).toThrow(ConditionError)
      expect(() => parse('bad ==')).toThrow(ConditionError)
    })
  })
})
