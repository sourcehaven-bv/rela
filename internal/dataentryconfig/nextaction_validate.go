package dataentryconfig

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// validateNextActions validates next_action_bands and next_actions.
//
// The load-bearing checks are reference integrity (a source naming a band or
// an action that does not exist) and the offer union (exactly one member set).
// Both are config-authoring mistakes that would otherwise surface as a
// suggestion silently never firing — the worst failure mode for an advisory
// system, because nothing is visibly broken and the operator concludes the
// feature does not work.
func validateNextActions(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string

	bandIDs := make(map[string]bool, len(cfg.NextActionBands))
	for i, b := range cfg.NextActionBands {
		switch {
		case b.ID == "":
			errs = append(errs, fmt.Sprintf(
				"next_action_bands[%d]: id is required", i))
		case bandIDs[b.ID]:
			// Duplicate ids make list-order-is-priority-order ambiguous:
			// which position does a source referencing it get?
			errs = append(errs, fmt.Sprintf(
				"next_action_bands[%d]: duplicate band id %q", i, b.ID))
		default:
			bandIDs[b.ID] = true
		}
		// An unknown prominence is rejected rather than defaulted: silently
		// falling back to `card` would render a band the operator asked to be
		// quiet at full volume, and nothing would say why.
		switch b.Prominence {
		case "", ProminenceBanner, ProminenceCard, ProminenceInline, ProminenceWhisper:
		default:
			errs = append(errs, fmt.Sprintf(
				"next_action_bands[%d]: unknown prominence %q (want %q, %q, %q or %q)",
				i, b.Prominence, ProminenceBanner, ProminenceCard,
				ProminenceInline, ProminenceWhisper))
		}
	}

	if len(cfg.NextActions) > 0 && len(cfg.NextActionBands) == 0 {
		errs = append(errs, "next_actions: no next_action_bands declared "+
			"(bands are the priority vocabulary; declare at least one)")
	}

	for _, id := range sortedNextActionIDs(cfg.NextActions) {
		errs = append(errs, validateNextActionSource(id, cfg.NextActions[id], cfg, meta, bandIDs)...)
	}
	return errs
}

// sortedNextActionIDs returns source ids in a stable order so validation
// messages do not shuffle between runs on the same config.
func sortedNextActionIDs(m map[string]NextActionSource) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func validateNextActionSource(
	id string, s NextActionSource, cfg *Config, meta *metamodel.Metamodel, bandIDs map[string]bool,
) []string {
	var errs []string
	where := fmt.Sprintf("next_actions[%q]", id)

	switch {
	case s.Band == "":
		errs = append(errs, where+": band is required")
	case !bandIDs[s.Band]:
		errs = append(errs, fmt.Sprintf(
			"%s: references unknown band %q%s", where, s.Band, suggestBand(s.Band, cfg)))
	}

	if s.Suggest == "" {
		errs = append(errs, where+": suggest is required (the message shown to the user)")
	}

	errs = append(errs, validateNextActionCandidateSource(where, s, meta)...)

	if s.Pick != "" {
		switch s.Pick {
		case PickStableRandom, PickFirst, PickLeastRecentlyShown:
		default:
			errs = append(errs, fmt.Sprintf(
				"%s: unknown pick %q (want %q, %q or %q)",
				where, s.Pick, PickStableRandom, PickFirst, PickLeastRecentlyShown))
		}
	}

	if s.Cooldown != "" {
		if _, err := ParseNextActionDuration(s.Cooldown); err != nil {
			errs = append(errs, fmt.Sprintf(
				"%s: invalid cooldown %q (want a duration like \"3d\", \"12h\")", where, s.Cooldown))
		}
	}

	for i, o := range s.Actions {
		errs = append(errs, validateNextActionOffer(where, i, o, cfg)...)
	}
	return errs
}

