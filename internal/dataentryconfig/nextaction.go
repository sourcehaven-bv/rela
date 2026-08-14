package dataentryconfig

// Next-action config: operator-declared rules that derive ONE suggested
// follow-up from graph state and surface it as an advisory hint.
//
// The framing constrains the whole shape and is worth stating where the
// types live, because several fields only make sense under it:
//
//   - Advisory, not a task queue. A hint, not a demand.
//   - Things a user COULD do, not SHOULD do — which is what separates this
//     from validation output, which has an opinion about correctness.
//   - ONE suggestion at a time. Not a todo list; the aim is a companion that
//     does not overload.
//   - Good, not optimal. Surfacing one of several good next actions is the
//     goal; avoiding a bad one is the bar.
//
// That last pair is why there is no per-source `limit:` and no numeric
// priority. Because only one suggestion is ever shown, a bounded candidate
// set is not lossy — sixty stalled tasks and six produce identical output —
// so the bound belongs to the engine, which knows the page budget, rather
// than to an operator writing a number whose right value depends on every
// other source.
//
// See RES-09YLLL for the full design and the decisions behind it.

// NextActionBand is one priority tier. Bands are declared by the OPERATOR as
// an ordered list; list order IS priority order, highest first.
//
// Bands rather than numbers because per-source numeric priorities do not
// compose: source A returns 90 because it feels important, source B returns 7
// on a different scale, and the comparison is meaningless — arbitrary
// ordering wearing the costume of ranking. For an advisory system
// explicability beats optimality, since an unexplainable suggestion reads as
// broken even when it is right.
//
// Not hardcoded: an ISMS deployment, a docs mirror and a consultancy want
// different vocabularies, and baking one set into the engine would force
// every operator to translate their domain into ours — the same violation
// metamodel.yaml exists to prevent.
type NextActionBand struct {
	// ID is referenced by NextActionSource.Band. Validation rejects a
	// source naming a band that was never declared.
	ID string `yaml:"id" json:"id"`
	// Label is an optional human-readable name for operator-facing UI
	// (a settings screen listing what can be muted). Empty falls back to ID.
	Label string `yaml:"label,omitempty" json:"label,omitempty"`
	// Prominence is how much this band interrupts. Empty means
	// [ProminenceStatusBar]. See [NextActionProminence].
	Prominence NextActionProminence `yaml:"prominence,omitempty" json:"prominence,omitempty"`
}

// NextActionProminence is how much a band's suggestion interrupts.
//
// A small CLOSED vocabulary rather than styling knobs, for the same reason
// bands beat numeric priorities: the operator declares how loud a tier should
// be, and the UI decides what that looks like. Exposing colors, borders and
// placement instead would let every deployment invent its own visual language
// and drift from the host UI — and would put the "is this urgent?" judgement
// in a stylesheet rather than in the band list where it is reviewable.
//
// The levels differ in WHAT THE USER MUST DO TO CLEAR IT, not in decoration —
// which is why there is no "card" tier: a bounded box and a banner both sit in
// your way and are cleared by the same shrug, so shipping both would be two
// spellings of one interruption model.
//
//	banner    — you must deal with it: act, snooze or mute. Sits above the
//	            page and does not scroll away. For onboarding ("nothing here
//	            yet") and for genuinely urgent things where someone else is
//	            blocked. Insistent, but never blocks the user's actual work.
//	notice    — the same place, much quieter: no accent, muted text, easy to
//	            read past. For things worth saying once a visit that carry no
//	            urgency.
//	statusbar — you must go looking. A chip in the status bar, expanding on
//	            click. For ongoing minor stuff that is true most of the time
//	            and urgent none of it.
//
// Deliberately no "modal" or "toast". A modal takes the interaction away from
// the user, and an advisory hint that blocks progress until dismissed is no
// longer advisory. A toast vanishes on a timer, so the impression that starts
// a cooldown would fire for something the user may never have read.
type NextActionProminence string

