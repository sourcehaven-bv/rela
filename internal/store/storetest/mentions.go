package storetest

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/mentions"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunMentionTests is the conformance suite for [store.MentionIndex]
// (TKT-Q6AKUN). Every in-tree backend implements the capability, so a
// store that does not is a failure, not a skip.
func RunMentionTests(t *testing.T, f Factory) {
	idx := func(t *testing.T, s store.Store) store.MentionIndex {
		t.Helper()
		mi, ok := s.(store.MentionIndex)
		require.True(t, ok, "store does not implement store.MentionIndex")
		return mi
	}
	body := func(id, typ, content string) *entity.Entity {
		e := entity.New(id, typ)
		e.SetString("title", id)
		e.Content = content
		return e
	}
	faced := func(t *testing.T, id, typ, face, content string) *entity.Entity {
		t.Helper()
		e := body(id, typ, content)
		p, err := entity.ParseFace(face)
		require.NoError(t, err)
		e.Face = p
		return e
	}
	sources := func(t *testing.T, s store.Store, target string) []entity.Ref {
		t.Helper()
		refs, err := idx(t, s).MentionSources(ctx(), target, entity.Ref{}, 1000)
		require.NoError(t, err)
		return refs
	}
	ids := func(refs []entity.Ref) []string {
		out := make([]string, 0, len(refs))
		for _, r := range refs {
			out = append(out, entity.FormatStateRef(r.ID, r.Face))
		}
		return out
	}
	create := func(t *testing.T, s store.Store, e *entity.Entity) {
		t.Helper()
		require.NoError(t, s.CreateEntity(ctx(), e))
	}

	// A lazy index (fsstore) builds on the first read and is maintained
	// incrementally after that, so every case runs both ways: cold, where
	// the first read builds from what the writes left behind, and warm,
	// where the writes must update an index that already exists.
	for _, warm := range []bool{false, true} {
		open := func(t *testing.T) store.Store {
			t.Helper()
			s := f(t)
			if warm {
				_ = sources(t, s, "WARM-0")
			}
			return s
		}
		name := "Cold"
		if warm {
			name = "Warm"
		}
		t.Run(name, func(t *testing.T) {
			t.Run("CreateIndexesBody", func(t *testing.T) {
				s := open(t)
				create(t, s, body("FEAT-001", "feature", "Depends on `REQ-001`."))
				create(t, s, body("FEAT-002", "feature", "Plain REQ-001 is not a mention."))
				create(t, s, body("REQ-001", "requirement", ""))
				assert.Equal(t, []string{"FEAT-001"}, ids(sources(t, s, "REQ-001")))
				assert.Empty(t, sources(t, s, "FEAT-001"))
			})

			t.Run("CodeBlocksAndLinksDoNotCount", func(t *testing.T) {
				s := open(t)
				create(t, s, body("FEAT-001", "feature", "```\n`REQ-001`\n```\n\n[`REQ-001`](http://x)\n\n`see REQ-001`"))
				assert.Empty(t, sources(t, s, "REQ-001"))
			})

			t.Run("TargetNeedNotExist", func(t *testing.T) {
				s := open(t)
				create(t, s, body("FEAT-001", "feature", "`REQ-404`"))
				assert.Equal(t, []string{"FEAT-001"}, ids(sources(t, s, "REQ-404")))
			})

			t.Run("UpdateReplacesTargets", func(t *testing.T) {
				s := open(t)
				create(t, s, body("FEAT-001", "feature", "`REQ-001`"))
				require.NoError(t, s.UpdateEntity(ctx(), body("FEAT-001", "feature", "`REQ-002`")))
				assert.Empty(t, sources(t, s, "REQ-001"))
				assert.Equal(t, []string{"FEAT-001"}, ids(sources(t, s, "REQ-002")))

				require.NoError(t, s.UpdateEntity(ctx(), body("FEAT-001", "feature", "no mentions")))
				assert.Empty(t, sources(t, s, "REQ-002"))
			})

			t.Run("DeleteFamilyDropsSource", func(t *testing.T) {
				s := open(t)
				create(t, s, body("FEAT-001", "feature", "`REQ-001`"))
				_, err := s.DeleteFamily(ctx(), "FEAT-001", true)
				require.NoError(t, err)
				assert.Empty(t, sources(t, s, "REQ-001"))

				// A recreated id starts from its new body, not the old one.
				create(t, s, body("FEAT-001", "feature", "nothing"))
				assert.Empty(t, sources(t, s, "REQ-001"))
			})

			t.Run("FacesIndexedSeparately", func(t *testing.T) {
				s := open(t)
				create(t, s, faced(t, "PAGE-1", "page", "draft", "`REQ-001`"))
				create(t, s, faced(t, "PAGE-1", "page", "published", "`REQ-002`"))
				assert.Equal(t, []string{"PAGE-1@draft"}, ids(sources(t, s, "REQ-001")))
				assert.Equal(t, []string{"PAGE-1@published"}, ids(sources(t, s, "REQ-002")))

				_, err := s.DeleteFace(ctx(), entity.Ref{ID: "PAGE-1", Face: "draft"})
				require.NoError(t, err)
				assert.Empty(t, sources(t, s, "REQ-001"))
				assert.Equal(t, []string{"PAGE-1@published"}, ids(sources(t, s, "REQ-002")))
			})

			t.Run("RenameSourceMovesIt", func(t *testing.T) {
				s := open(t)
				create(t, s, body("FEAT-001", "feature", "`REQ-001`"))
				_, err := s.RenameFamily(ctx(), "FEAT-001", "FEAT-900")
				require.NoError(t, err)
				assert.Equal(t, []string{"FEAT-900"}, ids(sources(t, s, "REQ-001")))
			})

			t.Run("RenameTargetLeavesTextAlone", func(t *testing.T) {
				s := open(t)
				create(t, s, body("REQ-001", "requirement", ""))
				create(t, s, body("FEAT-001", "feature", "`REQ-001`"))
				_, err := s.RenameFamily(ctx(), "REQ-001", "REQ-900")
				require.NoError(t, err)
				assert.Empty(t, sources(t, s, "REQ-900"), "the mention text still names the old id")
				assert.Equal(t, []string{"FEAT-001"}, ids(sources(t, s, "REQ-001")))
			})

			t.Run("SoftDeleteHidesAndUnmarkRestores", func(t *testing.T) {
				s := open(t)
				p, ok := s.(store.SoftDeleteProvider)
				if !ok {
					t.Skip("store has no soft delete")
				}
				create(t, s, body("FEAT-001", "feature", "`REQ-001`"))
				_, err := p.SoftDelete().MarkDeleted(ctx(), "FEAT-001", "tester")
				require.NoError(t, err)
				assert.Empty(t, sources(t, s, "REQ-001"))

				_, err = p.SoftDelete().Unmark(ctx(), "FEAT-001")
				require.NoError(t, err)
				assert.Equal(t, []string{"FEAT-001"}, ids(sources(t, s, "REQ-001")))

				_, err = p.SoftDelete().MarkDeleted(ctx(), "FEAT-001", "tester")
				require.NoError(t, err)
				_, err = p.SoftDelete().PurgeMarked(ctx(), "FEAT-001")
				require.NoError(t, err)
				create(t, s, body("FEAT-001", "feature", "nothing"))
				assert.Empty(t, sources(t, s, "REQ-001"), "a purged source must not come back with a new body")
			})

			t.Run("PagesInRefOrder", func(t *testing.T) {
				s := open(t)
				var want []string
				for i := range 7 {
					id := fmt.Sprintf("FEAT-%03d", i)
					create(t, s, body(id, "feature", "`REQ-001`"))
					want = append(want, id)
				}
				create(t, s, faced(t, "PAGE-1", "page", "draft", "`REQ-001`"))
				create(t, s, faced(t, "PAGE-1", "page", "published", "`REQ-001`"))
				want = append(want, "PAGE-1@draft", "PAGE-1@published")

				var got []string
				after := entity.Ref{}
				for range 10 {
					page, err := idx(t, s).MentionSources(ctx(), "REQ-001", after, 3)
					require.NoError(t, err)
					if len(page) == 0 {
						break
					}
					assert.LessOrEqual(t, len(page), 3)
					got = append(got, ids(page)...)
					after = page[len(page)-1]
				}
				assert.Equal(t, want, got)
			})

			t.Run("RejectsNonPositiveLimit", func(t *testing.T) {
				s := open(t)
				_, err := idx(t, s).MentionSources(ctx(), "REQ-001", entity.Ref{}, 0)
				assert.ErrorIs(t, err, store.ErrInvalidQuery)
			})

			t.Run("TargetCap", func(t *testing.T) {
				s := open(t)
				var b strings.Builder
				for i := range mentions.MaxTargets + 5 {
					fmt.Fprintf(&b, "`X-%05d` ", i)
				}
				create(t, s, body("FEAT-001", "feature", b.String()))
				assert.Equal(t, []string{"FEAT-001"}, ids(sources(t, s, "X-00000")))
				assert.Empty(t, sources(t, s, fmt.Sprintf("X-%05d", mentions.MaxTargets+1)))
			})

			t.Run("TxCommitIndexes", func(t *testing.T) {
				s := open(t)
				require.NoError(t, s.Tx(ctx(), func(tx store.Store) error {
					return tx.CreateEntity(ctx(), body("FEAT-001", "feature", "`REQ-001`"))
				}))
				assert.Equal(t, []string{"FEAT-001"}, ids(sources(t, s, "REQ-001")))
			})
		})
	}
}

// RunMentionRollbackTests pins that a rolled-back write leaves no index entry.
// Run by backends with the strong Tx contract (see Capabilities.TxRollback).
func RunMentionRollbackTests(t *testing.T, f Factory) {
	sentinel := errors.New("boom")
	s := f(t)
	err := s.Tx(ctx(), func(tx store.Store) error {
		e := entity.New("FEAT-001", "feature")
		e.Content = "`REQ-001`"
		if err := tx.CreateEntity(ctx(), e); err != nil {
			return err
		}
		return sentinel
	})
	require.ErrorIs(t, err, sentinel)
	mi, ok := s.(store.MentionIndex)
	require.True(t, ok)
	refs, err := mi.MentionSources(ctx(), "REQ-001", entity.Ref{}, 10)
	require.NoError(t, err)
	assert.Empty(t, refs)
}
