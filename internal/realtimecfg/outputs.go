package realtimecfg

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// OutputKind is what a session output writes.
type OutputKind string

const (
	// OutputSet patches properties of the channel's anchor entity.
	OutputSet OutputKind = "set"
	// OutputCreate creates one entity, optionally related to the anchor.
	OutputCreate OutputKind = "create"
)

// OutputsActionName is the action name of the outputs mark on a session
// notice. A message type of a channel type with a session may not declare
// an action of that name. It equals channels.OutputsAction.
const OutputsActionName = "outputs"

// OnFinalClose is the one value of `session.on_final`: a move into a final
// state closes the channel.
const OnFinalClose = "close"

// Output is one entry of a transition's `outputs:` list. It is immutable:
// its properties are unexported and [Output.Properties] returns a copy.
type Output struct {
	Kind OutputKind
	// Type is the entity type a create makes; empty for a set.
	Type string
	// Face is, for a create, the face the new entity is created at, set
	// exactly when Type declares faces. For a set it is the face written
	// on a family-anchored channel type whose attach_to types declare
	// faces; empty otherwise, when the set writes the channel's own anchor
	// face (a face-anchored type) or the zero face (an unfaced type).
	Face string
	// Relate is the relation type a create links the new entity to the
	// anchor with, or empty.
	Relate string
	props  map[string]Template
	// key is the output's key path in realtime.yaml, for the grant check's
	// problems.
	key string
}

// Properties returns the output's property templates by entity property
// name. The map is a copy.
func (o Output) Properties() map[string]Template { return maps.Clone(o.props) }

// PropertyNames returns the names of the output's properties, sorted.
func (o Output) PropertyNames() []string { return sortedKeys(o.props) }

// Outputs returns the transition's outputs in file order. The slice is a
// copy.
func (t Transition) Outputs() []Output { return slices.Clone(t.outputs) }

// Job reports whether a trigger can fire t, so its outputs write as the
// session's run_as when a trigger's post fires it.
func (t Transition) Job() bool { return t.job }

// Outputs returns the timer's outputs in file order. The slice is a copy.
// They write as [Session.RunAs].
func (t SessionTimer) Outputs() []Output { return slices.Clone(t.outputs) }

// RunAs is `session.run_as`: the job identity that writes the outputs of
// timer moves and of moves a trigger fires, or empty.
func (s *Session) RunAs() string { return s.runAs }

// OnFinal is `session.on_final`: [OnFinalClose], or empty.
func (s *Session) OnFinal() string { return s.onFinal }

// writeChecker checks what realtime.yaml writes to the graph on a
// validator's behalf: session outputs, their templates, and the entity type
// a promote or a create output makes. It keeps those checks off the
// validator, which is at its method budget.
type writeChecker struct{ *validator }

// outputs compiles the `outputs:` of a transition or a timer. c is the
// cause of the move; a message cause with an unusable message type
// compiles nothing.
func (v writeChecker) outputs(key string, raw []rawOutput, c outputCause, ct *ChannelType) []Output {
	if len(raw) == 0 {
		return nil
	}
	if len(raw) > realtimelimits.MaxTransitionOutputs {
		v.add(key, fmt.Sprintf("at most %d outputs per transition or timer", realtimelimits.MaxTransitionOutputs))
	}
	if c.kind != causeTimer && c.mt == nil {
		return nil
	}
	var out []Output
	sets := 0
	for i := range raw {
		ok := fmt.Sprintf("%s[%d]", key, i)
		r := raw[i]
		switch {
		case r.Set != nil && r.Create != nil:
			v.add(ok, "an output is either set or create, not both")
		case r.Set != nil:
			sets++
			if sets > 1 {
				v.add(ok, "at most one set per transition; put every property in one set")
				continue
			}
			out = append(out, v.setOutput(ok+".set", r.Set, c, ct))
		case r.Create != nil:
			out = append(out, v.createOutput(ok+".create", r.Create, c, ct))
		default:
			v.add(ok, "an output names set or create")
		}
	}
	return out
}

// setOutput compiles a `set:` output, which patches the channel's anchor.
// Its templates are compiled against the first attach_to type, so a
// property must have one type across all of them: a value is shaped for
// its property type.
func (v writeChecker) setOutput(key string, raw *rawSetOutput, c outputCause, ct *ChannelType) Output {
	o := Output{Kind: OutputSet, Face: raw.Face, key: key}
	v.setFace(key+".face", raw.Face, ct)
	first := map[string]string{}
	for _, typ := range ct.AttachTo {
		def, ok := v.meta.GetEntityDef(typ)
		if !ok {
			continue
		}
		props := v.outputProps(key+".properties", raw.Properties, typ, def, c)
		if o.props == nil {
			o.props = props
		}
		for _, name := range sortedKeys(raw.Properties) {
			pd, has := def.Properties[name]
			if !has {
				continue
			}
			was, seen := first[name]
			switch {
			case !seen:
				first[name] = typ
			case v.meta.Entities[was].Properties[name].Type != pd.Type:
				v.add(key+".properties."+name, fmt.Sprintf("%s.%s is %s and %s.%s is %s; a set writes one "+
					"value to every attach_to type, so the property needs one type", was, name,
					v.meta.Entities[was].Properties[name].Type, typ, name, pd.Type))
			}
		}
	}
	if len(raw.Properties) == 0 {
		v.add(key+".properties", "name at least one property to set")
	}
	return o
}

