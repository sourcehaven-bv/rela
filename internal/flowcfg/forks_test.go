package flowcfg

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// forkFlow is a flow that forks into an IT branch of two states and a
// legal branch of one, joined at joined.
const forkFlow = `flows:
  hire:
    subject: person
    run_as: system:onboarding-bot
    initial: start
    states:
      start:
        on:
          change: { to: [it, legal], when: "entity.status == 'hired'" }
      it:
        waiting_for: [hr]
        on:
          change: { to: it_done, roles: [hr] }
      it_done:
        on:
          done: { to: joined }
      legal:
        on:
          done: { to: joined }
      joined:
        join: all
        on:
          done: { to: finished }
      finished:
        final: true
`

func parseFork(t *testing.T, edits ...[2]string) (*Compiled, error) {
	t.Helper()
	src := forkFlow
	for _, e := range edits {
		if !strings.Contains(src, e[0]) {
			t.Fatalf("fork flow has no %q", e[0])
		}
		src = strings.Replace(src, e[0], e[1], 1)
	}
	return Parse([]byte(src), testMeta(t), testPolicy(t))
}

func TestParse_Fork(t *testing.T) {
	c, err := parseFork(t)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	f, _ := c.Flow("hire")
	forks := f.Forks()
	if len(forks) != 1 {
		t.Fatalf("forks = %+v; want one", forks)
	}
	fk := forks[0]
	if fk.State != "start" || !slices.Equal(fk.Targets, []string{"it", "legal"}) || fk.Join != "joined" || !fk.Waits {
		t.Errorf("fork = %+v", fk)
	}
	for state, branch := range map[string]string{"it": "it", "it_done": "it", "legal": "legal"} {
		got, b, ok := f.Region(state)
		if !ok || b != branch || got.Join != "joined" {
			t.Errorf("Region(%s) = %+v %q %v; want branch %s", state, got, b, ok, branch)
		}
	}
	for _, state := range []string{"start", "joined", "finished"} {
		if _, _, ok := f.Region(state); ok {
			t.Errorf("Region(%s) is in a branch", state)
		}
	}
	finished, _ := f.State("finished")
	if _, ok := finished.Join(); ok {
		t.Error("finished.Join() is a join")
	}
	joined, _ := f.State("joined")
	if j, ok := joined.Join(); !ok || j.Any || j.When != nil {
		t.Errorf("joined.Join() = %+v %v; want all", j, ok)
	}
	start, _ := f.State("start")
	if b := start.Branches(EventChange)[0]; b.To != "" || !slices.Equal(b.Targets(), []string{"it", "legal"}) {
		t.Errorf("fork branch = %+v", b)
	}
	m := f.MermaidAt("it_done", "legal")
	if !strings.Contains(m, "class s3,s2 current") {
		t.Errorf("MermaidAt marks not both branches:\n%s", m)
	}
	if !strings.Contains(m, "(parallel)") {
		t.Errorf("mermaid lacks the parallel label:\n%s", m)
	}
}

