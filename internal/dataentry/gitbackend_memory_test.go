package dataentry

import (
	"fmt"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// --- Property tests for MemoryGitBackend state invariants ---

// TestPropertyBranchConsistency verifies that the current branch always
// exists in the local branch list after any sequence of branch operations.
func TestPropertyBranchConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 200

	properties := gopter.NewProperties(parameters)

	properties.Property("current branch is always in local branches", prop.ForAll(
		func(ops []branchOp) bool {
			m := NewMemoryGitBackend("/tmp/test", true)
			for _, op := range ops {
				switch op.kind {
				case opCreate:
					_ = m.CheckoutNewBranch(op.name)
				case opCheckout:
					_ = m.Checkout(op.name)
				case opDelete:
					// Don't delete the current branch
					if op.name != m.branch {
						_ = m.DeleteBranch(op.name)
					}
				}
			}
			// Invariant: current branch is in the local list
			branch, _ := m.CurrentBranch()
			local, _, _ := m.ListBranches()
			for _, b := range local {
				if b == branch {
					return true
				}
			}
			return false
		},
		genBranchOps(20),
	))

	properties.TestingRun(t)
}

// TestPropertyCommitUnpushed verifies that each Commit() call increments
// the unpushed count by exactly 1, and that unpushed is always non-negative.
func TestPropertyCommitUnpushed(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 200

	properties := gopter.NewProperties(parameters)

	properties.Property("N commits produce N unpushed", prop.ForAll(
		func(n uint8) bool {
			m := NewMemoryGitBackend("/tmp/test", true)
			m.clean = false

			count := int(n%20) + 1 // 1..20
			for i := 0; i < count; i++ {
				if err := m.Commit(fmt.Sprintf("commit-%d", i)); err != nil {
					return false
				}
			}

			unpushed, _ := m.RevCount("origin/main..HEAD")
			return unpushed == count
		},
		gen.UInt8(),
	))

	properties.Property("unpushed is non-negative after any operation mix", prop.ForAll(
		func(commitCount, pushCount uint8) bool {
			m := NewMemoryGitBackend("/tmp/test", true)
			m.clean = false

			for i := 0; i < int(commitCount%10); i++ {
				_ = m.Commit(fmt.Sprintf("c-%d", i))
			}
			for i := 0; i < int(pushCount%5); i++ {
				_, _ = m.Push("main")
			}

			unpushed, _ := m.RevCount("origin/main..HEAD")
			return unpushed >= 0
		},
		gen.UInt8(),
		gen.UInt8(),
	))

	properties.TestingRun(t)
}

// TestPropertyPushResetsUnpushed verifies that a successful Push()
// always resets the unpushed count to zero.
func TestPropertyPushResetsUnpushed(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 200

	properties := gopter.NewProperties(parameters)

	properties.Property("push resets unpushed to zero", prop.ForAll(
		func(n uint8) bool {
			m := NewMemoryGitBackend("/tmp/test", true)
			m.clean = false

			count := int(n%15) + 1
			for i := 0; i < count; i++ {
				_ = m.Commit(fmt.Sprintf("c-%d", i))
			}

			_, err := m.Push("main")
			if err != nil {
				return false
			}

			unpushed, _ := m.RevCount("origin/main..HEAD")
			return unpushed == 0
		},
		gen.UInt8(),
	))

	properties.TestingRun(t)
}

// TestPropertyBranchCreateDelete verifies that creating then deleting
// a branch restores the original branch list.
func TestPropertyBranchCreateDelete(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 200

	properties := gopter.NewProperties(parameters)

	properties.Property("create then delete branch restores list", prop.ForAll(
		func(branchName string) bool {
			m := NewMemoryGitBackend("/tmp/test", true)
			origBranches, _, _ := m.ListBranches()
			origLen := len(origBranches)

			if err := m.CheckoutNewBranch(branchName); err != nil {
				return false
			}
			// Switch back to main before deleting
			_ = m.Checkout("main")
			if err := m.DeleteBranch(branchName); err != nil {
				return false
			}

			finalBranches, _, _ := m.ListBranches()
			return len(finalBranches) == origLen
		},
		gen.Identifier().SuchThat(func(s string) bool { return s != "main" }),
	))

	properties.TestingRun(t)
}

