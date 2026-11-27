package dataentry

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"html/template"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/htmlutil"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
	"github.com/Sourcehaven-BV/rela/internal/readtime"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/search/searchparser"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// nowFunc is the data-entry clock: the router stamps it on every request
// as the read time (see [attachReadTime]), and a path that runs outside a
// request reads it through [requestReadTime]. Tests override it to pin
// "now".
var nowFunc = func() time.Time { return time.Now().UTC() }

// requestReadTime is the operation's read time on ctx, else nowFunc().
func requestReadTime(ctx context.Context) time.Time {
	return readtime.Or(ctx, nowFunc)
}

// parseWhereAt parses where clauses and resolves their date variables at the
// read time now (see [filter.BindDateVariables]).
func parseWhereAt(where []string, now time.Time) ([]*filter.Filter, error) {
	filters, err := filter.ParseAll(where)
	if err != nil {
		return nil, err
	}
	return filter.BindDateVariables(filters, now), nil
}

// resolveFilterVariable substitutes variables in filter values: $user (the
// caller's principal id), $today with an optional offset ($today+7d,
// $today-2w; see [filter.ResolveDateVariable]), $now, $tomorrow and
// $yesterday.
//
// Dates are in UTC. Other values are returned unchanged, and so is $user when
// user is empty, so an anonymous caller does not match empty properties.
func resolveFilterVariable(value, user string, now time.Time) string {
	if value == "$user" {
		if user != "" {
			return user
		}
		return value
	}
	resolved, _ := filter.ResolveDateVariable(value, now)
	return resolved
}

// resolveFilterVariablesInList applies resolveFilterVariable to each
// comma-separated token in value. Used by the in/ne operators.
func resolveFilterVariablesInList(value, user string, now time.Time) string {
	if !strings.Contains(value, ",") {
		return resolveFilterVariable(value, user, now)
	}
	parts := strings.Split(value, ",")
	for i, p := range parts {
		parts[i] = resolveFilterVariable(strings.TrimSpace(p), user, now)
	}
	return strings.Join(parts, ",")
}

// TODO(TKT-HFEKVN): this list is the THIRD copy of the same layout set —
// metamodel.ParseDateValue and predicate.parseDateLiteral hold the other two,
// and their comments say they mirror each other by hand. They already disagree:
// this one accepts minute precision and ignores a property's declared format:
// (compareValues gets two bare strings and has no PropertyDef in scope), so a
// custom-format date compares correctly under --filter and incorrectly here.
//
// temporalLayouts are the layouts a filter value may use, tried in order.
// A bare date is listed first so it keeps winning for date-typed properties;
// the datetime layouts exist because a `datetime` property stores RFC3339 and
// comparing it against a window bound previously fell through to lexicographic
// string comparison (or errored against a bare-date bound), which silently
// excluded every entity — see TKT-IG54YO.
var temporalLayouts = []string{
	"2006-01-02",
	time.RFC3339,
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
}

