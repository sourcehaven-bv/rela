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
	require.Nil(t, svc.Deps().Comments.Reader)
}

// TestStdioComments_ListOnly pins the stdio shape: list_comments is backed,
// the write tools are not (TKT-QIQO1W).
func TestStdioComments_ListOnly(t *testing.T) {
	svc, err := mcpServicesForTest(t, seedCommentProject(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	deps := svc.Deps()
	require.NotNil(t, deps.Comments.Reader)
	require.Nil(t, deps.Comments.Writer, "stdio must not register the comment write tools")

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

	list, err := deps.Comments.Reader.ListComments(ctx, "ITEM-1")
	require.NoError(t, err)
	require.Len(t, list, 2)
	byID := map[string]comments.View{list[0].ID: list[0], list[1].ID: list[1]}
	require.False(t, byID[onTitle.ID].Detached)
	require.True(t, byID[onGone.ID].Detached)
	require.False(t, byID[onTitle.ID].Editable, "stdio offers no write, so no write affordance")

	_, err = deps.Comments.Reader.ListComments(ctx, "ITEM-404")
	var opErr *comments.OpError
	require.ErrorAs(t, err, &opErr)
	require.Equal(t, "not_found", opErr.Code)
}

// seedAddCommentProject has a commentable plain type (item), a commentable
// faced type (page) and a type without comments (note).
func seedAddCommentProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"entities", "relations", ".rela"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, dir), 0o755))
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "schema.yaml"), []byte(`
entities:
  item:
    label: Item
    id_type: sequential
    id_prefix: ITEM-
    properties:
      title: {type: string}
  page:
    label: Page
    id_type: sequential
    id_prefix: PG-
    faces:
      published: {}
      draft: {}
    properties:
      title: {type: string}
  note:
    label: Note
    id_type: sequential
    id_prefix: NOTE-
    properties:
      title: {type: string}
relations: {}
comments:
  enabled: true
  on: [item, page]
`), 0o644))
	return root
}

func TestStdioComments_AdderOnlyForAnAuthoringRunAs(t *testing.T) {
	tests := []struct {
		name  string
		runAs string
		wired bool
	}{
		{name: "plain local session", runAs: ""},
		{name: "integration identity", runAs: "integration:runner", wired: true},
		{name: "system identity", runAs: "system:mission-bot"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, err := mcpServicesRunAs(t, seedCommentProject(t), tc.runAs)
			require.NoError(t, err)
			t.Cleanup(func() { _ = svc.Close() })

			deps := svc.Deps()
			require.Equal(t, tc.wired, deps.Comments.Adder != nil)
			require.Nil(t, deps.Comments.Writer, "stdio never registers edit, delete or accept")
		})
	}
}

func TestStdioComments_AddComment(t *testing.T) {
	svc, err := mcpServicesRunAs(t, seedAddCommentProject(t), "integration:runner")
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	deps := svc.Deps()
	require.NotNil(t, deps.Comments.Adder)

	ctx := principal.With(t.Context(), principal.Principal{User: "integration:runner", Tool: principal.ToolMCP})
	for _, e := range []*entity.Entity{
		{ID: "ITEM-1", Type: "item", Properties: map[string]any{"title": "One"},
			Content: "A body with a passage worth quoting.\n"},
		{ID: "PG-1", Type: "page", Face: "draft", Properties: map[string]any{"title": "draft"}},
		{ID: "PG-1", Type: "page", Face: "published", Properties: map[string]any{"title": "live"}},
		{ID: "NOTE-1", Type: "note", Properties: map[string]any{"title": "n"}},
	} {
		require.NoError(t, svc.svc.Store().CreateEntity(ctx, e))
	}

	onTitle := comments.AddInput{Kind: comments.AnchorProperty, Ref: "title", Body: "Started on this."}
	tests := []struct {
		name    string
		address string
		in      comments.AddInput
		code    string // OpError code; empty for success
	}{
		{name: "property anchor", address: "ITEM-1", in: onTitle},
		{name: "text anchor", address: "ITEM-1", in: comments.AddInput{
			Kind: comments.AnchorText, Quote: "passage worth quoting", Body: "Is this still true?",
		}},
		{name: "named face", address: "PG-1@draft", in: onTitle},
		{name: "bare id on a faced type", address: "PG-1", in: onTitle, code: "face_required"},
		{name: "type without comments", address: "NOTE-1", in: onTitle, code: "comments_not_enabled"},
		{name: "missing entity", address: "ITEM-404", in: onTitle, code: "not_found"},
		{name: "quote not in the body", address: "ITEM-1", in: comments.AddInput{
			Kind: comments.AnchorText, Quote: "nowhere to be found", Body: "x",
		}, code: "invalid_comment"},
		{name: "empty body", address: "ITEM-1", in: comments.AddInput{
			Kind: comments.AnchorProperty, Ref: "title",
		}, code: "invalid_comment"},
		{name: "unknown face", address: "PG-1@archived", in: onTitle, code: "not_found"},
		{name: "face on a type without faces", address: "ITEM-1@draft", in: onTitle, code: "not_found"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			created, err := deps.Comments.Adder.AddComment(ctx, tc.address, tc.in)
			if tc.code != "" {
				var opErr *comments.OpError
				require.ErrorAs(t, err, &opErr)
				require.Equal(t, tc.code, opErr.Code)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "integration:runner", created.Author)

			list, err := deps.Comments.Reader.ListComments(ctx, tc.address)
			require.NoError(t, err)
			ids := make([]string, len(list))
			for i, v := range list {
				ids[i] = v.ID
			}
			require.Contains(t, ids, created.ID)
		})
	}
}
