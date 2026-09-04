// Package nextaction resolves ONE advisory suggestion from operator-declared
// sources.
//
// # The one-slot constraint does the work
//
// Because exactly one suggestion is ever shown, several things that look like
// omissions are deliberate:
//
//   - No per-source `limit:`. A bounded candidate set is not lossy when only
//     one candidate is displayed — sixty stalled tasks and six produce
//     identical output — so the bound belongs to the engine, which knows the
//     page budget, not to an operator guessing a number whose right value
//     depends on every other source.
//   - No ranking within a band. See [pickStableRandom].
//   - No cache. See the Resolve doc.
//
// # Evaluation order
//
// Sources are grouped by band, bands are evaluated in operator-declared order
// (highest priority first), and evaluation STOPS at the first band that
// yields a candidate. A typical page therefore runs one or two queries rather
// than all of them, and the ambient/content source at the bottom is reached
// only when everything above is empty — which is also when it is cheapest to
// be there.
//
// See RES-09YLLL for the design and the rejected alternatives.
package nextaction

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/userstate"
)

// DefaultCooldown applies to a source that declares none.
//
// Deliberately non-zero: an operator who omits a cooldown has probably not
// thought about nag frequency, so the default assumes they got it wrong
// rather than assuming they meant "show this every single page load". Being
// too quiet is recoverable; being a nag is how the whole surface gets ignored.
const DefaultCooldown = 24 * time.Hour

// DefaultCandidateCap bounds how many candidates a source may contribute to
// the pick. Engine-owned, not configurable — see the package doc.
const DefaultCandidateCap = 20

// Candidate is one entity a source proposes, already ACL-filtered by the
// caller's reader.
type Candidate struct {
	Entity *entity.Entity
}

// PickOption is one choice in a pick_one affordance.
type PickOption struct {
	EntityID string
	Label    string
}

// OptionFunc resolves a pick_one option list. Supplied by the wiring site for
// the same reason as [CandidateFunc]: the query must go through the caller's
// read gate, and the engine must not learn how to reach a store.
//
// It receives the SOURCE it is resolving for, not merely the query string, so
// the wiring site can scope the option query exactly as it scopes that
// source's candidates. Without it an options query would silently run in a
// different world than the candidates it offers actions on — the wiring site
// is the only layer that knows what "a world" is, and it cannot apply one it
// is not told about.
//
// A nil OptionFunc is not an error — the engine then renders the pick_one
// affordance with no options, which the UI omits. That keeps a deployment
// that has not wired it from failing every page.
type OptionFunc func(
	ctx context.Context, src dataentryconfig.NextActionSource, query string, limit int,
) ([]PickOption, error)

// MatcherFunc reports whether a candidate satisfies one source's `condition:`.
// Supplied by the wiring site for the same reason as [CandidateFunc]: the
// expression is compiled against the metamodel by a predicate engine this
// package must not learn about, so the engine stays free of both.
//
// Returns (nil, false) for a source that declares no condition — the caller
// then keeps every candidate. A compiled matcher is expected to be built ONCE
// at config load, so an unparseable expression fails loudly at startup rather
// than silently suppressing a suggestion forever.
//
// An evaluation error is NOT treated as "does not match": it is returned, so a
// broken condition surfaces instead of quietly emptying a source.
type MatcherFunc func(sourceID string) (Matcher, bool)

// Matcher evaluates one source's compiled condition against a candidate.
type Matcher interface {
	Match(ctx context.Context, e *entity.Entity) (bool, error)
}

// CandidateFunc produces candidates for one source. Supplied by the wiring
// site so this package depends on no store, searcher or ACL type: it is the
// consumer-side interface that keeps the engine testable without a graph.
//
// Implementations MUST apply the caller's read gate. The engine never sees
// an entity the principal may not read, which is also why there is no cache
// here — see [Engine.Resolve].
type CandidateFunc func(ctx context.Context, src dataentryconfig.NextActionSource) ([]Candidate, error)

