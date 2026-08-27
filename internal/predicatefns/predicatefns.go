// Package predicatefns is the shared host-function standard library for
// the predicate engine. It lives outside internal/predicate (which
// depends on nothing — see .go-arch-lint.yml) because these functions
// reach into internal/filter for the RE2 pattern/trigram machinery.
//
// The functions are string matchers (glob/regex/fuzzy), a list
// membership test (contains), and a today() date builder. Register them
// on a predicate.Env with Declare and on a predicate.Bindings with
// Bind; the two must agree, so use the paired helpers here rather than
// wiring names by hand.
//
// # ReDoS safety (RR-N176T)
//
// The predicate eval step budget bounds the NUMBER of IR steps but not
// wall-clock time inside a single host call. regex/glob/fuzzy are safe
// only because Go's regexp is RE2 (non-backtracking, linear time): a
// pathological pattern cannot cause catastrophic backtracking. This is
// enforced structurally by routing every pattern through
// internal/filter's regexp.Compile-based helpers — never a backtracking
// engine. Do not add a matcher here that uses anything other than
// stdlib regexp.
package predicatefns

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// errArg is returned when a host func receives an argument whose runtime
// type doesn't match its declared signature. The compile-time type
// checker should prevent this; the runtime guard fails the Eval.
var errArg = errors.New("predicatefns: host function argument type mismatch")

// Names of the host functions this package registers. Exported so
// callers can reference them in generated predicate source without
// stringly-typed drift.
const (
	FuncMatch    = "match"    // match(s, glob) bool
	FuncRegex    = "regex"    // regex(s, pattern) bool
	FuncFuzzy    = "fuzzy"    // fuzzy(s, target) bool
	FuncContains = "contains" // contains(list, elem) bool
	FuncLen      = "len"      // len(list) number
	FuncToday    = "today"    // today() date
)

// fuzzyThreshold mirrors filter.DefaultFuzzyThreshold semantics for the
// fuzzy() host func: a trigram similarity at or above this counts as a
// match. Kept local so a future tuning of one path doesn't silently
// shift the other.
const fuzzyThreshold = filter.DefaultFuzzyThreshold

// Declare registers every stdlib function on env. Call once per Env,
// after declaring variables, before Compile. today() returns DateType;
// the string matchers and contains return bool.
func Declare(env *predicate.Env) error {
	str := predicate.StringType
	strList := predicate.ListType{Elem: predicate.StringType}
	twoStr := predicate.FuncSig{Params: []predicate.Type{str, str}, Return: predicate.BoolType}

	decls := []struct {
		name string
		sig  predicate.FuncSig
	}{
		{FuncMatch, twoStr},
		{FuncRegex, twoStr},
		{FuncFuzzy, twoStr},
		{FuncContains, predicate.FuncSig{Params: []predicate.Type{strList, str}, Return: predicate.BoolType}},
		{FuncLen, predicate.FuncSig{Params: []predicate.Type{strList}, Return: predicate.NumberType}},
		{FuncToday, predicate.FuncSig{Return: predicate.DateType, SQLPortable: true}},
		{FuncDaysBetween, predicate.FuncSig{
			Params: []predicate.Type{predicate.DateType, predicate.DateType},
			Return: predicate.IntType, SQLPortable: true,
		}},
		{FuncDateAdd, predicate.FuncSig{
			Params: []predicate.Type{predicate.DateType, predicate.NumberType, str},
			Return: predicate.DateType, SQLPortable: true,
		}},
		{FuncRruleNext, predicate.FuncSig{
			Params: []predicate.Type{str, predicate.DateType},
			Return: predicate.DateType,
		}},
	}
	for _, d := range decls {
		if err := env.DeclareFunc(d.name, d.sig); err != nil {
			return fmt.Errorf("predicatefns: declare %s: %w", d.name, err)
		}
	}
	return nil
}

