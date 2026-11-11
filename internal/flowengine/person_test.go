package flowengine

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
)

func holdsRoles(roles ...string) func(string) bool {
	return func(r string) bool { return slices.Contains(roles, r) }
}

// startSample starts the sample onboarding flow on PER-1 and returns the
// run, which waits in collect for hr.
func startSample(t *testing.T, h harness, props map[string]any) flowrun.Run {
	t.Helper()
	h.f.addPerson("PER-1", props)
	id, err := h.eng.Start(context.Background(), "onboarding", entity.Ref{ID: "PER-1"})
	require.NoError(t, err)
	run, err := h.eng.Run(context.Background(), id)
	require.NoError(t, err)
	return run
}

func TestEngine_Change(t *testing.T) {
	tests := []struct {
		name     string
		docs     bool
		roles    []string
		gen      int64 // 0: the run's
		want     Changed
		wantErr  error
		wantLogs int
	}{
		{name: "holder, guard holds", docs: true, roles: []string{"hr"},
			want: Changed{Moved: true, Finished: true}, wantLogs: 3},
		{name: "holder, guard does not hold", docs: false, roles: []string{"hr"},
			want: Changed{State: "collect"}, wantLogs: 1},
		{name: "role not held", docs: true, roles: []string{"manager"},
			want: Changed{State: "collect"}, wantLogs: 1},
		{name: "stale generation", docs: true, roles: []string{"hr"}, gen: 7, wantErr: ErrStale},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, "")
			ctx := context.Background()
			run := startSample(t, h, map[string]any{"name": "Ada", "documents": tc.docs})
			gen := run.Instance.Generation
			if tc.gen != 0 {
				gen = tc.gen
			}
			got, err := h.eng.Change(ctx, run.ID, gen, "alice", holdsRoles(tc.roles...))
			require.ErrorIs(t, err, tc.wantErr)
			if tc.wantErr != nil {
				return
			}
			if tc.want.Finished {
				require.True(t, got.Moved)
				require.True(t, got.Finished)
				require.Equal(t, "completed", h.f.header(run.ID)["status"])
				return
			}
			require.Equal(t, tc.want, got)
			stored, err := h.eng.Run(ctx, run.ID)
			require.NoError(t, err)
			require.Len(t, stored.Log, tc.wantLogs)
		})
	}
}

func TestEngine_ChangeLogsTheMove(t *testing.T) {
	h := newHarness(t, "")
	ctx := context.Background()
	run := startSample(t, h, map[string]any{"name": "Ada"})
	require.Equal(t, []flowrun.Step{{At: *h.clock, State: "collect", Via: flowrun.ViaStart}}, run.Log)

	// The stalled timer escalates; the escalated state's change branch has
	// no roles.
	*h.clock = h.clock.Add(7*24*time.Hour + time.Minute)
	_, err := h.eng.Due(ctx)
	require.NoError(t, err)
	run, err = h.eng.Run(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, flowrun.Step{At: *h.clock, State: "escalated", From: "collect", Via: flowrun.ViaTimer,
		Timer: "stalled"}, run.Log[1])

	h.f.addPerson("PER-1", map[string]any{"name": "Ada", "documents": true})
	*h.clock = h.clock.Add(time.Minute)
	got, err := h.eng.Change(ctx, run.ID, run.Instance.Generation, "bob", holdsRoles())
	require.NoError(t, err)
	require.True(t, got.Moved)
	require.True(t, got.Finished)
}

func TestEngine_ClaimRefusesASecondCompletion(t *testing.T) {
	h := newHarness(t, "")
	ctx := context.Background()
	run := startSample(t, h, map[string]any{"name": "Ada"})
	gen := run.Instance.Generation

	require.NoError(t, h.eng.Claim(ctx, run.ID, gen, "alice"))
	require.NoError(t, h.eng.Claim(ctx, run.ID, gen, "alice"), "the holder renews")
	require.ErrorIs(t, h.eng.Claim(ctx, run.ID, gen, "bob"), ErrClaimed)
	_, err := h.eng.Change(ctx, run.ID, gen, "bob", holdsRoles("hr"))
	require.ErrorIs(t, err, ErrClaimed)
	require.ErrorIs(t, h.eng.Claim(ctx, run.ID, gen+1, "bob"), ErrStale)

	// A claim runs out.
	*h.clock = h.clock.Add(ClaimTTL)
	require.NoError(t, h.eng.Claim(ctx, run.ID, gen, "bob"))

	// A change that moves nothing releases the claim.
	_, err = h.eng.Change(ctx, run.ID, gen, "bob", holdsRoles("hr"))
	require.NoError(t, err)
	stored, err := h.eng.Run(ctx, run.ID)
	require.NoError(t, err)
	require.Nil(t, stored.Claim)
}

// TestEngine_ReleaseClaim pins that only the holder's claim at the
// generation is released, so a step whose write failed does not keep
// other holders out.
func TestEngine_ReleaseClaim(t *testing.T) {
	h := newHarness(t, "")
	ctx := context.Background()
	run := startSample(t, h, map[string]any{"name": "Ada"})
	gen := run.Instance.Generation
	require.NoError(t, h.eng.Claim(ctx, run.ID, gen, "alice"))

	for _, tc := range []struct {
		by  string
		gen int64
	}{{"bob", gen}, {"alice", gen + 1}} {
		h.eng.ReleaseClaim(ctx, run.ID, tc.gen, tc.by)
		require.ErrorIs(t, h.eng.Claim(ctx, run.ID, gen, "bob"), ErrClaimed, "%s at %d released", tc.by, tc.gen)
	}
	h.eng.ReleaseClaim(ctx, run.ID, gen, "alice")
	require.NoError(t, h.eng.Claim(ctx, run.ID, gen, "bob"))
	h.eng.ReleaseClaim(ctx, "FR-NONE", gen, "bob")
}