// Suggestion is the resolved hint.
type Suggestion struct {
	// Source is the config id that produced this. Also the mute unit and half
	// the suggestion key.
	Source string
	// Band is the id of the band it came from.
	Band string
	// EntityID is empty for a count-based (entity-less) source.
	EntityID string
	// Message is Suggest with {property} placeholders interpolated.
	Message string
	// Actions are the affordances, copied from config.
	Actions []dataentryconfig.NextActionOffer
	// PickOptions holds the render-time options for a pick_one affordance,
	// keyed by the offer's index in Actions. Empty for every other kind.
	//
	// Resolved here rather than by the UI because the options come from a
	// QUERY, and the query must run through the same ACL-gated path as the
	// suggestion itself — a client-side fetch would be a second read surface
	// to gate.
	PickOptions map[int][]PickOption
	// Key is the identity used for cooldown, snooze and dismissal.
	Key userstate.Key
}

// Engine resolves suggestions. Construct with [New]; safe for concurrent use
// provided the injected collaborators are.
type Engine struct {
	cfg        *dataentryconfig.Config
	state      userstate.Store
	candidates CandidateFunc
	options    OptionFunc
	matchers   MatcherFunc
	cap        int
	// displayWorld is the world the READER is currently browsing, used only
	// to evaluate each source's visible_worlds allow list. Empty means the
	// default world. Supplied per request via [WithDisplayWorld].
	//
	// This is the PRESENTATION axis and nothing else. It never reaches a
	// query — which world a source READS is decided entirely by the
	// CandidateFunc from the source's own config — and it is not a
	// confidentiality control: it decides whether an already-authorized
	// suggestion is worth showing here, not whether its content may be seen.
	displayWorld string
}

// Option configures an [Engine] at construction.
//
// A construction-time option rather than a builder method: a builder returning
// a copy is silently a no-op when the caller discards it, and one mutating the
// receiver would race, since [Engine] is documented as safe for concurrent use
// and callers are invited to cache one. Neither mistake is available here.
type Option func(*Engine)

// WithOptions supplies the pick_one option resolver. Without it a pick_one
// affordance simply offers nothing rather than failing the page.
func WithOptions(fn OptionFunc) Option {
	return func(e *Engine) { e.options = fn }
}

// WithDisplayWorld sets the world the reader is currently browsing, against
// which each source's `visible_worlds` allow list is evaluated. Empty (the
// default) means the default world.
//
// A per-request option because the display world comes from the request while
// every other Engine collaborator comes from wiring. Sources with no
// `visible_worlds` are unaffected in either case.
func WithDisplayWorld(world string) Option {
	return func(e *Engine) { e.displayWorld = world }
}

// WithMatchers supplies the per-source condition matchers. Without it a
// source's `condition:` is not evaluated at all, so the option is required
// whenever any source declares one — [New] enforces exactly that.
func WithMatchers(fn MatcherFunc) Option {
	return func(e *Engine) { e.matchers = fn }
}

// New builds an Engine. Every collaborator is required: a nil userstate.Store
// would silently stop honoring snoozes and mutes, which the user experiences
// as the system ignoring them — precisely the deferred-failure-to-downstream-
// symptom the project's constructor rule exists to prevent.
func New(
	cfg *dataentryconfig.Config, state userstate.Store, candidates CandidateFunc, opts ...Option,
) (*Engine, error) {
	if cfg == nil {
		return nil, errors.New("nextaction: config must be non-nil")
	}
	if state == nil {
		return nil, errors.New("nextaction: userstate store must be non-nil")
	}
	if candidates == nil {
		return nil, errors.New("nextaction: candidate func must be non-nil")
	}
	e := &Engine{cfg: cfg, state: state, candidates: candidates, cap: DefaultCandidateCap}
	for _, opt := range opts {
		opt(e)
	}
	// A source declaring a condition with no matcher wired would evaluate as
	// "keep every candidate" — the suggestion still appears, but for entities
	// the operator explicitly excluded. That is worse than not starting: it
	// looks like the feature works. Unlike a nil OptionFunc, which degrades to
	// an affordance with no options, there is no safe degradation here.
	if e.matchers == nil {
		var withCondition []string
		for id, src := range cfg.NextActions {
			if src.Condition != "" {
				withCondition = append(withCondition, id)
			}
		}
		if len(withCondition) > 0 {
			sort.Strings(withCondition) // deterministic message
			return nil, fmt.Errorf(
				"nextaction: source %q declares a condition but no matchers were supplied "+
					"(pass WithMatchers)", withCondition[0])
		}
	}
	return e, nil
}

