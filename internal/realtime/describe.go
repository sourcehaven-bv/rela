package realtime

import (
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// ChannelTypeView describes a channel type of realtime.yaml to a client
// that renders channels: its message types and their properties, the
// labels of its operations and the remove reason rule. It carries no gate,
// role or condition; the affordances on each channel say what a caller may
// do. The configuration is not a secret, so it is the same for everyone.
//
// It holds plain values, not realtimecfg types, so a change to the parsed
// configuration does not ripple into the HTTP mapping. Every Label is the
// authored label or, when none is authored, the name; it is never empty.
// CreateLabel is the one exception.
type ChannelTypeView struct {
	Name     string
	Label    string
	AttachTo []string
	// Anchor is "family" or "face"; Cardinality is "one" or "many".
	Anchor      string
	Cardinality string
	// CreateLabel is create.label; empty for a cardinality one type and
	// when unset.
	CreateLabel          string
	RemoveReasonRequired bool
	// Typing reports whether the type offers the built-in typing indicator.
	Typing bool
	// Lifecycle lists the declared lifecycle operations in the order of
	// [realtimecfg.LifecycleOpNames].
	Lifecycle    []LabeledName
	MessageTypes []MessageTypeDescription
	// Session describes the type's session; nil when it has none.
	Session *SessionDescription
}

// SessionDescription describes a channel type's session: its label, its
// initial state and its states in declaration order. It holds no
// transitions: the transitions out of a channel's current state come with
// the channel ([SessionView]), so a stale descriptor cannot disagree with
// the channel about where the session can go.
type SessionDescription struct {
	Label   string
	Initial string
	States  []SessionStateDescription
}

// SessionStateDescription describes one state of a session. Final is set
// for a state no transition leaves. Status is the state's
// [realtimecfg.SessionStatus] and WaitingFor names its waiting roles; both
// are config, the same for every reader.
type SessionStateDescription struct {
	Name       string
	Label      string
	Final      bool
	Status     string
	WaitingFor []string
}

// LabeledName is a name of realtime.yaml and its label.
type LabeledName struct {
	Name  string
	Label string
}

// MessageTypeDescription describes one message type. Persist is "log" or
// "ephemeral". TTLSeconds and Reply follow the rule of [MessageTypeView].
type MessageTypeDescription struct {
	Name       string
	Label      string
	Persist    string
	TTLSeconds int
	Reply      bool
	// Properties are sorted with the required ones first, then by name:
	// realtime.yaml declares them in a map, which has no order.
	Properties []PropertyDescription
	// Actions are sorted by name. PromoteTo is the entity type a promoting
	// action creates, empty otherwise.
	Actions []ActionDescription
	// Choices is the type's `choices:`; nil when it has none.
	Choices *ChoicesDescription
}

// ChoicesDescription describes `choices:` on a question type: the list
// property From holds the options, a click posts message type Answer with
// the option in its property Into, and Other adds a free-text button. See
// [realtimecfg.Choices].
type ChoicesDescription struct {
	From   string `json:"from"`
	Answer string `json:"answer"`
	Into   string `json:"into"`
	Other  bool   `json:"other"`
}

// describeChoices describes c; nil for nil.
func describeChoices(c *realtimecfg.Choices) *ChoicesDescription {
	if c == nil {
		return nil
	}
	return &ChoicesDescription{From: c.From, Answer: c.Answer, Into: c.Into, Other: c.Other}
}

// PropertyDescription describes one message property. Type is string,
// markdown, integer, boolean, date, datetime, ref or enum. Max is the most
// code points a string or markdown value may hold, 0 for no limit. Enum
// names the schema.yaml enum type and Values its values; To lists the
// entity types a ref may name. List is set for a list of strings; Max then
// limits each item and MaxItems the number of items.
type PropertyDescription struct {
	Name     string
	Label    string
	Type     string
	Required bool
	Max      int
	Enum     string
	Values   []string
	To       []string
	List     bool
	MaxItems int
}

// ActionDescription describes one message action.
type ActionDescription struct {
	Name      string
	Label     string
	PromoteTo string
}

// Describe describes every channel type in force now, sorted by name. It
// reads one config snapshot; with realtime off it returns an empty list.
func (s *Service) Describe() []ChannelTypeView {
	cfg := s.cfg.Load().Config
	cts := cfg.ChannelTypes()
	out := make([]ChannelTypeView, 0, len(cts))
	for _, ct := range cts {
		out = append(out, describeChannelType(ct))
	}
	return out
}

func describeChannelType(ct realtimecfg.ChannelType) ChannelTypeView {
	v := ChannelTypeView{
		Name: ct.Name, Label: ct.Label, AttachTo: slices.Clone(ct.AttachTo), Anchor: string(ct.Anchor),
		Cardinality: string(ct.Cardinality), Typing: ct.Typing, Lifecycle: []LabeledName{},
		MessageTypes: []MessageTypeDescription{},
	}
	if ct.Create != nil {
		v.CreateLabel = ct.Create.Label
	}
	if ct.MessageOps.Remove != nil {
		v.RemoveReasonRequired = ct.MessageOps.Remove.ReasonRequired
	}
	for _, name := range realtimecfg.LifecycleOpNames() {
		if op := ct.Lifecycle.Op(name); op != nil {
			v.Lifecycle = append(v.Lifecycle, LabeledName{Name: name, Label: op.Label})
		}
	}
	for _, name := range sortedKeys(ct.MessageTypes) {
		v.MessageTypes = append(v.MessageTypes, describeMessageType(ct.MessageTypes[name]))
	}
	v.Session = describeSession(ct.Session())
	return v
}

func describeSession(s *realtimecfg.Session) *SessionDescription {
	if s == nil {
		return nil
	}
	states := s.States()
	d := &SessionDescription{
		Label: s.Label(), Initial: s.Initial(), States: make([]SessionStateDescription, 0, len(states)),
	}
	for _, st := range states {
		d.States = append(d.States, SessionStateDescription{
			Name: st.Name, Label: st.Label, Final: st.Final, Status: string(st.Status), WaitingFor: st.WaitingFor(),
		})
	}
	return d
}

func describeMessageType(mt *realtimecfg.MessageType) MessageTypeDescription {
	ttl, reply := messageTypeTiming(mt)
	d := MessageTypeDescription{
		Name: mt.Name, Label: mt.Label, Persist: string(mt.Persist), TTLSeconds: ttl, Reply: reply,
		Properties: make([]PropertyDescription, 0, len(mt.Properties)),
		Actions:    []ActionDescription{},
		Choices:    describeChoices(mt.Choices),
	}
	for _, p := range mt.Properties {
		d.Properties = append(d.Properties, PropertyDescription{
			Name: p.Name, Label: p.Label, Type: string(p.Type), Required: p.Required, Max: p.Max,
			Enum: p.Enum, Values: slices.Clone(p.Values), To: slices.Clone(p.To),
			List: p.List, MaxItems: p.MaxItems,
		})
	}
	slices.SortFunc(d.Properties, func(a, b PropertyDescription) int {
		switch {
		case a.Required && !b.Required:
			return -1
		case !a.Required && b.Required:
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	for _, name := range sortedKeys(mt.Actions) {
		a := mt.Actions[name]
		ad := ActionDescription{Name: a.Name, Label: a.Label}
		if a.PromoteTo != nil {
			ad.PromoteTo = a.PromoteTo.Type
		}
		d.Actions = append(d.Actions, ad)
	}
	return d
}

// messageTypeTiming is how long a signal of mt lives in seconds, 0 for a
// log type, and whether a message of mt may reply to another. Read's
// message type descriptions and Describe both take them from here.
func messageTypeTiming(mt *realtimecfg.MessageType) (ttlSeconds int, reply bool) {
	if mt.Persist == realtimecfg.PersistEphemeral {
		return int(signalTTL(mt).Seconds()), false
	}
	return 0, true
}
