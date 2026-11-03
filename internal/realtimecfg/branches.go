package realtimecfg

import (
	"fmt"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// posterClass is whom a load check asks about when it judges which entry
// of an `on.<type>` list a poster reaches: the holder of a role, or a
// trigger.
type posterClass struct {
	role    string
	trigger bool
}

// triggerPoster is the [posterClass] of a trigger's post.
var triggerPoster = posterClass{trigger: true}

// Admits is the poster rule of t, the one rule the decider and the load
// checks share. holds tells which roles a person or agent holds; it is nil
// for a trigger's post, since a trigger holds no role. With roles, a
// poster holding one passes and a trigger never does. With by: trigger,
// only a trigger passes. With neither, every poster passes: they were
// allowed to post the message type.
func (t Transition) Admits(holds func(role string) bool) bool {
	switch {
	case len(t.Roles) > 0:
		return holds != nil && slices.ContainsFunc(t.Roles, holds)
	case t.By == TransitionByTrigger:
		return holds == nil
	default:
		return true
	}
}

// entryAdmits reports whether t's poster rule admits who; see
// [Transition.Admits].
func entryAdmits(t Transition, who posterClass) bool {
	if who.trigger {
		return t.Admits(nil)
	}
	return t.Admits(func(role string) bool { return role == who.role })
}

// shadowed reports whether an entry of prefix without `when:` admits who.
// The decider takes the first entry that applies, so such an entry always
// wins for who over every later entry. A guard is not evaluated; an entry
// with a `when:` may not hold, so it shadows nothing.
func shadowed(prefix []Transition, who posterClass) bool {
	return slices.ContainsFunc(prefix, func(e Transition) bool { return !e.guarded && entryAdmits(e, who) })
}

// Offers reports whether st offers entry t to a poster who holds the
// roles holds reports: t admits them, no earlier entry of its list without
// `when:` admits them, and, for an entry with a button, no earlier entry
// the load recorded as taking the button's preset first, or as undecidable
// at load, admits them (see [TransitionButton]). The load's reachability
// check of a button and the per-caller list of entries both ask it, so
// the two cannot drift apart.
//
// Foreign t: an entry not taken from [SessionState.Transitions] or
// [Session.Branches] for this state is never offered.
func (st SessionState) Offers(t Transition, holds func(role string) bool) bool {
	list := st.on[t.MessageType]
	if t.index >= len(list) || list[t.index].key != t.key {
		return false
	}
	return offers(list, t, holds)
}

// offers is [SessionState.Offers] over list, the entries of t's message
// type.
func offers(list []Transition, t Transition, holds func(role string) bool) bool {
	if !t.Admits(holds) {
		return false
	}
	prefix := list[:min(t.index, len(list))]
	if slices.ContainsFunc(prefix, func(e Transition) bool { return !e.guarded && e.Admits(holds) }) {
		return false
	}
	if t.button == nil {
		return true
	}
	for _, i := range t.button.hiddenBy() {
		if i < len(prefix) && prefix[i].Admits(holds) {
			return false
		}
	}
	return true
}

// firesFor reports whether entry t of st can fire for who: t admits who,
// and no earlier entry of its list shadows it for who (see [shadowed]).
func (st SessionState) firesFor(t Transition, who posterClass) bool {
	list := st.on[t.MessageType]
	return entryAdmits(t, who) && !shadowed(list[:min(t.index, len(list))], who)
}

// branchCompiler compiles one `on.<message type>` value, a single entry or
// a list of them, and runs the checks that compare the entries of a list.
// It keeps these checks off the validator, which is at its method budget.
// state names the state that owns the value.
type branchCompiler struct {
	*validator
	state  string
	limits Limits
}

// transitionTypeProblem is why msgType cannot key a transition of ct, or
// "" when it can. It is a property of the message type, so the load
// reports it once at `on.<msgType>`, not per entry.
func transitionTypeProblem(ct *ChannelType, msgType string) string {
	mt, ok := ct.MessageTypes[msgType]
	switch {
	case !ok:
		return fmt.Sprintf("unknown message type %q; a transition is keyed by a message type of "+
			"this channel type", msgType)
	case mt.Persist == PersistEphemeral:
		return "an ephemeral message is never stored, so it cannot move a session"
	}
	return ""
}

// compile compiles the value of `on.<msgType>` at key. It returns the
// usable entries in file order, or nil when the message type is unusable.
// An unusable message type is reported once at key; the entries are still
// compiled, so their own problems show in the same load.
func (b branchCompiler) compile(key, msgType string, raw rawBranches, s *Session, ct *ChannelType) []Transition {
	p := transitionTypeProblem(ct, msgType)
	if p != "" {
		b.add(key, p)
	}
	if !raw.list {
		// A null value decodes to no entries; it compiles as one empty
		// entry, so the problem is the missing to:, as for an empty mapping.
		var e rawTransition
		if len(raw.entries) > 0 {
			e = raw.entries[0]
		}
		if t, ok := b.transition(key, msgType, &e, s, ct, false); ok {
			out := []Transition{t}
			b.buttons([]rawTransition{e}, out, nil, s, ct.MessageTypes[msgType])
			return out
		}
		return nil
	}
	switch {
	case len(raw.entries) == 0:
		b.add(key, "name at least one entry, or drop on."+msgType)
		return nil
	case len(raw.entries) > realtimelimits.MaxTransitionBranches:
		b.add(key, fmt.Sprintf("at most %d entries per message type", realtimelimits.MaxTransitionBranches))
		return nil
	}
	out := make([]Transition, 0, len(raw.entries))
	for i := range raw.entries {
		t, _ := b.transition(fmt.Sprintf("%s[%d]", key, i), msgType, &raw.entries[i], s, ct,
			shadowed(out, triggerPoster))
		t.index = i
		out = append(out, t)
	}
	if p != "" {
		return nil
	}
	b.duplicates(key, raw.entries, out)
	refused := b.unreachable(key, out, ct.MessageTypes[msgType])
	b.buttons(raw.entries, out, refused, s, ct.MessageTypes[msgType])
	if !slices.ContainsFunc(out, func(t Transition) bool { return !t.guarded }) {
		b.warn(key, "every entry has a when; a post that matches none is stored and does not move the "+
			"session. End the list with an entry without when if every post should move it.")
	}
	return out
}

// buttons compiles the `button:` of each entry of list, whose raw entries
// are raw. It skips an entry that [branchCompiler.unreachable] refused, as
// refused records, so that entry gets no second problem.
func (b branchCompiler) buttons(raw []rawTransition, list []Transition, refused []bool, s *Session, mt *MessageType) {
	sc := buttonScope{mt: mt, s: s, state: b.state, limits: b.limits}
	for j := range list {
		if raw[j].Button == nil || (j < len(refused) && refused[j]) {
			continue
		}
		list[j].button = buttonChecker{b.validator}.compile(raw[j].Button, list, j, sc)
	}
}

// duplicates refuses a later entry with the same target as an earlier one
// when either has outputs: an outputs mark and the decider's gate find the
// entry that fired by (state, message type, target), so that must name one
// entry. Entries without outputs may share a target, such as one a lead
// fires and one anyone fires when a guard holds.
func (b branchCompiler) duplicates(key string, raw []rawTransition, list []Transition) {
	for j := range list {
		for i := range j {
			if list[i].To == "" || list[i].To != list[j].To ||
				(len(raw[i].Outputs) == 0 && len(raw[j].Outputs) == 0) {

				continue
			}
			b.add(fmt.Sprintf("%s[%d].to", key, j), fmt.Sprintf("entry %d already leads to %s and one of the two "+
				"has outputs; a move with outputs must name one entry, so give them different targets or move "+
				"the outputs to one entry", i, list[j].To))
			break
		}
	}
}

// unreachable refuses an entry that no poster can reach: every poster it
// admits (each of its roles, a trigger, or for an entry with no rule every
// role that may post mt and a trigger) is admitted by an earlier entry
// without `when:`, which always wins. It reads whether an entry has a
// `when:`, not whether the guard compiled, so a broken guard adds no
// second problem here. It returns, per entry, whether it refused it.
func (b branchCompiler) unreachable(key string, list []Transition, mt *MessageType) []bool {
	refused := make([]bool, len(list))
	for j := 1; j < len(list); j++ {
		var classes []posterClass
		switch t := list[j]; {
		case t.By == TransitionByTrigger:
			classes = []posterClass{triggerPoster}
		case len(t.Roles) > 0:
			for _, r := range t.Roles {
				classes = append(classes, posterClass{role: r})
			}
		default:
			classes = append(classes, triggerPoster)
			if mt != nil {
				for _, r := range mt.Post {
					classes = append(classes, posterClass{role: r})
				}
			}
		}
		covered := !slices.ContainsFunc(classes, func(who posterClass) bool { return !shadowed(list[:j], who) })
		if covered {
			b.add(fmt.Sprintf("%s[%d]", key, j), "earlier entries without when admit every poster this entry "+
				"admits, so it can never fire; move it up or give the earlier entry a when")
			refused[j] = true
		}
	}
	return refused
}
