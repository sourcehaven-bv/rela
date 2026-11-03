package appbuild_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// TestRealtimeChannelSchema pins the adapter the data-entry `channels:`
// check reads realtime.yaml through, and that realtime off is a nil
// interface.
func TestRealtimeChannelSchema(t *testing.T) {
	require.Nil(t, appbuild.RealtimeChannelSchema(nil))
	var holder *appbuild.RealtimeConfig
	require.Nil(t, holder.ChannelSchema())

	meta, err := metamodel.Parse([]byte(`version: "1.0"
entities:
  doc:
    label: Doc
    id_prefix: "DOC-"
    properties:
      title: {type: string}
`))
	require.NoError(t, err)
	c, err := realtimecfg.Parse([]byte(`version: 1
channel_types:
  room:
    attach_to: [doc]
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: markdown, required: true }, about: { type: ref, to: [doc] } }
        post: [member]
      typing: { persist: ephemeral, post: [member] }
`), meta, nil)
	require.NoError(t, err)
	s := appbuild.RealtimeChannelSchema(c)
	require.Equal(t, []string{"room"}, s.ChannelTypeNames())
}

// TestRealtimeChannelSchema_MessageType pins the message facts the
// `channels:` check reads (TKT-O6RC8Z AC 8): the properties sorted by name,
// and CountsUnread false for a type with `unread: false` and for a signal
// type, which is never stored; false for an unknown channel or message
// type. A hidden signal type gets no hidden-but-counting warning, because
// realtime.yaml refuses `unread:` on it.
func TestRealtimeChannelSchema_MessageType(t *testing.T) {
	meta, err := metamodel.Parse([]byte(`version: "1.0"
entities:
  doc:
    label: Doc
    id_prefix: "DOC-"
    properties:
      title: {type: string}
`))
	require.NoError(t, err)
	c, err := realtimecfg.Parse([]byte(`version: 1
channel_types:
  room:
    attach_to: [doc]
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      text:
        properties: { body: { type: markdown, required: true }, about: { type: ref, to: [doc] } }
        post: [member]
      claim: { post: [member], unread: false }
      typing: { persist: ephemeral, post: [member] }
`), meta, nil)
	require.NoError(t, err)
	s := appbuild.RealtimeChannelSchema(c)
	text, ok := s.MessageType("room", "text")
	require.True(t, ok)
	require.Equal(t, dataentryconfig.MessageFacts{
		Properties: []dataentryconfig.ChannelProperty{
			{Name: "about", Type: "ref"}, {Name: "body", Type: "markdown", Required: true},
		},
		CountsUnread: true,
	}, text)
	claim, ok := s.MessageType("room", "claim")
	require.True(t, ok)
	require.False(t, claim.CountsUnread)
	require.Empty(t, claim.Properties)
	typing, ok := s.MessageType("room", "typing")
	require.True(t, ok, "a signal type is declared")
	require.False(t, typing.CountsUnread, "a signal is never stored, so it never counts")
	errs, warnings := dataentryconfig.ValidateChannels(&dataentryconfig.Config{
		Channels: map[string]dataentryconfig.ChannelPresentation{"room": {
			Messages: map[string]dataentryconfig.MessagePresentation{
				"typing": {Display: dataentryconfig.MessageDisplayHidden},
			},
		}},
	}, s)
	require.Empty(t, errs)
	require.Empty(t, warnings, "no advice that realtime.yaml would refuse")
	_, ok = s.MessageType("room", "poll")
	require.False(t, ok)
	_, ok = s.MessageType("hall", "text")
	require.False(t, ok)
}

// TestRealtimeChannelSchema_BusyFacts pins what a kanban busy field is
// checked against (TKT-21G6JS): the attached types, whether a session
// exists, and whether the initial state reads working only by default.
func TestRealtimeChannelSchema_BusyFacts(t *testing.T) {
	meta, err := metamodel.Parse([]byte(`version: "1.0"
entities:
  doc:
    label: Doc
    id_prefix: "DOC-"
    properties:
      title: {type: string}
      owner: {type: string}
`))
	require.NoError(t, err)
	c, err := realtimecfg.Parse([]byte(`version: 1
channel_types:
  room:
    attach_to: [doc]
    read: { from: attached }
    roles: &roles
      owner: { from: entity.owner }
    message_types: &types
      go: { post: [owner] }
  run:
    attach_to: [doc]
    read: { from: attached }
    roles: *roles
    message_types: *types
    session:
      initial: queued
      states:
        queued:  { waiting_for: [owner], on: { go: { to: working } } }
        working: { status: working, on: { go: { to: queued } } }
  declared:
    attach_to: [doc]
    read: { from: attached }
    roles: *roles
    message_types: *types
    session:
      initial: working
      states:
        working: { status: working, on: { go: { to: done } } }
        done:    { final: true }
  defaulted:
    attach_to: [doc]
    read: { from: attached }
    roles: *roles
    message_types: *types
    session:
      initial: working
      states:
        working: { on: { go: { to: done } } }
        done:    { final: true }
  idle:
    attach_to: [doc]
    read: { from: attached }
    roles: *roles
    message_types: *types
    session:
      initial: queued
      states:
        queued: { waiting_for: [owner], on: { go: { to: done } } }
        done:   { final: true }
`), meta, nil)
	require.NoError(t, err)
	s := appbuild.RealtimeChannelSchema(c)
	cases := []struct {
		typ  string
		want dataentryconfig.BusyFacts
		ok   bool
	}{
		{typ: "nope"},
		{typ: "room", ok: true, want: dataentryconfig.BusyFacts{AttachTo: []string{"doc"}}},
		{typ: "run", ok: true, want: dataentryconfig.BusyFacts{
			AttachTo: []string{"doc"}, HasSession: true, HasWorkingState: true,
		}},
		{typ: "declared", ok: true, want: dataentryconfig.BusyFacts{
			AttachTo: []string{"doc"}, HasSession: true, HasWorkingState: true,
		}},
		{typ: "defaulted", ok: true, want: dataentryconfig.BusyFacts{
			AttachTo: []string{"doc"}, HasSession: true, HasWorkingState: true, InitialDefaultWorking: true,
		}},
		{typ: "idle", ok: true, want: dataentryconfig.BusyFacts{AttachTo: []string{"doc"}, HasSession: true}},
	}
	for _, tc := range cases {
		t.Run(tc.typ, func(t *testing.T) {
			got, ok := s.BusyFacts(tc.typ)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.want, got)
		})
	}
}
