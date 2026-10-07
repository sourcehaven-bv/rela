//go:build !sqlite

package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDBImportFS_NeedsSQLiteBuild(t *testing.T) {
	cmd := &DBImportFSCmd{Source: t.TempDir(), Target: t.TempDir() + "/x"}
	require.ErrorIs(t, cmd.Run(context.Background()), errImportFSNeedsSQLite)
}