// Resolve returns the single suggestion to show `user` now, or ok=false when
// nothing is owed.
//
// `now` is injected rather than read from the clock so callers (and tests)
// control it, matching userstate.Store and predicatefns.Bind.
//
// Deliberately NOT cached. rela gates reads by principal, with row-level
// semantics where a hidden entity is nonexistent, so a cache keyed on
// anything but the principal would defeat that gate — and the failure mode is
// showing someone a suggestion about an entity whose very existence is meant
// to be secret. Per-principal caching would be safe but is not worth it
// against one or two fast queries.
func (e *Engine) Resolve(ctx context.Context, user string, now time.Time) (Suggestion, bool, error) {
	for _, band := range e.cfg.NextActionBands {
		s, ok, err := e.resolveBand(ctx, user, band.ID, now)
		if err != nil {
			return Suggestion{}, false, err
		}
		if ok {
			// Options are resolved for the WINNER only, after the band is
			// decided. Resolving during candidate collection would run an
			// extra query per candidate for a list only one of them will ever
			// display — the same reasoning as short-circuiting itself.
			e.resolvePickOptions(ctx, &s)
			// Short-circuit: a lower band cannot outrank this, so the
			// remaining sources are not worth querying.
			return s, true, nil
		}
	}
	return Suggestion{}, false, nil
}

// resolvePickOptions fills in the render-time option lists.
//
// Failures are logged and dropped rather than propagated: an option list that
// cannot be built costs the user one affordance, whereas failing the whole
// resolve costs them the suggestion — and this is an advisory surface that
// must never break the page it sits on.
func (e *Engine) resolvePickOptions(ctx context.Context, s *Suggestion) {
	if e.options == nil {
		return
	}
	// The source config, so the wiring site can scope the option query the
	// same way it scoped this source's candidates. A suggestion always names
	// a configured source; a missing entry yields the zero source, which the
	// wiring site treats as the default scoping.
	src := e.cfg.NextActions[s.Source]
	for i, offer := range s.Actions {
		if offer.PickOne == nil {
			continue
		}
		opts, err := e.options(ctx, src, offer.PickOne.Query, offer.PickOne.ResolvedLimit())
		if err != nil {
			slog.Warn("nextaction: pick_one options unavailable",
				"source", s.Source, "error", err)
			continue
		}
		if len(opts) == 0 {
			continue
		}
		if s.PickOptions == nil {
			s.PickOptions = make(map[int][]PickOption, 1)
		}
		s.PickOptions[i] = opts
	}
}

// resolveBand collects eligible suggestions from every source in one band and
// picks one. Sources within a band are unordered by design — see pick.
func (e *Engine) resolveBand(
	ctx context.Context, user, bandID string, now time.Time,
) (Suggestion, bool, error) {
	var eligible []Suggestion

	for _, id := range e.sourceIDsForBand(bandID) {
		src := e.cfg.NextActions[id]

		muted, err := e.state.Muted(ctx, user, id)
		if err != nil {
			return Suggestion{}, false, fmt.Errorf("nextaction: mute lookup for %q: %w", id, err)
		}
		if muted {
			continue
		}

		found, err := e.eligibleFromSource(ctx, user, id, src, bandID, now)
		if err != nil {
			return Suggestion{}, false, err
		}
		eligible = append(eligible, found...)
	}

	if len(eligible) == 0 {
		return Suggestion{}, false, nil
	}
	return e.pick(user, eligible, now), true, nil
}

// applyCondition filters candidates by the source's compiled `condition:`.
// A source with no condition, or an engine with no matchers wired, keeps every
// candidate — [New] rejects the combination where that would be silent.
func (e *Engine) applyCondition(
	ctx context.Context, id string, src dataentryconfig.NextActionSource, cands []Candidate,
) ([]Candidate, error) {
	if src.Condition == "" {
		return cands, nil
	}
	// A declared condition with no usable matcher must NOT fall through to
	// "keep everything": that shows the suggestion for entities the operator
	// explicitly excluded, which looks like the feature working. New() rejects
	// the nil-matchers case at construction; this covers a lookup that has no
	// entry for this source, which construction cannot see.
	if e.matchers == nil {
		return nil, fmt.Errorf("nextaction: source %q declares a condition but no matchers were supplied", id)
	}
	m, ok := e.matchers(id)
	if !ok || m == nil {
		return nil, fmt.Errorf("nextaction: source %q declares a condition but no matcher was compiled for it", id)
	}
	out := make([]Candidate, 0, len(cands))
	for _, c := range cands {
		match, err := m.Match(ctx, c.Entity)
		if err != nil {
			return nil, fmt.Errorf("nextaction: condition for %q: %w", id, err)
		}
		if match {
			out = append(out, c)
		}
	}
	return out, nil
}

