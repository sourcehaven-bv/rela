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
		return reportChannelsValidation(context.Background(), result, false)
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

	require.True(t, reportChannelsValidation(context.Background(), &projectsetup.ValidateResult{
		ProjectRoot: root, DataEntryValid: true,
	}, true), "an earlier failure skips the check and stays reported")
}
