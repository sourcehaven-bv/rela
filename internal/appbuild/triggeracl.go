package appbuild

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/configedit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg/triggers"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
)

// TriggerEditor is the principal who edits triggers in the Configure space,
// as [CheckTriggerACL] needs it.
type TriggerEditor interface {
	// Roles names the editor's global roles.
	Roles() []string
	// Grants reports whether the editor, holding role, may perform verb on
	// the face of entityType, after the client ceiling.
	Grants(role string, verb acl.Verb, entityType string, face entity.Face) bool
}

// RequestEditor is the [TriggerEditor] of a bound request: its global roles
// and the grants [acl.Request.RoleGrantsVerbOnFace] answers. Roles conferred
// on single records (role_relations) are not counted, so an editor may be
// refused a write they hold through one.
//
// Nil: rejected; there is no editor to judge without a request.
func RequestEditor(req *acl.Request) (TriggerEditor, error) {
	if req == nil {
		return nil, errors.New("appbuild: RequestEditor: request is required")
	}
	return requestEditor{req: req}, nil
}

type requestEditor struct{ req *acl.Request }

func (e requestEditor) Roles() []string {
	var out []string
	for _, a := range e.req.Globals().Attributions {
		if !slices.Contains(out, a.Role) {
			out = append(out, a.Role)
		}
	}
	return out
}

func (e requestEditor) Grants(role string, verb acl.Verb, entityType string, face entity.Face) bool {
	return e.req.RoleGrantsVerbOnFace(role, verb, entityType, face)
}

// CheckTriggerACL judges an edit of the flows.yaml triggers made in the
// Configure space by editor against acl.yaml. A trigger output runs as a cascade of
// the write that fired it, without the row ACL or the field gate, so a
// trigger edited there:
//
//   - may not run a script, apply a template variant, replace existing
//     records or link to a fixed record: none of these can be bounded;
//   - may not read a property acl.yaml hides from some reader (`visible:`,
//     a client or scope `visible:`/`redact:`), in `on`, `where`, `when` or a
//     placeholder, nor follow relations in `when`;
//   - may not create a record that someone reads who may not read the
//     record that fired it;
//   - may not write what acl.yaml decides access by (the principal property,
//     a property or relation a grant's `when:` names, an `options:` field,
//     the membership relation, a role or inherited relation, a relation with
//     its own grant, the principal type), before or after the edit;
//   - may not write what the editor could not write directly, nor what a
//     flow's `outside_edits:` or a state's `writes:` locks;
//   - may not make an unchanged trigger write any of that, or create a
//     record read more widely than the edited trigger's subject, through a
//     record it creates.
//
// A changed trigger is judged whole. When schema.yaml changes, an unchanged
// trigger is refused only for what the schema change adds. The policy is
// parsed from the bytes given and checked against each schema, so an
// abstract or wildcard grant covers the types the draft adds.
//
// An error means the edit could not be judged; the caller refuses it.
func CheckTriggerACL(in configedit.TriggerEdit, editor TriggerEditor) ([]configedit.TriggerProblem, error) {
	if editor == nil {
		return nil, errors.New("no editor to judge the change for")
	}
	if len(bytes.TrimSpace(in.ACL)) == 0 {
		return nil, errors.New("acl.yaml is absent or empty")
	}
	before, err := newTriggerJudge(in.ACL, in.SchemaBefore, in.FlowsBefore, editor)
	if err != nil {
		return nil, fmt.Errorf("the configuration as read: %w", err)
	}
	after, err := newTriggerJudge(in.ACL, in.SchemaAfter, in.FlowsAfter, editor)
	if err != nil {
		return nil, fmt.Errorf("the configuration as saved: %w", err)
	}
	schemaChanged := !bytes.Equal(in.SchemaBefore, in.SchemaAfter)

	var out triggerProblems
	var judged []triggers.Rule
	for _, r := range after.rules {
		old, had := before.rule(r.Name)
		if had && sameRule(old, r) {
			if schemaChanged {
				was := before.violations(old, true)
				for _, msg := range after.violations(r, true) {
					if !slices.Contains(was, msg) {
						out.add(r.Name, msg)
					}
				}
			}
			continue
		}
		judged = append(judged, r)
		for _, msg := range after.violations(r, true) {
			out.add(r.Name, msg)
		}
	}
	for _, old := range before.rules {
		if now, has := after.rule(old.Name); has && sameRule(old, now) {
			continue
		}
		for _, msg := range before.writeViolations(old, false) {
			out.add(old.Name, msg+" Changing or removing it would stop that write, so it is done in flows.yaml directly.")
		}
	}
	after.cascade(judged, &out)
	return out.list, nil
}

