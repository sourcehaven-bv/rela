package dataentryconfig

import (
	"fmt"
	"regexp"
	"strings"

	rrule "github.com/teambition/rrule-go"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// isFeedDateType reports whether a property type is usable as a feed
// date/end_date source: `date` (all-day event) or `datetime` (timed event).
func isFeedDateType(t string) bool {
	return t == metamodel.PropertyTypeDate || t == metamodel.PropertyTypeDatetime
}

// rruleIsLiteral reports whether an rrule config value is a literal RFC 5545
// rule (rather than a property reference). The two are disambiguated by syntax:
// a literal contains "=" (RRULE parts are KEY=VALUE), which a bare property
// identifier never does. Syntax-based (not schema-based) so the same config
// line means the same thing regardless of the project's properties.
func rruleIsLiteral(v string) bool { return strings.Contains(v, "=") }

// parseLiteralRRule validates a literal RRULE string via rrule-go. rrule-go's
// StrToRRule wants the bare rule (no "RRULE:" prefix), so any prefix is stripped.
func parseLiteralRRule(v string) error {
	s := strings.TrimSpace(v)
	if u := strings.ToUpper(s); strings.HasPrefix(u, "RRULE:") {
		s = s[len("RRULE:"):]
	}
	_, err := rrule.StrToRRule(s)
	return err
}

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
// exist (date must be date- or datetime-typed; a datetime source yields a timed
// event), each where clause must parse and reference a real property, and any
// alarm must be a valid RFC 5545 duration. Errors are
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

//nolint:gocognit // linear validation dispatcher: one independent config-vs-metamodel check per branch; splitting would scatter the rule set without lowering real complexity.
func validateFeedSource(feedID string, i int, src FeedSource, meta *metamodel.Metamodel) []string {
	var errs []string
	prefix := fmt.Sprintf("feed %q: source[%d]", feedID, i)

	entDef, ok := meta.GetEntityDef(src.EntityType)
	if !ok {
		return append(errs, fmt.Sprintf("%s: unknown entity type %q", prefix, src.EntityType))
	}

	// Date: required, must exist, must be date- or datetime-typed. A datetime
	// source yields a timed event; a date source stays all-day.
	if src.Date == "" {
		errs = append(errs, prefix+": 'date' is required")
	} else if def, ok := entDef.Properties[src.Date]; !ok {
		errs = append(errs, fmt.Sprintf("%s: date property %q not in metamodel for entity %q", prefix, src.Date, src.EntityType))
	} else if !isFeedDateType(def.Type) {
		errs = append(errs, fmt.Sprintf("%s: date property %q must be date- or datetime-typed, is %q", prefix, src.Date, def.Type))
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
		if entity.IsEntityPropertyKey(f.Property) {
			if _, ok := entDef.Properties[f.Property]; !ok {
				errs = append(errs, fmt.Sprintf("%s: where[%d] references unknown property %q", prefix, j, f.Property))
			}
		}
	}

	// EndDate: optional, must exist and be date- or datetime-typed. It must
	// also be the SAME kind as `date` — iCal forbids mixing an all-day
	// DTSTART with a timed DTEND (or vice versa) in one event.
	if src.EndDate != "" {
		if def, ok := entDef.Properties[src.EndDate]; !ok {
			errs = append(errs, fmt.Sprintf("%s: end_date property %q not in metamodel for entity %q", prefix, src.EndDate, src.EntityType))
		} else if !isFeedDateType(def.Type) {
			errs = append(errs, fmt.Sprintf("%s: end_date property %q must be date- or datetime-typed, is %q", prefix, src.EndDate, def.Type))
		} else if dateDef, ok := entDef.Properties[src.Date]; ok && isFeedDateType(dateDef.Type) && dateDef.Type != def.Type {
			errs = append(errs, fmt.Sprintf(
				"%s: date property %q is %q but end_date property %q is %q — a feed event must be all-day or timed, not a mix",
				prefix, src.Date, dateDef.Type, src.EndDate, def.Type))
		}
	}

	// Rrule: optional. A value with "=" is a literal RRULE (validate via
	// rrule-go); a bare identifier is a property reference (validate existence).
	if src.Rrule != "" {
		if rruleIsLiteral(src.Rrule) {
			if err := parseLiteralRRule(src.Rrule); err != nil {
				errs = append(errs, fmt.Sprintf("%s: rrule %q is not a valid RFC 5545 recurrence rule: %v", prefix, src.Rrule, err))
			}
		} else if _, ok := entDef.Properties[src.Rrule]; !ok {
			errs = append(errs, fmt.Sprintf("%s: rrule %q is neither a valid RRULE (needs '=') nor a property of entity %q", prefix, src.Rrule, src.EntityType))
		}
	}

	// Alarm: optional, must be a valid, non-empty RFC 5545 duration.
	if src.Alarm != "" && (!icalDurationRe.MatchString(src.Alarm) || !hasDurationComponent(src.Alarm)) {
		errs = append(errs, fmt.Sprintf("%s: alarm %q is not a valid RFC 5545 duration (e.g. -PT9H)", prefix, src.Alarm))
	}

	return errs
}