// Bind registers the implementations on b, matching Declare. `now` is
// the instant today() returns, truncated to UTC midnight; passing it in
// keeps the functions pure and testable (predicate scripts must not
// read the wall clock during Eval — the caller decides "now").
//
// today() is truncated in UTC, not now.Location(), to match how date
// literals are parsed (parseDateLiteral / time.Parse yield UTC when the
// layout carries no zone). A local-midnight today() compared against a
// UTC-midnight literal-derived Date would skew up to a day (RR-YPYTP);
// pinning both to UTC keeps `entity.due < today()` consistent.
func Bind(b *predicate.Bindings, now time.Time) error {
	utc := now.UTC()
	day := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	binds := []struct {
		name string
		fn   predicate.FuncFunc
	}{
		{FuncMatch, matchGlob},
		{FuncRegex, matchRegex},
		{FuncFuzzy, matchFuzzy},
		{FuncContains, contains},
		{FuncLen, listLen},
		{FuncToday, func(context.Context, []predicate.Value) (predicate.Value, error) {
			return predicate.NewDate(day), nil
		}},
		{FuncDaysBetween, daysBetween},
		{FuncDateAdd, dateAdd},
		{FuncRruleNext, rruleNext},
	}
	for _, bd := range binds {
		if err := b.SetFunc(bd.name, bd.fn); err != nil {
			return fmt.Errorf("predicatefns: bind %s: %w", bd.name, err)
		}
	}
	return nil
}

// matchGlob implements match(s, glob): glob-style wildcard match,
// compiled to an RE2 regexp via internal/filter.ParsePattern.
func matchGlob(_ context.Context, args []predicate.Value) (predicate.Value, error) {
	s, pat, err := twoStrings(args)
	if err != nil {
		return nil, err
	}
	re, _, perr := filter.ParsePattern(pat)
	if perr != nil {
		return nil, fmt.Errorf("predicatefns: match: %w", perr)
	}
	return predicate.NewBool(re.MatchString(s)), nil
}

// matchRegex implements regex(s, pattern): stdlib (RE2) regexp match.
func matchRegex(_ context.Context, args []predicate.Value) (predicate.Value, error) {
	s, pat, err := twoStrings(args)
	if err != nil {
		return nil, err
	}
	re, cErr := regexp.Compile(pat)
	if cErr != nil {
		return nil, fmt.Errorf("predicatefns: regex: %w", cErr)
	}
	return predicate.NewBool(re.MatchString(s)), nil
}

// matchFuzzy implements fuzzy(s, target): trigram similarity at or above
// the threshold, reusing internal/filter.TrigramSimilarity.
func matchFuzzy(_ context.Context, args []predicate.Value) (predicate.Value, error) {
	s, target, err := twoStrings(args)
	if err != nil {
		return nil, err
	}
	if target == "" {
		return predicate.NewBool(false), nil
	}
	return predicate.NewBool(filter.TrigramSimilarity(s, target) >= fuzzyThreshold), nil
}

// contains implements contains(list, elem): true iff any element of the
// string list equals elem.
func contains(_ context.Context, args []predicate.Value) (predicate.Value, error) {
	if len(args) != 2 {
		return nil, errArg
	}
	list, ok := args[0].(predicate.List)
	if !ok {
		return nil, errArg
	}
	elem, ok := args[1].(predicate.String)
	if !ok {
		return nil, errArg
	}
	for _, e := range list.Elems() {
		if s, ok := e.(predicate.String); ok && s.String() == elem.String() {
			return predicate.NewBool(true), nil
		}
	}
	return predicate.NewBool(false), nil
}

// listLen implements len(list): the number of elements. Used by the
// filter transpiler to distinguish an empty/missing list (which filter's
// list `!=` treats as "matches nothing") from a populated one.
func listLen(_ context.Context, args []predicate.Value) (predicate.Value, error) {
	if len(args) != 1 {
		return nil, errArg
	}
	list, ok := args[0].(predicate.List)
	if !ok {
		return nil, errArg
	}
	return predicate.NewNumberFromInt(len(list.Elems())), nil
}

// twoStrings extracts exactly two string args.
func twoStrings(args []predicate.Value) (first, second string, err error) {
	if len(args) != 2 {
		return "", "", errArg
	}
	a, ok := args[0].(predicate.String)
	if !ok {
		return "", "", errArg
	}
	b, ok := args[1].(predicate.String)
	if !ok {
		return "", "", errArg
	}
	return a.String(), b.String(), nil
}
