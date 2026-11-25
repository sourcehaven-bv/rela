package dataentry

import (
	"context"
	"net/url"
	"slices"
	"strings"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
)

// Per-row field affordances on a list page (TKT-M0AT73B).
//
// A list row carries no `_fields` by default; only per-entity responses do.
// A list column that writes in place (a toggle) needs to know, per row,
// whether the property is writable and which values its transitions allow.
// `?field_verdicts=a,b` asks for exactly that: each row carries `_fields` and
// `_redacted` restricted to the named properties. The verdicts come from the
// page's primed resolver, so the page costs no query per row.

// rowFieldsParam names the properties whose verdicts each row should carry.
const rowFieldsParam = "field_verdicts"

// maxRowFields bounds the names one request may ask for; a list has few
// toggle columns, and the bound keeps the per-row map small.
const maxRowFields = 16

// wantRowFields returns the property names `?field_verdicts=` asks for, deduplicated
// and in request order, or nil when the request did not ask.
func wantRowFields(query map[string][]string) []string {
	raw := queryGet(query, rowFieldsParam)
	if raw == "" {
		return nil
	}
	var names []string
	for n := range strings.SplitSeq(raw, ",") {
		n = strings.TrimSpace(n)
		if n == "" || slices.Contains(names, n) {
			continue
		}
		names = append(names, n)
		if len(names) == maxRowFields {
			break
		}
	}
	return names
}

// attachRowFields writes the row's `_fields` and `_redacted` for names. ctx
// must carry the page's primed verdicts ([primeVerdicts]).
func attachRowFields(
	ctx context.Context, svc affordanceService, e *entityPkg.Entity, wire *v1.Entity, names []string,
) {
	if len(names) == 0 {
		return
	}
	verdicts := svc.fieldVerdicts(ctx, e)
	all := computeFieldAffordancesFrom(verdicts)
	fields := make(map[string]v1.FieldAffordance, len(names))
	for _, n := range names {
		if fa, ok := all[n]; ok {
			fields[n] = fa
		}
	}
	redacted := []string{}
	for _, n := range redactedPropertyNames(verdicts) {
		if slices.Contains(names, n) {
			redacted = append(redacted, n)
		}
	}
	wire.FieldAffordances = &fields
	wire.Redacted = &redacted
}

// primeRowFields primes the page's field verdicts and, when the request
// names field_verdicts, its flow runs, so every row's _fields comes from
// one batch.
func primeRowFields(
	ctx context.Context, res FieldVerdictResolver, rows []*entityPkg.Entity, query url.Values,
) (pageCtx context.Context, names []string) {
	pageCtx, names = primeVerdicts(ctx, res, rows), wantRowFields(query)
	if len(names) > 0 {
		pageCtx = primeFlowRuns(pageCtx, res, rows)
	}
	return pageCtx, names
}
