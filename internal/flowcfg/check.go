package flowcfg

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// flowCheck checks a parsed flow as a whole: reachability, cycles, forks,
// waiting and grants. It reports problems to its loader.
type flowCheck struct {
	l *loader
}

// reachable refuses a state no run can reach from the initial state.
func (c flowCheck) reachable(key string, f *Flow) {
	seen := map[string]bool{f.initial: true}
	queue := []string{f.initial}
	for len(queue) > 0 {
		st, _ := f.State(queue[0])
		queue = queue[1:]
		for _, to := range successors(st) {
			if !seen[to] {
				seen[to] = true
				queue = append(queue, to)
			}
		}
	}
	for _, st := range f.states {
		if !seen[st.Name] {
			c.l.add(key+".states."+st.Name, fmt.Sprintf("no run reaches it from the initial state %q", f.initial))
		}
	}
}

// doneCycles refuses a cycle made only of done branches. A done branch
// fires without anyone acting, so such a cycle would move a run, and write
// its outputs, forever.
func (c flowCheck) doneCycles(key string, f *Flow) {
	const (
		unvisited = iota
		active
		finished
	)
	mark := map[string]int{}
	var visit func(name string, path []string) bool
	visit = func(name string, path []string) bool {
		switch mark[name] {
		case active:
			start := slices.Index(path, name)
			cycle := append(slices.Clone(path[start:]), name)
			c.l.add(key+".states."+name+".on.done", "is part of a cycle of done branches, which no one has to act on: "+
				strings.Join(cycle, " -> "))
			return true
		case finished:
			return false
		}
		mark[name] = active
		st, _ := f.State(name)
		for _, b := range st.on[EventDone] {
			for _, to := range b.Targets() {
				if visit(to, append(path, name)) {
					return true
				}
			}
		}
		mark[name] = finished
		return false
	}
	for _, st := range f.states {
		if mark[st.Name] == unvisited && visit(st.Name, nil) {
			return
		}
	}
}

// waiting refuses a waiting role that cannot answer: a state waits for a
// role only if a change branch out of it admits that role. A branch
// without roles admits every role.
func (c flowCheck) waiting(key string, f *Flow) {
	for _, st := range f.states {
		for i, role := range st.waitingFor {
			answers := slices.ContainsFunc(st.on[EventChange], func(b Branch) bool {
				return len(b.Roles) == 0 || slices.Contains(b.Roles, role)
			})
			if !answers {
				c.l.add(fmt.Sprintf("%s.states.%s.waiting_for[%d]", key, st.Name, i),
					fmt.Sprintf("the state waits for %s, but no on.change entry admits that role", role))
			}
		}
	}
}

// noGrant ends every grant problem.
const noGrant = "no role assigned to it or to everyone grants it"

// grants checks, from acl.yaml alone, that the flow's run_as may write
// every output (see [acl.JobIdentityGrants]). Without a policy there is
// nothing to check: every write is allowed. The check cannot see `when:`
// conditions, local roles or a templated option value, so it is an early
// check, not the gate: the runtime writes every output through
// entitymanager as the run_as principal, which checks each write again.
func (c flowCheck) grants(key string, f *Flow) {
	if c.l.policy == nil || f.runAs == "" {
		return
	}
	g := acl.JobIdentityGrants(c.l.policy, f.runAs)
	for _, st := range f.states {
		for i, o := range st.outputs {
			c.l.grantOutput(fmt.Sprintf("%s.states.%s.outputs[%d]", key, st.Name, i), g, f.runAs, o)
		}
	}
}

// grantOutput checks one output against the run_as grants g.
func (l *loader) grantOutput(key string, g acl.JobGrants, runAs string, o Output) {
	op := acl.OpUpdate
	if o.Kind == OutputCreate {
		op = acl.OpCreate
		key += ".create"
	} else {
		key += ".set"
	}
	if !g.Verb(op, o.Type, "") {
		l.add(key, fmt.Sprintf("%s cannot %s %s: %s", runAs, op, o.Type, noGrant))
		return
	}
	for _, name := range o.PropertyNames() {
		pk := key + ".properties." + name
		if !g.FieldWrite(op, o.Type, "", name) {
			l.add(pk, fmt.Sprintf("%s cannot write %s.%s: %s", runAs, o.Type, name, noGrant))
			continue
		}
		if tpl := o.props[name]; tpl.Literal() {
			if s, isString := tpl.literal.(string); isString && !g.Option(op, o.Type, "", name, s) {
				l.add(pk, fmt.Sprintf("%s cannot set %s.%s to %q: %s", runAs, o.Type, name, s, noGrant))
			}
		}
	}
	if o.Relate != "" && !g.Relates(o.Relate, o.Type, "", nil) {
		l.add(key+".relate", fmt.Sprintf("%s cannot create a %s relation from %s: %s", runAs, o.Relate, o.Type,
			noGrant))
	}
}
