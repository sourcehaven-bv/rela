package presence

import (
	"fmt"
	"slices"
	"strconv"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)

func id(user string) Identity { return Identity{User: user, Tool: "data-entry"} }

func slot(conn, sub string) Slot { return Slot{Conn: conn, Sub: sub} }

func users(vs []Viewer) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.User
	}
	return out
}

// Viewers shows a join only after the tick publishes it, and an expired
// grace only after the tick that sweeps it (M1).
func TestTracker_ViewersAsOfLastSweep(t *testing.T) {
	tr := New()
	if !tr.Join("K", slot("c1", "s"), id("alice"), t0) {
		t.Fatal("join refused")
	}
	if got := tr.Viewers("K"); len(got) != 0 {
		t.Fatalf("viewers before the tick = %v, want none", got)
	}
	if got := tr.TakeChanged(t0); !slices.Equal(got, []string{"K"}) {
		t.Fatalf("changed = %v, want [K]", got)
	}
	if got := users(tr.Viewers("K")); !slices.Equal(got, []string{"alice"}) {
		t.Fatalf("viewers = %v, want [alice]", got)
	}

	tr.Leave(slot("c1", "s"), t0, true)
	// Past the grace, but no tick yet: still shown.
	if got := tr.Viewers("K"); len(got) != 1 {
		t.Fatalf("viewers after leave, before the tick = %v, want alice", got)
	}
	if got := tr.TakeChanged(t0.Add(Grace - time.Millisecond)); len(got) != 0 {
		t.Fatalf("changed inside grace = %v, want none", got)
	}
	if got := tr.TakeChanged(t0.Add(Grace)); !slices.Equal(got, []string{"K"}) {
		t.Fatalf("changed at grace end = %v, want [K]", got)
	}
	if got := tr.Viewers("K"); len(got) != 0 {
		t.Fatalf("viewers after grace = %v, want none", got)
	}
	if tr.Len() != 0 || tr.Count(id("alice")) != 0 {
		t.Fatalf("len = %d count = %d, want 0", tr.Len(), tr.Count(id("alice")))
	}
}

func TestTracker_JoinLeave(t *testing.T) {
	cases := []struct {
		name string
		run  func(tr *Tracker) // steps after alice joins K on c1/s at t0
		// wantSince is alice's since on K after the run; zero: not a viewer.
		wantSince time.Time
	}{
		{"rejoin within grace keeps since", func(tr *Tracker) {
			tr.Leave(slot("c1", "s"), t0.Add(time.Second), true)
			tr.Join("K", slot("c2", "s"), id("alice"), t0.Add(10*time.Second))
			tr.TakeChanged(t0.Add(30 * time.Second))
		}, t0},
		{"rejoin after grace starts again", func(tr *Tracker) {
			tr.Leave(slot("c1", "s"), t0, true)
			tr.TakeChanged(t0.Add(Grace))
			tr.Join("K", slot("c2", "s"), id("alice"), t0.Add(20*time.Second))
			tr.TakeChanged(t0.Add(20 * time.Second))
		}, t0.Add(20 * time.Second)},
		{"leave without grace goes at once", func(tr *Tracker) {
			tr.Leave(slot("c1", "s"), t0, false)
			tr.TakeChanged(t0)
		}, time.Time{}},
		{"second tab keeps the entry when one closes", func(tr *Tracker) {
			tr.Join("K", slot("c2", "s"), id("alice"), t0.Add(time.Second))
			tr.Leave(slot("c1", "s"), t0.Add(2*time.Second), false)
			tr.TakeChanged(t0.Add(time.Hour))
		}, t0},
		{"LeaveConn leaves with grace", func(tr *Tracker) {
			tr.LeaveConn("c1", t0)
			tr.TakeChanged(t0.Add(Grace - time.Second))
		}, t0},
		{"LeaveConn grace ends", func(tr *Tracker) {
			tr.LeaveConn("c1", t0)
			tr.TakeChanged(t0.Add(Grace))
		}, time.Time{}},
		{"LeaveConn of another stream is ignored", func(tr *Tracker) {
			tr.LeaveConn("c2", t0)
			tr.TakeChanged(t0.Add(time.Hour))
		}, t0},
		{"slot moving to another key leaves the old one at once", func(tr *Tracker) {
			tr.Join("L", slot("c1", "s"), id("alice"), t0.Add(time.Second))
			tr.TakeChanged(t0.Add(time.Second))
		}, time.Time{}},
		{"repeated join is one entry", func(tr *Tracker) {
			tr.Join("K", slot("c1", "s"), id("alice"), t0.Add(time.Second))
			tr.Leave(slot("c1", "s"), t0.Add(time.Second), false)
			tr.TakeChanged(t0.Add(time.Second))
		}, time.Time{}},
		{"leave of an unknown slot is ignored", func(tr *Tracker) {
			tr.Leave(slot("c9", "s"), t0, false)
			tr.TakeChanged(t0)
		}, t0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tr := New()
			tr.Join("K", slot("c1", "s"), id("alice"), t0)
			tc.run(tr)
			vs := tr.Viewers("K")
			if tc.wantSince.IsZero() {
				if len(vs) != 0 {
					t.Fatalf("viewers = %v, want none", vs)
				}
				return
			}
			if len(vs) != 1 || !vs[0].Since.Equal(tc.wantSince) {
				t.Fatalf("viewers = %v, want alice since %v", vs, tc.wantSince)
			}
		})
	}
}

