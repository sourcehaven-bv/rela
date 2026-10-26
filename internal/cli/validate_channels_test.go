package cli

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/projectsetup"
)

// TestValidate_ChannelsAgainstRealtime pins the `rela validate` check of
// data-entry.yaml's channels: section against realtime.yaml, and that a
// realtime.yaml that does not load is a failed check, not an abort.
func TestValidate_ChannelsAgainstRealtime(t *testing.T) {
	root := seedProject(t)
	check := func() bool {
		t.Helper()
		result, err := projectsetup.Validate(context.Background(), root)
		require.NoError(t, err)
		require.NotNil(t, result.Metamodel)
		result.DataEntryValid = true
		return reportRealtimeValidation(context.Background(), result, false)
	}
	// No channels: section: nothing to check.
	writeProjectFile(t, root, "data-entry.yaml", "version: \"1.0\"\n")
	require.False(t, check())

	section := "version: \"1.0\"\nchannels:\n  room:\n    messages:\n      text:\n        fields: [{ property: %s }]\n"
	writeProjectFile(t, root, "data-entry.yaml", fmt.Sprintf(section, "body"))
	require.False(t, check(), "realtime off is a warning")

	writeProjectFile(t, root, "realtime.yaml", stdioRealtimeYAML)
	require.False(t, check())

	writeProjectFile(t, root, "data-entry.yaml", fmt.Sprintf(section, "nope"))
	require.True(t, check())

	writeProjectFile(t, root, "data-entry.yaml", fmt.Sprintf(section, "body"))
	writeProjectFile(t, root, "realtime.yaml", "version: 1\nchannel_types: {room: {bogus: 1}}\n")
	require.True(t, check(), "a broken realtime.yaml is a failed check")

	require.True(t, reportRealtimeValidation(context.Background(), &projectsetup.ValidateResult{
		ProjectRoot: root, DataEntryValid: true,
	}, true), "an earlier failure skips the check and stays reported")
}

// TestValidate_RealtimeRunAsGrant pins that `rela validate` loads
// realtime.yaml, and runs its session run_as grant check, when
// data-entry.yaml has no channels: section: an ungranted run_as fails the
// command, and granting it passes.
func TestValidate_RealtimeRunAsGrant(t *testing.T) {
	root := seedProject(t)
	prev := projectPath
	projectPath = root
	t.Cleanup(func() { projectPath = prev })
	writeProjectFile(t, root, "realtime.yaml", `version: 1
channel_types:
  watch:
    attach_to: [item]
    roles:
      member: { from: attached }
    message_types:
      text: { properties: { body: { type: string } }, post: [member] }
    session:
      initial: waiting
      run_as: system:bot
      states:
        waiting:
          after:
            soon:
              period: 1m
              to: done
              outputs:
                - set: { properties: { title: late } }
        done: { final: true }
`)
	run := func() (string, error) {
		return captureStdout(t, func() error { return (&ValidateCmd{}).Run(context.Background()) })
	}
	writeProjectFile(t, root, "acl.yaml", "roles:\n  bot: { read: [item] }\nassignments: { system:bot: bot }\n")
	out, err := run()
	require.Error(t, err)
	require.Contains(t, out, "system:bot cannot update item")

	writeProjectFile(t, root, "acl.yaml", "roles:\n  bot: { read: [item], update: [item] }\n"+
		"assignments: { system:bot: bot }\n")
	out, err = run()
	require.NoError(t, err, out)
	require.Contains(t, out, "✓ realtime.yaml is valid")
}
