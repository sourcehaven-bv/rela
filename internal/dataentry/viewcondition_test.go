package dataentry

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/relresolve"
)

// stubMatcher matches rows whose `status` equals want, or fails on failOn.
type stubMatcher struct {
	want   string
	failOn string
}

func (m stubMatcher) MatchPage(
	_ context.Context, rows []*entityPkg.Entity, _ relresolve.Gate, _ relresolve.Match,
) ([]bool, error) {
	out := make([]bool, len(rows))
	for i, e := range rows {
		if m.failOn != "" && e.ID == m.failOn {
			return nil, errors.New("boom")
		}
		s, _ := e.Properties["status"].(string)
		out[i] = s == m.want
	}
	return out, nil
}

func rows(ids ...string) []*entityPkg.Entity {
	out := make([]*entityPkg.Entity, 0, len(ids))
	for _, id := range ids {
		status := "open"
		if id == "B" {
			status = "done"
		}
		out = append(out, &entityPkg.Entity{
			ID: id, Type: "taak", Properties: map[string]any{"status": status},
		})
	}
	return out
}

func TestApplyViewCondition(t *testing.T) {
	ctx := context.Background()

	t.Run("nil matcher is a no-op, not an empty result", func(t *testing.T) {
		in := rows("A", "B")
		got, err := applyViewCondition(ctx, in, nil, nil, nil)
		require.NoError(t, err)
		require.Equal(t, in, got)
	})

	t.Run("filters and preserves order", func(t *testing.T) {
		got, err := applyViewCondition(ctx, rows("A", "B", "C"), stubMatcher{want: "open"}, nil, nil)
		require.NoError(t, err)
		require.Len(t, got, 2)
		require.Equal(t, "A", got[0].ID)
		require.Equal(t, "C", got[1].ID)
	})

	// An unjudgeable row must not be silently dropped: that would narrow the
	// view with no diagnostic, the inverse of BUG-WHEREWIDE.
	t.Run("an evaluation error aborts rather than dropping the row", func(t *testing.T) {
		_, err := applyViewCondition(ctx, rows("A", "B"), stubMatcher{want: "open", failOn: "A"}, nil, nil)
		require.Error(t, err)
	})
}

func TestViewConditionFor(t *testing.T) {
	lookup := func(kind, id string) (ViewConditionMatcher, bool) {
		if kind == viewKindList && id == "has_cond" {
			return stubMatcher{want: "open"}, true
		}
		return nil, false
	}

	t.Run("resolves a configured condition", func(t *testing.T) {
		require.NotNil(t, viewConditionFor(lookup, viewKindList, "has_cond"))
	})

	// The generic endpoint stays generic. A condition is presentation, not
	// authorization, so no id means the ACL-scoped superset — never a refusal
	// that would break MCP/CLI callers reading the type directly.
	t.Run("no id means no constraint", func(t *testing.T) {
		require.Nil(t, viewConditionFor(lookup, viewKindList, ""))
	})

	t.Run("a view without a condition means no constraint", func(t *testing.T) {
		require.Nil(t, viewConditionFor(lookup, viewKindList, "plain"))
	})

	t.Run("an unwired lookup means no constraint", func(t *testing.T) {
		require.Nil(t, viewConditionFor(nil, viewKindList, "has_cond"))
	})

	// A list and a kanban may share an id; the kind must disambiguate or one
	// view would silently inherit the other's rule.
	t.Run("kind disambiguates a shared id", func(t *testing.T) {
		require.Nil(t, viewConditionFor(lookup, viewKindKanban, "has_cond"))
	})
}
