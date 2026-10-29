package realtimecfg

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// jobOutput is an output of a job move: a timer, or a transition a trigger
// can fire. msgType is the transition's message type, empty for a timer.
type jobOutput struct {
	out     Output
	msgType string
}

// jobOutputs returns the outputs of s that write as [Session.RunAs], in
// declaration order: per state, its timers, then its job transitions.
func (s *Session) jobOutputs() []jobOutput {
	var out []jobOutput
	for _, st := range s.states {
		for _, t := range st.timers {
			for _, o := range t.outputs {
				out = append(out, jobOutput{out: o})
			}
		}
		for _, t := range st.entries() {
			if !t.job {
				continue
			}
			for _, o := range t.outputs {
				out = append(out, jobOutput{out: o, msgType: t.MessageType})
			}
		}
	}
	return out
}

// runAs checks `session.run_as` against the session's job outputs and
// returns the identity, or "" when it is absent or refused. raw is nil
// when the key is absent.
func (v writeChecker) runAs(key string, raw *string, s *Session) string {
	jobs := s.jobOutputs()
	switch {
	case raw == nil && len(jobs) > 0:
		v.add(jobs[0].out.key, "outputs of a timer move, or of a move a trigger can fire, write as "+
			"session.run_as; set session.run_as to a job identity acl.yaml grants")
		return ""
	case raw == nil:
		return ""
	}
	if err := principal.ValidJobIdentity(*raw); err != nil {
		v.add(key, err.Error())
		return ""
	}
	if len(jobs) == 0 {
		v.warn(key, "session.run_as is not used: no timer or trigger move of this session has outputs")
	}
	return *raw
}

// grantChecker checks, after compile, that each session's run_as holds
// every grant its job outputs need, from acl.yaml alone (see
// [acl.JobIdentityGrants]). Person-move outputs are not checked here: the
// poster is known only at the write, which asks the ACL again. It keeps
// these checks off the validator, which is at its method budget.
type grantChecker struct {
	v *validator
	c *Compiled
}

// noGrant ends every grant problem.
const noGrant = "no role assigned to it or to everyone grants it"

// check runs the grant check. Without a policy there is nothing to check.
func (g grantChecker) check() {
	if g.v.policy == nil {
		return
	}
	for _, name := range sortedKeys(g.c.channelTypes) {
		ct := g.c.channelTypes[name]
		s := ct.session
		if s == nil || s.runAs == "" {
			continue
		}
		grants := acl.JobIdentityGrants(g.v.policy, s.runAs)
		for _, jo := range s.jobOutputs() {
			props := g.triggerProps(ct.Name, jo.msgType)
			switch jo.out.Kind {
			case OutputSet:
				g.set(grants, s.runAs, ct, jo.out, props)
			case OutputCreate:
				g.create(grants, s.runAs, jo.out, props)
			}
		}
	}
}

// triggerProps returns the literal props of every trigger that posts
// msgType into channel type ct. A templated value of a trigger move is one
// of these.
func (g grantChecker) triggerProps(ct, msgType string) []map[string]string {
	if msgType == "" {
		return nil
	}
	var out []map[string]string
	for _, name := range sortedKeys(g.c.triggers) {
		t := g.c.triggers[name]
		if t.Channel == ct && t.PostType == msgType {
			out = append(out, t.Props)
		}
	}
	return out
}

// set checks a set output: update and the field and option grants on
// every face the set can write. A face-anchored channel may anchor any
// face of its type, so every declared face is checked. A type without
// faces is written at the zero face, whatever face the set names: the
// face applies only to the attach_to types that declare it.
func (g grantChecker) set(grants acl.JobGrants, runAs string, ct *ChannelType, o Output, props []map[string]string) {
	for _, typ := range ct.AttachTo {
		def, ok := g.v.meta.GetEntityDef(typ)
		if !ok {
			continue
		}
		faces := []entity.Face{entity.Face(o.Face)}
		if ct.Anchor == AnchorFace || len(metamodel.FaceOrderOf(g.v.meta, typ)) == 0 {
			faces = declaredFaces(g.v.meta, typ)
		}
		for _, face := range faces {
			if !grants.Verb(acl.OpUpdate, typ, face) {
				g.v.add(o.key, fmt.Sprintf("%s cannot update %s: %s", runAs, faceAddr(typ, face), noGrant))
				continue
			}
			g.fields(grants, runAs, acl.OpUpdate, typ, face, def, o, props)
		}
	}
}

