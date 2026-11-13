package flowcfg

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
)

// Job steps (TKT-PY2G11W).
//
// A state with `job:` runs one target through the job queue once its
// outputs are written, and follows its `done` branch when the target
// succeeds. The engine owns the retries: each attempt is one queue job.

// JobTarget is what a job step runs.
type JobTarget string

const (
	// JobScript runs a Lua script under scripts/.
	JobScript JobTarget = "script"
	// JobAction runs an action data-entry.yaml declares.
	JobAction JobTarget = "action"
	// JobWebhook posts the run's ids to a URL (`kind: webhook`).
	JobWebhook JobTarget = "webhook"
)

// Job is a state's `job:`.
type Job struct {
	Target JobTarget
	// Script is the path under scripts/, for [JobScript].
	Script string
	// Capabilities are what the script may reach, for [JobScript]. An
	// action declares its own.
	Capabilities metamodel.Capabilities
	// Action is the data-entry.yaml action id, for [JobAction].
	Action string
	// URL is where a webhook posts, for [JobWebhook].
	URL string
	// Secret names the global secret that signs a webhook; empty for an
	// unsigned one.
	Secret string
	// Idempotent lets an attempt whose outcome is unknown (a timeout, a
	// lost worker) run again.
	Idempotent bool
	// Retry bounds the attempts.
	Retry Retry
}

// Retry is the retry policy of a job step.
type Retry struct {
	// Attempts counts every execution, the first included: 1 to
	// [MaxJobAttempts].
	Attempts int
	// Initial is the wait before the second attempt.
	Initial time.Duration
	// Factor multiplies the wait for each attempt after that.
	Factor float64
	// Max caps the wait.
	Max time.Duration
	// Timeout bounds the wait for a worker, and then the attempt from its
	// start.
	Timeout time.Duration
}

// Delay is the wait after failed failed attempts, failed at least 1.
func (r Retry) Delay(failed int) time.Duration {
	d := float64(r.Initial)
	for i := 1; i < failed; i++ {
		if d *= r.Factor; d >= float64(r.Max) {
			return r.Max
		}
	}
	return min(time.Duration(d), r.Max)
}

// Limits and defaults of a job step's retry policy.
const (
	MaxJobAttempts     = 10
	MaxJobTimeout      = 10 * time.Minute
	MinJobTimeout      = time.Second
	MaxJobBackoff      = time.Hour
	MaxJobBackoffScale = 10
	maxScriptPath      = 256
	maxWebhookURL      = 2048
)

// DefaultRetry is the policy of a job that does not set `retry:`.
var DefaultRetry = Retry{
	Attempts: 3, Initial: 10 * time.Second, Factor: 2, Max: 5 * time.Minute, Timeout: 5 * time.Minute,
}

// rawJob is a state's `job:`.
type rawJob struct {
	Script       string                  `yaml:"script"`
	Action       string                  `yaml:"action"`
	Kind         string                  `yaml:"kind"`
	URL          string                  `yaml:"url"`
	Secret       string                  `yaml:"secret"`
	Capabilities *metamodel.Capabilities `yaml:"capabilities"`
	Idempotent   bool                    `yaml:"idempotent"`
	// Retry is false or a mapping (rawRetry).
	Retry yaml.Node `yaml:"retry"`
}

type rawRetry struct {
	Attempts *int        `yaml:"attempts"`
	Backoff  *rawBackoff `yaml:"backoff"`
	Timeout  string      `yaml:"timeout"`
}

type rawBackoff struct {
	Initial string   `yaml:"initial"`
	Factor  *float64 `yaml:"factor"`
	Max     string   `yaml:"max"`
}

// actionIDPattern is the grammar of a data-entry.yaml action id.
var actionIDPattern = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

// jobLoader compiles job steps for a [loader].
type jobLoader struct{ *loader }

// job compiles a state's `job:` and checks that the state can carry one.
func (l jobLoader) job(key string, n *yaml.Node, raw *rawState) *Job {
	if n.Kind == 0 || n.Kind == yaml.ScalarNode && n.Tag == "!!null" {
		return nil
	}
	var r rawJob
	if err := decodeNode(n, &r); err != nil {
		l.add(key, err.Error())
		return nil
	}
	l.jobState(key, raw)
	j := &Job{Idempotent: r.Idempotent, Retry: l.retry(key+".retry", &r.Retry)}
	set := 0
	for _, v := range []string{r.Script, r.Action, r.Kind} {
		if v != "" {
			set++
		}
	}
	if set != 1 {
		l.add(key, "names exactly one of script:, action: or kind:")
		return nil
	}
	if r.Capabilities != nil && r.Script == "" {
		l.add(key+".capabilities", "only a script job declares capabilities; an action declares its own")
	}
	if (r.URL != "" || r.Secret != "") && r.Kind == "" {
		l.add(key, "url: and secret: belong to kind: webhook")
	}
	switch {
	case r.Script != "":
		j.Target, j.Script = JobScript, r.Script
		if r.Capabilities != nil {
			j.Capabilities = *r.Capabilities
		}
		l.scriptPath(key+".script", r.Script)
	case r.Action != "":
		j.Target, j.Action = JobAction, r.Action
		if !actionIDPattern.MatchString(r.Action) {
			l.add(key+".action", "an action id is 1 to 64 lower-case letters, digits, '-' or '_'")
		}
	default:
		l.webhook(key, &r, j)
	}
	return j
}