// setFace checks a set's face against the channel type's anchor. A set
// never picks a face by rank.
func (v writeChecker) setFace(key, face string, ct *ChannelType) {
	var faced []string
	for _, typ := range ct.AttachTo {
		faces := metamodel.FaceOrderOf(v.meta, typ)
		if len(faces) == 0 {
			continue
		}
		faced = append(faced, typ)
		if face != "" && ct.Anchor != AnchorFace && !slices.Contains(faces, face) {
			v.add(key, fmt.Sprintf("%s has no face %q; use one of %s", typ, face, strings.Join(faces, ", ")))
		}
	}
	switch {
	case ct.Anchor == AnchorFace && face != "":
		v.add(key, "the channel is anchored to one face; set writes that face")
	case ct.Anchor != AnchorFace && face == "" && len(faced) > 0:
		v.add(key, fmt.Sprintf("%s declares faces and the channel is anchored to the whole entity; "+
			"name the face to set", strings.Join(faced, ", ")))
	case face != "" && len(faced) == 0:
		v.add(key, strings.Join(ct.AttachTo, ", ")+" declares no faces")
	}
}

// createOutput compiles a `create:` output. Its type, face, required
// property and relate checks are the promote_to checks.
func (v writeChecker) createOutput(key string, raw *rawCreateOutput, c outputCause, ct *ChannelType) Output {
	o := Output{Kind: OutputCreate, Type: raw.Type, Face: raw.Face, key: key}
	if raw.Relate != nil {
		o.Relate = raw.Relate.Type
	}
	if len(raw.Properties) == 0 {
		v.add(key+".properties", "name at least one property of the new entity")
	}
	def := v.entityTarget(key, raw.Type, raw.Face, raw.Relate, ct, "an output")
	if def == nil {
		return o
	}
	o.props = v.outputProps(key+".properties", raw.Properties, raw.Type, def, c)
	for _, ep := range requiredProps(def) {
		rv, set := raw.Properties[ep]
		if !set {
			v.add(key+".properties", fmt.Sprintf("%s requires property %q; set it", raw.Type, ep))
			continue
		}
		s, _ := rv.(string)
		if parts, _ := splitTemplate(s); len(parts) == 1 && parts[0].ref != "" && c.mt != nil {
			if p, ok := c.mt.Properties[strings.TrimPrefix(parts[0].ref, msgPrefix)]; ok && !p.Required {
				v.add(key+".properties."+ep, fmt.Sprintf("%s requires property %q, but message property %s is "+
					"optional; mark it required", raw.Type, ep, p.Name))
			}
		}
	}
	return o
}

// outputProps compiles the property templates of an output against
// entity type typ. A property with a problem is left out.
func (v writeChecker) outputProps(
	key string, raw map[string]any, typ string, def *metamodel.EntityDef, c outputCause,
) map[string]Template {
	out := make(map[string]Template, len(raw))
	for _, name := range sortedKeys(raw) {
		pk := key + "." + name
		pd, ok := def.Properties[name]
		switch {
		case !ok:
			v.add(pk, fmt.Sprintf("%s has no property %q", typ, name))
			continue
		case pd.Computed != "":
			v.add(pk, fmt.Sprintf("%s.%s is computed; it cannot be set", typ, name))
			continue
		case pd.Type == metamodel.PropertyTypeFile:
			v.add(pk, fmt.Sprintf("%s.%s is a file property; an output cannot attach a file", typ, name))
			continue
		case pd.Type == metamodel.PropertyTypeExternalRef:
			v.add(pk, fmt.Sprintf("%s.%s is an external_ref property; only an integration writes it", typ, name))
			continue
		}
		t, reason := v.compileTemplate(raw[name], templateTarget{entityType: typ, name: name, def: pd}, c)
		if reason != "" {
			v.add(pk, reason)
			continue
		}
		out[name] = t
	}
	return out
}

// transitionCause classifies a transition: a job message when only a
// trigger fires it (by: trigger) or a trigger can (no roles, and a trigger
// posts its message type into ct), else a person.
func transitionCause(file *rawFile, ct string, t Transition, mt *MessageType) outputCause {
	c := outputCause{kind: causePerson, mt: mt}
	if t.By == TransitionByTrigger || (len(t.Roles) == 0 && triggerReachable(file, ct, t.MessageType) != "") {
		c.kind = causeJobMessage
	}
	return c
}

// triggerReachable reports the trigger that posts message type msgType
// into channel type ct, or "" when none does.
func triggerReachable(file *rawFile, ct, msgType string) string {
	for _, name := range sortedKeys(file.Triggers) {
		tr := file.Triggers[name]
		if tr.Channel == ct && tr.Post.Type == msgType {
			return name
		}
	}
	return ""
}
