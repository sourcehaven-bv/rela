package dataentryconfig

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Summary is one aggregate over a set of entities, such as the number of
// cards in a kanban column or the sum of their amounts (TKT-SVLQQ6).
//
// It is the shared shape for every surface that summarises a set of
// entities. The kanban column header is the first; list footers, group_by
// headers and dashboard cards are meant to adopt the same type, so authors
// learn one syntax. Validate it with [validateSummaries].
type Summary struct {
	// Function is the aggregate: count, sum, avg, min or max.
	Function SummaryFunction `yaml:"function" json:"function"`
	// Property is the property to aggregate. count takes no property: it
	// counts the entities themselves.
	Property string `yaml:"property,omitempty" json:"property,omitempty"`
	// Label prefixes the value, as in "Total: € 11.500". It defaults to the
	// function's name.
	Label string `yaml:"label,omitempty" json:"label,omitempty"`
}

// SummaryFunction names the aggregate a [Summary] computes.
type SummaryFunction string

// The summary functions. sum and avg need a single-valued integer property;
// min and max also accept a date or datetime property.
const (
	SummaryCount SummaryFunction = "count"
	SummarySum   SummaryFunction = "sum"
	SummaryAvg   SummaryFunction = "avg"
	SummaryMin   SummaryFunction = "min"
	SummaryMax   SummaryFunction = "max"
)

var summaryFunctions = []SummaryFunction{SummaryCount, SummarySum, SummaryAvg, SummaryMin, SummaryMax}

// summaryPropertyTypes lists the property types each function accepts.
// rela has no decimal type, so integer is the only numeric one.
var summaryPropertyTypes = map[SummaryFunction][]string{
	SummarySum: {metamodel.PropertyTypeInteger},
	SummaryAvg: {metamodel.PropertyTypeInteger},
	SummaryMin: {metamodel.PropertyTypeInteger, metamodel.PropertyTypeDate, metamodel.PropertyTypeDatetime},
	SummaryMax: {metamodel.PropertyTypeInteger, metamodel.PropertyTypeDate, metamodel.PropertyTypeDatetime},
}

// validateSummaries checks each summary against the entity type it
// aggregates. at prefixes every message, such as `kanban "deals": column_summary`.
func validateSummaries(at string, summaries []Summary, entityType string, entDef *metamodel.EntityDef) []string {
	var errs []string
	for i, s := range summaries {
		errs = append(errs, validateSummary(fmt.Sprintf("%s[%d]", at, i), s, entityType, entDef)...)
	}
	return errs
}

func validateSummary(at string, s Summary, entityType string, entDef *metamodel.EntityDef) []string {
	if s.Function == SummaryCount {
		// Refused rather than ignored: a count of the entities that set a
		// property is a plausible later meaning, and accepting the key now
		// would make that a silent change.
		if s.Property != "" {
			return []string{at + ": count takes no property; it counts the entities"}
		}
		return nil
	}
	types, ok := summaryPropertyTypes[s.Function]
	if !ok {
		names := make([]string, len(summaryFunctions))
		for i, f := range summaryFunctions {
			names[i] = string(f)
		}
		return []string{fmt.Sprintf("%s: function %q is not one of %s", at, s.Function, strings.Join(names, ", "))}
	}
	if s.Property == "" {
		return []string{fmt.Sprintf("%s: %s needs a property", at, s.Function)}
	}
	def, ok := entDef.Properties[s.Property]
	if !ok {
		return []string{fmt.Sprintf("%s: property %q not in entity %q", at, s.Property, entityType)}
	}
	if !slices.Contains(types, def.Type) {
		return []string{fmt.Sprintf("%s: %s needs a property of type %s; %q is %q",
			at, s.Function, strings.Join(types, ", "), s.Property, def.Type)}
	}
	if def.List {
		// A list property arrives as an array, which cannot be aggregated as
		// one value, so it would silently count as empty.
		return []string{fmt.Sprintf("%s: property %q is a list; use a single-valued property", at, s.Property)}
	}
	return nil
}
