package fsstore

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestParseDocument_ConflictMarker pins what parseDocument refuses as an
// unresolved merge: a complete conflict block at column 0, fenced or not. A
// substring match (BUG-WN6D) and a lone marker line (BUG-GYM7ZBS) are
// content.
func TestParseDocument_ConflictMarker(t *testing.T) {
	t.Parallel()

	const block = "<<<<<<< HEAD\nfoo\n=======\nbar\n>>>>>>> branch\n"
	tests := []struct {
		name      string
		content   string
		wantError bool
	}{
		{
			name:      "block in the body is a conflict",
			content:   "---\nid: x\n---\n\n" + block,
			wantError: true,
		},
		{
			name:      "block in the frontmatter is a conflict",
			content:   "---\nid: x\n<<<<<<< HEAD\ns: a\n=======\ns: b\n>>>>>>> branch\n---\n",
			wantError: true,
		},
		{
			name:      "block starting on the first line is a conflict",
			content:   "<<<<<<< HEAD\n---\nid: x\n---\n=======\n---\nid: y\n---\n>>>>>>> branch\n",
			wantError: true,
		},
		{
			name:      "lone start marker line is not a conflict",
			content:   "---\nid: x\n---\n\n<<<<<<< not a merge\n",
			wantError: false,
		},
		{
			name:      "block inside a fenced code block is a conflict",
			content:   "---\nid: x\n---\n\nGit writes:\n\n```text\n" + block + "```\n",
			wantError: true,
		},
		{
			name:      "marker inside inline code span is not a conflict",
			content:   "---\nid: x\ntitle: how markers look\n---\n\nThe detector matches `<<<<<<<` at column 0.\n",
			wantError: false,
		},
		{
			name:      "marker mid-line in prose is not a conflict",
			content:   "---\nid: x\n---\n\nA literal <<<<<<< appears inline; not a conflict.\n",
			wantError: false,
		},
		{
			name:      "indented block is not a conflict",
			content:   "---\nid: x\n---\n\n  <<<<<<< HEAD\n  =======\n  >>>>>>> branch\n",
			wantError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseDocument(tt.content)
			gotErr := errors.Is(err, errConflictedFile)
			if gotErr != tt.wantError {
				t.Errorf("parseDocument: errConflictedFile = %v, want %v (err=%v)",
					gotErr, tt.wantError, err)
			}
		})
	}
}

// TestWrite_ConflictBlockBodyStaysReadable pins the write side of
// BUG-GYM7ZBS: a write never produces a file that reads as an unresolved
// merge. Body formatting breaks the end marker of a block in prose, so such
// a body is stored readable. It leaves fenced code alone, so a block in a
// fence is refused and nothing is stored. formatDocumentOrdered refuses any
// document that would read as conflicted, so a later formatter change cannot
// bring the unreadable row back.
func TestWrite_ConflictBlockBodyStaysReadable(t *testing.T) {
	t.Parallel()
	const block = "<<<<<<< HEAD\nours\n=======\ntheirs\n>>>>>>> feature\n"

	s, _ := newTestStore(t)
	ctx := context.Background()
	e := entity.New("TKT-1", "ticket")
	e.Content = block
	require.NoError(t, s.CreateEntity(ctx, e))
	_, err := s.GetEntity(ctx, entity.Ref{ID: "TKT-1"})
	require.NoError(t, err, "the row written with a conflict block in its body stays readable")

	require.NoError(t, s.CreateEntity(ctx, entity.New("TKT-2", "ticket")))
	key := entity.RelationKey{From: "TKT-1", Type: "rel", To: "TKT-2"}
	_, err = s.CreateRelation(ctx, key, &store.RelationData{Content: block})
	require.NoError(t, err)
	_, err = s.GetRelation(ctx, key)
	require.NoError(t, err, "the relation written with a conflict block in its body stays readable")

	fenced := entity.New("TKT-3", "ticket")
	fenced.Content = "```\n" + block + "```\n"
	require.ErrorIs(t, s.CreateEntity(ctx, fenced), store.ErrConflictBlock)
	_, err = s.GetEntity(ctx, entity.Ref{ID: "TKT-3"})
	require.ErrorIs(t, err, store.ErrNotFound, "a refused create stores nothing")

	_, err = formatDocumentOrdered(map[string]any{"id": "TKT-4"}, block, nil)
	require.ErrorIs(t, err, store.ErrConflictBlock)
}
