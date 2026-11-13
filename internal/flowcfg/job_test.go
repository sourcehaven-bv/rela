package flowcfg

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// jobFlow runs a script job and ends.
const jobFlow = `flows:
  ship:
    subject: person
    run_as: system:onboarding-bot
    initial: send
    states:
      send:
        job:
          script: flows/send.lua
        on:
          done: { to: sent }
      sent:
        final: true
`

func parseJob(t *testing.T, edits ...[2]string) (*Compiled, error) {
	t.Helper()
	src := jobFlow
	for _, e := range edits {
		if !strings.Contains(src, e[0]) {
			t.Fatalf("job flow has no %q", e[0])
		}
		src = strings.Replace(src, e[0], e[1], 1)
	}
	return Parse([]byte(src), testMeta(t), testPolicy(t))
}

func jobOf(t *testing.T, c *Compiled) Job {
	t.Helper()
	f, _ := c.Flow("ship")
	st, _ := f.State("send")
	j, ok := st.Job()
	if !ok {
		t.Fatal("send has no job")
	}
	return j
}

func TestParse_JobDefaults(t *testing.T) {
	c, err := parseJob(t)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	j := jobOf(t, c)
	if j.Target != JobScript || j.Script != "flows/send.lua" || j.Idempotent {
		t.Errorf("job = %+v", j)
	}
	if j.Retry != DefaultRetry {
		t.Errorf("retry = %+v, want %+v", j.Retry, DefaultRetry)
	}
	f, _ := c.Flow("ship")
	if st, _ := f.State("sent"); func() bool { _, ok := st.Job(); return ok }() {
		t.Error("a state without job: has a job")
	}
}

func TestParse_JobTargetsAndRetry(t *testing.T) {
	tests := []struct {
		name  string
		edits [][2]string
		check func(t *testing.T, j Job)
	}{
		{"webhook", [][2]string{{"script: flows/send.lua",
			"kind: webhook\n          url: https://hooks.example.com/x\n          secret: hook_key\n          idempotent: true"}},
			func(t *testing.T, j Job) {
				t.Helper()
				if j.Target != JobWebhook || j.URL != "https://hooks.example.com/x" || j.Secret != "hook_key" ||
					!j.Idempotent {

					t.Errorf("job = %+v", j)
				}
			}},
		{"action", [][2]string{{"script: flows/send.lua", "action: notify-hr"}}, func(t *testing.T, j Job) {
			t.Helper()
			if j.Target != JobAction || j.Action != "notify-hr" {
				t.Errorf("job = %+v", j)
			}
		}},
		{"script capabilities", [][2]string{{"script: flows/send.lua",
			"script: flows/send.lua\n          capabilities: { http: true }"}}, func(t *testing.T, j Job) {
			t.Helper()
			if http, _, _, _, _, _ := j.Capabilities.Fields(); !http {
				t.Errorf("capabilities = %+v", j.Capabilities)
			}
		}},
		{"retry false", [][2]string{{"script: flows/send.lua", "script: flows/send.lua\n          retry: false"}},
			func(t *testing.T, j Job) {
				t.Helper()
				if j.Retry.Attempts != 1 {
					t.Errorf("attempts = %d, want 1", j.Retry.Attempts)
				}
			}},
		{"retry tuned", [][2]string{{"script: flows/send.lua", "script: flows/send.lua\n          retry: " +
			"{ attempts: 5, backoff: { initial: 30s, factor: 3, max: 10m }, timeout: 2m }"}},
			func(t *testing.T, j Job) {
				t.Helper()
				want := Retry{Attempts: 5, Initial: 30 * time.Second, Factor: 3, Max: 10 * time.Minute,
					Timeout: 2 * time.Minute}
				if j.Retry != want {
					t.Errorf("retry = %+v, want %+v", j.Retry, want)
				}
			}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := parseJob(t, tc.edits...)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			tc.check(t, jobOf(t, c))
		})
	}
}

func TestRetry_Delay(t *testing.T) {
	r := Retry{Initial: 10 * time.Second, Factor: 2, Max: time.Minute}
	for failed, want := range map[int]time.Duration{
		1: 10 * time.Second, 2: 20 * time.Second, 3: 40 * time.Second, 4: time.Minute, 50: time.Minute,
	} {
		if got := r.Delay(failed); got != want {
			t.Errorf("Delay(%d) = %s, want %s", failed, got, want)
		}
	}
}

