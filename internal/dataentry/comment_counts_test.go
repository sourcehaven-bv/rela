package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/comments/memcomments"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// countingComments counts Count calls and fails them when err is set; every
// other method passes through.
type countingComments struct {
	comments.Store
	counts int
	err    error
}

func (c *countingComments) Count(ctx context.Context, targets []comments.Target) (map[string]int, error) {
	c.counts++
	if c.err != nil {
		return nil, c.err
	}
	return c.Store.Count(ctx, targets)
}

// commentCountsApp is commentsApp with a second, comment-free ticket and two
// comments on TKT-001.
func commentCountsApp(t *testing.T) *App {
	t.Helper()
	app := commentsApp(t)
	seedEntity(app, &entity.Entity{
		ID: "TKT-002", Type: "ticket",
		Properties: map[string]any{"title": "Quiet ticket", "status": "open"},
	})
	for _, body := range []string{"first", "second"} {
		_, err := app.comments.svc.Add(principalCtx("alice"), comments.Target{Type: "ticket", ID: "TKT-001"},
			comments.AddRequest{Anchor: comments.Anchor{Kind: comments.AnchorProperty, Ref: "status"}, Body: body})
		require.NoError(t, err)
	}
	return app
}

func commentCountsByID(resp v1.ListResponse) map[string]*int {
	out := map[string]*int{}
	for _, e := range resp.Data {
		out[e.ID] = e.CommentCount
	}
	return out
}

func TestCommentCounts_ListRows(t *testing.T) {
	globalRead := &acl.Policy{
		Roles: map[string]acl.RoleDef{"viewer": {
			Read: []string{"ticket"}, Permissions: []string{acl.PermCommentRead},
		}},
		Assignments: map[string]string{"alice": "viewer"},
	}
	noCommentRead := &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}
	localRead := &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"viewer":    {Read: []string{"ticket"}},
			"commenter": {Read: []string{"ticket"}, Permissions: []string{acl.PermCommentRead}},
		},
		Assignments:   map[string]string{"alice": "viewer"},
		RoleRelations: map[string]acl.RoleRelationDef{"owned-by": {Confers: "commenter"}},
	}

	two, zero := 2, 0
	tests := []struct {
		name   string
		policy *acl.Policy
		query  string
		want   map[string]*int
		// threadReadable asserts the thread route serves TKT-001 to alice, so an
		// absent count is the list's choice and not a missing grant.
		threadReadable bool
	}{
		{"global comment:read counts every row", globalRead, "comment_counts=1",
			map[string]*int{"TKT-001": &two, "TKT-002": &zero}, false},
		{"not requested", globalRead, "", map[string]*int{"TKT-001": nil, "TKT-002": nil}, false},
		{"unparseable flag is off", globalRead, "comment_counts=yes", map[string]*int{"TKT-001": nil, "TKT-002": nil}, false},
		{"no comment:read", noCommentRead, "comment_counts=1", map[string]*int{"TKT-001": nil, "TKT-002": nil}, false},
		// Fails closed by design: the per-entity grant is a per-row walk.
		{"local-role comment:read only", localRead, "comment_counts=1",
			map[string]*int{"TKT-001": nil, "TKT-002": nil}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := commentCountsApp(t)
			_, err := app.store.CreateRelation(t.Context(),
				entity.RelationKey{From: "alice", Type: "owned-by", To: "TKT-001"}, nil)
			require.NoError(t, err)
			d := mustNewACL(t, tc.policy, app.store)
			app.acl = d

			resp, rec := listEntitiesAs(principalCtx("alice"), t, app, d, "ticket", "tickets", tc.query)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Equal(t, tc.want, commentCountsByID(resp))
			if tc.threadReadable {
				thread := doCommentsAs(t, app, d, http.MethodGet, "/api/v1/_comments/ticket/TKT-001", "", "alice")
				require.Equal(t, http.StatusOK, thread.Code, thread.Body.String())
			}
		})
	}
}

