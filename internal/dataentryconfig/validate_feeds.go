package dataentryconfig

import (
	"fmt"
	"regexp"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// icalDurationRe matches an RFC 5545 duration as used for a VALARM trigger:
// an optional sign, "P", then either a week form ("1W") or a day/time form
// with at least one component ("1D", "T9H", "1DT30M"). A bare "P" or "PT" with
// no components is rejected. Pragmatic subset sufficient for alarm offsets
// (e.g. "-PT9H", "-P1D", "PT15M").
var icalDurationRe = regexp.MustCompile(
	`^[+-]?P(?:\d+W|(?:\d+D)?(?:T(?:\d+H)?(?:\d+M)?(?:\d+S)?)?)$`)

// hasDurationComponent reports whether an RFC 5545 duration string contains at
// least one numeric component (so a bare "P" / "PT" is rejected). Paired with
// icalDurationRe, which validates the structure.
func hasDurationComponent(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

// validateFeeds checks each declarative feed against the metamodel: every source
// must name a known entity type, its date/summary/description properties must
// exist (date must be date-typed), each where clause must parse and reference a
// real property, and any alarm must be a valid RFC 5545 duration. Errors are
// reported per feed + source index so authors can pinpoint the problem, and
// surface at config load rather than at first calendar poll.
func validateFeeds(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string

	for feedID, feed := range cfg.Feeds {
		if len(feed.Sources) == 0 {
			errs = append(errs, fmt.Sprintf("feed %q: must declare at least one source", feedID))
			continue
		}
		for i, src := range feed.Sources {
			errs = append(errs, validateFeedSource(feedID, i, src, meta)...)
		}
	}
	return errs
}

func validateFeedSource(feedID string, i int, src FeedSource, meta *metamodel.Metamodel) []string {
	var errs []string
	prefix := fmt.Sprintf("feed %q: source[%d]", feedID, i)

	entDef, ok := meta.GetEntityDef(src.EntityType)
	if !ok {
		return append(errs, fmt.Sprintf("%s: unknown entity type %q", prefix, src.EntityType))
	}

	// Date: required, must exist, must be date-typed.
	if src.Date == "" {
		errs = append(errs, prefix+": 'date' is required")
	} else if def, ok := entDef.Properties[src.Date]; !ok {
		errs = append(errs, fmt.Sprintf("%s: date property %q not in metamodel for entity %q", prefix, src.Date, src.EntityType))
	} else if def.Type != metamodel.PropertyTypeDate {
		errs = append(errs, fmt.Sprintf("%s: date property %q must be date-typed, is %q", prefix, src.Date, def.Type))
	}

	// Summary: optional, but if omitted the type needs a display property.
	if src.Summary == "" {
		if entDef.GetPrimaryProperty() == "" {
			errs = append(errs, fmt.Sprintf("%s: 'summary' omitted and entity %q has no display property to fall back to", prefix, src.EntityType))
		}
	} else if _, ok := entDef.Properties[src.Summary]; !ok {
		errs = append(errs, fmt.Sprintf("%s: summary property %q not in metamodel for entity %q", prefix, src.Summary, src.EntityType))
	}

	// Description: optional, must exist if set.
	if src.Description != "" {
		if _, ok := entDef.Properties[src.Description]; !ok {
			errs = append(errs, fmt.Sprintf("%s: description property %q not in metamodel for entity %q", prefix, src.Description, src.EntityType))
		}
	}

	// Where: each clause must parse and reference a real property.
	for j, clause := range src.Where {
		f, err := filter.Parse(clause)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: where[%d] %q: %v", prefix, j, clause, err))
			continue
		}
		if f.Property != "id" && f.Property != "type" {
			if _, ok := entDef.Properties[f.Property]; !ok {
				errs = append(errs, fmt.Sprintf("%s: where[%d] references unknown property %q", prefix, j, f.Property))
			}
		}
	}

	// Alarm: optional, must be a valid, non-empty RFC 5545 duration.
	if src.Alarm != "" && (!icalDurationRe.MatchString(src.Alarm) || !hasDurationComponent(src.Alarm)) {
		errs = append(errs, fmt.Sprintf("%s: alarm %q is not a valid RFC 5545 duration (e.g. -PT9H)", prefix, src.Alarm))
	}

	return errs
}
