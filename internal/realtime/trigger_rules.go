package realtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"slices"
	"strconv"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// rule is one trigger, or one `auto.when` for one attach_to type, as a
// round evaluates it. Both share the seen-set, the queues and the post
// budget; they differ in what they do on their edge.
type rule struct {
	id string
	// name names the rule in logs: the trigger's name, or
	// <channel type>.<op>.auto for an auto rule.
	name       string
	entityType string
	when       realtimecfg.Rule
	// on is the edge the rule acts on. An auto rule acts on enter only, so
	// a person who reopens a channel while the rule still holds is not
	// overruled.
	on    realtimecfg.TriggerOn
	ct    realtimecfg.ChannelType
	def   *metamodel.EntityDef
	faces FaceMode

	// trigger is set for a trigger; mt and props are what it posts.
	trigger *realtimecfg.Trigger
	mt      *realtimecfg.MessageType
	props   map[string]any

	// auto is set for an auto rule: the operation it runs, the state it
	// moves to and the states it moves from.
	auto *autoRule
}

// autoRule is what an `auto.when` rule does on enter.
type autoRule struct {
	op   string
	to   channels.State
	from []channels.State
}

// triggeredBy is the label a rule's writes carry. An auto rule shares the
// label of the operation's `auto.after`: both are the operation's auto
// step.
func (r *rule) triggeredBy() string {
	if r.auto != nil {
		return autoLabel(r.ct.Name, r.auto.op)
	}
	return "trigger:" + r.name
}

// rulesFor returns the triggers of snap sorted by name, then the auto
// rules sorted by channel type, operation and entity type. The order puts
// a trigger's post before an auto rule closes the channel in the same
// round. It is empty when realtime is off or degraded: the engine fails
// closed.
func rulesFor(snap Snapshot, meta *metamodel.Metamodel) []rule {
	if snap.Config == nil || snap.Degraded {
		return nil
	}
	var out []rule
	for _, tr := range snap.Config.Triggers() {
		ct, ok := snap.Config.ChannelType(tr.Channel)
		if !ok || tr.When.Program == nil {
			continue
		}
		mt := ct.MessageTypes[tr.PostType]
		def, ok := meta.GetEntityDef(tr.EntityType)
		if mt == nil || !ok {
			continue
		}
		props := make(map[string]any, len(tr.Props))
		for k, v := range tr.Props {
			props[k] = v
		}
		faces := facesOf(ct)
		out = append(out, rule{
			id: ruleID(tr, faces), name: tr.Name, entityType: tr.EntityType, when: tr.When, on: tr.On,
			ct: ct, def: def, faces: faces, trigger: &tr, mt: mt, props: props,
		})
	}
	for _, ct := range snap.Config.ChannelTypes() {
		for _, name := range []string{realtimecfg.LifecycleClose, realtimecfg.LifecycleArchive} {
			op := ct.Lifecycle.Op(name)
			if op == nil || op.Auto == nil {
				continue
			}
			for _, typ := range sortedKeys(op.Auto.When) {
				when := op.Auto.When[typ]
				def, ok := meta.GetEntityDef(typ)
				if !ok || when.Program == nil {
					continue
				}
				faces := facesOf(ct)
				out = append(out, rule{
					id: autoRuleID(ct.Name, name, typ, when, faces), name: ct.Name + "." + name + ".auto",
					entityType: typ, when: when, on: realtimecfg.TriggerEnter, ct: ct, def: def, faces: faces,
					auto: &autoRule{op: name, to: lifecycleTarget(name), from: opFrom(name, op)},
				})
			}
		}
	}
	return out
}

// facesOf is the face mode a rule on channel type ct reads: the face its
// channel hangs off, or the default world's prime face for a channel on
// the whole family.
func facesOf(ct realtimecfg.ChannelType) FaceMode {
	if ct.Anchor == realtimecfg.AnchorFace {
		return FaceEach
	}
	return FacePrime
}

// ruleID identifies a trigger's seen-set. It changes with the entity type,
// the rule and the faces read, so an edit that changes what matches starts
// a new seen-set (and a silent baseline) instead of posting a burst.
func ruleID(tr realtimecfg.Trigger, faces FaceMode) string {
	sum := sha256.Sum256([]byte("trigger\x00" + tr.EntityType + "\x00" + tr.When.Source + "\x00" +
		strconv.Itoa(int(faces))))
	return "t-" + tr.Name + "-" + hex.EncodeToString(sum[:8])
}

// autoRuleID identifies an auto rule's seen-set, as [ruleID] does a
// trigger's. A new or edited `auto.when` therefore starts from now: what
// already matches is recorded, and no channel is moved for it.
func autoRuleID(channelType, op, entityType string, when realtimecfg.Rule, faces FaceMode) string {
	sum := sha256.Sum256([]byte("auto\x00" + channelType + "\x00" + op + "\x00" + entityType + "\x00" +
		when.Source + "\x00" + strconv.Itoa(int(faces))))
	return "a-" + channelType + "-" + op + "-" + hex.EncodeToString(sum[:8])
}

// ref is the seen-set key of row under r: the id, or id@face for a rule
// that reads each face.
func (r *rule) ref(row EntityRow) string {
	if r.faces == FaceEach {
		return entity.FormatStateRef(row.ID, row.Face)
	}
	return row.ID
}

// anchor is the anchor of the channels r acts on for the entity ref.
func (r *rule) anchor(ref string) channels.Anchor {
	if r.faces == FaceEach {
		if id, face, err := entity.ParseStateRef(ref); err == nil {
			return channels.Anchor{ID: id, Face: face}
		}
	}
	return channels.Anchor{ID: ref}
}

// key is the channel a trigger posts into for the entity ref.
func (r *rule) key(ref string) channels.Key {
	return channels.Key{Anchor: r.anchor(ref), Type: r.ct.Name}
}

// idOf is the entity id of a seen-set key.
func idOf(ref string) string {
	id, _, err := entity.ParseStateRef(ref)
	if err != nil {
		return ref
	}
	return id
}

// eval reports whether row matches r and the fingerprint of what r read.
// b holds the round's functions; eval binds the entity into it. ok is
// false when the answer is unknown: a property r reads cannot be read, or
// the rule fails to evaluate. The seen-set then stays as it was, so a
// broken rule never posts leave.
//
// An auto rule acts on enter only and so needs no fingerprint: its entries
// are empty, and a change to a matching entity rewrites no shard.
func (r *rule) eval(ctx context.Context, meta *metamodel.Metamodel, b *predicate.Bindings, row EntityRow,
) (match bool, fp string, ok bool) {
	for _, f := range row.Inaccessible {
		if slices.Contains(r.when.Fields, f) {
			return false, "", false
		}
	}
	err := b.SetVar("entity", predicatefns.EntityRecord(meta, r.def, row.ID, r.entityType, row.Props))
	var v predicate.Value
	if err == nil {
		v, err = r.when.Program.Eval(ctx, b)
	}
	if err != nil {
		slog.DebugContext(ctx, "realtime: trigger rule failed to evaluate; the entity is left as it was",
			"rule", r.name, "entity", row.ID, "error", err)
		return false, "", false
	}
	got, isBool := v.(predicate.Bool)
	if !isBool {
		return false, "", false
	}
	if r.auto != nil {
		return got.Bool(), "", true
	}
	return got.Bool(), fingerprint(r.when.Fields, row.Props), true
}