// validateNextActionCandidateSource checks the three mutually exclusive ways
// a source names its candidates: Query (global), Context (entity-anchored),
// or Count (entity-less aggregate). Exactly one must be set.
func validateNextActionCandidateSource(where string, s NextActionSource, meta *metamodel.Metamodel) []string {
	var errs []string

	var set []string
	if s.Query != "" {
		set = append(set, "query")
	}
	if s.Context != "" {
		set = append(set, "context")
	}
	if s.Count != "" {
		set = append(set, "count")
	}

	switch len(set) {
	case 0:
		errs = append(errs,
			where+": needs one of query (global), context (entity-anchored) or count (whole-graph)")
	case 1:
	default:
		errs = append(errs, fmt.Sprintf(
			"%s: %s are mutually exclusive (a source has one kind of candidate)",
			where, strings.Join(set, " and ")))
	}

	if s.Context != "" && meta != nil {
		if _, ok := meta.GetEntityDef(s.Context); !ok {
			errs = append(errs, fmt.Sprintf(
				"%s: context references unknown entity type %q", where, s.Context))
		}
	}

	if s.Count != "" {
		errs = append(errs, validateNextActionCount(where, s.Count, meta)...)
	}
	// count_ungated only means something alongside count. Silently ignoring
	// it elsewhere would let an operator believe they had opted out of the
	// read gate on a source that never consults it.
	if s.CountUngated && s.Count == "" {
		errs = append(errs, where+": count_ungated only applies to a count source")
	}
	return errs
}

// validateNextActionCount checks the only supported aggregate form,
// "<entity_type> == 0" — the first-run case. Deliberately narrow: a general
// aggregate language is not needed to say "the graph is empty", and every
// operator writing one would be inventing syntax the engine must then honor.
func validateNextActionCount(where, count string, meta *metamodel.Metamodel) []string {
	fields := strings.Fields(count)
	if len(fields) != 3 || fields[1] != "==" || fields[2] != "0" {
		return []string{fmt.Sprintf(
			"%s: count must be of the form \"<entity_type> == 0\", got %q", where, count)}
	}
	if meta != nil {
		if _, ok := meta.GetEntityDef(fields[0]); !ok {
			return []string{fmt.Sprintf(
				"%s: count references unknown entity type %q", where, fields[0])}
		}
	}
	return nil
}

func validateNextActionOffer(where string, i int, o NextActionOffer, cfg *Config) []string {
	var errs []string
	at := fmt.Sprintf("%s: actions[%d]", where, i)

	kinds := o.setKinds()
	switch len(kinds) {
	case 0:
		errs = append(errs,
			at+": empty (set one of action, set, navigate, snooze, dismiss, acknowledge)")
		return errs
	case 1:
	default:
		errs = append(errs, fmt.Sprintf(
			"%s: sets %s — an affordance is exactly one of them", at, strings.Join(kinds, " and ")))
	}

	if o.Action != "" {
		if _, ok := cfg.Actions[o.Action]; !ok {
			errs = append(errs, fmt.Sprintf(
				"%s: references unknown action %q", at, o.Action))
		}
	}

	for _, d := range o.Snooze {
		if _, err := ParseNextActionDuration(d); err != nil {
			errs = append(errs, fmt.Sprintf(
				"%s: invalid snooze duration %q (want e.g. \"1d\", \"7d\", \"12h\")", at, d))
		}
	}

	// Confirm only means something for an in-place mutation. Silently
	// ignoring it on a navigate/snooze would teach an operator it works.
	if o.Confirm && o.Action == "" && len(o.Set) == 0 {
		errs = append(errs, at+": confirm only applies to action or set")
	}
	return errs
}

// ParseNextActionDuration parses a cooldown or snooze duration, extending
// time.ParseDuration with a day unit: "3d" is the natural way to write a
// cooldown, and ParseDuration has no "d". Everything else is delegated
// unchanged, so "12h", "90m" and "1h30m" keep working.
//
// Exported because the engine must parse these the same way the validator
// does — a config that validates but then parses differently at runtime is
// exactly the drift a single function prevents.
// hoursPerDay backs the "d" unit ParseNextActionDuration adds.
const hoursPerDay = 24

func ParseNextActionDuration(s string) (time.Duration, error) {
	if num, ok := strings.CutSuffix(s, "d"); ok && num != "" {
		days, err := strconv.ParseFloat(num, 64)
		if err == nil {
			return time.Duration(days * float64(hoursPerDay*time.Hour)), nil
		}
		// Not a plain number before the "d" (e.g. "1h30d"): fall through and
		// let ParseDuration produce its own error for the whole string.
	}
	return time.ParseDuration(s)
}

// suggestBand offers a near-miss band id for an unknown reference, matching
// the "did you mean" style the view/list validators use.
func suggestBand(want string, cfg *Config) string {
	for _, b := range cfg.NextActionBands {
		if strings.EqualFold(b.ID, want) {
			return fmt.Sprintf(" (did you mean %q?)", b.ID)
		}
	}
	return ""
}
