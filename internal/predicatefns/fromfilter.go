package predicatefns

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// FromFilter transpiles a parsed filter (the legacy `--where` /
// metamodel `When:`/`Then:` string syntax) into an equivalent predicate
// SOURCE expression over the `entity` record. The result is a string
// meant to be predicate.Compile'd against an Env built with
// EntityRecordType(meta, def) + Declare (so the host funcs match/regex/
// fuzzy/contains and the typed literals resolve).
//
// It lives in internal/predicatefns — NOT internal/filter — because it
// references the host-function names (FuncMatch etc.) and predicatefns
// already imports filter (a transpiler in filter would be a cycle).
//
// Typing: a value is emitted as a typed literal matching the property's
// declared predicate type (so `count>9` compiles numeric, `due<'2026-..'`
// as a date), driven by ScalarTypeForProp / the entity def. An unknown
// property, or an operator the property type can't support, is a
// transpile ERROR — never a silently-different predicate (RR-NKWJS6).
//
// The empty/missing-value contract (RR-S251K, pinned in
// parity_missing_test.go) is reproduced with presence guards:
//
//	prop=value   -> entity.prop ~= nil and entity.prop == <value>
//	prop!=value  -> entity.prop ~= nil and entity.prop ~= <value>
//	prop=        -> entity.prop == nil or entity.prop == ''
//	prop!=       -> entity.prop ~= nil and entity.prop ~= ''
//
// User values are escaped as Lua string literals via luaStringLiteral so
// a value containing a quote/backslash/newline cannot break out of the
// generated source (RR-TQEHO4).
func FromFilter(meta *metamodel.Metamodel, def *metamodel.EntityDef, f *filter.Filter) (string, error) {
	prop, ok := def.Properties[f.Property]
	if !ok {
		return "", fmt.Errorf("predicatefns: unknown property %q on entity type", f.Property)
	}
	if _, modeled := ScalarTypeForProp(meta, &prop); !modeled {
		return "", fmt.Errorf("predicatefns: property %q has a type (%s) that predicate cannot model", f.Property, prop.Type)
	}

	acc := "entity." + f.Property

	// Empty-value forms: prop= / prop!= are presence/emptiness checks,
	// independent of the property's type.
	if f.Value == "" && !f.IsGlob {
		//nolint:exhaustive // only the two empty-value ops are special here; others fall through
		switch f.Operator {
		case filter.OpEqual:
			return fmt.Sprintf("%s == nil or %s == ''", acc, acc), nil
		case filter.OpNotEqual:
			return fmt.Sprintf("%s ~= nil and %s ~= ''", acc, acc), nil
		}
	}

	// List properties: filter's `=` is "ANY element matches", `!=` is
	// "NO element matches" — but a missing/empty list matches NEITHER
	// (match.go: missing/empty matches nothing except `prop=` empty).
	if prop.List {
		return fromFilterList(acc, f)
	}

	return fromFilterScalar(acc, prop, f)
}

// presentGuard is the predicate prefix that reproduces filter's
// universal rule: a missing OR empty property value matches NOTHING
// (except the `prop=` empty form, handled separately). For a string
// property the value can be a non-nil empty string, so both `~= nil`
// and `~= ”` are required; for typed fields (int/date/bool) an empty
// stored value already coerces to Nil, so `~= nil` suffices. Returns a
// guard ending in " and " ready to prefix the comparison.
func presentGuard(acc string, prop metamodel.PropertyDef) string {
	switch prop.Type {
	case metamodel.PropertyTypeInteger, metamodel.PropertyTypeBoolean,
		metamodel.PropertyTypeDate, metamodel.PropertyTypeDatetime:
		return acc + " ~= nil and "
	default:
		return fmt.Sprintf("%s ~= nil and %s ~= '' and ", acc, acc)
	}
}

// fromFilterList maps a list property's = / != to contains(...). filter
// only supports = / != on lists (match.go matchList); any ordered/regex
// op on a list is a transpile error.
func fromFilterList(acc string, f *filter.Filter) (string, error) {
	elem := luaStringLiteral(f.Value)
	switch f.Operator {
	case filter.OpEqual:
		// ANY element equals value. A missing/empty list contains
		// nothing, so contains(...) is already false — no guard needed.
		return fmt.Sprintf("%s(%s, %s)", FuncContains, acc, elem), nil
	case filter.OpNotEqual:
		// filter's list != is "NO element equals value" BUT a missing/
		// empty list matches nothing (returns false), not true. So the
		// list must be non-empty AND not contain the value.
		return fmt.Sprintf("%s(%s) > 0 and not %s(%s, %s)", FuncLen, acc, FuncContains, acc, elem), nil
	default:
		return "", fmt.Errorf("predicatefns: operator %q is not supported on a list property", f.Operator)
	}
}

