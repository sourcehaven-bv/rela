package dataentry

import (
	"net/http"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// channelDescriber describes the channel types in force.
type channelDescriber interface {
	Describe() []realtime.ChannelTypeView
}

// serveChannelSchema answers `GET /api/v1/_schema/channels`. The answer is
// the same for every principal: the configuration is not a secret. With
// channels not wired, or realtime off, it lists no channel type, so the
// SPA can ask before it knows whether realtime is on.
func serveChannelSchema(w http.ResponseWriter, r *http.Request, d channelDescriber) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	out := v1.ChannelSchema{ChannelTypes: []v1.ChannelTypeSchema{}}
	if d != nil {
		for _, ct := range d.Describe() {
			out.ChannelTypes = append(out.ChannelTypes, wireChannelTypeSchema(ct))
		}
	}
	writeV1JSON(w, http.StatusOK, out)
}

func wireChannelTypeSchema(ct realtime.ChannelTypeView) v1.ChannelTypeSchema {
	out := v1.ChannelTypeSchema{
		Name: ct.Name, Label: ct.Label, AttachTo: ct.AttachTo, Anchor: ct.Anchor,
		Cardinality: ct.Cardinality, CreateLabel: ct.CreateLabel,
		RemoveReasonRequired: ct.RemoveReasonRequired, Typing: ct.Typing,
		Lifecycle:    make([]v1.ChannelLifecycleSchema, 0, len(ct.Lifecycle)),
		MessageTypes: make([]v1.ChannelMessageTypeSchema, 0, len(ct.MessageTypes)),
	}
	if out.AttachTo == nil {
		out.AttachTo = []string{}
	}
	for _, op := range ct.Lifecycle {
		out.Lifecycle = append(out.Lifecycle, v1.ChannelLifecycleSchema{Name: op.Name, Label: op.Label})
	}
	for _, mt := range ct.MessageTypes {
		out.MessageTypes = append(out.MessageTypes, wireMessageTypeSchema(mt))
	}
	out.Session = wireSessionSchema(ct.Session)
	return out
}

func wireSessionSchema(s *realtime.SessionDescription) *v1.ChannelSessionSchema {
	if s == nil {
		return nil
	}
	out := &v1.ChannelSessionSchema{
		Label: s.Label, Initial: s.Initial, States: make([]v1.ChannelSessionStateSchema, 0, len(s.States)),
	}
	for _, st := range s.States {
		out.States = append(out.States, v1.ChannelSessionStateSchema{
			Name: st.Name, Label: st.Label, Final: st.Final, Status: st.Status, WaitingFor: st.WaitingFor,
		})
	}
	return out
}

func wireMessageTypeSchema(mt realtime.MessageTypeDescription) v1.ChannelMessageTypeSchema {
	out := v1.ChannelMessageTypeSchema{
		Name: mt.Name, Label: mt.Label, Persist: mt.Persist, TTLSeconds: mt.TTLSeconds, Reply: mt.Reply,
		Properties: make([]v1.ChannelPropertySchema, 0, len(mt.Properties)),
		Actions:    make([]v1.ChannelActionSchema, 0, len(mt.Actions)),
	}
	for _, p := range mt.Properties {
		out.Properties = append(out.Properties, v1.ChannelPropertySchema{
			Name: p.Name, Label: p.Label, Type: p.Type, Required: p.Required, Max: p.Max,
			Enum: p.Enum, Values: p.Values, To: p.To,
		})
	}
	for _, a := range mt.Actions {
		out.Actions = append(out.Actions, v1.ChannelActionSchema{Name: a.Name, Label: a.Label, PromoteTo: a.PromoteTo})
	}
	return out
}