func TestEngine_ClaimConcurrent(t *testing.T) {
	h := newHarness(t, "")
	ctx := context.Background()
	run := startSample(t, h, map[string]any{"name": "Ada"})
	const n = 8
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			errs[i] = h.eng.Claim(ctx, run.ID, run.Instance.Generation, string(rune('a'+i)))
		})
	}
	wg.Wait()
	won := 0
	for _, err := range errs {
		if err == nil {
			won++
		} else {
			require.ErrorIs(t, err, ErrClaimed)
		}
	}
	require.Equal(t, 1, won)
}

func TestEngine_Retry(t *testing.T) {
	h := newHarness(t, quickFlow)
	ctx := context.Background()
	h.f.addPerson("PER-1", map[string]any{"name": "Ada", "documents": true})
	h.f.failSet = context.DeadlineExceeded
	id, err := h.eng.Start(ctx, "quick", entity.Ref{ID: "PER-1"})
	require.Error(t, err)
	run, err := h.eng.Run(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, run.Pending)

	xs, err := h.eng.Explain(ctx, run)
	require.NoError(t, err)
	x := xs[0]
	require.Equal(t, &PendingStep{Failures: 1, NextAttempt: h.clock.Add(minBackoff)}, x.Pending)

	require.Error(t, h.eng.Retry(ctx, id), "still failing")
	h.f.failSet = nil
	require.NoError(t, h.eng.Retry(ctx, id))
	require.Equal(t, "completed", h.f.header(id)["status"])
	require.ErrorIs(t, h.eng.Retry(ctx, id), ErrRunNotFound)
}

func TestEngine_RetryNotPending(t *testing.T) {
	h := newHarness(t, "")
	run := startSample(t, h, map[string]any{"name": "Ada"})
	require.ErrorIs(t, h.eng.Retry(context.Background(), run.ID), ErrNotPending)
}

func TestEngine_ChangeWhileSettling(t *testing.T) {
	h := newHarness(t, quickFlow)
	ctx := context.Background()
	h.f.addPerson("PER-1", map[string]any{"name": "Ada"})
	h.f.failSet = context.DeadlineExceeded
	id, _ := h.eng.Start(ctx, "quick", entity.Ref{ID: "PER-1"})
	run, err := h.eng.Run(ctx, id)
	require.NoError(t, err)
	_, err = h.eng.Change(ctx, id, run.Instance.Generation, "alice", holdsRoles())
	require.ErrorIs(t, err, ErrSettling)
	require.ErrorIs(t, h.eng.Claim(ctx, id, run.Instance.Generation, "alice"), ErrSettling)
}

func TestEngine_Explain(t *testing.T) {
	h := newHarness(t, "")
	ctx := context.Background()
	run := startSample(t, h, map[string]any{"name": "Ada", "documents": true})
	require.NoError(t, h.eng.Claim(ctx, run.ID, run.Instance.Generation, "alice"))
	run, err := h.eng.Run(ctx, run.ID)
	require.NoError(t, err)

	xs, err := h.eng.Explain(ctx, run, run)
	require.NoError(t, err)
	require.Len(t, xs, 2)
	x := xs[1]
	require.Equal(t, "collect", x.State)
	require.Equal(t, "Collect documents", x.Label)
	require.Equal(t, flow.StatusInputRequired, x.Status)
	require.Equal(t, []string{"hr"}, x.WaitingFor)
	require.Equal(t, []string{"documents"}, x.Writes)
	require.Equal(t, []TimerDue{{Name: "stalled", Label: "No documents in a week", To: "escalated",
		Due: h.clock.Add(7 * 24 * time.Hour)}}, x.Timers)
	require.Equal(t, []BranchCheck{{To: "review", Roles: []string{"hr"}, Guard: "entity.documents == true",
		Holds: true}}, x.Branches)
	require.Nil(t, x.Pending)
	require.NotNil(t, x.Claim)
	require.Equal(t, "alice", x.Claim.By)

	// A stranded run explains as stranded rather than failing.
	run.Instance.State = "gone"
	xs, err = h.eng.Explain(ctx, run)
	require.NoError(t, err)
	require.True(t, xs[0].Stranded)
}

// TestEngine_OneUpdatePath fails when engine code updates a run other than
// through put, which sets the due hint of a forked run (RR-SHLMJUL).
func TestEngine_OneUpdatePath(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		require.NoError(t, err)
		file, err := parser.ParseFile(fset, name, src, 0)
		require.NoError(t, err)
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name == "put" {
				continue
			}
			ast.Inspect(fn, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if ok && sel.Sel.Name == "Update" {
					if inner, ok := sel.X.(*ast.SelectorExpr); ok && inner.Sel.Name == "Runs" {
						t.Errorf("%s: %s updates a run directly; use put", fset.Position(sel.Pos()), fn.Name.Name)
					}
				}
				return true
			})
		}
	}
}