// triggerProblems collects problems once each, in the order found.
type triggerProblems struct{ list []configedit.TriggerProblem }

func (p *triggerProblems) add(trigger, msg string) {
	got := configedit.TriggerProblem{Trigger: trigger, Message: fmt.Sprintf("Automation %q %s", trigger, msg)}
	if !slices.Contains(p.list, got) {
		p.list = append(p.list, got)
	}
}

// triggerJudge holds one side of the edit: the schema, the policy checked
// against it, and the triggers.
type triggerJudge struct {
	meta      *metamodel.Metamodel
	policy    *acl.Policy
	rules     []triggers.Rule
	editor    TriggerEditor
	eval      *predicatefns.Evaluator
	whenWords map[string]bool
	// locked holds, per entity type, the properties a flow freezes
	// against outside edits.
	locked map[string]map[string]string
}

func newTriggerJudge(aclData, schema, flows []byte, editor TriggerEditor) (*triggerJudge, error) {
	meta, err := metamodel.Parse(schema)
	if err != nil {
		return nil, err
	}
	policy, err := acl.ParsePolicy(aclData, "acl.yaml")
	if err != nil {
		return nil, err
	}
	if verr := ValidateACLPolicy(policy, meta); verr != nil {
		return nil, verr
	}
	rules, err := triggers.Parse(flows, meta)
	if err != nil {
		return nil, err
	}
	locked, err := flowLocks(flows, meta, policy)
	if err != nil {
		return nil, err
	}
	return &triggerJudge{
		meta: meta, policy: policy, rules: rules, editor: editor,
		eval: predicatefns.NewEvaluator(meta), whenWords: policyWhenWords(policy), locked: locked,
	}, nil
}

// flowLocks returns, per entity type, the properties some flow over it
// does not let an outside edit change, each with the flow's name: a
// property its `outside_edits:` denies or puts behind a permission in some
// state, or one a state `writes:`. A trigger writes as an outside edit
// without that gate.
func flowLocks(flows []byte, meta *metamodel.Metamodel, policy *acl.Policy) (map[string]map[string]string, error) {
	out := map[string]map[string]string{}
	if len(bytes.TrimSpace(flows)) == 0 {
		return out, nil
	}
	compiled, err := flowcfg.Parse(flows, meta, policy)
	if err != nil {
		return nil, err
	}
	for _, f := range compiled.Flows() {
		for _, typ := range metamodel.ExpandTypeNames(meta, []string{f.Subject()}) {
			if def, ok := meta.GetEntityDef(typ); ok {
				for _, prop := range flowLockedProperties(f, slices.Sorted(maps.Keys(def.Properties))) {
					if out[typ] == nil {
						out[typ] = map[string]string{}
					}
					if _, had := out[typ][prop]; !had {
						out[typ][prop] = f.Name()
					}
				}
			}
		}
	}
	return out, nil
}

// flowLockedProperties lists the properties, of props, that f does not let
// an outside edit change in some state, and those a state writes.
func flowLockedProperties(f *flowcfg.Flow, props []string) []string {
	var out []string
	for _, st := range f.States() {
		out = append(out, st.Writes()...)
		for _, prop := range props {
			if f.OutsideEdits().Rule(st.Name, prop).Kind != flowcfg.EditAllow {
				out = append(out, prop)
			}
		}
	}
	return out
}

func (j *triggerJudge) rule(name string) (triggers.Rule, bool) {
	for _, r := range j.rules {
		if r.Name == name {
			return r, true
		}
	}
	return triggers.Rule{}, false
}

// violations lists why r may not be saved from the app. withEditor adds
// the editor's own rights; an unchanged trigger is judged without them.
func (j *triggerJudge) violations(r triggers.Rule, withEditor bool) []string {
	out := j.unbounded(r)
	out = append(out, j.readViolations(r)...)
	out = append(out, j.writeViolations(r, withEditor)...)
	return append(out, j.rowViolations(r)...)
}