// Viewers sort by since, then user.
// LeaveConn leaves every slot of its stream, on any key, and only those.
func TestTracker_LeaveConnLeavesOnlyItsSlots(t *testing.T) {
	tr := New()
	tr.Join("K", slot("c1", "a"), id("alice"), t0)
	tr.Join("L", slot("c1", "b"), id("alice"), t0)
	tr.Join("K", slot("c2", "a"), id("bob"), t0)
	tr.LeaveConn("c1", t0)
	tr.TakeChanged(t0.Add(Grace))
	if got := users(tr.Viewers("K")); !slices.Equal(got, []string{"bob"}) {
		t.Fatalf("viewers of K = %v, want [bob]", got)
	}
	if got := tr.Viewers("L"); len(got) != 0 {
		t.Fatalf("viewers of L = %v, want none", got)
	}
	// The stream's slots are gone: a second LeaveConn and a rejoin start
	// afresh.
	tr.LeaveConn("c1", t0.Add(Grace))
	if !tr.Join("L", slot("c1", "b"), id("alice"), t0.Add(Grace)) || tr.Len() != 2 {
		t.Fatalf("rejoin: len = %d, want 2", tr.Len())
	}
}

func TestTracker_ViewerOrder(t *testing.T) {
	tr := New()
	tr.Join("K", slot("c1", "s"), id("carol"), t0.Add(time.Second))
	tr.Join("K", slot("c2", "s"), id("bob"), t0)
	tr.Join("K", slot("c3", "s"), id("alice"), t0.Add(time.Second))
	tr.TakeChanged(t0.Add(time.Second))
	if got := users(tr.Viewers("K")); !slices.Equal(got, []string{"bob", "alice", "carol"}) {
		t.Fatalf("order = %v", got)
	}
}

// S2: churn by one identity never holds more than MaxPerIdentity entries,
// in-grace ones included, and never blocks another identity.
func TestTracker_IdentityCapUnderChurn(t *testing.T) {
	tr := New()
	now := t0
	for round := range 10 {
		for i := range 16 {
			key := fmt.Sprintf("K%d-%d", round, i)
			s := slot("c1", strconv.Itoa(i))
			tr.Leave(s, now, true)
			tr.Join(key, s, id("alice"), now)
			if n := tr.Count(id("alice")); n > MaxPerIdentity {
				t.Fatalf("round %d: alice holds %d entries, cap %d", round, n, MaxPerIdentity)
			}
		}
		now = now.Add(time.Second)
		tr.TakeChanged(now)
	}
	if n := tr.Count(id("alice")); n != MaxPerIdentity {
		t.Fatalf("alice holds %d entries, want the cap %d", n, MaxPerIdentity)
	}
	if !tr.Join("K0-0", slot("c2", "s"), id("bob"), now) {
		t.Fatal("alice's churn blocked bob")
	}
	// When the grace ends the cap frees up again.
	tr.TakeChanged(now.Add(Grace))
	if n := tr.Count(id("alice")); n > liveSlots {
		t.Fatalf("after grace alice holds %d entries, want at most %d", n, liveSlots)
	}
}

// liveSlots is how many slots the churn test keeps open at once.
const liveSlots = 16

// AC6: the entry past MaxEntries is refused; a rejoin of an existing entry
// is not.
func TestTracker_GlobalCap(t *testing.T) {
	tr := New()
	for i := range MaxEntries {
		if !tr.Join("K", slot(strconv.Itoa(i), "s"), id(fmt.Sprint("u", i)), t0) {
			t.Fatalf("join %d refused", i)
		}
	}
	if tr.Join("K", slot("extra", "s"), id("late"), t0) {
		t.Fatalf("join %d accepted past the cap", MaxEntries+1)
	}
	if !tr.Join("K", slot("tab2", "s"), id("u0"), t0) {
		t.Fatal("a second tab of an existing viewer was refused")
	}
	if tr.Len() != MaxEntries {
		t.Fatalf("len = %d, want %d", tr.Len(), MaxEntries)
	}
}

// Viewers returns a copy.
func TestTracker_ViewersIsACopy(t *testing.T) {
	tr := New()
	tr.Join("K", slot("c1", "s"), id("alice"), t0)
	tr.TakeChanged(t0)
	tr.Viewers("K")[0].User = "mallory"
	if got := tr.Viewers("K")[0].User; got != "alice" {
		t.Fatalf("viewer = %q after mutating the copy", got)
	}
}