// parseTemporal parses value as a date or datetime, returning the instant it
// denotes. A bare date denotes midnight UTC, so a bare-date bound compared
// against a datetime is a start-of-day bound — callers wanting an inclusive
// upper bound should use a half-open range against the next day rather than
// relying on end-of-day semantics.
//
// Nil: never returns a nil time — ok=false means the value is not temporal.
func parseTemporal(value string) (t time.Time, ok bool) {
	for _, layout := range temporalLayouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

// compareValues compares two values with lt, lte, gt or gte. Two temporal
// values (date or datetime) compare as instants, so a bare date bounds an
// RFC3339 value at midnight; two numbers compare numerically; otherwise
// strings compare lexicographically.
//
// On a type mismatch it returns false and an error, so "2026-04-07" <
// "tomorrow" never matches by string order.
func compareValues(left, right, operator string) (match bool, err error) {
	lt, lIsTime := parseTemporal(left)
	rt, rIsTime := parseTemporal(right)
	switch {
	case lIsTime && rIsTime:
		return compareOrdered(lt.Compare(rt), 0, operator), nil
	case lIsTime || rIsTime:
		return false, fmt.Errorf("cannot compare date %q with non-date %q",
			pickDate(left, right, lIsTime), pickNonDate(left, right, lIsTime))
	}

	lf, lNumErr := strconv.ParseFloat(left, 64)
	rf, rNumErr := strconv.ParseFloat(right, 64)
	switch {
	case lNumErr == nil && rNumErr == nil:
		return compareOrdered(lf, rf, operator), nil
	case lNumErr == nil || rNumErr == nil:
		return false, fmt.Errorf("cannot compare number %q with non-number %q",
			pickNumber(left, right, lNumErr == nil), pickNonNumber(left, right, lNumErr == nil))
	}

	return compareOrdered(left, right, operator), nil
}

// pickDate returns the side that successfully parsed as a date.
func pickDate(left, right string, leftIsDate bool) string {
	if leftIsDate {
		return left
	}
	return right
}

// pickNonDate returns the side that did NOT parse as a date.
func pickNonDate(left, right string, leftIsDate bool) string {
	if leftIsDate {
		return right
	}
	return left
}

// pickNumber / pickNonNumber are the numeric equivalents.
func pickNumber(left, right string, leftIsNum bool) string {
	if leftIsNum {
		return left
	}
	return right
}

func pickNonNumber(left, right string, leftIsNum bool) string {
	if leftIsNum {
		return right
	}
	return left
}

// compareOrdered applies an ordering operator to two ordered values.
// Returns false for unknown operators (caller is expected to validate).
func compareOrdered[T cmp.Ordered](left, right T, operator string) bool {
	c := cmp.Compare(left, right)
	switch operator {
	case "lt":
		return c < 0
	case "lte":
		return c <= 0
	case "gt":
		return c > 0
	case "gte":
		return c >= 0
	}
	return false
}

// propertyContains checks if a property value contains the given string.
// Handles string, []string, and []interface{} property types.
func propertyContains(prop any, value string) bool {
	return anyElement(prop, func(el string) bool { return el == value })
}

// propertyIsEmpty checks if a property value is empty/nil.
func propertyIsEmpty(prop any) bool {
	if prop == nil {
		return true
	}
	switch v := prop.(type) {
	case string:
		return v == ""
	case []string:
		return len(v) == 0
	case []any:
		return len(v) == 0
	default:
		return false
	}
}

// applyFilters filters entities by a set of filter conditions. A `$user`
// value is user (see [resolveFilterVariable]); with no identified user it
// matches no row, as on the list path. Other `$` variables are skipped.
//
//nolint:gocognit // applies each filter operator against each entity property; the branches are per-operator match semantics, not shared logic to extract.
func applyFilters(entities []*entity.Entity, filters []FilterConfig, user string) []*entity.Entity {
	if len(filters) == 0 {
		return entities
	}
	var result []*entity.Entity
	for _, e := range entities {
		match := true
		for _, f := range filters {
			if f.Value == "$user" {
				if user == "" {
					match = false
					break
				}
				f.Value = user
			}
			if strings.HasPrefix(f.Value, "$") {
				continue
			}
			prop := e.Properties[f.Property]
			switch f.Operator {
			case "=":
				if f.Value == "" {
					if !propertyIsEmpty(prop) {
						match = false
					}
				} else if !propertyContains(prop, f.Value) {
					match = false
				}
			case "!=":
				if f.Value == "" {
					if propertyIsEmpty(prop) {
						match = false
					}
				} else if propertyContains(prop, f.Value) {
					match = false
				}
			}
		}
		if match {
			result = append(result, e)
		}
	}
	return result
}

// resolvePropertyValues returns allowed values for a property from its definition or custom type.
func resolvePropertyValues(prop metamodel.PropertyDef, meta *metamodel.Metamodel) []string {
	if len(prop.Values) > 0 {
		return prop.Values
	}
	if ct, ok := meta.Types[prop.Type]; ok {
		return ct.Values
	}
	return nil
}

// resolveWidget returns the appropriate widget type for a property.
func resolveWidget(prop metamodel.PropertyDef, meta *metamodel.Metamodel) string {
	_, isCustomType := meta.Types[prop.Type]
	isEnum := prop.Type == metamodel.PropertyTypeEnum || isCustomType
	if prop.List && isEnum {
		return WidgetMultiSelect
	}
	if prop.Display != nil && prop.Display.Link {
		return WidgetURL
	}

	return metamodel.ResolveWidgetFromType(meta, prop.Type)
}

// coalesce returns the first non-empty string.
func coalesce(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// slugify converts a string to a URL-safe slug (lowercase, hyphens, no special chars).
func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prev := byte('-')
	for i := range len(s) {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			b.WriteByte(c)
			prev = c
		} else if prev != '-' {
			b.WriteByte('-')
			prev = '-'
		}
	}
	return strings.Trim(b.String(), "-")
}

// resolvePropertyType returns the metamodel type name for a property on an entity type.
func resolvePropertyType(prop, entityType string, meta *metamodel.Metamodel) string {
	entDef, ok := meta.GetEntityDef(entityType)
	if !ok {
		return ""
	}
	propDef, ok := entDef.Properties[prop]
	if !ok {
		return ""
	}
	return propDef.Type
}

// mdConverter is the goldmark instance with GFM extensions (tables, task lists, etc.).
var mdConverter = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

