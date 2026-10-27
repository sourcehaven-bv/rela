package realtimecfg

import (
	"maps"
	"slices"
	"strings"
)

// WaitingPair is one entity property that a waiting role reads: some
// input_required state of a channel type in Types waits for a role whose
// holders are named by Property on an entity of EntityType. An inbox asks
// one gated holder query per pair.
type WaitingPair struct {
	// EntityType is an attach_to type of every channel type in Types.
	EntityType string
	// Property is the role's anchor property (`from: entity.<property>`).
	Property string
	// Face is set when the channel types anchor to a face, so the holder
	// is the face, not the family.
	Face bool
	// Types are the channel types that use this pair, sorted.
	Types []string
}

// WaitingIndex is what an inbox needs to know of the configuration: who can
// be waited on, and in which stored session states. It is built once, when
// the file compiles, from config only.
type WaitingIndex struct {
	// Pairs are sorted by EntityType, Property and Face.
	Pairs []WaitingPair
	// Creator lists, sorted, the channel types with a creator waiting role.
	Creator []string
	// States maps a channel type to its stored session states that can
	// wait, sorted. "" stands for a session that never moved, listed when
	// the initial state waits. A type without such a state is absent.
	States map[string][]string
}

// Waiting returns the waiting index. The value is a copy.
func (c *Compiled) Waiting() WaitingIndex {
	if c == nil {
		return WaitingIndex{States: map[string][]string{}}
	}
	w := c.waiting
	out := WaitingIndex{
		Pairs:   make([]WaitingPair, len(w.Pairs)),
		Creator: slices.Clone(w.Creator),
		States:  make(map[string][]string, len(w.States)),
	}
	for i, p := range w.Pairs {
		p.Types = slices.Clone(p.Types)
		out.Pairs[i] = p
	}
	for typ, states := range w.States {
		out.States[typ] = slices.Clone(states)
	}
	return out
}

// waitingPairKey names one holder query of the index.
type waitingPairKey struct {
	entityType, property string
	face                 bool
}

// waitingBuilder collects the index while [buildWaiting] walks the types.
type waitingBuilder struct {
	pairs   map[waitingPairKey]map[string]bool
	creator map[string]bool
	states  map[string][]string
}

// buildWaiting indexes the input_required states of every channel type in
// types. Only bounded roles can wait (see [waitChecker]), so each waiting
// role is a named role or a creator role.
func buildWaiting(types map[string]*ChannelType) WaitingIndex {
	b := waitingBuilder{
		pairs: map[waitingPairKey]map[string]bool{}, creator: map[string]bool{}, states: map[string][]string{},
	}
	for _, name := range slices.Sorted(maps.Keys(types)) {
		ct := types[name]
		if ct.session == nil {
			continue
		}
		for i := range ct.session.states {
			if st := &ct.session.states[i]; st.Status == StatusInputRequired && len(st.waitingFor) > 0 {
				b.addState(name, ct, st)
			}
		}
	}
	return b.index()
}

// addState records st of the channel type name as waiting, with its roles.
func (b *waitingBuilder) addState(name string, ct *ChannelType, st *SessionState) {
	b.states[name] = append(b.states[name], st.Name)
	if st.Name == ct.session.initial {
		b.states[name] = append(b.states[name], "")
	}
	for _, role := range st.waitingFor {
		r, ok := ct.Roles[role]
		if !ok {
			continue
		}
		switch r.Source.Kind { //nolint:exhaustive // only bounded roles can wait; see waitChecker
		case SourceCreator:
			b.creator[name] = true
		case SourceEntityProperty:
			for _, typ := range ct.AttachTo {
				k := waitingPairKey{typ, r.Source.Property, ct.Anchor == AnchorFace}
				if b.pairs[k] == nil {
					b.pairs[k] = map[string]bool{}
				}
				b.pairs[k][name] = true
			}
		}
	}
}

// index returns the collected index in a stable order.
func (b *waitingBuilder) index() WaitingIndex {
	out := WaitingIndex{States: map[string][]string{}}
	for k, used := range b.pairs {
		out.Pairs = append(out.Pairs, WaitingPair{
			EntityType: k.entityType, Property: k.property, Face: k.face, Types: slices.Sorted(maps.Keys(used)),
		})
	}
	slices.SortFunc(out.Pairs, func(a, b WaitingPair) int {
		if c := strings.Compare(a.EntityType, b.EntityType); c != 0 {
			return c
		}
		if c := strings.Compare(a.Property, b.Property); c != 0 {
			return c
		}
		switch {
		case a.Face == b.Face:
			return 0
		case !a.Face:
			return -1
		}
		return 1
	})
	out.Creator = slices.Sorted(maps.Keys(b.creator))
	for typ, list := range b.states {
		slices.Sort(list)
		out.States[typ] = slices.Compact(list)
	}
	return out
}