// create checks a create output: create, the field and option grants at
// the output's face, and the relation to the anchor.
func (g grantChecker) create(grants acl.JobGrants, runAs string, o Output, props []map[string]string) {
	def, ok := g.v.meta.GetEntityDef(o.Type)
	if !ok {
		return
	}
	face := entity.Face(o.Face)
	if !grants.Verb(acl.OpCreate, o.Type, face) {
		g.v.add(o.key, fmt.Sprintf("%s cannot create %s: %s", runAs, faceAddr(o.Type, face), noGrant))
		return
	}
	g.fields(grants, runAs, acl.OpCreate, o.Type, face, def, o, props)
	if o.Relate == "" {
		return
	}
	tail := face
	if rd, ok := g.v.meta.GetRelationDef(o.Relate); !ok || rd.Scope.IsIdentity() {
		tail = ""
	}
	var family []entity.Face
	if tail.IsImplicit() {
		family = declaredFaces(g.v.meta, o.Type)
	}
	if !grants.Relates(o.Relate, o.Type, tail, family) {
		g.v.add(o.key+".relate", fmt.Sprintf("%s cannot create a %s relation from %s: %s", runAs, o.Relate,
			faceAddr(o.Type, face), noGrant))
	}
}

// fields checks the field grant of every property o writes, and the
// option grant of every enum value it can write.
func (g grantChecker) fields(
	grants acl.JobGrants, runAs string, op acl.Op, typ string, face entity.Face, def *metamodel.EntityDef, o Output,
	props []map[string]string,
) {
	for _, name := range o.PropertyNames() {
		pd, ok := def.Properties[name]
		if !ok {
			continue
		}
		pk := o.key + ".properties." + name
		if !grants.FieldWrite(op, typ, face, name) {
			g.v.add(pk, fmt.Sprintf("%s cannot %s %s.%s on %s: %s", runAs, writeVerb(op), typ, name,
				faceAddr(typ, face), noGrant))
			continue
		}
		if len(g.enumValues(pd)) == 0 {
			continue
		}
		for _, value := range templateValues(o.props[name], props) {
			if !grants.Option(op, typ, face, name, value) {
				g.v.add(pk, fmt.Sprintf("%s cannot %s %s.%s to %q on %s: %s", runAs, writeVerb(op), typ, name,
					value, faceAddr(typ, face), noGrant))
			}
		}
	}
}

// enumValues returns the values an enum property declares, inline or
// through a custom type; nil for a property that is not an enum.
func (g grantChecker) enumValues(pd metamodel.PropertyDef) []string {
	if len(pd.Values) > 0 {
		return pd.Values
	}
	if ct, ok := g.v.meta.Types[pd.Type]; ok {
		return ct.Values
	}
	return nil
}

// templateValues returns the enum values t can write: its literal, or the
// literal each trigger posts for the message property it reads.
func templateValues(t Template, props []map[string]string) []string {
	if lit, ok := t.literal(); ok {
		return stringValues(lit)
	}
	name := t.messageRef()
	if name == "" {
		return nil
	}
	var out []string
	for _, p := range props {
		if v, ok := p[name]; ok {
			out = append(out, v)
		}
	}
	return out
}

// stringValues returns a literal's values as strings: one for a scalar,
// one per element for a list.
func stringValues(v any) []string {
	if list, ok := v.([]any); ok {
		out := make([]string, 0, len(list))
		for _, e := range list {
			out = append(out, fmt.Sprint(e))
		}
		return out
	}
	return []string{fmt.Sprint(v)}
}

// writeVerb names op in a problem.
func writeVerb(op acl.Op) string {
	if op == acl.OpCreate {
		return "create"
	}
	return "set"
}

// faceAddr is typ, or typ@face for a named face.
func faceAddr(typ string, face entity.Face) string {
	if face.IsImplicit() {
		return typ
	}
	return typ + entity.StateRefSeparator + string(face)
}

// declaredFaces returns the faces typ declares, or the zero face alone for
// an unfaced type.
func declaredFaces(meta *metamodel.Metamodel, typ string) []entity.Face {
	names := metamodel.FaceOrderOf(meta, typ)
	if len(names) == 0 {
		return []entity.Face{""}
	}
	out := make([]entity.Face, len(names))
	for i, n := range names {
		out[i] = entity.Face(n)
	}
	return out
}