// unbounded lists the outputs whose effect the check cannot bound.
func (j *triggerJudge) unbounded(r triggers.Rule) []string {
	var out []string
	for _, a := range r.Do {
		if a.Lua != "" || a.LuaFile != "" {
			out = append(out, "runs a script, whose reads and writes cannot be checked against acl.yaml.")
		}
		if a.CreateEntity != nil && a.CreateEntity.IfExists == triggers.IfExistsReplace {
			out = append(out, "replaces existing records (if_exists: replace), which deletes them without an access check.")
		}
		if a.CreateEntity != nil && a.CreateEntity.Template != "" {
			out = append(out, "applies a template variant, whose properties cannot be checked against acl.yaml.")
		}
	}
	return out
}

// readViolations lists the reads of r that acl.yaml may hide, and the
// reads the check cannot bound.
func (j *triggerJudge) readViolations(r triggers.Rule) []string {
	var out []string
	var reads []string
	if r.On.Property != "" {
		reads = append(reads, r.On.Property)
	}
	for _, f := range r.On.When {
		reads = append(reads, f.Property)
	}
	for _, v := range r.Validate {
		f, err := filter.Parse(v.Check)
		if err != nil {
			out = append(out, fmt.Sprintf("has a check whose where clause %q cannot be read: %v.", v.Check, err))
		} else {
			reads = append(reads, f.Property)
		}
		reads = append(reads, placeholderReads(v.Message)...)
	}
	for _, a := range r.Do {
		aReads, problems := j.actionReads(a)
		reads = append(reads, aReads...)
		out = append(out, problems...)
	}
	for _, s := range j.subjects(r) {
		sReads := reads
		if r.On.Condition != "" {
			attrs, problems := j.conditionReads(s, r.On.Condition)
			out = append(out, problems...)
			sReads = append(slices.Clone(reads), attrs...)
		}
		for _, p := range sReads {
			if j.restricted(s, p) {
				out = append(out, fmt.Sprintf("reads %s.%s, which acl.yaml hides from some readers.", s, p))
			}
		}
	}
	return dedupe(out)
}

// actionReads returns the properties of the subject an output reads, and
// the reads of it the check cannot bound.
func (j *triggerJudge) actionReads(a triggers.Action) (reads, problems []string) {
	reads = placeholderReads(a.Value)
	switch {
	case a.CreateRelation == nil:
	case strings.Contains(a.CreateRelation.To, "{{"):
		problems = append(problems, "names its relation target with a placeholder, so whoever writes the value "+
			"can link to, and probe for, any record.")
	default:
		problems = append(problems, fmt.Sprintf("links to the fixed record %q; the check cannot tell whether you "+
			"may read it.", a.CreateRelation.To))
	}
	if c := a.CreateEntity; c != nil {
		for _, k := range slices.Sorted(maps.Keys(c.Properties)) {
			reads = append(reads, placeholderReads(c.Properties[k])...)
		}
	}
	if p := a.AddToPile; p != nil {
		if strings.Contains(p.Pile, "{{") {
			problems = append(problems, "names its pile with a placeholder; the pile owner would see the value.")
		}
		reads = append(reads, placeholderReads(p.Owner)...)
	}
	if n := a.Notify; n != nil {
		if len(n.Via) == 0 {
			reads = append(reads, n.Property)
		} else {
			problems = append(problems, j.notifyViolations(n)...)
		}
	}
	return reads, problems
}

// conditionReads returns the properties a `when:` reads on s, or why it
// cannot be bounded. The allowed functions are those of a flow guard.
func (j *triggerJudge) conditionReads(s, src string) (attrs, problems []string) {
	prog, err := j.eval.Compile(s, src)
	if err != nil {
		return nil, []string{fmt.Sprintf("has a when condition that does not compile for %s: %v.", s, err)}
	}
	var out []string
	if len(prog.Traversals()) > 0 {
		out = append(out, "follows relations in its when condition; what that reads cannot be checked against acl.yaml.")
	}
	if prog.UsesWhole("entity") {
		out = append(out, "passes the whole record to a function in its when condition; "+
			"use entity.<property> so the check can see what it reads.")
	}
	for _, fn := range prog.Functions() {
		if !slices.Contains(triggerConditionFunctions, fn) {
			out = append(out, fmt.Sprintf("calls %s in its when condition; only %s are allowed here.",
				fn, strings.Join(triggerConditionFunctions, ", ")))
		}
	}
	return prog.Attributes("entity"), out
}