const (
	// ProminenceBanner is the insistent tier: above the page, accented, and
	// answered rather than ignored.
	ProminenceBanner NextActionProminence = "banner"
	// ProminenceNotice is banner's quiet sibling — same position, no accent,
	// muted. Easy to read past on purpose.
	ProminenceNotice NextActionProminence = "notice"
	// ProminenceStatusBar is a chip in the status bar that expands on click.
	// The DEFAULT for an unset band.
	ProminenceStatusBar NextActionProminence = "statusbar"
)

// Resolved returns the effective prominence, applying the default.
//
// Defaults to statusbar — the quietest tier — because an operator who has not
// thought about prominence has not earned the top of the page. An unnoticed
// suggestion is recoverable (turn it up); a nagging one teaches users to
// ignore the whole surface, which is not.
func (b NextActionBand) Resolved() NextActionProminence {
	if b.Prominence == "" {
		return ProminenceStatusBar
	}
	return b.Prominence
}

// NextActionSource is one rule producing at most one suggestion per entity.
//
// Sources are pluggable and INDEPENDENT — none knows the others exist. That
// independence is the unit of iteration: adding a rule is adding a source,
// and a bad source can be deleted without perturbing the rest. It is also
// why a union-all query across sources was rejected: it would couple them at
// the SQL layer, so one malformed source breaks the statement serving all.
//
// Two kinds, discriminated by the presence of Context — mirroring how
// DocumentConfig discriminates standalone from entity-anchored on EntityType:
//
//   - Global (Context empty): candidates come from Query; renders on the
//     dashboard; answers "what is the one thing?"
//   - Context-aware (Context set): the candidate IS the entity being viewed;
//     renders on that entity's detail page; answers "what next for this?"
//
// Empty Context is a first-class kind, not a missing required field.
type NextActionSource struct {
	// Band names a NextActionBand.ID. Required.
	Band string `yaml:"band" json:"band"`

	// Context, when set, makes this a context-aware source scoped to that
	// entity type. Empty means global. Mutually exclusive with Query.
	Context string `yaml:"context,omitempty" json:"context,omitempty"`

	// Query selects candidates for a global source, in the existing filter
	// syntax (e.g. "type:task prop:status=doing"). Required for a global
	// source unless Count is set; rejected on a context-aware one, whose
	// candidate is the viewed entity.
	Query string `yaml:"query,omitempty" json:"query,omitempty"`

	// Count makes this an entity-LESS source: it fires on a whole-graph
	// aggregate rather than about any particular entity. The only supported
	// form today is "<entity_type> == 0" — the first-run case ("nothing here
	// yet, start with a client?"), which no entity-shaped source can express
	// because an empty graph has no entities to suggest about.
	//
	// The count is evaluated through the caller's READ GATE by default, so
	// it counts what this principal can see. See CountUngated for the
	// opt-out and why it is not the default.
	//
	// A suggestion from a Count source has no entity id, so its key
	// degenerates to the source id alone.
	Count string `yaml:"count,omitempty" json:"count,omitempty"`

	// CountUngated evaluates Count against the WHOLE graph rather than the
	// caller's visible subset. Only meaningful with Count.
	//
	// Default (false) is gated, and that is deliberate. A gated count asks
	// "do I have any clients?"; an ungated one asks "does anyone?". The
	// ungated form leaks a real fact — that entities of this type exist —
	// to a principal who may read none of them, and rela's read model treats
	// entity existence as the strongest secret it keeps.
	//
	// The failure mode of the safe default is mild and self-correcting: a
	// principal who can see no clients keeps being offered "add a client",
	// which an operator notices and fixes in config. The failure mode of the
	// unsafe default is silent and permanent — a disclosure nobody observes.
	// Prefer the loud, fixable error.
	//
	// Set this only when the count is genuinely operator-level ("has this
	// deployment been set up at all?") rather than about the caller's data.
	CountUngated bool `yaml:"count_ungated,omitempty" json:"count_ungated,omitempty"`

	// Pick chooses among multiple candidates. Empty means the engine's
	// default (stable-random). See NextActionPick.
	Pick NextActionPick `yaml:"pick,omitempty" json:"pick,omitempty"`

	// Suggest is the message template. Supports {property} interpolation
	// against the candidate entity, e.g. "{title} has been in progress since
	// {started_on}. Still on it?". Required.
	Suggest string `yaml:"suggest" json:"suggest"`

	// Actions are the affordances offered alongside the suggestion, ordered
	// by what they cost the user.
	//
	// dismiss/snooze are not decoration: without a way to say "not now", the
	// only way to clear a suggestion is to comply, which makes it a demand
	// rather than a hint. They are also the best signal available — a source
	// dismissed every time is a source to delete.
	Actions []NextActionOffer `yaml:"actions,omitempty" json:"actions,omitempty"`

	// Cooldown suppresses a suggestion for this long after it was last
	// SHOWN, as a duration string ("3d", "12h"). Empty takes the engine
	// default, which is deliberately conservative: an operator who omits it
	// probably has not thought about nag frequency, so the default assumes
	// they got it wrong rather than assuming zero.
	Cooldown string `yaml:"cooldown,omitempty" json:"cooldown,omitempty"`

	// KeyProps optionally extends the suggestion key with the values of
	// these properties, so a condition that RESETS is recognized as new.
	//
	// The key is (source_id, entity_id, optional key-prop values). Without
	// the property component a proposal going draft -> sent -> draft keeps
	// one key, so an old snooze suppresses a genuinely new stall. With
	// KeyProps: [status] the key changes and the snooze no longer matches.
	KeyProps []string `yaml:"key_props,omitempty" json:"key_props,omitempty"`
}