// TestParse_ForkAlternatives: two branches of a state that fork to the
// same states, in any order, are one fork.
func TestParse_ForkAlternatives(t *testing.T) {
	c, err := parseFork(t, [2]string{
		`change: { to: [it, legal], when: "entity.status == 'hired'" }`,
		`change: [{ to: [it, legal], roles: [hr] }, { to: [legal, it], roles: [manager] }]`,
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	f, _ := c.Flow("hire")
	if got := f.Forks(); len(got) != 1 {
		t.Fatalf("forks = %+v; want one", got)
	}
}

func TestParse_JoinKinds(t *testing.T) {
	tests := []struct {
		name, join string
		any, when  bool
	}{
		{"any", "join: any", true, false},
		{"when", `join: { when: "arrived >= 1 and entity.documents == true" }`, false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := parseFork(t, [2]string{"join: all", tc.join})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			f, _ := c.Flow("hire")
			st, _ := f.State("joined")
			j, ok := st.Join()
			if !ok || j.Any != tc.any || (j.When != nil) != tc.when {
				t.Errorf("Join() = %+v %v", j, ok)
			}
		})
	}
}

func TestParse_ForkProblems(t *testing.T) {
	const fork = "flows.hire.states.start.on.change"
	tests := []struct {
		name  string
		edits [][2]string
		key   string
		want  string
	}{
		{"one target", [][2]string{{"to: [it, legal]", "to: [it]"}}, fork + ".to", "at least two"},
		{"duplicate target", [][2]string{{"to: [it, legal]", "to: [it, it]"}}, fork + ".to", "twice"},
		{"too many targets", [][2]string{{"to: [it, legal]", "to: [it, legal, a, b, c, d, e, f, g]"}},
			fork + ".to", "at most 8"},
		{"unknown target", [][2]string{{"to: [it, legal]", "to: [it, lega]"}}, fork + ".to[1]", "does not declare"},
		{"target is final", [][2]string{{"to: [it, legal]", "to: [it, legal, finished]"}}, fork, "final or join"},
		{"target is the join", [][2]string{{"to: [it, legal]", "to: [it, legal, joined]"}}, fork, "final or join"},
		{"bad join", [][2]string{{"join: all", "join: some"}}, "flows.hire.states.joined.join", "all, any"},
		{"join without when", [][2]string{{"join: all", "join: { }"}}, "flows.hire.states.joined.join.when",
			"required"},
		{"join when does not compile", [][2]string{{"join: all", `join: { when: "arrived == 'x'" }`}},
			"flows.hire.states.joined.join.when", "does not compile"},
		{"join on a final state", [][2]string{{"final: true", "final: true\n        join: all"}},
			"flows.hire.states.finished.join", "final"},
		{"orphan join", [][2]string{{"join: all\n        on:\n          done: { to: finished }",
			"join: all\n        on:\n          done: { to: spare }\n      spare:\n        join: any\n" +
				"        on:\n          done: { to: finished }"}},
			"flows.hire.states.spare.join", "closes no fork"},
		{"several joins", [][2]string{{"done: { to: finished }", "done: { to: finished }\n      spare:\n        join: all\n" +
			"        on:\n          done: { to: finished }"}, {"change: { to: it_done, roles: [hr] }",
			"change: { to: it_done, roles: [hr] }\n          done: { to: spare }"}},
			fork, "several joins"},
		{"branch reaches no join", [][2]string{{"done: { to: joined }\n      legal", "done: { to: finished }\n      legal"}},
			fork, "reaches no join"},
		{"branches share a state", [][2]string{{"to: [it, legal]", "to: [it, it_done]"}}, fork, "branches share"},
		{"branch loops to the fork", [][2]string{{"to: it_done, roles: [hr]", "to: start, roles: [hr]"}}, fork,
			"leads back to"},
		{"nested fork", [][2]string{{"to: it_done, roles: [hr]", "to: [it_done, legal2], roles: [hr]"},
			{"      legal:\n", "      legal2:\n        on:\n          done: { to: joined }\n      legal:\n"}},
			fork, "forks do not nest"},
		{"edge into a branch", [][2]string{{"done: { to: finished }", "done: { to: finished }\n          change: { to: legal }"}},
			fork, "a state of a branch it is not in"},
		{"edge between branches", [][2]string{{"done: { to: joined }\n      legal", "done: { to: legal }\n      legal"}},
			fork, "branches share"},
		{"edge into the join", [][2]string{{"to: [it, legal], when", "to: [it, legal], when"},
			{"      start:\n        on:\n", "      start:\n        on:\n          done: { to: joined }\n"}},
			fork, "from outside"},
		{"stuck branch state", [][2]string{{"change: { to: it_done, roles: [hr] }",
			"change: { to: it_done, roles: [hr] }\n          done: { to: loop }"},
			{"      legal:\n", "      loop:\n        on:\n          change: { to: loop }\n      legal:\n"}},
			fork, "reaches neither"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseFork(t, tc.edits...)
			var pe *Error
			if !errors.As(err, &pe) {
				t.Fatalf("err = %v, want a load error", err)
			}
			if !strings.Contains(err.Error(), tc.key) || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v\nwant %q at %s", err, tc.want, tc.key)
			}
		})
	}
}