func TestCommentCounts_AbsentWhenNotServed(t *testing.T) {
	policy := &acl.Policy{
		Roles: map[string]acl.RoleDef{"viewer": {
			Read: []string{"ticket"}, Permissions: []string{acl.PermCommentRead},
		}},
		Assignments: map[string]string{"alice": "viewer"},
	}

	t.Run("commenting disabled", func(t *testing.T) {
		app := commentCountsApp(t)
		app.SetComments(nil)
		d := mustNewACL(t, policy, app.store)
		app.acl = d
		resp, rec := listEntitiesAs(principalCtx("alice"), t, app, d, "ticket", "tickets", "comment_counts=1")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Equal(t, map[string]*int{"TKT-001": nil, "TKT-002": nil}, commentCountsByID(resp))
	})

	t.Run("count failed", func(t *testing.T) {
		app := commentCountsApp(t)
		svc, err := comments.NewService(&countingComments{Store: memcomments.New(), err: errors.New("disk")}, nil)
		require.NoError(t, err)
		app.SetComments(svc)
		d := mustNewACL(t, policy, app.store)
		app.acl = d
		resp, rec := listEntitiesAs(principalCtx("alice"), t, app, d, "ticket", "tickets", "comment_counts=1")
		require.Equal(t, http.StatusOK, rec.Code, "a failed count must not fail the list: %s", rec.Body)
		require.Equal(t, map[string]*int{"TKT-001": nil, "TKT-002": nil}, commentCountsByID(resp))
	})

	t.Run("type not commentable", func(t *testing.T) {
		app := commentCountsApp(t)
		app.State().Meta.Comments = &metamodel.CommentsConfig{Enabled: true, On: []string{"feature"}}
		d := mustNewACL(t, policy, app.store)
		app.acl = d
		resp, rec := listEntitiesAs(principalCtx("alice"), t, app, d, "ticket", "tickets", "comment_counts=1")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Equal(t, map[string]*int{"TKT-001": nil, "TKT-002": nil}, commentCountsByID(resp))
	})
}

// TestCommentCounts_PerFace pins that a faced row carries the count of the
// face the world served, not of its id: comments are per face.
func TestCommentCounts_PerFace(t *testing.T) {
	app, _ := facedCommentsApp(t, nil)
	add := func(face entity.Face, n int) {
		for range n {
			_, err := app.comments.svc.Add(principalCtx("alice"),
				comments.Target{Type: "policy", ID: "POL-1", Face: face},
				comments.AddRequest{Anchor: comments.Anchor{Kind: comments.AnchorProperty, Ref: "title"}, Body: "x"})
			require.NoError(t, err)
		}
	}
	add("draft", 2)
	add("published", 1)

	one, two, zero := 1, 2, 0
	tests := []struct {
		name  string
		chain []entity.Face
		want  map[string]*int
	}{
		{"published first", []entity.Face{"published", "draft"}, map[string]*int{"POL-1": &one, "POL-2": &zero}},
		{"draft first", []entity.Face{"draft", "published"}, map[string]*int{"POL-1": &two, "POL-2": &zero}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scope := store.NewWorldScope(map[string]store.TypeResolution{
				"policy": {Chain: tc.chain, Fallback: store.FallbackExclude},
			})
			ctx := withWorld(withReadGate(principalCtx("alice"), nopReadGate{}), worldHandle{name: "w", scope: scope})
			req := httptest.NewRequest(http.MethodGet, "/api/v1/policies?comment_counts=1", http.NoBody)
			rec := httptest.NewRecorder()
			app.handleV1ListEntities(rec, req.WithContext(ctx), "policy", "policies")
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			var resp v1.ListResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			require.Equal(t, tc.want, commentCountsByID(resp))
		})
	}
}

// TestQueryBudget_CommentCountsAreOneCallPerPage pins the collection-reads
// rule for the new per-row value: one comment-store call per page whatever its
// size, and no extra graph-store reads over a plain list page.
func TestQueryBudget_CommentCountsAreOneCallPerPage(t *testing.T) {
	for _, n := range []int{10, 50} {
		app, counting, _, ctx := newBudgetAppOn(t, n, memstore.New())
		app.State().Meta.Comments = &metamodel.CommentsConfig{Enabled: true, On: []string{"ticket"}}
		cc := &countingComments{Store: memcomments.New()}
		svc, err := comments.NewService(cc, nil)
		require.NoError(t, err)
		app.SetComments(svc)
		// The budget app's editor role, plus comment:read.
		d := mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{"editor": {
				Read: []string{"*"}, Create: []string{"ticket"}, Update: []string{"ticket"}, Delete: []string{"ticket"},
				Permissions: []string{acl.PermCommentRead},
			}},
			Assignments: map[string]string{"T1": "editor"},
		}, app.store)
		app.acl = d
		counting.Reset()

		resp, rec := listEntitiesAs(ctx, t, app, d, "ticket", "tickets", "per_page=100&comment_counts=1")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Len(t, resp.Data, n)
		require.NotNil(t, resp.Data[0].CommentCount, "the budget principal must be served counts")
		require.Equal(t, 1, cc.counts, "comment-store calls at %d rows", n)
		require.Equal(t, listPageBudget, counting.Reads(), "graph-store reads at %d rows (%s)", n, counting)
	}
}
