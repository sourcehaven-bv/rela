package tenant

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeDue struct {
	due []DueTenant
	err error
}

func (f *fakeDue) DueTenants(context.Context, time.Time) ([]DueTenant, error) { return f.due, f.err }

type fakeSweepReg struct {
	mu       sync.Mutex
	resident map[string]bool
	opened   []string
	held     int
	failOrg  string
}

func (f *fakeSweepReg) IsResident(org string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.resident[org]
}

func (f *fakeSweepReg) acquireFor(_ context.Context, org string) (func(), error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if org == f.failOrg {
		return nil, errors.New("boom")
	}
	f.opened = append(f.opened, org)
	f.held++
	return func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.held--
	}, nil
}

func dueOrgs(orgs ...string) *fakeDue {
	f := &fakeDue{}
	for _, o := range orgs {
		f.due = append(f.due, DueTenant{OrgID: o})
	}
	return f
}

// TestSweeper_Pass pins which due tenants one pass opens: not resident ones,
// at most perPass, and none inside its cooldown (TKT-ZOU5BCT).
func TestSweeper_Pass(t *testing.T) {
	t.Parallel()
	reg := &fakeSweepReg{resident: map[string]bool{"b": true}, failOrg: "c"}
	now := time.Unix(1000, 0)
	s := newSweeper(dueOrgs("a", "b", "c", "d", "e"), reg, func() time.Time { return now })
	s.hold = time.Millisecond
	ctx := t.Context()

	if n := s.Pass(ctx); n != 2 {
		t.Fatalf("first pass opened %d, want 2", n)
	}
	s.held.Wait()
	if got := reg.opened; len(got) != 2 || got[0] != "a" || got[1] != "d" {
		t.Fatalf("opened %v, want [a d]: b is resident and c fails", got)
	}
	if reg.held != 0 {
		t.Errorf("%d leases still held after the hold", reg.held)
	}

	// Within the cooldown a, c and d are left alone; e is next.
	if n := s.Pass(ctx); n != 1 || reg.opened[2] != "e" {
		t.Fatalf("second pass opened %d (%v), want e only", n, reg.opened)
	}
	s.held.Wait()
	if n := s.Pass(ctx); n != 0 {
		t.Errorf("third pass inside the cooldown opened %d, want 0", n)
	}

	now = now.Add(sweepCooldown)
	if n := s.Pass(ctx); n != 2 {
		t.Errorf("pass after the cooldown opened %d, want 2", n)
	}
	s.held.Wait()
}

// TestSweeper_BacksOff pins that an org whose due time stays the same after
// an open waits twice as long each time, up to the cap, and that work which
// moved on resets the wait.
func TestSweeper_BacksOff(t *testing.T) {
	t.Parallel()
	reg := &fakeSweepReg{}
	now := time.Unix(100000, 0)
	due := &fakeDue{due: []DueTenant{{OrgID: "a", DueAt: time.Unix(500, 0)}}}
	s := newSweeper(due, reg, func() time.Time { return now })
	s.hold = time.Millisecond
	ctx := t.Context()

	wait := sweepCooldown
	for range 8 {
		if n := s.Pass(ctx); n != 1 {
			t.Fatalf("pass after a wait of %v opened %d, want 1", wait, n)
		}
		s.held.Wait()
		now = now.Add(wait - time.Second)
		if n := s.Pass(ctx); n != 0 {
			t.Fatalf("pass inside a wait of %v opened %d, want 0", wait, n)
		}
		now = now.Add(time.Second)
		wait = min(2*wait, sweepMaxCooldown)
	}
	if wait != sweepMaxCooldown {
		t.Fatalf("wait = %v, want the cap", wait)
	}

	// The work moved on: the next open waits the base cooldown again.
	if n := s.Pass(ctx); n != 1 {
		t.Fatal("pass at the cap opened nothing")
	}
	s.held.Wait()
	due.due[0].DueAt = time.Unix(900, 0)
	now = now.Add(sweepMaxCooldown)
	if n := s.Pass(ctx); n != 1 {
		t.Fatal("pass for moved work opened nothing")
	}
	s.held.Wait()
	now = now.Add(sweepCooldown)
	if n := s.Pass(ctx); n != 1 {
		t.Error("work that moved on still waited the doubled cooldown")
	}
	s.held.Wait()
}

// TestSweeper_ListFailure pins that a failed listing opens nothing.
func TestSweeper_ListFailure(t *testing.T) {
	t.Parallel()
	reg := &fakeSweepReg{}
	s := newSweeper(&fakeDue{err: errors.New("down")}, reg, time.Now)
	if n := s.Pass(t.Context()); n != 0 || len(reg.opened) != 0 {
		t.Errorf("pass over a failed listing opened %d", n)
	}
}

// TestSweeper_RunReleasesOnCancel pins that Run ends with ctx and releases
// what it holds before it returns.
func TestSweeper_RunReleasesOnCancel(t *testing.T) {
	t.Parallel()
	reg := &fakeSweepReg{}
	s := newSweeper(dueOrgs("a"), reg, time.Now)
	s.hold = time.Hour
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { s.Run(ctx, time.Millisecond); close(done) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		reg.mu.Lock()
		n := len(reg.opened)
		reg.mu.Unlock()
		if n > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
	if reg.held != 0 || len(reg.opened) != 1 {
		t.Errorf("after Run: held %d, opened %d; want 0 and 1", reg.held, len(reg.opened))
	}
}

func TestNewSweeper_RejectsNil(t *testing.T) {
	t.Parallel()
	if _, err := NewSweeper[*closeOnly](nil, nil); err == nil {
		t.Error("NewSweeper(nil, nil) succeeded")
	}
}

type closeOnly struct{}

func (*closeOnly) Close() error { return nil }