func TestParse_JobProblems(t *testing.T) {
	const job = "flows.ship.states.send.job"
	tests := []struct {
		name  string
		edits [][2]string
		key   string
		want  string
	}{
		{"two targets", [][2]string{{"script: flows/send.lua", "script: flows/send.lua\n          action: x"}},
			job, "exactly one"},
		{"no target", [][2]string{{"script: flows/send.lua", "idempotent: true"}}, job, "exactly one"},
		{"unknown kind", [][2]string{{"script: flows/send.lua", "kind: mail"}}, job + ".kind", "webhook"},
		{"webhook without url", [][2]string{{"script: flows/send.lua", "kind: webhook"}}, job + ".url",
			"required"},
		{"webhook bad scheme", [][2]string{{"script: flows/send.lua", "kind: webhook\n          url: ftp://x/y"}},
			job + ".url", "http or https"},
		{"webhook with credentials", [][2]string{{"script: flows/send.lua",
			"kind: webhook\n          url: https://u:p@x/y"}}, job + ".url", "without credentials"},
		{"webhook token secret", [][2]string{{"script: flows/send.lua",
			"kind: webhook\n          url: https://x/y\n          secret: remote_credential.up"}}, job + ".secret",
			"no job may read"},
		{"url on a script", [][2]string{{"script: flows/send.lua", "script: flows/send.lua\n          url: https://x"}},
			job, "kind: webhook"},
		{"capabilities on an action", [][2]string{{"script: flows/send.lua",
			"action: x\n          capabilities: { http: true }"}}, job + ".capabilities", "only a script"},
		{"bad action id", [][2]string{{"script: flows/send.lua", "action: Bad/Id"}}, job + ".action", "action id"},
		{"script outside scripts", [][2]string{{"script: flows/send.lua", "script: ../x.lua"}}, job + ".script",
			"clean relative path"},
		{"script not lua", [][2]string{{"script: flows/send.lua", "script: x.sh"}}, job + ".script", ".lua"},
		{"attempts zero", [][2]string{{"script: flows/send.lua", "script: x.lua\n          retry: { attempts: 0 }"}},
			job + ".retry.attempts", "1 to 10"},
		{"attempts too many", [][2]string{{"script: flows/send.lua",
			"script: x.lua\n          retry: { attempts: 11 }"}}, job + ".retry.attempts", "1 to 10"},
		{"timeout too long", [][2]string{{"script: flows/send.lua",
			"script: x.lua\n          retry: { timeout: 1h }"}}, job + ".retry.timeout", "fixed period"},
		{"timeout in months", [][2]string{{"script: flows/send.lua",
			"script: x.lua\n          retry: { timeout: 1mo }"}}, job + ".retry.timeout", "fixed period"},
		{"factor too large", [][2]string{{"script: flows/send.lua",
			"script: x.lua\n          retry: { backoff: { factor: 11 } }"}}, job + ".retry.backoff.factor", "1 to 10"},
		{"max below initial", [][2]string{{"script: flows/send.lua",
			"script: x.lua\n          retry: { backoff: { initial: 1m, max: 10s } }"}}, job + ".retry.backoff",
			"shorter than initial"},
		{"retry true", [][2]string{{"script: flows/send.lua", "script: x.lua\n          retry: true"}},
			job + ".retry", "false, or a mapping"},
		{"final job state", [][2]string{{"        on:\n          done: { to: sent }", "        final: true"}},
			job, "final state"},
		{"waiting for a person", [][2]string{{"      send:\n", "      send:\n        waiting_for: [hr]\n"}},
			job, "not for a person"},
		{"a timer", [][2]string{{"      send:\n", "      send:\n        after:\n          t: { period: 1d, to: sent }\n"}},
			job, "no after"},
		{"a change event", [][2]string{{"done: { to: sent }", "done: { to: sent }\n          change: { to: sent }"}},
			job, "done only"},
		{"no done", [][2]string{{"done: { to: sent }", "change: { to: sent }"}}, job, "needs on.done"},
		{"guarded last done", [][2]string{{"done: { to: sent }",
			`done: { to: sent, when: "entity.documents == true" }`}}, job, "no when"},
		{"run_as missing", [][2]string{{"    run_as: system:onboarding-bot\n", ""}}, "flows.ship.run_as",
			"outputs and jobs"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseJob(t, tc.edits...)
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