// eligibleFromSource returns the suggestions one source contributes after
// snooze and cooldown suppression.
func (e *Engine) eligibleFromSource(
	ctx context.Context, user, id string, src dataentryconfig.NextActionSource, bandID string, now time.Time,
) ([]Suggestion, error) {
	cands, err := e.candidates(ctx, src)
	if err != nil {
		return nil, fmt.Errorf("nextaction: candidates for %q: %w", id, err)
	}

	// The condition runs BEFORE the cap, and the order is load-bearing.
	//
	// The cap is justified by "a bounded candidate set is not lossy when only
	// one candidate is displayed" (see the package doc) — true for suppression,
	// which is per-suggestion bookkeeping. A condition is a SELECTION
	// predicate, so truncating first makes the bound lossy: a condition
	// matching only the 21st candidate would silently never fire, and the
	// source would look correctly quiet rather than broken.
	cands, err = e.applyCondition(ctx, id, src, cands)
	if err != nil {
		return nil, err
	}

	if len(cands) > e.cap {
		cands = cands[:e.cap]
	}

	out := make([]Suggestion, 0, len(cands))
	for _, c := range cands {
		sug := buildSuggestion(id, bandID, src, c)
		sug.Key.User = user

		suppressed, err := e.suppressed(ctx, sug.Key, src, now)
		if err != nil {
			return nil, err
		}
		if !suppressed {
			out = append(out, sug)
		}
	}
	return out, nil
}

// suppressed reports whether a snooze or a cooldown hides this suggestion.
func (e *Engine) suppressed(
	ctx context.Context, k userstate.Key, src dataentryconfig.NextActionSource, now time.Time,
) (bool, error) {
	if _, snoozed, err := e.state.SnoozedUntil(ctx, k, now); err != nil {
		return false, fmt.Errorf("nextaction: snooze lookup for %q: %w", k.Source, err)
	} else if snoozed {
		return true, nil
	}

	lastShown, everShown, err := e.state.LastShown(ctx, k)
	if err != nil {
		return false, fmt.Errorf("nextaction: last-shown lookup for %q: %w", k.Source, err)
	}
	if !everShown {
		return false, nil
	}

	cooldown := DefaultCooldown
	if src.Cooldown != "" {
		// Already validated at config load; a parse failure here means the
		// config was constructed in code, so fall back rather than fail the
		// whole page for one malformed source.
		if d, perr := dataentryconfig.ParseNextActionDuration(src.Cooldown); perr == nil {
			cooldown = d
		}
	}
	return now.Before(lastShown.Add(cooldown)), nil
}