// TestPropertyFastForwardClearsBehind verifies that FastForwardMerge()
// always resets the behind count to zero.
func TestPropertyFastForwardClearsBehind(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 200

	properties := gopter.NewProperties(parameters)

	properties.Property("fast-forward clears behind count", prop.ForAll(
		func(behind uint8) bool {
			m := NewMemoryGitBackend("/tmp/test", true)
			m.behind = int(behind)

			if err := m.FastForwardMerge("origin/main"); err != nil {
				return false
			}

			behindCount, _ := m.RevCount("HEAD..origin/main")
			return behindCount == 0
		},
		gen.UInt8(),
	))

	properties.TestingRun(t)
}

// TestPropertyCheckoutIdempotent verifies that checking out the current
// branch is a no-op that doesn't change any state.
func TestPropertyCheckoutIdempotent(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("checkout current branch is a no-op", prop.ForAll(
		func(n uint8) bool {
			m := NewMemoryGitBackend("/tmp/test", true)
			m.clean = false

			// Commit some things first
			for i := 0; i < int(n%5); i++ {
				_ = m.Commit(fmt.Sprintf("c-%d", i))
			}

			branch, _ := m.CurrentBranch()
			unpushedBefore, _ := m.RevCount("origin/main..HEAD")
			commitsBefore := len(m.commits)

			// Checkout the same branch
			err := m.Checkout(branch)
			if err != nil {
				return false
			}

			branchAfter, _ := m.CurrentBranch()
			unpushedAfter, _ := m.RevCount("origin/main..HEAD")
			commitsAfter := len(m.commits)

			return branchAfter == branch &&
				unpushedAfter == unpushedBefore &&
				commitsAfter == commitsBefore
		},
		gen.UInt8(),
	))

	properties.TestingRun(t)
}

// TestPropertyCommitMessages verifies that LogMessages returns exactly
// the messages from unpushed commits in order.
func TestPropertyCommitMessages(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 200

	properties := gopter.NewProperties(parameters)

	properties.Property("LogMessages returns unpushed commit messages", prop.ForAll(
		func(messages []string) bool {
			if len(messages) == 0 {
				return true
			}
			m := NewMemoryGitBackend("/tmp/test", true)
			m.clean = false

			for _, msg := range messages {
				_ = m.Commit(msg)
			}

			logs, err := m.LogMessages("origin/main..HEAD")
			if err != nil {
				return false
			}

			if len(logs) != len(messages) {
				return false
			}
			for i, msg := range messages {
				if logs[i] != msg {
					return false
				}
			}
			return true
		},
		gen.SliceOfN(10, gen.AlphaString().SuchThat(func(s string) bool { return s != "" })),
	))

	properties.TestingRun(t)
}

// --- Generators ---

type branchOpKind int

const (
	opCreate branchOpKind = iota
	opCheckout
	opDelete
)

type branchOp struct {
	kind branchOpKind
	name string
}

func genBranchOps(maxLen int) gopter.Gen {
	branchNames := []string{"main", "dev", "feature-a", "feature-b", "fix-1"}

	return func(params *gopter.GenParameters) *gopter.GenResult {
		n := int(params.NextUint64()%uint64(maxLen)) + 1
		ops := make([]branchOp, n)
		for i := range ops {
			kindVal := params.NextUint64() % 3
			nameIdx := params.NextUint64() % uint64(len(branchNames))
			ops[i] = branchOp{
				kind: branchOpKind(kindVal),
				name: branchNames[nameIdx],
			}
		}
		return gopter.NewGenResult(ops, gopter.NoShrinker)
	}
}
