//go:build !postgres && !memorybackend && !sqlite

package appbuild_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// A post succeeds when its hint cannot be sent: the journal's directory is
// a file here, so every append fails (AC 8).
func TestChannelHints_FailingSendKeepsPost(t *testing.T) {
	svc, root := promoteServices(t)
	feed := filepath.Join(root, ".rela", "channels", "_feed")
	require.NoError(t, os.MkdirAll(filepath.Dir(feed), 0o755))
	require.NoError(t, os.WriteFile(feed, []byte("not a directory"), 0o600))

	res, err := appbuild.ChannelService(svc).Post(promoteAs(t, svc, "alice"), promoteRoom,
		realtime.PostInput{Type: "proposal", Props: map[string]any{"summary": "still posted"}})
	require.NoError(t, err)
	require.Positive(t, res.Message.Seq)
	require.NoError(t, svc.Close(), "a failed flush is logged, not returned")
}
