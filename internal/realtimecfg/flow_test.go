package realtimecfg

import (
	"slices"
	"testing"
)

// TestSession_FlowMirrorsCompiled pins that the engine's definition carries
// the same states, timers and reminders as the compiled session.
func TestSession_FlowMirrorsCompiled(t *testing.T) {
	c, err := Parse([]byte(golden(t)), testMeta(t), testPolicy(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ct, _ := c.ChannelType("breach-response")
	s := ct.Session()
	d := s.Flow()
	if d == nil {
		t.Fatal("Flow() = nil")
	}
	if d.Initial() != s.Initial() {
		t.Errorf("initial = %q; want %q", d.Initial(), s.Initial())
	}
	for _, st := range s.States() {
		fs, ok := d.State(st.Name)
		if !ok {
			t.Errorf("state %q missing from the definition", st.Name)
			continue
		}
		if fs.Final() != st.Final {
			t.Errorf("state %q final = %v; want %v", st.Name, fs.Final(), st.Final)
		}
		var want, got []string
		for _, tm := range st.Timers() {
			want = append(want, tm.Name+">"+tm.To+"/"+string(tm.Since))
		}
		for _, tm := range fs.Timers() {
			got = append(got, tm.Name+">"+tm.To+"/"+string(tm.Since))
		}
		if !slices.Equal(got, want) {
			t.Errorf("state %q timers = %v; want %v", st.Name, got, want)
		}
	}
	rs := s.Reminders()
	if len(d.Reminders()) != len(rs) {
		t.Fatalf("reminders = %d; want %d", len(d.Reminders()), len(rs))
	}
	for i, r := range rs {
		fr := d.Reminders()[i]
		if fr.Name != r.Name || !slices.Equal(fr.In(), r.In()) || fr.Every != r.Every ||
			fr.First != r.First || fr.Max != r.Max {

			t.Errorf("reminder %d = %+v; want %+v", i, fr, r.Reminder)
		}
	}
}
