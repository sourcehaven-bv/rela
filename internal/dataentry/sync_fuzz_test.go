package dataentry

import (
	"fmt"
	"testing"
	"time"
)

// FuzzSyncManagerOperations feeds random operation sequences to a
// SyncManager backed by a MemoryGitBackend. It verifies that no
// operation sequence causes panics, deadlocks, or invalid states.
//
// Run with: go test -fuzz=FuzzSyncManagerOperations -fuzztime=30s ./internal/dataentry/
func FuzzSyncManagerOperations(f *testing.F) {
	// Seed corpus: representative operation sequences
	seeds := [][]byte{
		// Single commit + push
		{0xD0, 0x00, 0x50},
		// Multiple commits (squash path)
		{0xD0, 0x00, 0x01, 0x02, 0x50},
		// Commit + set behind + push (rebase path)
		{0xD0, 0x00, 0xE0, 0x50},
		// Branch create + switch
		{0x90, 0xB0},
		// Async commits + close
		{0xD0, 0x30, 0x31, 0x32},
		// Fast-forward pull
		{0xE2, 0x70},
		// Rebase conflict path
		{0xD0, 0x00, 0xE1, 0x50},
		// All operations interleaved
		{0xD0, 0x00, 0x30, 0x90, 0xB0, 0xE1, 0x50, 0x70, 0x00, 0x50},
		// Empty (no-op)
		{},
		// Long commit sequence
		{0xD0, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09},
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		mem := NewMemoryGitBackend("/tmp/fuzz", true)
		s := NewSyncManager(mem, SyncOptions{})

		branchCounter := 0
		branchNames := []string{"main"} // track created branches

		// Process each byte as an operation
		for _, b := range data {
			switch {
			case b <= 0x2F:
				// Commit (synchronous)
				_ = s.Commit(fmt.Sprintf("rela: fuzz commit %d", b))

			case b <= 0x4F:
				// CommitAsync
				s.CommitAsync(fmt.Sprintf("rela: fuzz async %d", b))

			case b <= 0x6F:
				// Push
				_ = s.Push()

			case b <= 0x8F:
				// Pull
				_ = s.Pull()

			case b <= 0xAF:
				// CreateBranch
				name := fmt.Sprintf("fuzz-branch-%d", branchCounter)
				branchCounter++
				if err := s.CreateBranch(name); err == nil {
					branchNames = append(branchNames, name)
				}

			case b <= 0xCF:
				// SwitchBranch (pick from known branches)
				if len(branchNames) > 0 {
					idx := int(b) % len(branchNames)
					_ = s.SwitchBranch(branchNames[idx])
				}

			case b <= 0xDF:
				// Set dirty
				mem.clean = false

			case b <= 0xEF:
				// Set behind count (0..15)
				mem.behind = int(b & 0x0F)

			default:
				// no-op / padding
			}
		}

		// Verify invariants after all operations
		state := s.State()
		if !validStates[state] {
			t.Errorf("invalid state after operations: %q", state)
		}

		branch := s.Branch()
		if branch == "" {
			t.Error("branch is empty after operations")
		}

		status := s.Status()
		if status.Unpushed < 0 {
			t.Errorf("negative unpushed count: %d", status.Unpushed)
		}
		if !status.Enabled {
			t.Error("expected enabled=true")
		}

		// Close with timeout to detect deadlocks
		done := make(chan struct{})
		go func() {
			s.Close()
			close(done)
		}()
		select {
		case <-done:
			// ok
		case <-time.After(5 * time.Second):
			t.Fatal("Close() deadlocked")
		}
	})
}