// triggerConditionFunctions are the functions a `when:` edited in the app
// may call: those of a flow guard, which read nothing but their arguments.
var triggerConditionFunctions = []string{
	predicatefns.FuncContains,
	predicatefns.FuncDateAdd,
	predicatefns.FuncDaysBetween,
	predicatefns.FuncLen,
	predicatefns.FuncToday,
}

// notifyViolations judges a notify output whose recipient is found along a
// relation path. The path is not resolved here, so the recipient property
// may be hidden on no entity type that declares it.
func (j *triggerJudge) notifyViolations(n *triggers.NotifyAction) []string {
	var out []string
	for _, t := range sortedTypes(j.meta) {
		def, _ := j.meta.GetEntityDef(t)
		if _, ok := def.Properties[n.Property]; ok && j.restricted(t, n.Property) {
			out = append(out, fmt.Sprintf("finds notify recipients in %s.%s, which acl.yaml hides from some readers.",
				t, n.Property))
		}
	}
	return out
}

// writeViolations lists the writes of r that acl.yaml decides access by
// and, withEditor, those the editor could not make directly.
func (j *triggerJudge) writeViolations(r triggers.Rule, withEditor bool) []string {
	var out []string
	for _, a := range r.Do {
		switch {
		case a.Set != "":
			for _, s := range j.subjects(r) {
				out = append(out, j.propertyViolations(s, a.Set, "sets %s.%s, which you may not change yourself.",
					withEditor && !j.editorMay(acl.VerbUpdate, s, j.faces(r, s), a.Set))...)
			}
		case a.CreateRelation != nil:
			out = append(out, j.relationViolations(r, a.CreateRelation.Relation, withEditor)...)
		case a.CreateEntity != nil:
			out = append(out, j.createViolations(r, a.CreateEntity, withEditor)...)
		}
	}
	return dedupe(out)
}

// createViolations is writeViolations for one create output.
func (j *triggerJudge) createViolations(r triggers.Rule, c *triggers.CreateEntityAction, withEditor bool) []string {
	var out []string
	typ := j.meta.ResolveAlias(c.Type)
	if typ == j.policy.UserEntityType {
		out = append(out, fmt.Sprintf("creates %s records, which acl.yaml uses as principals.", typ))
	}
	for _, k := range slices.Sorted(maps.Keys(c.Properties)) {
		out = append(out, j.propertyViolations(typ, k, "creates %s records with %s, which you may not do yourself.",
			withEditor && !j.editorMay(acl.VerbCreate, typ, j.typeFaces(typ), k))...)
	}
	if withEditor && len(c.Properties) == 0 && !j.editorMay(acl.VerbCreate, typ, j.typeFaces(typ), "") {
		out = append(out, fmt.Sprintf("creates %s records, which you may not do yourself.", typ))
	}
	if c.Relation != "" {
		out = append(out, j.relationViolations(r, c.Relation, withEditor)...)
	}
	return out
}

// propertyViolations judges a write of prop on typ: whether acl.yaml
// decides access by it, whether a flow locks it, and, when editorDenied,
// editorMsg formatted with typ and prop.
func (j *triggerJudge) propertyViolations(typ, prop, editorMsg string, editorDenied bool) []string {
	var out []string
	if j.decidesAccess(typ, prop) {
		out = append(out, fmt.Sprintf("sets %s.%s, which acl.yaml decides access by.", typ, prop))
	}
	if editorDenied {
		out = append(out, fmt.Sprintf(editorMsg, typ, prop))
	}
	if flow, ok := j.locked[typ][prop]; ok {
		out = append(out, fmt.Sprintf("sets %s.%s, which flow %q locks against outside edits.", typ, prop, flow))
	}
	return out
}

