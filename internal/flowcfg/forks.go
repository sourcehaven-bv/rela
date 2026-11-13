package flowcfg

import (
	"fmt"
	"slices"
	"strings"
)

// Parallel branches (TKT-ZO9IB6V). A branch whose `to:` is a list forks
// the run: each target starts a branch, and the branches continue as one
// run at the state whose `join:` closes the fork.
//
// The region of a branch is every state reachable from its target without
// passing the join or a final state. In this release forks do not nest,
// so the checks below are one pass over each fork:
//
//   - each region reaches exactly the fork's join, which closes no other
//     fork, and from every region state the join or a final state is
//     reachable;
//   - the regions of a fork are disjoint and hold no fork, no join, not
//     the initial state and not the state that forked;
//   - nothing outside a region moves into it, and only the regions of its
//     fork move into a join;
//   - a fork target is neither final nor a join.

// successors returns the states st moves to: every branch target and
// every timer target.
func successors(st State) []string {
	var out []string
	for _, ev := range Events {
		for _, b := range st.on[ev] {
			out = append(out, b.Targets()...)
		}
	}
	for _, t := range st.timers {
		out = append(out, t.To)
	}
	return out
}

// forks finds and checks every fork of f, and records them on f.
func (c flowCheck) forks(key string, f *Flow) {
	f.region = map[string]regionOf{}
	closedBy := map[string]string{} // join -> the state whose fork it closes
	failed := false
	// seen holds the forks checked, by state and sorted targets: two
	// branches of a state may fork to the same states, under other roles
	// or guards, and are one fork.
	seen := map[string]bool{}
	for _, st := range f.states {
		for _, ev := range Events {
			for _, b := range st.on[ev] {
				if b.Fork == nil {
					continue
				}
				id := forkKey(st.Name, b.Fork)
				if seen[id] {
					continue
				}
				seen[id] = true
				bkey := fmt.Sprintf("%s.states.%s.on.%s", key, st.Name, ev)
				if !c.fork(bkey, f, st.Name, b.Fork, closedBy) {
					failed = true
				}
			}
		}
	}
	if failed {
		return // a join of the fork that failed would read as closing none
	}
	for _, st := range f.states {
		if st.join != nil && closedBy[st.Name] == "" {
			c.l.add(key+".states."+st.Name+".join", "closes no fork: no fork's branches reach it")
		}
	}
}

// forkKey names the fork out of from to targets, whatever their order;
// "" for no fork.
func forkKey(from string, targets []string) string {
	if len(targets) == 0 {
		return ""
	}
	return from + "\x00" + strings.Join(slices.Sorted(slices.Values(targets)), "\x00")
}

// fork checks the fork out of from to targets, at key, and records it.
// It reports whether the fork passed.
func (c flowCheck) fork(key string, f *Flow, from string, targets []string, closedBy map[string]string) bool {
	regions := make([]map[string]bool, len(targets))
	joins := map[string]bool{}
	for i, t := range targets {
		st, _ := f.State(t)
		if st.Final || st.join != nil {
			c.l.add(key, fmt.Sprintf("forks to %q, a final or join state; a branch starts in a state it can leave", t))
			return false
		}
		region, reached, ok := c.region(key, f, from, t)
		if !ok {
			return false
		}
		switch len(reached) {
		case 0:
			c.l.add(key, fmt.Sprintf("the branch from %q reaches no join: give a state of it a way to a state with join:",
				t))
			return false
		case 1:
		default:
			c.l.add(key, fmt.Sprintf("the branch from %q reaches several joins (%s); a branch ends at one", t,
				strings.Join(sortedSet(reached), ", ")))
			return false
		}
		for j := range reached {
			joins[j] = true
		}
		regions[i] = region
	}
	if len(joins) != 1 {
		c.l.add(key, fmt.Sprintf("the branches reach different joins (%s); a fork is closed by one join",
			strings.Join(sortedSet(joins), ", ")))
		return false
	}
	join := sortedSet(joins)[0]
	if by := closedBy[join]; by != "" {
		c.l.add(key, fmt.Sprintf("closes at %q, which already closes the fork out of %q; a join closes one fork", join,
			by))
		return false
	}
	if !c.disjoint(key, f, targets, regions) || !c.closed(key, f, from, targets, regions, join) {
		return false
	}
	closedBy[join] = from
	idx := len(f.forks)
	fk := Fork{State: from, Targets: slices.Clone(targets), Join: join}
	for i, region := range regions {
		for s := range region {
			st, _ := f.State(s)
			fk.Waits = fk.Waits || len(st.waitingFor) > 0
			f.region[s] = regionOf{fork: idx, branch: targets[i]}
		}
	}
	f.forks = append(f.forks, fk)
	return true
}

