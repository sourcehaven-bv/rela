package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// seedCommentProject is seedProject with commenting enabled on `item`.
func seedCommentProject(t *testing.T) string {
	t.Helper()
	root := seedProject(t)
	f, err := os.OpenFile(filepath.Join(root, "metamodel.yaml"), os.O_APPEND|os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = f.WriteString("comments:\n  enabled: true\n  on: [item]\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return root
}

func TestStdioComments_NilWithoutCommentsBlock(t *testing.T) {
	svc, err := mcpServicesForTest(t, seedProject(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	require.Nil(t, svc.Deps().Comments)
}

// TestStdioComments_ListOnly pins the stdio shape: list_comments is backed,
// the write tools are not (TKT-QIQO1W).
func TestStdioComments_ListOnly(t *testing.T) {
	svc, err := mcpServicesForTest(t, seedCommentProject(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	deps := svc.Deps()
	require.NotNil(t, deps.Comments)
	require.Nil(t, deps.CommentWriter, "stdio must not register the comment write tools")

	ctx := principal.With(t.Context(), principal.Principal{User: "alice", Tool: principal.ToolCLI})
	require.NoError(t, svc.svc.Store().CreateEntity(ctx, &entity.Entity{
		ID: "ITEM-1", Type: "item", Properties: map[string]any{"title": "One"},
		Content: "A body with a passage worth quoting.\n",
	}))
	target := comments.Target{Type: "item", ID: "ITEM-1"}
	onTitle, err := svc.svc.Comments().Add(ctx, target, comments.AddRequest{
		Anchor: comments.Anchor{Kind: comments.AnchorProperty, Ref: "title"}, Body: "fine",
	})
	require.NoError(t, err)
	onGone, err := svc.svc.Comments().Add(ctx, target, comments.AddRequest{
		Anchor: comments.Anchor{Kind: comments.AnchorProperty, Ref: "removed"}, Body: "orphan",
	})
	require.NoError(t, err)

	list, err := deps.Comments.ListComments(ctx, "ITEM-1")
	require.NoError(t, err)
	require.Len(t, list, 2)
	byID := map[string]comments.View{list[0].ID: list[0], list[1].ID: list[1]}
	require.False(t, byID[onTitle.ID].Detached)
	require.True(t, byID[onGone.ID].Detached)
	require.False(t, byID[onTitle.ID].Editable, "stdio offers no write, so no write affordance")

	_, err = deps.Comments.ListComments(ctx, "ITEM-404")
	var opErr *comments.OpError
	require.ErrorAs(t, err, &opErr)
	require.Equal(t, "not_found", opErr.Code)
}