// simpleMarkdownToHTML converts markdown to HTML using goldmark with GFM extensions.
func simpleMarkdownToHTML(md string) template.HTML {
	if md == "" {
		return ""
	}

	var buf bytes.Buffer
	if err := mdConverter.Convert([]byte(md), &buf); err != nil {
		//nolint:gosec // fallback to escaped input on conversion error
		return template.HTML(template.HTMLEscapeString(md))
	}

	result := buf.String()

	result = strings.ReplaceAll(result, "<table>", `<table class="md-table">`)

	result = htmlutil.ConvertDiagramBlocks(result)

	result = addCheckboxIndices(result)

	//nolint:gosec // HTML is generated by goldmark from user markdown
	return template.HTML(result)
}

var checkboxRe = regexp.MustCompile(`<input[^>]*type="checkbox"[^>]*>`)

func addCheckboxIndices(s string) string {
	idx := 0
	return checkboxRe.ReplaceAllStringFunc(s, func(match string) string {
		result := strings.Replace(match, "<input", fmt.Sprintf(`<input data-cb-idx="%d"`, idx), 1)
		idx++
		return result
	})
}

// visibleListByTypes is executeQuery's no-free-text branch: it loads the
// visible entities of types under the per-type scope verdict, applying props
// in the store. It fails loud on every verdict path.
//
// The props are ANDed onto the verdict query, so they can never widen what the
// gate allows. No type means every type the scope grants, read as content-free
// headers per type. A stored type the metamodel no longer declares is skipped.
func visibleListByTypes(
	ctx context.Context, svc Services, types []string, scope map[string]search.TypeScope,
	props []store.PropPredicate,
) ([]*entity.Entity, error) {
	if len(types) == 0 {
		if _, wildcard := scope[search.WildcardType]; wildcard {
			types = svc.Meta.EntityTypes()
		} else {
			types = make([]string, 0, len(scope))
			for typ := range scope {
				types = append(types, typ)
			}
		}
		types = slices.Sorted(slices.Values(types))
	}

	var out []*entity.Entity
	for i, typ := range types {
		if slices.Contains(types[:i], typ) {
			continue
		}
		ts, ok := search.ResolveTypeScope(scope, typ)
		if !ok {
			continue
		}
		got, err := visibleEntitiesOfType(ctx, svc, typ, ts, props, 0)
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
	}
	return out, nil
}

// visibleEntitiesOfType loads one type's visible entities, applying the
// property predicates in the store.
//
// Three shapes, differing only in how the query is composed:
//
//   - AllowAll with no props: a plain type listing, byte-identical to the
//     pre-pushdown path.
//   - AllowAll with props: a type+props query rather than a full scan. The
//     error class stays errListLoad — no gate is involved here.
//   - Scope-restricted: the gate's own verdict query with the props appended,
//     ANDed by the store, so narrowing by property can never widen what the
//     gate allows. errACLListQuery, because a failure here IS a gate failure.
func visibleEntitiesOfType(
	ctx context.Context, svc Services, typ string,
	ts search.TypeScope, props []store.PropPredicate, offset int,
) ([]*entity.Entity, error) {
	rqr := acl.ReadQueryResult{AllowAll: ts.AllowAll, Query: ts.Query, Faces: ts.Faces}
	req := scopeRequest{Type: typ, Props: props, Faces: ts.Faces}
	q, errClass, withheld, err := scopedQuery(ctx, rqr, req)
	if withheld || err != nil {
		return nil, err
	}
	q.Offset = offset
	headers, err := scanHeaders(ctx, svc, q, errClass, req)
	if err != nil {
		return nil, err
	}
	return headerEntities(headers), nil
}

// visibleEntitiesOfTypeWindow is visibleEntitiesOfType reading one window of
// the type's visible rows in id order: limit rows after offset. limit must be
// positive. The window is cut after the row gate, in the store query.
func visibleEntitiesOfTypeWindow(
	ctx context.Context, svc Services, typ string,
	ts search.TypeScope, props []store.PropPredicate, limit, offset int,
) ([]*entity.Entity, error) {
	rqr := acl.ReadQueryResult{AllowAll: ts.AllowAll, Query: ts.Query, Faces: ts.Faces}
	entities, err := scopedEntities(ctx, svc, rqr, scopeRequest{
		Type: typ, Props: props, Faces: ts.Faces, Limit: limit, Offset: offset,
	})
	return entities, err
}

// freeTextIDsForTypeResult is what freeTextIDsForType returns: the set of
// matching ids when the searcher succeeded, plus a flag distinguishing
// "empty / non-free-text query, skip intersection" from "real result."
type freeTextIDsForTypeResult struct {
	IDs       map[string]struct{}
	HasFilter bool
}