// region walks the branch from target: every state reachable without
// passing a join or a final state. It returns those states and the joins
// it reaches; ok is false after a problem.
func (c flowCheck) region(key string, f *Flow, from, target string) (region, joins map[string]bool, ok bool) {
	region, joins = map[string]bool{target: true}, map[string]bool{}
	queue := []string{target}
	for len(queue) > 0 {
		st, _ := f.State(queue[0])
		queue = queue[1:]
		switch {
		case st.Name == from:
			c.l.add(key, fmt.Sprintf("the branch from %q leads back to %q, the state that forked", target, from))
			return nil, nil, false
		case st.Name == f.initial:
			c.l.add(key, fmt.Sprintf("the branch from %q holds the initial state %q", target, f.initial))
			return nil, nil, false
		case hasFork(st):
			c.l.add(key, fmt.Sprintf("the branch from %q holds %q, which forks again; forks do not nest", target,
				st.Name))
			return nil, nil, false
		}
		for _, to := range successors(st) {
			next, _ := f.State(to)
			switch {
			case next.join != nil:
				joins[to] = true
			case next.Final, region[to]:
			default:
				region[to] = true
				queue = append(queue, to)
			}
		}
	}
	return region, joins, true
}

func hasFork(st State) bool {
	for _, ev := range Events {
		for _, b := range st.on[ev] {
			if b.Fork != nil {
				return true
			}
		}
	}
	return false
}

// disjoint refuses a state in two branches, of this fork or an earlier
// one.
func (c flowCheck) disjoint(key string, f *Flow, targets []string, regions []map[string]bool) bool {
	owner := map[string]string{}
	for i, region := range regions {
		for _, s := range sortedSet(region) {
			if o, taken := owner[s]; taken {
				c.l.add(key, fmt.Sprintf("%q is in the branches from %q and %q; branches share no state", s, o,
					targets[i]))
				return false
			}
			if _, taken := f.region[s]; taken {
				c.l.add(key, fmt.Sprintf("%q is in the branch from %q and in another fork's branch", s, targets[i]))
				return false
			}
			owner[s] = targets[i]
		}
	}
	return true
}

// closed refuses a way into a branch or the join from outside the fork,
// and a branch state that can reach neither the join nor a final state.
func (c flowCheck) closed(key string, f *Flow, from string, targets []string, regions []map[string]bool,
	join string) bool {
	in := map[string]int{} // state -> index of its branch
	for i, region := range regions {
		for s := range region {
			in[s] = i
		}
	}
	fork := forkKey(from, targets)
	for _, st := range f.states {
		home, inside := in[st.Name]
		for _, to := range movesOut(st, fork) {
			if !c.edgeAllowed(key, st.Name, to, join, home, inside, in) {
				return false
			}
		}
	}
	for i, region := range regions {
		if s, stuck := firstStuck(f, region, join); stuck {
			c.l.add(key, fmt.Sprintf("from %q, in the branch from %q, the run reaches neither %q nor a final state",
				s, targets[i], join))
			return false
		}
	}
	return true
}

// movesOut returns the states st moves into, leaving out the fork named
// fork, and the same fork under other roles or guards.
func movesOut(st State, fork string) []string {
	var out []string
	for _, ev := range Events {
		for _, b := range st.on[ev] {
			if b.Fork != nil && forkKey(st.Name, b.Fork) == fork {
				continue
			}
			out = append(out, b.Targets()...)
		}
	}
	for _, t := range st.timers {
		out = append(out, t.To)
	}
	return out
}

// edgeAllowed checks one move from src to dst against the fork's
// branches: in maps a branch state to its branch.
func (c flowCheck) edgeAllowed(key, src, dst, join string, home int, inside bool, in map[string]int) bool {
	if dst == join && !inside {
		c.l.add(key, fmt.Sprintf("%q moves into the join %q from outside the fork's branches", src, join))
		return false
	}
	if to, ok := in[dst]; ok && (!inside || to != home) {
		c.l.add(key, fmt.Sprintf("%q moves into %q, a state of a branch it is not in; only the fork starts a branch",
			src, dst))
		return false
	}
	return true
}

// firstStuck returns a region state from which neither join nor a final
// state is reachable, in name order.
func firstStuck(f *Flow, region map[string]bool, join string) (string, bool) {
	done := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for s := range region {
			if done[s] {
				continue
			}
			st, _ := f.State(s)
			for _, to := range successors(st) {
				next, _ := f.State(to)
				if to == join || next.Final || done[to] {
					done[s], changed = true, true
					break
				}
			}
		}
	}
	for _, s := range sortedSet(region) {
		if !done[s] {
			return s, true
		}
	}
	return "", false
}

func sortedSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for s := range m {
		out = append(out, s)
	}
	slices.Sort(out)
	return out
}
