package flowcfg

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// Limits on one file. They keep a load, and every engine step over a
// flow, bounded.
const (
	// MaxFlows caps the flows one file declares.
	MaxFlows = 64
	// MaxStarts caps the start triggers of one flow.
	MaxStarts = 8
	// MaxBranches caps the entries of one `on.<event>`.
	MaxBranches = 16
	// MaxOutputs caps the outputs of one state.
	MaxOutputs = 8
	// MaxFileBytes caps the size of flows.yaml.
	MaxFileBytes = 1 << 20
)

// Problem is one thing the load refused or warned about: the key path in
// flows.yaml and what is wrong there.
type Problem struct {
	Key     string
	Message string
}

func (p Problem) String() string {
	if p.Key == "" {
		return p.Message
	}
	return p.Key + ": " + p.Message
}

// Error is a refused load: every problem found, in key order.
type Error struct {
	Problems []Problem
}

func (e *Error) Error() string {
	lines := make([]string, 0, len(e.Problems))
	for _, p := range e.Problems {
		lines = append(lines, p.String())
	}
	return "flows.yaml: " + strings.Join(lines, "; ")
}

// guardFunctions are the host functions a guard may call: pure functions
// of their arguments and the date. A guard runs as the engine and its
// outcome moves a run every reader of the subject can see, so nothing
// that reads beyond the subject's own properties is allowed.
var guardFunctions = []string{
	predicatefns.FuncContains,
	predicatefns.FuncDateAdd,
	predicatefns.FuncDaysBetween,
	predicatefns.FuncLen,
	predicatefns.FuncToday,
}

// Parse loads flows.yaml from data against the schema and acl.yaml. An
// empty file, or one without `flows:`, declares no flows. policy may be
// nil (no acl.yaml): roles and permissions then cannot resolve, and the
// grant check has nothing to check against.
//
// Nil: meta is rejected.
func Parse(data []byte, meta *metamodel.Metamodel, policy *acl.Policy) (*Compiled, error) {
	if meta == nil {
		return nil, errors.New("flowcfg: Parse requires a metamodel")
	}
	if len(data) > MaxFileBytes {
		return nil, &Error{Problems: []Problem{{Message: fmt.Sprintf("over %d bytes", MaxFileBytes)}}}
	}
	var raw rawFile
	if err := strictDecode(data, &raw); err != nil {
		return nil, &Error{Problems: []Problem{{Message: err.Error()}}}
	}
	if hasAlias(&raw.Flows) {
		// Each flow is decoded on its own, so an alias to an anchor in
		// another flow would not resolve, and the version would hash the
		// alias rather than what it stands for.
		return nil, &Error{Problems: []Problem{{Key: "flows",
			Message: "uses an anchor or alias; flows.yaml does not support them"}}}
	}
	l := &loader{meta: meta, policy: policy, c: &Compiled{flows: map[string]*Flow{}}}
	l.flows(&raw.Flows)
	l.outsideEditsNeedPolicy()
	if len(l.problems) > 0 {
		sortProblems(l.problems)
		return nil, &Error{Problems: l.problems}
	}
	sortProblems(l.warnings)
	l.c.warnings = l.warnings
	return l.c, nil
}