// NextActionPick selects among multiple candidates in the winning band.
type NextActionPick string

const (
	// PickStableRandom picks pseudo-randomly, seeded per user per day so a
	// refresh cannot re-roll it. The DEFAULT, and deliberately not a ranking.
	//
	// Dwell-time ordering ("oldest stuck thing first") was considered and
	// rejected. A band holding many simultaneous candidates is a
	// CONFIGURATION bug — a rule too broad — and no tiebreak fixes that, it
	// only hides it; random is honest about it. Random is also total (dwell
	// is undefined for content and for count-based sources), needs no stored
	// onset, and resists starvation: dwell ordering lets the oldest item
	// block its band indefinitely if the user is never going to act on it.
	PickStableRandom NextActionPick = "stable-random"

	// PickFirst takes the first candidate in query order. Useful when the
	// query itself already encodes the intended ordering.
	PickFirst NextActionPick = "first"

	// PickLeastRecentlyShown rotates through candidates. Costs a per-render
	// write to user state, so it is opt-in rather than the default.
	PickLeastRecentlyShown NextActionPick = "least-recently-shown"
)

// NextActionOffer is one affordance on a suggestion: a discriminated union
// keyed by exactly which field is set. Validation rejects zero or multiple.
//
// Multi-step wizards are deliberately NOT here. A big form is a destination a
// button points at, not part of this vocabulary — Navigate reaches
// Form.Steps. Keeping the set small is what lets this whole layer need
// nothing from interactive flows over HTTP.
type NextActionOffer struct {
	// Label overrides the default button text.
	Label string `yaml:"label,omitempty" json:"label,omitempty"`

	// Action names an entry in the top-level Actions map — act in place.
	Action string `yaml:"action,omitempty" json:"action,omitempty"`

	// Set applies property mutations inline — the shorthand for a one-off
	// that does not warrant a named action.
	Set map[string]string `yaml:"set,omitempty" json:"set,omitempty"`

	// Confirm requires a confirmation step. Only meaningful with Action or
	// Set.
	Confirm bool `yaml:"confirm,omitempty" json:"confirm,omitempty"`

	// Navigate is a URL to hand off to, with {id} interpolated.
	Navigate string `yaml:"navigate,omitempty" json:"navigate,omitempty"`

	// Snooze offers "not now" for these durations (e.g. ["1d", "7d"]).
	Snooze []string `yaml:"snooze,omitempty" json:"snooze,omitempty"`

	// Dismiss offers "not this one" — suppressed until the suggestion key
	// changes.
	Dismiss bool `yaml:"dismiss,omitempty" json:"dismiss,omitempty"`

	// Acknowledge offers "seen it" — the content-source affordance, which
	// records the view without implying the user did anything.
	Acknowledge bool `yaml:"acknowledge,omitempty" json:"acknowledge,omitempty"`

	// PickOne offers a SHORT LIST of entities to act on, resolved by a query
	// at render time.
	//
	// The one affordance that cannot be a static button list: its options are
	// whatever the graph holds right now ("you have 40 minutes — here are
	// three small tasks"). Everything else in this union is fixed when the
	// operator writes the config.
	PickOne *NextActionPickOne `yaml:"pick_one,omitempty" json:"pick_one,omitempty"`
}