// fromFilterScalar maps a scalar property comparison. Ordered ops emit a
// typed literal; glob/regex/fuzzy emit a host-func call. String equality
// with a glob pattern routes through match(). Unsupported combinations
// (notably fuzzy-with-wildcard) are transpile errors (RR-NKWJS6).
func fromFilterScalar(acc string, prop metamodel.PropertyDef, f *filter.Filter) (string, error) {
	guard := presentGuard(acc, prop)
	switch f.Operator {
	case filter.OpEqual, filter.OpNotEqual:
		if f.IsGlob {
			return fromFilterGlob(acc, guard, f)
		}
		lit, err := typedLiteral(prop, f.Value)
		if err != nil {
			return "", err
		}
		op := "=="
		if f.Operator == filter.OpNotEqual {
			op = "~="
		}
		// Present-guarded so a missing/empty field matches neither = nor
		// != (parity with filter's missing/empty-value contract).
		return fmt.Sprintf("%s%s %s %s", guard, acc, op, lit), nil

	case filter.OpLess, filter.OpLessEqual, filter.OpGreater, filter.OpGreaterEqual:
		lit, err := typedLiteral(prop, f.Value)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s%s %s %s", guard, acc, orderedOp(f.Operator), lit), nil

	case filter.OpRegex:
		return fmt.Sprintf("%s%s(%s, %s)", guard, FuncRegex, acc, luaStringLiteral(f.Value)), nil

	case filter.OpFuzzy:
		// Two-phase fuzzy (fuzzy target + wildcard suffix) has no faithful
		// predicate host-fn equivalent — refuse rather than emit a
		// subtly-different predicate (RR-NKWJS6).
		if f.WildcardRe != nil {
			return "", fmt.Errorf("predicatefns: fuzzy-with-wildcard (%q) has no predicate equivalent; not transpilable", f.Value)
		}
		return fmt.Sprintf("%s%s(%s, %s)", guard, FuncFuzzy, acc, luaStringLiteral(f.FuzzyTarget)), nil
	}
	return "", fmt.Errorf("predicatefns: unsupported operator %q", f.Operator)
}

// fromFilterGlob maps a glob-pattern string equality/inequality to
// match(). != negates via `not`. The guard reproduces filter's
// missing/empty-matches-nothing rule.
func fromFilterGlob(acc, guard string, f *filter.Filter) (string, error) {
	call := fmt.Sprintf("%s(%s, %s)", FuncMatch, acc, luaStringLiteral(f.Value))
	if f.Operator == filter.OpNotEqual {
		return fmt.Sprintf("%snot %s", guard, call), nil
	}
	return fmt.Sprintf("%s%s", guard, call), nil
}

// typedLiteral renders a filter value as a predicate literal appropriate
// to the property's predicate type. The predicate's compile-time literal
// coercion (walkRelational) then retypes it against the declared field
// type: a date field's string literal parses to a Date, an int field's
// number literal to an Int.
//
//   - Int    -> bare integer number literal (validated here; a
//     non-integer value is a transpile error, matching filter which
//     rejects a non-integer against an integer field).
//   - Bool   -> true / false keyword (non-bool value is a transpile error).
//   - Date   -> a Lua string literal; predicate coerces it to a Date at
//     compile against the field layout. An unparseable date is caught
//     at Compile, not here (the layout lives in pt).
//   - String / everything else -> a Lua string literal.
func typedLiteral(prop metamodel.PropertyDef, value string) (string, error) {
	switch prop.Type {
	case metamodel.PropertyTypeInteger:
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return "", fmt.Errorf("predicatefns: %q is not an integer for an integer property", value)
		}
		return strconv.FormatInt(n, 10), nil
	case metamodel.PropertyTypeBoolean:
		switch value {
		case "true", "false":
			return value, nil
		}
		return "", fmt.Errorf("predicatefns: %q is not a boolean (true/false)", value)
	default:
		// String, enum, and date all take a Lua string literal here; for
		// a date, predicate's compile-time coercion parses it against the
		// field layout (the field is declared DateTypeWithLayout).
		return luaStringLiteral(value), nil
	}
}

func orderedOp(op filter.Operator) string {
	switch op {
	case filter.OpLess:
		return "<"
	case filter.OpLessEqual:
		return "<="
	case filter.OpGreater:
		return ">"
	case filter.OpGreaterEqual:
		return ">="
	default:
		return "?"
	}
}

// luaStringLiteral renders s as a single-quoted Lua string literal with
// every metacharacter escaped, so an arbitrary user value (incl. quotes,
// backslashes, newlines, control bytes) cannot break out of the
// generated predicate source (RR-TQEHO4). Uses Lua decimal `\ddd`
// escapes for control bytes.
func luaStringLiteral(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('\'')
	for i := range len(s) {
		c := s[i]
		switch c {
		case '\'':
			b.WriteString(`\'`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case 0:
			b.WriteString(`\0`)
		default:
			if c < 0x20 || c == 0x7f {
				fmt.Fprintf(&b, `\%d`, c)
			} else {
				b.WriteByte(c)
			}
		}
	}
	b.WriteByte('\'')
	return b.String()
}
