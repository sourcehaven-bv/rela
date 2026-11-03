package realtimecfg

import (
	"maps"
	"slices"
)

// The clone methods and cloneGate give [Compiled]'s accessors their copies. Each copies
// every map, slice and pointer a caller could write through. Compiled
// predicate programs are shared: a *predicate.Program is never mutated after
// it compiles.

func (ct *ChannelType) clone() ChannelType {
	out := *ct
	out.AttachTo = slices.Clone(ct.AttachTo)
	out.Create = clonePtr(ct.Create, cloneGate)
	out.Roles = maps.Clone(ct.Roles)
	out.MessageTypes = make(map[string]*MessageType, len(ct.MessageTypes))
	for name, mt := range ct.MessageTypes {
		c := mt.clone()
		out.MessageTypes[name] = &c
	}
	out.Lifecycle = Lifecycle{
		Close:   clonePtr(ct.Lifecycle.Close, LifecycleOp.clone),
		Reopen:  clonePtr(ct.Lifecycle.Reopen, LifecycleOp.clone),
		Archive: clonePtr(ct.Lifecycle.Archive, LifecycleOp.clone),
		Delete:  clonePtr(ct.Lifecycle.Delete, LifecycleOp.clone),
	}
	out.MessageOps = MessageOps{
		EditOwn: clonePtr(ct.MessageOps.EditOwn, func(e EditOwn) EditOwn {
			e.Gate = cloneGate(e.Gate)
			return e
		}),
		DeleteOwn: clonePtr(ct.MessageOps.DeleteOwn, func(d DeleteOwn) DeleteOwn {
			d.Gate = cloneGate(d.Gate)
			return d
		}),
		Remove: clonePtr(ct.MessageOps.Remove, func(r Remove) Remove {
			r.Gate = cloneGate(r.Gate)
			return r
		}),
	}
	return out
}

func cloneGate(g Gate) Gate {
	g.Roles = slices.Clone(g.Roles)
	return g
}

func (op LifecycleOp) clone() LifecycleOp {
	op.Gate = cloneGate(op.Gate)
	op.Conditions = maps.Clone(op.Conditions)
	op.Auto = clonePtr(op.Auto, func(a Auto) Auto {
		when := make(map[string]Rule, len(a.When))
		for t, r := range a.When {
			when[t] = r.clone()
		}
		a.When = when
		return a
	})
	return op
}

func (r Rule) clone() Rule {
	r.Fields = slices.Clone(r.Fields)
	return r
}

func (mt *MessageType) clone() MessageType {
	out := *mt
	out.Properties = make(map[string]Property, len(mt.Properties))
	for name, p := range mt.Properties {
		p.Values = slices.Clone(p.Values)
		p.To = slices.Clone(p.To)
		out.Properties[name] = p
	}
	out.Post = slices.Clone(mt.Post)
	out.In = slices.Clone(mt.In)
	out.Choices = clonePtr(mt.Choices, func(c Choices) Choices { return c })
	out.DecisionButton = clonePtr(mt.DecisionButton, func(o ButtonOptions) ButtonOptions { return o })
	out.Actions = make(map[string]*Action, len(mt.Actions))
	for name, a := range mt.Actions {
		c := *a
		c.Gate = cloneGate(a.Gate)
		c.PromoteTo = clonePtr(a.PromoteTo, Promote.clone)
		out.Actions[name] = &c
	}
	return out
}

func (p Promote) clone() Promote {
	p.Map = maps.Clone(p.Map)
	return p
}

func (t *Trigger) clone() Trigger {
	out := *t
	out.When = t.When.clone()
	out.Props = maps.Clone(t.Props)
	return out
}

// clonePtr returns a pointer to a copy of *p made by fn, or nil for nil.
func clonePtr[T any](p *T, fn func(T) T) *T {
	if p == nil {
		return nil
	}
	c := fn(*p)
	return &c
}