func (j *triggerJudge) relationViolations(r triggers.Rule, rel string, withEditor bool) []string {
	var out []string
	if j.relationDecidesAccess(rel) {
		out = append(out, fmt.Sprintf("creates %s relations, which acl.yaml decides access by.", rel))
	}
	if withEditor {
		for _, s := range j.subjects(r) {
			if !j.editorMay(acl.VerbUpdate, s, j.faces(r, s), "") {
				out = append(out, fmt.Sprintf("links %s records, which you may not change yourself.", s))
			}
		}
	}
	return out
}

// rowViolations refuses a created record that someone may read who may not
// read the record that fired the trigger: its existence, and any value
// copied into it, would reach them.
func (j *triggerJudge) rowViolations(r triggers.Rule) []string {
	return j.rowViolationsFrom(r, j.origins(r))
}

// origin is a type and the faces of it a trigger fires on.
type origin struct {
	typ   string
	faces []entity.Face
}

func (j *triggerJudge) origins(r triggers.Rule) []origin {
	subjects := j.subjects(r)
	out := make([]origin, 0, len(subjects))
	for _, s := range subjects {
		out = append(out, origin{s, j.faces(r, s)})
	}
	return out
}

// rowViolationsFrom is rowViolations for the records r creates when the
// data it copies came from origins.
func (j *triggerJudge) rowViolationsFrom(r triggers.Rule, origins []origin) []string {
	var out []string
	for _, a := range r.Do {
		if a.CreateEntity == nil {
			continue
		}
		typ := j.meta.ResolveAlias(a.CreateEntity.Type)
		for _, o := range origins {
			if !j.readersCovered(typ, o.typ, o.faces) {
				out = append(out, fmt.Sprintf("creates %s records from %s records, and some readers of %s may not "+
					"read %s.", typ, o.typ, typ, o.typ))
			}
		}
	}
	return out
}

// cascade judges the unchanged triggers that the judged ones reach. A
// reached trigger may copy what the edited one copied, so the records it
// creates must be readable only by readers of the edited trigger's
// subjects.
//
// cascade follows creation only. A
// record a trigger creates fires the `created` triggers of its type; a
// `set` is part of the write that fired it and relation events fire no
// trigger, so creation is the only way one trigger reaches another. A
// reached trigger is held to the write rules, editor included, because the
// edit makes it run where it did not before.
func (j *triggerJudge) cascade(judged []triggers.Rule, out *triggerProblems) {
	type event struct {
		name    string
		origins []origin
		created string
	}
	var queue []event
	enqueue := func(name string, origins []origin, r triggers.Rule) {
		for _, a := range r.Do {
			if a.CreateEntity != nil {
				queue = append(queue, event{name, origins, j.meta.ResolveAlias(a.CreateEntity.Type)})
			}
		}
	}
	changed := map[string]bool{}
	for _, r := range judged {
		changed[r.Name] = true
		enqueue(r.Name, j.origins(r), r)
	}
	reached := map[string]bool{}
	for len(queue) > 0 {
		ev := queue[0]
		queue = queue[1:]
		for _, u := range j.rules {
			if changed[u.Name] || reached[u.Name] || !u.On.Created {
				continue
			}
			if len(u.On.Entity) > 0 && !slices.Contains(u.On.Entity, ev.created) {
				continue
			}
			reached[u.Name] = true
			msgs := append(j.unbounded(u), j.writeViolations(u, true)...)
			msgs = append(msgs, j.rowViolationsFrom(u, ev.origins)...)
			for _, msg := range msgs {
				out.add(ev.name, fmt.Sprintf("creates %s records, which fire automation %q, which %s",
					ev.created, u.Name, msg))
			}
			enqueue(ev.name, ev.origins, u)
		}
	}
}

// subjects are the entity types r fires on; a trigger without `entity:`
// fires on every type.
func (j *triggerJudge) subjects(r triggers.Rule) []string {
	if len(r.On.Entity) > 0 {
		return r.On.Entity
	}
	return sortedTypes(j.meta)
}

// faces are the faces of s that r fires on: its `faces:`, else every face
// s declares, else the bare one.
func (j *triggerJudge) faces(r triggers.Rule, s string) []entity.Face {
	var out []entity.Face
	if len(r.On.Faces) > 0 {
		for _, f := range r.On.Faces {
			out = append(out, entity.Face(f))
		}
		return out
	}
	return j.typeFaces(s)
}