// maxSearchResults is the one bound on a search result. It caps the hits
// the searcher is asked to return, so executeQuery and the list-search path
// stay in lockstep, and it is the page size of a /_search read without a
// limit, free text or structured (TKT-T0DK37).
const maxSearchResults = 1000

// freeTextIDs runs an ACL-scoped search for a parsed SearchQuery under world
// scope w and returns the matching ids. Backend failures surface.
//
// It searches through [visibility.SearchScoped], so a hit that matched only a
// hidden property is dropped; otherwise `?q=<guess>` would confirm a hidden
// value. An uncompilable field scope yields errACLListQuery.
//
// It returns ids, not entities: the caller intersects them with a list it
// already loaded, and a bare-id reload would serve the wrong face.
func freeTextIDs(
	ctx context.Context, vs search.VisibleSearcher, fields visibility.FieldScoper,
	sq *searchparser.SearchQuery, scope map[string]search.TypeScope, w store.WorldScope, limit int,
) (map[string]struct{}, error) {
	parts := make([]string, 0, len(sq.FreeTextWords)+len(sq.FreeTextPhrases))
	parts = append(parts, sq.FreeTextWords...)
	for _, p := range sq.FreeTextPhrases {
		parts = append(parts, `"`+p+`"`)
	}
	q := search.Query{
		Text:  strings.Join(parts, " "),
		Types: sq.EntityTypes,
		Limit: limit,
		World: w,
	}
	ids := make(map[string]struct{})
	for hit, err := range visibility.SearchScoped(ctx, vs, ctxRowGate{}, fields, q, scope, sq.EntityTypes) {
		if err != nil {
			if errors.Is(err, search.ErrScope) {
				return nil, fmt.Errorf("%w: %w", errACLListQuery, err)
			}
			return nil, fmt.Errorf("free-text search: %w", err)
		}
		ids[hit.ID] = struct{}{}
	}
	return ids, nil
}

// relationDirection maps the data-entry config direction type to the
// store's direction enum.
func relationDirection(d dataentryconfig.Direction) store.Direction {
	if d.IsIncoming() {
		return store.DirectionIncoming
	}
	return store.DirectionOutgoing
}

// pushdownPrefilters returns the store-evaluable subset of the property
// filters as a PRE-FILTER only; the caller still runs every filter through the
// Go pass. A pre-filter must never remove a row the Go pass would keep.
//
// So only plain equality on a `string` or untyped property pushes down, and
// only when every named type declares it string-comparable. Not-equal, globs,
// ordered and typed comparisons stay in Go, and a query without a type pushes
// nothing. internal/propmatch keeps emptiness consistent across both paths.
func pushdownPrefilters(
	filters []*filter.Filter, meta *metamodel.Metamodel, types []string,
) []store.PropPredicate {
	return queryplan.PushdownPrefilters(filters, meta, types)
}

// propertyElements returns a property value as the elements a filter operator
// compares against: one element for a scalar, one per item for a list. A list
// has no single string form, so comparing per element is what lets `eq` and
// `ne` work on list properties, as [propertyContains] does for static filters.
//
// Non-string elements render with `%v`, like propmatch's Stringify. A nil
// property yields one empty string, never the matchable literal "<nil>".
func propertyElements(prop any) []string {
	if prop == nil {
		return []string{""}
	}
	switch v := prop.(type) {
	case string:
		return []string{v}
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
				continue
			}
			out = append(out, fmt.Sprintf("%v", item))
		}
		return out
	default:
		return []string{fmt.Sprintf("%v", prop)}
	}
}

// anyElement reports whether any element of the property satisfies match.
// Used by the ANY-element operators (eq, in, contains); `ne` is its negation,
// so "no element matches" stays the exact complement of "some element matches"
// and the two cannot drift apart.
func anyElement(prop any, match func(string) bool) bool {
	return slices.ContainsFunc(propertyElements(prop), match)
}

// filterSetValues expands the params of an `in`/`ne` filter into its value
// set. Without the `[]` suffix each param splits on commas
// (`filter[x][in]=a,b`); with it each param is one verbatim member, so a value
// may contain a comma. The suffix decides, never the param count.
//
// Each member is trimmed and variable-resolved, so both forms agree.
func filterSetValues(values []string, isArrayForm bool, user string, now time.Time) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if isArrayForm {
			out = append(out, resolveFilterVariable(strings.TrimSpace(v), user, now))
			continue
		}
		for part := range strings.SplitSeq(v, ",") {
			out = append(out, resolveFilterVariable(strings.TrimSpace(part), user, now))
		}
	}
	return out
}

// isListValue reports whether a property value is list-typed. Used to decline
// operators that have no defined meaning over a list rather than answering
// with a comparison against the rendered slice.
func isListValue(prop any) bool {
	switch prop.(type) {
	case []string, []any:
		return true
	}
	return false
}