// ParseFlow compiles one flow from the declaration a run stored at start
// ([Flow.Source]) against the current schema and acl.yaml, so a run can
// finish on the version it started on (RR-7E3BWVM). The result has
// version want; a source that hashes to another version is an error. A
// source that no longer compiles, because the schema changed say, is an
// [*Error]: the caller strands that run and reports it, and other runs
// go on.
//
// Nil: meta is rejected.
func ParseFlow(name string, source []byte, want string, meta *metamodel.Metamodel,
	policy *acl.Policy) (*Flow, error) {
	if meta == nil {
		return nil, errors.New("flowcfg: ParseFlow requires a metamodel")
	}
	key := "flows." + name
	fail := func(msg string) (*Flow, error) {
		return nil, &Error{Problems: []Problem{{Key: key, Message: msg}}}
	}
	if !flow.ValidName(name) {
		return fail("is not a valid flow name")
	}
	if len(source) > MaxFileBytes {
		return fail(fmt.Sprintf("over %d bytes", MaxFileBytes))
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(source, &doc); err != nil {
		return fail(err.Error())
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 {
		return fail("is not one flow declaration")
	}
	n := doc.Content[0]
	if hasAlias(n) {
		return fail("uses an anchor or alias; flows.yaml does not support them")
	}
	l := &loader{meta: meta, policy: policy, c: &Compiled{flows: map[string]*Flow{}}, stored: true}
	f := l.flow(key, name, n)
	if len(l.problems) > 0 || f == nil {
		sortProblems(l.problems)
		return nil, &Error{Problems: l.problems}
	}
	if f.version != want {
		return fail(fmt.Sprintf("hashes to version %s, not %s", f.version, want))
	}
	return f, nil
}

// DeclaresFlows reports whether data, the bytes of flows.yaml, declares at
// least one flow. A file that does not decode counts as declaring flows,
// so the caller's [Parse] reports the problem rather than skipping it.
func DeclaresFlows(data []byte) bool {
	if len(bytes.TrimSpace(data)) == 0 {
		return false
	}
	var raw rawFile
	if err := strictDecode(data, &raw); err != nil {
		return true
	}
	n := &raw.Flows
	switch {
	case n.Kind == 0, n.Kind == yaml.ScalarNode && n.Tag == "!!null":
		return false
	case n.Kind == yaml.MappingNode:
		return len(n.Content) > 0
	default:
		return true
	}
}

// hasAlias reports whether n or a node below it is an alias or an anchor.
func hasAlias(n *yaml.Node) bool {
	if n.Kind == yaml.AliasNode || n.Anchor != "" {
		return true
	}
	return slices.ContainsFunc(n.Content, hasAlias)
}

func sortProblems(ps []Problem) {
	sort.SliceStable(ps, func(i, j int) bool { return ps[i].Key < ps[j].Key })
}

// strictDecode decodes YAML into out, refusing unknown keys.
func strictDecode(data []byte, out any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	// An empty document decodes to io.EOF: no flows.
	if err := dec.Decode(out); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// decodeNode decodes one node strictly, by encoding it again.
func decodeNode(n *yaml.Node, out any) error {
	data, err := yaml.Marshal(n)
	if err != nil {
		return err
	}
	return strictDecode(data, out)
}

// loader collects the problems of one load.
type loader struct {
	meta     *metamodel.Metamodel
	policy   *acl.Policy
	c        *Compiled
	problems []Problem
	warnings []Problem
	// stored marks a load of a run's stored declaration ([ParseFlow]). It
	// skips `outside_edits:`: the rules always come from the current
	// flows.yaml, so checking an old version's rules against today's
	// schema could only strand the run.
	stored bool
}

func (l *loader) add(key, msg string) {
	l.problems = append(l.problems, Problem{Key: key, Message: msg})
}
func (l *loader) warn(key, msg string) {
	l.warnings = append(l.warnings, Problem{Key: key, Message: msg})
}

// mapping returns the key/value pairs of a mapping node in file order. A
// zero node or a null is an empty mapping; any other kind is a problem at
// key. yaml.v3 does not refuse a repeated key in a node it keeps whole, so
// mapping does: the second declaration would otherwise shadow the first.
func (l *loader) mapping(key string, n *yaml.Node) (keys []string, values []*yaml.Node, ok bool) {
	switch {
	case n.Kind == 0, n.Kind == yaml.ScalarNode && n.Tag == "!!null":
		return nil, nil, true
	case n.Kind != yaml.MappingNode:
		l.add(key, "must be a mapping")
		return nil, nil, false
	}
	seen := map[string]bool{}
	ok = true
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i].Value
		if seen[k] {
			l.add(key+"."+k, "is declared twice")
			ok = false
			continue
		}
		seen[k] = true
		keys = append(keys, k)
		values = append(values, n.Content[i+1])
	}
	return keys, values, ok
}

func (l *loader) flows(n *yaml.Node) {
	names, nodes, ok := l.mapping("flows", n)
	if !ok {
		return
	}
	if len(names) > MaxFlows {
		l.add("flows", fmt.Sprintf("at most %d flows", MaxFlows))
		return
	}
	for i, name := range names {
		key := "flows." + name
		if !flow.ValidName(name) {
			l.add(key, "a flow name is 1 to 64 lower-case letters, digits, '-' or '_', starting with a letter")
			continue
		}
		if f := l.flow(key, name, nodes[i]); f != nil {
			l.c.flows[name] = f
		}
	}
}

// flow compiles one flow, or returns nil after reporting its problems.
func (l *loader) flow(key, name string, n *yaml.Node) *Flow {
	var raw rawFlow
	if err := decodeNode(n, &raw); err != nil {
		l.add(key, err.Error())
		return nil
	}
	before := len(l.problems)
	f := &Flow{name: name, label: raw.Label, index: map[string]int{}}
	if f.label == "" {
		f.label = name
	}
	f.version, f.source = version(n)
	if !l.subject(key+".subject", raw.Subject) {
		return nil
	}
	f.subject = raw.Subject
	f.perSubject = l.perSubject(key+".per_subject", raw.PerSubject)
	f.deadline = l.period(key+".deadline", raw.Deadline, false)
	f.retention = l.retention(key+".retention", raw.Retention)
	l.states(key, &raw, f)
	if !l.stored {
		f.outsideEdits = l.outsideEdits(key+".outside_edits", &raw.OutsideEdits, f)
	}
	f.starts = l.starts(key+".start", raw.Start, f.subject, f.perSubject)
	f.runAs = l.runAs(key+".run_as", raw.RunAs, f)
	if len(l.problems) > before {
		return nil
	}
	c := flowCheck{l: l}
	c.reachable(key, f)
	c.doneCycles(key, f)
	c.forks(key, f)
	c.waiting(key, f)
	c.grants(key, f)
	if len(l.problems) > before {
		return nil
	}
	f.def = definition(f)
	return f
}

// version hashes a flow's declaration without its comments, so a comment
// edit keeps the version. It returns the hash and the bytes it hashed.
func version(n *yaml.Node) (hash string, source []byte) {
	c := clearComments(n)
	data, err := yaml.Marshal(c)
	if err != nil { // coverage-ignore: a node the decoder produced always marshals
		data = nil
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:16], data
}

func clearComments(n *yaml.Node) *yaml.Node {
	c := *n
	c.HeadComment, c.LineComment, c.FootComment = "", "", ""
	c.Line, c.Column = 0, 0
	if len(n.Content) > 0 {
		c.Content = make([]*yaml.Node, len(n.Content))
		for i, child := range n.Content {
			c.Content[i] = clearComments(child)
		}
	}
	return &c
}

func (l *loader) subject(key, typ string) bool {
	if typ == "" {
		l.add(key, "is required: the entity type a run carries")
		return false
	}
	if entity.IsManagedName(typ) {
		l.add(key, fmt.Sprintf("names %q, which rela manages itself; a flow cannot carry it", typ))
		return false
	}
	if _, ok := l.meta.GetEntityDef(typ); !ok {
		l.add(key, fmt.Sprintf("names %q, which schema.yaml does not declare", typ))
		return false
	}
	// FieldRestricted and the relation checks match type names exactly, so
	// an alias here would bypass them.
	if canon := l.meta.ResolveAlias(typ); canon != typ {
		l.add(key, fmt.Sprintf("%q is an alias of %q; name the type itself", typ, canon))
		return false
	}
	if len(metamodel.FaceOrderOf(l.meta, typ)) > 0 {
		l.add(key, fmt.Sprintf("names %q, which declares faces; a flow cannot carry a faced type yet", typ))
		return false
	}
	return true
}

func (l *loader) perSubject(key, raw string) PerSubject {
	switch PerSubject(raw) {
	case "", PerSubjectOne:
		return PerSubjectOne
	case PerSubjectMany:
		return PerSubjectMany
	}
	l.add(key, fmt.Sprintf("is %q; it is one or many", raw))
	return PerSubjectOne
}

// period parses a period. An empty one is zero unless required.
func (l *loader) period(key, raw string, required bool) flow.Period {
	if raw == "" {
		if required {
			l.add(key, "is required")
		}
		return flow.Period{}
	}
	p, err := flow.ParsePeriod(raw)
	if err != nil {
		l.add(key, err.Error())
		return flow.Period{}
	}
	if d, fixed := p.Fixed(); fixed && d < flow.MinTimer {
		l.add(key, fmt.Sprintf("is shorter than %s", flow.MinTimer))
	}
	return p
}

func (l *loader) retention(key string, raw *rawRetention) Retention {
	r := Retention{Action: RetentionArchive}
	if raw == nil {
		return r
	}
	r.After = l.period(key+".after", raw.After, true)
	switch RetentionAction(raw.Action) {
	case "", RetentionArchive:
	case RetentionDelete:
		r.Action = RetentionDelete
	default:
		l.add(key+".action", fmt.Sprintf("is %q; it is archive or delete", raw.Action))
	}
	return r
}

// runAs checks `run_as` against the flow's outputs and returns it, or ""
// when it is absent or refused.
func (l *loader) runAs(key string, raw *string, f *Flow) string {
	writes := false
	for _, st := range f.states {
		writes = writes || len(st.outputs) > 0
	}
	switch {
	case raw == nil && writes:
		l.add(key, "is required: the flow's outputs write as run_as; set it to a job identity acl.yaml grants")
		return ""
	case raw == nil:
		return ""
	}
	if err := principal.ValidJobIdentity(*raw); err != nil {
		l.add(key, err.Error())
		return ""
	}
	if !writes {
		l.warn(key, "is not used: no state of the flow has outputs")
	}
	return *raw
}

// starts compiles the start triggers. A flow without one loads, with a
// warning: nothing can start it.
func (l *loader) starts(key string, raw []rawStart, subject string, per PerSubject) []Start {
	if len(raw) == 0 {
		l.warn(key, "has no entries: nothing starts a run of this flow")
	}
	if len(raw) > MaxStarts {
		l.add(key, fmt.Sprintf("at most %d start triggers", MaxStarts))
		return nil
	}
	out := make([]Start, 0, len(raw))
	for i, r := range raw {
		sk := fmt.Sprintf("%s[%d]", key, i)
		s := Start{On: StartOn(r.On)}
		switch s.On {
		case StartCreate:
			l.noKeys(sk, r, "permission", "every")
		case StartPredicate:
			l.noKeys(sk, r, "permission", "every")
			if r.When == "" {
				l.add(sk+".when", "is required for on: predicate")
			}
		case StartManual:
			l.noKeys(sk, r, "when", "every")
			s.Permission = l.permission(sk+".permission", r.Permission)
		case StartSchedule:
			l.noKeys(sk, r, "permission")
			s.Every = l.period(sk+".every", r.Every, true)
			if per == PerSubjectMany {
				// Each tick would start another run on every matching
				// subject, without bound.
				l.add(sk, "on: schedule needs per_subject: one; with many, every tick starts another run")
			}
		default:
			l.add(sk+".on", fmt.Sprintf("is %q; it is create, predicate, manual or schedule", r.On))
			continue
		}
		if r.When != "" {
			s.When = l.guard(sk+".when", subject, r.When)
		}
		out = append(out, s)
	}
	return out
}

// noKeys reports each named key that r sets, which its `on:` does not take.
func (l *loader) noKeys(key string, r rawStart, names ...string) {
	set := map[string]bool{"when": r.When != "", "permission": r.Permission != "", "every": r.Every != ""}
	for _, n := range names {
		if set[n] {
			l.add(key+"."+n, "does not apply to on: "+r.On)
		}
	}
}

func (l *loader) permission(key, name string) string {
	if name == "" {
		l.add(key, "is required for on: manual: the permission a person needs to start a run")
		return ""
	}
	if l.policy == nil || l.policyGrants(name) {
		return name
	}
	l.warn(key, fmt.Sprintf("no role in acl.yaml grants permission %q, so nobody can start the flow by hand", name))
	return name
}

// guard compiles a `when:` over the subject and holds it to the guard
// allowlist: no traversal, no function outside [guardFunctions], the
// subject only as entity.<property>, and no property a `visible:` grant
// restricts. A run's state is visible to every reader of its subject, so
// a guard on a hidden property would disclose its value.
func (l *loader) guard(key, subject, src string) *Guard {
	return l.guardWith(key, subject, src, nil)
}

// The variables a join's `when:` sees besides `entity`: the branches that
// reached the join, and the branches of the fork.
const (
	VarArrived  = "arrived"
	VarBranches = "branches"
)

// joinVars declares [VarArrived] and [VarBranches] as ints.
var joinVars = []string{VarArrived, VarBranches}

// guardWith is [loader.guard] with the int variables ints declared
// besides `entity`.
func (l *loader) guardWith(key, subject, src string, ints []string) *Guard {
	def, _ := l.meta.GetEntityDef(subject)
	env := predicate.NewEnv()
	if err := env.DeclareVar("entity", predicatefns.EntityRecordType(l.meta, def)); err != nil {
		l.add(key, err.Error()) // coverage-ignore: a fresh env declares one variable
		return nil
	}
	for _, name := range ints {
		if err := env.DeclareVar(name, predicate.IntType); err != nil {
			l.add(key, err.Error()) // coverage-ignore: the names are fixed and distinct
			return nil
		}
	}
	if err := predicatefns.Declare(env); err != nil {
		l.add(key, err.Error()) // coverage-ignore: the standard library declares on a fresh env
		return nil
	}
	prog, err := predicate.Compile(env, src)
	if err != nil {
		l.add(key, fmt.Sprintf("does not compile for %s: %v", subject, err))
		return nil
	}
	ok := true
	if len(prog.Traversals()) > 0 {
		l.add(key, "may not follow relations (related); a guard reads only the subject's own properties")
		ok = false
	}
	for _, fn := range prog.Functions() {
		if !slices.Contains(guardFunctions, fn) {
			l.add(key, fmt.Sprintf("may not call %s; a guard may call only %s", fn, strings.Join(guardFunctions, ", ")))
			ok = false
		}
	}
	if prog.UsesWhole("entity") {
		l.add(key, "may use the subject only as entity.<property>")
		ok = false
	}
	fields := prog.Attributes("entity")
	for _, name := range fields {
		if l.policy != nil && l.policy.FieldRestricted(subject, name) {
			l.add(key, fmt.Sprintf("reads %s.%s, which a visible: grant in acl.yaml restricts; the run's state "+
				"would disclose its value to every reader of the subject", subject, name))
			ok = false
		}
	}
	if !ok {
		return nil
	}
	return &Guard{program: prog, source: src, fields: fields}
}

// definition builds the engine's definition of f.
func definition(f *Flow) *flow.Definition {
	states := make([]flow.State, 0, len(f.states))
	for _, st := range f.states {
		timers := make([]flow.Timer, 0, len(st.timers))
		for _, t := range st.timers {
			timers = append(timers, t.Timer)
		}
		states = append(states, flow.NewState(st.Name, st.Final, timers))
	}
	return flow.NewDefinition(f.initial, states, nil)
}
