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
	require.Equal(t, []string{"text", "typing"}, s.MessageTypeNames("room"))
	require.Nil(t, s.MessageTypeNames("hall"))
	require.Equal(t, []dataentryconfig.ChannelProperty{
		{Name: "about", Type: "ref"}, {Name: "body", Type: "markdown", Required: true},
	}, s.MessageProperties("room", "text"))
	require.Nil(t, s.MessageProperties("room", "poll"))
	require.Nil(t, s.MessageProperties("hall", "text"))
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