// typeFaces are every face typ declares, else the bare one.
func (j *triggerJudge) typeFaces(typ string) []entity.Face {
	var out []entity.Face
	def, ok := j.meta.GetEntityDef(typ)
	if !ok || len(def.Faces) == 0 {
		return []entity.Face{""}
	}
	for _, f := range slices.Sorted(maps.Keys(def.Faces)) {
		out = append(out, entity.Face(f))
	}
	return out
}

// restricted reports whether acl.yaml may hide prop of typ from a reader:
// by a role's `visible:`, or by a client baseline or scope grant.
func (j *triggerJudge) restricted(typ, prop string) bool {
	if j.policy.FieldRestricted(typ, prop) {
		return true
	}
	for _, g := range j.policy.ScopeGrants {
		for key, fields := range g.Visible {
			if acl.GrantKeyApplies(key, typ) && !slices.Contains(fields, prop) {
				return true
			}
		}
		for key, fields := range g.Redact {
			if acl.GrantKeyApplies(key, typ) && slices.Contains(fields, prop) {
				return true
			}
		}
	}
	return false
}

// decidesAccess reports whether writing prop of typ can change who may do
// what: the principal property, a field whose options a role is granted,
// or a name a grant's `when:` reads.
func (j *triggerJudge) decidesAccess(typ, prop string) bool {
	if typ == j.policy.UserEntityType && prop == j.policy.PrincipalProperty {
		return true
	}
	if j.whenWords[prop] {
		return true
	}
	for _, role := range j.policy.Roles {
		for key, grants := range role.Options {
			if !acl.GrantKeyApplies(key, typ) {
				continue
			}
			for _, g := range grants {
				if g.Field == prop {
					return true
				}
			}
		}
	}
	return false
}

// relationDecidesAccess reports whether creating rel can change who may do
// what. rel is matched under its own name and its inverse's.
func (j *triggerJudge) relationDecidesAccess(rel string) bool {
	p := j.policy
	for _, name := range j.relationNames(rel) {
		if name == p.EffectiveMembershipRelation() || j.whenWords[name] ||
			slices.Contains(p.InheritRolesThrough, name) {

			return true
		}
		if _, ok := p.RoleRelations[name]; ok {
			return true
		}
		if _, ok := p.RelationWriteGrants[name]; ok {
			return true
		}
		for _, role := range p.Roles {
			for _, grants := range role.Relations {
				for _, g := range grants {
					if g.Relation == name {
						return true
					}
				}
			}
		}
	}
	return false
}

// relationNames is rel and the other name of the same relation.
func (j *triggerJudge) relationNames(rel string) []string {
	out := []string{rel}
	if owner, ok := j.meta.InverseOwner(rel); ok {
		out = append(out, owner)
	}
	if def, ok := j.meta.GetRelationDef(rel); ok && def.Inverse != nil && def.Inverse.ID != "" {
		out = append(out, def.Inverse.ID)
	}
	return out
}

// editorMay reports whether one of the editor's roles grants verb on every
// face of typ, in the policy being saved and in the one serving the
// editor's request, and, when prop is named, grants writing it.
func (j *triggerJudge) editorMay(verb acl.Verb, typ string, faces []entity.Face, prop string) bool {
	for _, name := range j.editor.Roles() {
		role, ok := j.policy.Roles[name]
		if !ok {
			continue
		}
		granted := true
		for _, f := range faces {
			if !acl.GrantsVerbOnFace(role, verb, typ, f) || !j.editor.Grants(name, verb, typ, f) ||
				(prop != "" && !fieldGranted(role, typ, f, prop)) {

				granted = false
				break
			}
		}
		if granted {
			return true
		}
	}
	return false
}

// fieldGranted reports whether role may write prop on face of typ: it
// declares no `fields:` for that face, or grants prop there without a
// condition.
func fieldGranted(role acl.RoleDef, typ string, face entity.Face, prop string) bool {
	declared := false
	for key, grants := range role.Fields {
		if key != typ && key != "*" && key != typ+"@"+string(face) {
			continue
		}
		declared = true
		for _, g := range grants {
			if g.Field == prop && g.When == "" {
				return true
			}
		}
	}
	return !declared
}

