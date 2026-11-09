package flowcfg

import (
	"fmt"
	"maps"
	"slices"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// states compiles `states:` and `initial:` into f.
func (l *loader) states(key string, raw *rawFlow, f *Flow) {
	sk := key + ".states"
	names, nodes, ok := l.mapping(sk, &raw.States)
	if !ok {
		return
	}
	if len(names) == 0 {
		l.add(sk, "is required: a flow has at least one state besides a final one")
		return
	}
	if len(names) > flow.MaxStates {
		l.add(sk, fmt.Sprintf("at most %d states", flow.MaxStates))
		return
	}
	raws := make([]rawState, len(names))
	for i, name := range names {
		k := sk + "." + name
		if !flow.ValidName(name) {
			l.add(k, "a state name is 1 to 64 lower-case letters, digits, '-' or '_', starting with a letter")
			continue
		}
		if err := decodeNode(nodes[i], &raws[i]); err != nil {
			l.add(k, err.Error())
			continue
		}
		f.index[name] = len(f.states)
		f.states = append(f.states, State{Name: name})
	}
	// Every state is declared before any branch names one.
	for i, name := range names {
		j, declared := f.index[name]
		if !declared {
			continue
		}
		f.states[j] = l.state(sk+"."+name, name, &raws[i], f)
	}
	l.initial(key+".initial", raw.Initial, f)
}

func (l *loader) initial(key, name string, f *Flow) {
	switch st, ok := f.State(name); {
	case name == "":
		l.add(key, "is required: the state a run starts in")
	case !ok:
		l.add(key, fmt.Sprintf("names %q, which states does not declare", name))
	case st.Final:
		l.add(key, fmt.Sprintf("names %q, a final state; a run would end before it starts", name))
	default:
		f.initial = name
	}
}

func (l *loader) state(key, name string, raw *rawState, f *Flow) State {
	st := State{Name: name, Label: raw.Label, Final: raw.Final, on: map[Event][]Branch{}}
	if st.Label == "" {
		st.Label = name
	}
	st.waitingFor = l.roles(key+".waiting_for", raw.WaitingFor)
	st.Status = l.status(key+".status", raw.Status, raw.Final, len(st.waitingFor) > 0)
	if raw.Final {
		if len(raw.On) > 0 || raw.After.Kind != 0 || len(raw.WaitingFor) > 0 {
			l.add(key, "a final state has no on:, after: or waiting_for: a run ends there")
		}
	}
	st.outputs = l.outputs(key+".outputs", raw.Outputs, f.subject)
	for _, ev := range slices.Sorted(maps.Keys(raw.On)) {
		n := raw.On[ev]
		if bs := l.branches(key+".on."+ev, Event(ev), &n, f); len(bs) > 0 {
			st.on[Event(ev)] = bs
		}
	}
	st.timers = l.timers(key+".after", &raw.After, f)
	if !raw.Final && len(st.on) == 0 && len(st.timers) == 0 {
		l.add(key, "has no way out: give it an on: or an after:, or make it final")
	}
	return st
}

// status returns the state's status: the one written, or the default.
func (l *loader) status(key, raw string, final, waits bool) flow.Status {
	if raw == "" {
		switch {
		case final:
			return flow.StatusCompleted
		case waits:
			return flow.StatusInputRequired
		default:
			return flow.StatusWorking
		}
	}
	s := flow.Status(raw)
	if final && s.Terminal() || !final && (s == flow.StatusWorking || s == flow.StatusInputRequired) {
		return s
	}
	if final {
		l.add(key, fmt.Sprintf("is %q; a final state is completed, cancelled or failed", raw))
	} else {
		l.add(key, fmt.Sprintf("is %q; a state that is not final is working or input_required", raw))
	}
	return ""
}

// roles checks role names against acl.yaml and returns them.
func (l *loader) roles(key string, names []string) []string {
	for i, r := range names {
		if l.policy == nil {
			l.add(fmt.Sprintf("%s[%d]", key, i), fmt.Sprintf("names role %q, but the project has no acl.yaml", r))
			continue
		}
		if _, ok := l.policy.Roles[r]; !ok {
			l.add(fmt.Sprintf("%s[%d]", key, i), fmt.Sprintf("names role %q, which acl.yaml does not declare", r))
		}
	}
	return slices.Clone(names)
}

// branches compiles one `on.<event>`: one mapping, or a list of them.
func (l *loader) branches(key string, ev Event, n *yaml.Node, f *Flow) []Branch {
	if !slices.Contains(Events, ev) {
		l.add(key, fmt.Sprintf("names event %q; an event is change or done", ev))
		return nil
	}
	var raws []rawBranch
	var err error
	if n.Kind == yaml.SequenceNode {
		err = decodeNode(n, &raws)
	} else {
		var one rawBranch
		err = decodeNode(n, &one)
		raws = []rawBranch{one}
	}
	if err != nil {
		l.add(key, err.Error())
		return nil
	}
	if len(raws) > MaxBranches {
		l.add(key, fmt.Sprintf("at most %d entries", MaxBranches))
		return nil
	}
	out := make([]Branch, 0, len(raws))
	for i, r := range raws {
		bk := key
		if n.Kind == yaml.SequenceNode {
			bk = fmt.Sprintf("%s[%d]", key, i)
		}
		b := Branch{To: r.To, Label: r.Label}
		l.target(bk+".to", r.To, f)
		if ev == EventDone && len(r.Roles) > 0 {
			l.add(bk+".roles", "a done branch fires when the step's outputs are written; nobody causes it")
		}
		b.Roles = l.roles(bk+".roles", r.Roles)
		if r.When != "" {
			b.When = l.guard(bk+".when", f.subject, r.When)
		}
		out = append(out, b)
	}
	for i := range len(out) - 1 {
		if out[i].When == nil && len(out[i].Roles) == 0 {
			l.warn(fmt.Sprintf("%s[%d]", key, i+1), "is never tried: the entry before it has no roles and no when:")
			break
		}
	}
	return out
}

// target checks that a branch or timer moves to a declared state.
func (l *loader) target(key, to string, f *Flow) {
	if to == "" {
		l.add(key, "is required: the state the run moves to")
		return
	}
	if _, ok := f.index[to]; !ok {
		l.add(key, fmt.Sprintf("names %q, which states does not declare", to))
	}
}

// timers compiles `after:`.
func (l *loader) timers(key string, n *yaml.Node, f *Flow) []Timer {
	names, nodes, ok := l.mapping(key, n)
	if !ok {
		return nil
	}
	if len(names) > flow.MaxTimers {
		l.add(key, fmt.Sprintf("at most %d timers per state", flow.MaxTimers))
		return nil
	}
	out := make([]Timer, 0, len(names))
	for i, name := range names {
		tk := key + "." + name
		if !flow.ValidName(name) {
			l.add(tk, "a timer name is 1 to 64 lower-case letters, digits, '-' or '_', starting with a letter")
			continue
		}
		var r rawTimer
		if err := decodeNode(nodes[i], &r); err != nil {
			l.add(tk, err.Error())
			continue
		}
		t := Timer{Timer: flow.Timer{Name: name, To: r.To, Period: l.period(tk+".period", r.Period, true)}, Label: r.Label}
		l.target(tk+".to", r.To, f)
		switch r.Since {
		case "", "state":
			t.Since = flow.SinceState
		case "run":
			t.Since = flow.SinceSession
		default:
			l.add(tk+".since", fmt.Sprintf("is %q; it is state or run", r.Since))
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// outputs compiles a state's `outputs:`.
func (l *loader) outputs(key string, raw []rawOutput, subject string) []Output {
	if len(raw) > MaxOutputs {
		l.add(key, fmt.Sprintf("at most %d outputs per state", MaxOutputs))
		return nil
	}
	var out []Output
	sets := 0
	for i, r := range raw {
		ok := fmt.Sprintf("%s[%d]", key, i)
		switch {
		case r.Set != nil && r.Create != nil:
			l.add(ok, "an output is either set or create, not both")
		case r.Set != nil:
			sets++
			if sets > 1 {
				l.add(ok, "at most one set per state; put every property in one set")
				continue
			}
			o := Output{Kind: OutputSet, Type: subject}
			o.props = l.outputProps(ok+".set.properties", r.Set.Properties, subject, subject)
			if len(o.props) == 0 && len(r.Set.Properties) == 0 {
				l.add(ok+".set.properties", "is required: the subject properties the output sets")
			}
			out = append(out, o)
		case r.Create != nil:
			if o, valid := l.createOutput(ok+".create", r.Create, subject); valid {
				out = append(out, o)
			}
		default:
			l.add(ok, "an output names set or create")
		}
	}
	return out
}

func (l *loader) createOutput(key string, raw *rawCreate, subject string) (Output, bool) {
	def, ok := l.meta.GetEntityDef(raw.Type)
	canon := l.meta.ResolveAlias(raw.Type)
	switch {
	case raw.Type == "":
		l.add(key+".type", "is required: the entity type to create")
		return Output{}, false
	case entity.IsManagedName(raw.Type):
		l.add(key+".type", fmt.Sprintf("names %q, which rela manages itself; an output cannot create it", raw.Type))
		return Output{}, false
	case !ok:
		l.add(key+".type", fmt.Sprintf("names %q, which schema.yaml does not declare", raw.Type))
		return Output{}, false
	case canon != raw.Type:
		l.add(key+".type", fmt.Sprintf("%q is an alias of %q; name the type itself", raw.Type, canon))
		return Output{}, false
	case def.IDType == metamodel.IDTypeManual:
		l.add(key+".type", fmt.Sprintf("names %q, which has id_type: manual; an output needs generated ids", raw.Type))
		return Output{}, false
	case len(metamodel.FaceOrderOf(l.meta, raw.Type)) > 0:
		l.add(key+".type", fmt.Sprintf("names %q, which declares faces; an output cannot create a faced type yet",
			raw.Type))
		return Output{}, false
	}
	o := Output{Kind: OutputCreate, Type: raw.Type}
	o.props = l.outputProps(key+".properties", raw.Properties, raw.Type, subject)
	for _, name := range slices.Sorted(maps.Keys(def.Properties)) {
		pd := def.Properties[name]
		_, given := raw.Properties[name]
		if pd.Required && pd.Default == "" && pd.Computed == "" && pd.Rollup == nil && !given {
			l.add(key+".properties", fmt.Sprintf("leaves %s.%s unset, which is required and has no default",
				raw.Type, name))
		}
	}
	if raw.Relate != nil {
		o.Relate = raw.Relate.Type
		l.relate(key+".relate.type", raw.Relate.Type, raw.Type, subject)
	}
	return o, true
}

// relate checks that relation rel may link a new typ to the subject.
func (l *loader) relate(key, rel, typ, subject string) {
	rd, ok := l.meta.GetRelationDef(rel)
	switch {
	case rel == "":
		l.add(key, "is required: the relation type from the new entity to the subject")
	case entity.IsManagedName(rel):
		l.add(key, fmt.Sprintf("names %q, which rela manages itself", rel))
	case !ok:
		l.add(key, fmt.Sprintf("names %q, which schema.yaml does not declare", rel))
	case !slices.Contains(rd.From, typ) || !slices.Contains(rd.To, subject):
		l.add(key, fmt.Sprintf("names %q, which does not link %s to %s", rel, typ, subject))
	}
}

// outputProps compiles the property templates of an output that writes
// typ. subject is the flow's subject type, which the templates read.
func (l *loader) outputProps(key string, raw map[string]yaml.Node, typ, subject string) map[string]Template {
	def, _ := l.meta.GetEntityDef(typ)
	out := make(map[string]Template, len(raw))
	for _, name := range slices.Sorted(maps.Keys(raw)) {
		pk := key + "." + name
		pd, ok := def.Properties[name]
		switch {
		case !ok:
			l.add(pk, fmt.Sprintf("%s has no property %q", typ, name))
			continue
		case pd.Computed != "" || pd.Rollup != nil:
			l.add(pk, fmt.Sprintf("%s.%s is derived; an output cannot write it", typ, name))
			continue
		}
		n := raw[name]
		var value any
		if err := n.Decode(&value); err != nil {
			l.add(pk, err.Error())
			continue
		}
		tpl, reason := l.compileTemplate(name, value, pd, subject)
		if reason != "" {
			l.add(pk, reason)
			continue
		}
		out[name] = tpl
	}
	return out
}