// NextActionPickOne resolves a small set of entities to choose between.
//
// Bounded on purpose: this is a suggestion's affordance row, not a list view.
// [DefaultPickOneLimit] applies when Limit is unset, and the engine caps it —
// an operator asking for fifty options has misunderstood the surface, and
// rendering fifty buttons would make the one-suggestion promise a lie.
type NextActionPickOne struct {
	// Query selects the options, in the same search syntax as a source's
	// Query (e.g. "type:task prop:effort=xs prop:status=todo").
	Query string `yaml:"query" json:"query"`

	// Limit caps how many options are offered. Zero means
	// [DefaultPickOneLimit].
	Limit int `yaml:"limit,omitempty" json:"limit,omitempty"`

	// Action names the entry in the top-level `actions:` map to run against
	// whichever option the user picks. Required: an option list with nothing
	// to do on selection is a list, not an affordance.
	Action string `yaml:"action" json:"action"`
}

// DefaultPickOneLimit bounds a pick_one option list when the operator sets no
// Limit. Three is a glance, not a menu.
const DefaultPickOneLimit = 3

// MaxPickOneLimit is the hard ceiling the engine enforces regardless of
// configuration, for the same reason the candidate cap is engine-owned: the
// operator cannot know the page budget, and a suggestion offering a dozen
// choices has stopped being one suggestion.
const MaxPickOneLimit = 5

// ResolvedLimit returns the effective option count.
func (p NextActionPickOne) ResolvedLimit() int {
	switch {
	case p.Limit <= 0:
		return DefaultPickOneLimit
	case p.Limit > MaxPickOneLimit:
		return MaxPickOneLimit
	default:
		return p.Limit
	}
}

// Kind returns which member of the union is set, or "" when none is.
// Used by validation and by the wire layer to avoid re-deriving it.
func (o NextActionOffer) Kind() string {
	switch {
	case o.Action != "":
		return "action"
	case len(o.Set) > 0:
		return "set"
	case o.Navigate != "":
		return "navigate"
	case len(o.Snooze) > 0:
		return "snooze"
	case o.Dismiss:
		return "dismiss"
	case o.Acknowledge:
		return "acknowledge"
	case o.PickOne != nil:
		return "pick_one"
	default:
		return ""
	}
}

// setKinds returns every union member set on this offer, so validation can
// report an ambiguous entry precisely rather than just "invalid".
func (o NextActionOffer) setKinds() []string {
	var kinds []string
	if o.Action != "" {
		kinds = append(kinds, "action")
	}
	if len(o.Set) > 0 {
		kinds = append(kinds, "set")
	}
	if o.Navigate != "" {
		kinds = append(kinds, "navigate")
	}
	if len(o.Snooze) > 0 {
		kinds = append(kinds, "snooze")
	}
	if o.Dismiss {
		kinds = append(kinds, "dismiss")
	}
	if o.Acknowledge {
		kinds = append(kinds, "acknowledge")
	}
	if o.PickOne != nil {
		kinds = append(kinds, "pick_one")
	}
	return kinds
}
