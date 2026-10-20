package cli

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/projectsetup"
)

// TestValidate_ChannelsAgainstRealtime pins the `rela validate` check of
// data-entry.yaml's channels: section against realtime.yaml.
func TestValidate_ChannelsAgainstRealtime(t *testing.T) {
	root := seedProject(t)
	result := &projectsetup.ValidateResult{ProjectRoot: root, DataEntryValid: true}
	check := func() (bool, error) { return reportChannelsValidation(context.Background(), result, false) }
	// No channels: section: nothing to check, and no services are built.
	writeProjectFile(t, root, "data-entry.yaml", "version: \"1.0\"\n")
	bad, err := check()
	require.NoError(t, err)
	require.False(t, bad)

	section := "version: \"1.0\"\nchannels:\n  room:\n    messages:\n      text:\n        fields: [{ property: %s }]\n"
	writeProjectFile(t, root, "data-entry.yaml", fmt.Sprintf(section, "body"))
	bad, err = check()
	require.NoError(t, err)
	require.False(t, bad, "realtime off is a warning")

	writeProjectFile(t, root, "realtime.yaml", stdioRealtimeYAML)
	bad, err = check()
	require.NoError(t, err)
	require.False(t, bad)

	writeProjectFile(t, root, "data-entry.yaml", fmt.Sprintf(section, "nope"))
	bad, err = check()
	require.NoError(t, err)
	require.True(t, bad)

	bad, err = reportChannelsValidation(context.Background(), result, true)
	require.NoError(t, err)
	require.True(t, bad, "an earlier failure skips the check and stays reported")
}