// readersCovered reports whether everyone who may read a created record
// may also read every face of the record that fired the trigger. A role
// conferred on single records (role_relations) reads only those, so one
// that reads created records fails the test.
func (j *triggerJudge) readersCovered(created, subject string, faces []entity.Face) bool {
	for name, role := range j.policy.Roles {
		if entity.IsManagedName(name) || !acl.GrantsRead(role, created) {
			continue
		}
		if j.confersLocally(name) || len(role.Worlds) > 0 {
			// A world grant reads faces ReadFaces does not show.
			return false
		}
		got, all := acl.ReadFaces(role, subject)
		if all {
			continue
		}
		for _, f := range faces {
			if !slices.Contains(got, f) {
				return false
			}
		}
	}
	var ceilings []acl.Restriction
	for _, b := range j.policy.ClientBaselines {
		ceilings = append(ceilings, b.Restriction)
	}
	for _, g := range j.policy.ScopeGrants {
		ceilings = append(ceilings, g.Restriction)
	}
	for _, c := range ceilings {
		if mayRead(c, created) && (!surelyReads(c, subject) || len(c.Worlds) > 0 || len(c.DenyWorlds) > 0) {
			return false
		}
	}
	return true
}

func (j *triggerJudge) confersLocally(role string) bool {
	for _, def := range j.policy.RoleRelations {
		if def.Confers == role {
			return true
		}
	}
	return false
}

// mayRead reports whether a ceiling may let some face of typ be read.
func mayRead(c acl.Restriction, typ string) bool {
	if slices.Contains(c.DenyRead, typ) || slices.Contains(c.DenyRead, "*") {
		return false
	}
	if len(c.Read) == 0 {
		return true
	}
	return slices.ContainsFunc(c.Read, func(k string) bool { return acl.GrantKeyApplies(k, typ) })
}

// surelyReads reports whether a ceiling lets every face of typ be read.
func surelyReads(c acl.Restriction, typ string) bool {
	if slices.ContainsFunc(c.DenyRead, func(k string) bool { return acl.GrantKeyApplies(k, typ) }) {
		return false
	}
	return len(c.Read) == 0 || slices.Contains(c.Read, typ) || slices.Contains(c.Read, "*")
}

var policyWord = regexp.MustCompile(`[A-Za-z0-9_-]+`)

// policyWhenWords collects every name a grant's `when:` mentions. A word
// match over-approximates what the conditions read, which errs toward
// refusing.
func policyWhenWords(p *acl.Policy) map[string]bool {
	out := map[string]bool{}
	add := func(src string) {
		for _, w := range policyWord.FindAllString(src, -1) {
			out[w] = true
		}
	}
	for _, role := range p.Roles {
		for _, gs := range []map[string][]acl.FieldGrant{role.Fields, role.Visible} {
			for _, grants := range gs {
				for _, g := range grants {
					add(g.When)
				}
			}
		}
		for _, grants := range role.Options {
			for _, g := range grants {
				add(g.When)
			}
		}
		for _, grants := range role.Relations {
			for _, g := range grants {
				add(g.When)
				for _, f := range append(slices.Clone(g.Fields), g.Visible...) {
					add(f.When)
				}
			}
		}
	}
	return out
}

// placeholderReads returns the properties the placeholders of s read.
func placeholderReads(s string) []string { return triggers.PropertyReads(s) }

// sameRule reports whether two rules behave alike. The description is
// prose, and a where clause is compared by what it tests.
func sameRule(a, b triggers.Rule) bool {
	return reflect.DeepEqual(ruleShape(a), ruleShape(b))
}

type filterShape struct {
	Property string
	Operator filter.Operator
	Value    string
}

type ruleShapeT struct {
	Rule  triggers.Rule
	Where []filterShape
}

func ruleShape(r triggers.Rule) ruleShapeT {
	where := make([]filterShape, 0, len(r.On.When))
	for _, f := range r.On.When {
		where = append(where, filterShape{f.Property, f.Operator, f.Value})
	}
	r.Description = ""
	r.On.When = nil
	return ruleShapeT{Rule: r, Where: where}
}

func sortedTypes(meta *metamodel.Metamodel) []string {
	out := metamodel.ExpandTypeNames(meta, meta.EntityTypes())
	slices.Sort(out)
	return out
}

func dedupe(in []string) []string {
	var out []string
	for _, s := range in {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}