// jobState refuses what a job state cannot also be: final, waiting for a
// person, moved by a change or a timer. Its way out is `done`, and the last
// `done` branch has no guard, so a success always moves the run on.
func (l jobLoader) jobState(key string, raw *rawState) {
	switch {
	case raw.Final:
		l.add(key, "a final state ends the run; it cannot run a job")
		return
	case len(raw.WaitingFor) > 0 || len(raw.Writes) > 0:
		l.add(key, "a job state waits for its job, not for a person: drop waiting_for: and writes:")
	case raw.After.Kind != 0:
		l.add(key, "a job state has no after: timers; the job's retry: bounds it")
	}
	for ev := range raw.On {
		if Event(ev) != EventDone {
			l.add(key, "a job state moves on done only, not on "+ev)
		}
	}
	n, ok := raw.On[string(EventDone)]
	if !ok {
		l.add(key, "a job state needs on.done: where the run goes when the job succeeds")
		return
	}
	last := &n
	if n.Kind == yaml.SequenceNode && len(n.Content) > 0 {
		last = n.Content[len(n.Content)-1]
	}
	var b rawBranch
	if err := decodeNode(last, &b); err == nil && b.When != "" {
		l.add(key, "the last on.done branch of a job state has no when:, so a success always moves the run on")
	}
}

// scriptPath checks a script path: relative, clean, under scripts/, a
// .lua file.
func (l jobLoader) scriptPath(key, p string) {
	switch {
	case len(p) > maxScriptPath:
		l.add(key, fmt.Sprintf("is longer than %d characters", maxScriptPath))
	case strings.ContainsAny(p, "\\\x00") || path.IsAbs(p) || path.Clean(p) != p ||
		p == ".." || strings.HasPrefix(p, "../"):
		l.add(key, "is a clean relative path under scripts/")
	case path.Ext(p) != ".lua":
		l.add(key, "names a .lua file")
	}
}

// webhook compiles `kind: webhook`.
func (l jobLoader) webhook(key string, r *rawJob, j *Job) {
	if r.Kind != string(JobWebhook) {
		l.add(key+".kind", fmt.Sprintf("is %q; the built-in kind is webhook", r.Kind))
		return
	}
	j.Target, j.URL, j.Secret = JobWebhook, r.URL, r.Secret
	u, err := url.Parse(r.URL)
	switch {
	case r.URL == "":
		l.add(key+".url", "is required: where the webhook posts")
	case len(r.URL) > maxWebhookURL:
		l.add(key+".url", fmt.Sprintf("is longer than %d characters", maxWebhookURL))
	case err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil ||
		u.Fragment != "":
		l.add(key+".url", "is an http or https URL with a host, without credentials or a fragment")
	}
	if r.Secret != "" && !secrets.ScriptReadable(r.Secret) {
		l.add(key+".secret", fmt.Sprintf("names %q, which no job may read", r.Secret))
	}
}

// retry compiles `retry:`: false, or a mapping that tunes the defaults.
func (l jobLoader) retry(key string, n *yaml.Node) Retry {
	r := DefaultRetry
	switch {
	case n.Kind == 0, n.Kind == yaml.ScalarNode && n.Tag == "!!null":
		return r
	case n.Kind == yaml.ScalarNode && n.Tag == "!!bool" && n.Value == "false":
		r.Attempts = 1
		return r
	case n.Kind != yaml.MappingNode:
		l.add(key, "is false, or a mapping with attempts:, backoff: and timeout:")
		return r
	}
	var raw rawRetry
	if err := decodeNode(n, &raw); err != nil {
		l.add(key, err.Error())
		return r
	}
	if raw.Attempts != nil {
		if *raw.Attempts < 1 || *raw.Attempts > MaxJobAttempts {
			l.add(key+".attempts", fmt.Sprintf("is 1 to %d: every execution counts, the first included",
				MaxJobAttempts))
		}
		r.Attempts = *raw.Attempts
	}
	if raw.Timeout != "" {
		r.Timeout = l.fixedPeriod(key+".timeout", raw.Timeout, MinJobTimeout, MaxJobTimeout)
	}
	if b := raw.Backoff; b != nil {
		if b.Initial != "" {
			r.Initial = l.fixedPeriod(key+".backoff.initial", b.Initial, time.Second, MaxJobBackoff)
		}
		if b.Max != "" {
			r.Max = l.fixedPeriod(key+".backoff.max", b.Max, time.Second, MaxJobBackoff)
		}
		if b.Factor != nil {
			if *b.Factor < 1 || *b.Factor > MaxJobBackoffScale {
				l.add(key+".backoff.factor", fmt.Sprintf("is 1 to %d", MaxJobBackoffScale))
			}
			r.Factor = *b.Factor
		}
	}
	if r.Max < r.Initial {
		l.add(key+".backoff", "max is shorter than initial")
	}
	return r
}

// fixedPeriod reads a period of seconds, minutes or hours from lo to hi.
func (l jobLoader) fixedPeriod(key, raw string, lo, hi time.Duration) time.Duration {
	p, err := flow.ParsePeriod(raw)
	if err != nil {
		l.add(key, err.Error())
		return lo
	}
	d, fixed := p.Fixed()
	if !fixed || d < lo || d > hi {
		l.add(key, fmt.Sprintf("is a fixed period from %s to %s", lo, hi))
		return lo
	}
	return d
}
