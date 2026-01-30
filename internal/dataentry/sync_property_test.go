package dataentry

import (
	"fmt"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// --- Property tests for SyncManager state machine ---

// syncOp represents a single operation on the SyncManager.
type syncOp struct {
	kind      syncOpKind
	message   string
	branch    string
	setBehind int
}

type syncOpKind int

const (
	syncOpCommit syncOpKind = iota
	syncOpPush
	syncOpPull
	syncOpCreateBranch
	syncOpSwitchBranch
	syncOpSetDirty
	syncOpSetBehind
)

// validStates is the set of all valid SyncState values.
var validStates = map[SyncState]bool{
	SyncDisabled: true,
	SyncClean:    true,
	SyncAhead:    true,
	SyncSyncing:  true,
	SyncError:    true,
	SyncConflict: true,
	SyncOffline:  true,
}

// applySyncOps creates a SyncManager, runs a sequence of operations,
// and returns the manager and backend for inspection.
func applySyncOps(ops []syncOp) *SyncManager {
	mem := NewMemoryGitBackend("/tmp/test", true)
	s := NewSyncManager(mem, SyncOptions{})

	for _, op := range ops {
		switch op.kind {
		case syncOpCommit:
			_ = s.Commit(op.message)
		case syncOpPush:
			_ = s.Push()
		case syncOpPull:
			_ = s.Pull()
		case syncOpCreateBranch:
			_ = s.CreateBranch(op.branch)
		case syncOpSwitchBranch:
			_ = s.SwitchBranch(op.branch)
		case syncOpSetDirty:
			mem.clean = false
		case syncOpSetBehind:
			mem.behind = op.setBehind
		}
	}

	return s
}

// TestPropertySyncManagerStateAlwaysValid verifies that after any sequence
// of operations, the SyncManager's state is always a valid SyncState value.
func TestPropertySyncManagerStateAlwaysValid(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 200

	properties := gopter.NewProperties(parameters)

	properties.Property("state is always valid after any operation sequence", prop.ForAll(
		func(ops []syncOp) bool {
			s := applySyncOps(ops)
			defer s.Close()

			state := s.State()
			return validStates[state]
		},
		genSyncOps(15),
	))

	properties.TestingRun(t)
}

// TestPropertySyncManagerBranchNeverEmpty verifies that the branch name
// is never empty when the manager is enabled.
func TestPropertySyncManagerBranchNeverEmpty(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 200

	properties := gopter.NewProperties(parameters)

	properties.Property("branch is never empty when enabled", prop.ForAll(
		func(ops []syncOp) bool {
			s := applySyncOps(ops)
			defer s.Close()

			return s.Branch() != ""
		},
		genSyncOps(15),
	))

	properties.TestingRun(t)
}

// TestPropertySyncManagerUnpushedNonNegative verifies that the unpushed
// count in the status is never negative.
func TestPropertySyncManagerUnpushedNonNegative(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 200

	properties := gopter.NewProperties(parameters)

	properties.Property("unpushed is never negative", prop.ForAll(
		func(ops []syncOp) bool {
			s := applySyncOps(ops)
			defer s.Close()

			return s.Status().Unpushed >= 0
		},
		genSyncOps(15),
	))

	properties.TestingRun(t)
}

// TestPropertySyncManagerCommitProducesAhead verifies that committing
// with a dirty tree always transitions state to SyncAhead.
func TestPropertySyncManagerCommitProducesAhead(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 200

	properties := gopter.NewProperties(parameters)

	properties.Property("commit with dirty tree produces SyncAhead", prop.ForAll(
		func(msg string) bool {
			mem := NewMemoryGitBackend("/tmp/test", true)
			mem.clean = false
			s := NewSyncManager(mem, SyncOptions{})
			defer s.Close()

			err := s.Commit(msg)
			if err != nil {
				return false
			}

			return s.State() == SyncAhead
		},
		gen.AlphaString().SuchThat(func(s string) bool { return s != "" }),
	))

	properties.TestingRun(t)
}

// TestPropertySyncManagerDisabledInert verifies that all operations
// on a disabled SyncManager are no-ops (return nil, don't panic).
func TestPropertySyncManagerDisabledInert(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("disabled manager: all ops are no-ops", prop.ForAll(
		func(ops []syncOp) bool {
			s := NewSyncManager(nil, SyncOptions{})

			for _, op := range ops {
				switch op.kind {
				case syncOpCommit:
					if err := s.Commit(op.message); err != nil {
						return false
					}
				case syncOpPush:
					if err := s.Push(); err != nil {
						return false
					}
				case syncOpPull:
					if err := s.Pull(); err != nil {
						return false
					}
				default:
					// CommitAsync, other ops don't return errors
					s.CommitAsync("disabled-noop")
				}
			}

			// Must remain disabled
			if s.State() != SyncDisabled {
				return false
			}
			// Close must not panic
			s.Close()
			return true
		},
		genSyncOps(10),
	))

	properties.TestingRun(t)
}

// TestPropertySyncManagerPushCleanAfterSync verifies that pushing when
// there is nothing behind produces a SyncClean state.
func TestPropertySyncManagerPushCleanAfterSync(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 200

	properties := gopter.NewProperties(parameters)

	properties.Property("push with commits and nothing behind produces clean", prop.ForAll(
		func(n uint8) bool {
			mem := NewMemoryGitBackend("/tmp/test", true)
			mem.clean = false
			s := NewSyncManager(mem, SyncOptions{})
			defer s.Close()

			// Create some commits
			count := int(n%10) + 1
			for i := 0; i < count; i++ {
				_ = s.Commit(fmt.Sprintf("rela: test %d", i))
			}

			// Push (no behind, no errors)
			err := s.Push()
			if err != nil {
				return false
			}

			return s.State() == SyncClean && s.Status().Unpushed == 0
		},
		gen.UInt8(),
	))

	properties.TestingRun(t)
}

// TestPropertySyncManagerSubscribersGetUpdates verifies that subscribers
// receive at least one status update after state-changing operations.
func TestPropertySyncManagerSubscribersGetUpdates(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("subscriber receives updates on state change", prop.ForAll(
		func(msg string) bool {
			mem := NewMemoryGitBackend("/tmp/test", true)
			mem.clean = false
			s := NewSyncManager(mem, SyncOptions{})
			defer s.Close()

			id, ch := s.Subscribe()
			defer s.Unsubscribe(id)

			_ = s.Commit(msg)

			// Drain channel — should have at least one message
			received := 0
			for {
				select {
				case <-ch:
					received++
				default:
					return received > 0
				}
			}
		},
		gen.AlphaString().SuchThat(func(s string) bool { return s != "" }),
	))

	properties.TestingRun(t)
}

// --- Generator for SyncManager operations ---

func genSyncOps(maxLen int) gopter.Gen {
	branchNames := []string{"main", "dev", "feature-x", "fix-1"}
	messages := []string{"rela: create TKT-001", "rela: update TKT-002", "rela: delete TKT-003"}

	return func(params *gopter.GenParameters) *gopter.GenResult {
		n := int(params.NextUint64()%uint64(maxLen)) + 1
		ops := make([]syncOp, n)
		for i := range ops {
			kindVal := params.NextUint64() % 7
			msgIdx := params.NextUint64() % uint64(len(messages))
			branchIdx := params.NextUint64() % uint64(len(branchNames))
			ops[i] = syncOp{
				kind:      syncOpKind(kindVal),
				message:   messages[msgIdx],
				branch:    branchNames[branchIdx],
				setBehind: int(params.NextUint64() % 5),
			}
		}
		return gopter.NewGenResult(ops, gopter.NoShrinker)
	}
}
