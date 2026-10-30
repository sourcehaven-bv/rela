package appbuild

import (
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// ChannelSchema returns the realtime configuration in force as the
// data-entry `channels:` check sees it. It returns nil when realtime is
// off, which [dataentryconfig.ValidateChannels] reports as a warning.
//
// Nil: r is accepted and returns nil, as for every method.
func (r *RealtimeConfig) ChannelSchema() dataentryconfig.ChannelSchema {
	return RealtimeChannelSchema(r.Load().Config)
}

// RealtimeChannelSchema adapts a compiled realtime.yaml to
// [dataentryconfig.ChannelSchema], the way [ValidateACLPolicy] adapts the
// metamodel for acl.yaml.
//
// Nil: c is accepted and means realtime off; the result is then a nil
// interface, not a typed nil.
func RealtimeChannelSchema(c *realtimecfg.Compiled) dataentryconfig.ChannelSchema {
	if c == nil {
		return nil
	}
	return realtimeChannelSchema{c: c}
}

type realtimeChannelSchema struct {
	c *realtimecfg.Compiled
}

func (s realtimeChannelSchema) ChannelTypeNames() []string {
	cts := s.c.ChannelTypes()
	out := make([]string, 0, len(cts))
	for _, ct := range cts {
		out = append(out, ct.Name)
	}
	return out
}

// MessageType answers the message type's properties, sorted by name, and
// whether its messages count toward unread: a stored type without
// `unread: false`. A signal type never counts, since it is never stored.
func (s realtimeChannelSchema) MessageType(channelType, messageType string) (dataentryconfig.MessageFacts, bool) {
	ct, ok := s.c.ChannelType(channelType)
	if !ok {
		return dataentryconfig.MessageFacts{}, false
	}
	mt := ct.MessageTypes[messageType]
	if mt == nil {
		return dataentryconfig.MessageFacts{}, false
	}
	props := make([]dataentryconfig.ChannelProperty, 0, len(mt.Properties))
	for _, p := range mt.Properties {
		props = append(props, dataentryconfig.ChannelProperty{Name: p.Name, Type: string(p.Type), Required: p.Required})
	}
	sort.Slice(props, func(i, j int) bool { return props[i].Name < props[j].Name })
	counts := mt.Persist == realtimecfg.PersistLog && !mt.NoUnread
	return dataentryconfig.MessageFacts{Properties: props, CountsUnread: counts}, true
}

func (s realtimeChannelSchema) BusyFacts(channelType string) (dataentryconfig.BusyFacts, bool) {
	ct, ok := s.c.ChannelType(channelType)
	if !ok {
		return dataentryconfig.BusyFacts{}, false
	}
	facts := dataentryconfig.BusyFacts{AttachTo: append([]string(nil), ct.AttachTo...)}
	sess := ct.Session()
	if sess == nil {
		return facts, true
	}
	facts.HasSession = true
	facts.HasWorkingState = len(sess.WorkingStates()) > 0
	if st, ok := sess.State(sess.Initial()); ok {
		facts.InitialDefaultWorking = st.Status == realtimecfg.StatusWorking && !st.StatusDeclared()
	}
	return facts, true
}