// sourceIDsForBand returns the ids in a band, sorted for determinism. Sort
// order is NOT priority — within a band nothing outranks anything, and pick
// chooses among them.
func (e *Engine) sourceIDsForBand(bandID string) []string {
	var ids []string
	for id, src := range e.cfg.NextActions {
		if src.Band != bandID {
			continue
		}
		// The visible_worlds allow list is applied HERE, before the source is
		// queried at all, rather than by filtering the winner afterwards.
		//
		// Filtering after the pick would break the band short-circuit: Resolve
		// stops at the first band that yields anything, so discarding the
		// winner at the end would return "nothing to suggest" while a lower
		// band held a perfectly visible suggestion. Excluding the source up
		// front keeps every band's result honest — and skips the source's
		// query entirely, since a suggestion that cannot be displayed here is
		// not worth the reads.
		if !src.VisibleInWorld(e.displayWorld) {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// pick selects one suggestion from the winning band.
//
// Stable-random, seeded per user per day: a refresh must not re-roll the
// suggestion, or the companion looks flaky, but tomorrow should be free to
// differ.
//
// Dwell-time ordering was considered and rejected. A band holding many
// simultaneous candidates is a CONFIGURATION bug — a rule too broad — and no
// tiebreak fixes that, it only hides it. Random is honest about it, is total
// (dwell is undefined for content and count sources), needs no stored onset,
// and resists starvation: dwell ordering lets the oldest item block its band
// indefinitely if the user is never going to act on it. "Why this one?" still
// answers in one sentence — these were equally eligible, you get one.
func (e *Engine) pick(user string, eligible []Suggestion, now time.Time) Suggestion {
	if len(eligible) == 1 {
		return eligible[0]
	}
	// Sort by key so the input order (map iteration inside a source) cannot
	// shift the choice; the seed alone decides.
	sort.Slice(eligible, func(i, j int) bool {
		return keyString(eligible[i].Key) < keyString(eligible[j].Key)
	})
	return eligible[pickStableRandom(user, now, len(eligible))]
}

// pickStableRandom returns an index in [0,n) that is stable for one user on
// one UTC day. UTC rather than local time so the roll does not shift when a
// user travels, matching the UTC-truncation rule predicatefns.Bind documents
// (a local-vs-UTC midnight skew was a real bug there, RR-YPYTP).
func pickStableRandom(user string, now time.Time, n int) int {
	if n <= 1 {
		return 0
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(user))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(now.UTC().Format(time.DateOnly)))
	// n is a candidate count bounded by DefaultCandidateCap, so the modulus
	// is far inside int range on every supported platform.
	return int(h.Sum64() % uint64(n)) //nolint:gosec // bounded by the candidate cap
}

// keyString renders a key for stable sorting.
func keyString(k userstate.Key) string {
	return k.Source + "\x00" + k.EntityID + "\x00" + k.Variant
}

// buildSuggestion assembles the suggestion for one candidate.
func buildSuggestion(
	id, bandID string, src dataentryconfig.NextActionSource, c Candidate,
) Suggestion {
	s := Suggestion{
		Source:  id,
		Band:    bandID,
		Actions: src.Actions,
		Key:     userstate.Key{Source: id},
	}
	if c.Entity != nil {
		// EntityID always names the candidate — the UI links to it, and the
		// message interpolates from it.
		s.EntityID = c.Entity.ID
		// The KEY, though, follows the source's defer scope. Source-scoped
		// sources leave the entity out, so declining one suggestion defers
		// the whole source rather than handing back the same suggestion with
		// a different entity.
		if src.ResolvedDeferScope() != dataentryconfig.DeferScopeSource {
			s.Key.EntityID = c.Entity.ID
			s.Key.Variant = variantFor(src.KeyProps, c.Entity)
		}
	}
	s.Message = interpolate(src.Suggest, c.Entity)
	return s
}

// variantFor renders the key_props values that make a RESET condition read as
// new. Empty when the source declares none.
func variantFor(props []string, e *entity.Entity) string {
	if len(props) == 0 || e == nil {
		return ""
	}
	parts := make([]string, 0, len(props))
	for _, p := range props {
		parts = append(parts, p+"="+propString(e, p))
	}
	return strings.Join(parts, ";")
}

// interpolate substitutes {property} placeholders from the entity. An unknown
// placeholder is left verbatim: silently emptying it would turn a config typo
// into a message with a hole in it, which reads as a product bug rather than
// as the operator error it is.
func interpolate(tmpl string, e *entity.Entity) string {
	if e == nil || !strings.ContainsRune(tmpl, '{') {
		return tmpl
	}
	var b strings.Builder
	rest := tmpl
	for {
		open := strings.IndexByte(rest, '{')
		if open < 0 {
			b.WriteString(rest)
			return b.String()
		}
		closeIdx := strings.IndexByte(rest[open:], '}')
		if closeIdx < 0 {
			b.WriteString(rest)
			return b.String()
		}
		closeIdx += open
		name := rest[open+1 : closeIdx]
		b.WriteString(rest[:open])
		if v := propString(e, name); v != "" {
			b.WriteString(v)
		} else {
			b.WriteString(rest[open : closeIdx+1])
		}
		rest = rest[closeIdx+1:]
	}
}

// propString renders one property (or the id) as text.
func propString(e *entity.Entity, name string) string {
	if e == nil {
		return ""
	}
	if name == "id" {
		return e.ID
	}
	v, ok := e.Properties[name]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// MarkShown records that a suggestion was surfaced, starting its cooldown.
// Separate from Resolve because resolving is a read: a caller that resolves
// for a preview, or discards the result, must not start the clock.
func (e *Engine) MarkShown(ctx context.Context, s Suggestion, at time.Time) error {
	return e.state.MarkShown(ctx, s.Key, at)
}
