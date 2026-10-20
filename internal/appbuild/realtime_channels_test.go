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
