//go:build maildemo

package appbuild

// TestDemo_ScheduledMailOverHTTP drives the whole declarative-mail chain end to
// end against a local server standing in for SimpleMailService APIv2:
//
//	schedules.yaml `for_each`  →  one child job per recipient
//	                           →  per-recipient ACL-scoped render
//	                           →  transport: http POST
//
// Build-tag gated (`-tags maildemo`) for the reason internal/mail/manual_e2e_test.go
// states: a manual harness that has to be *remembered* to be deleted is how debug
// code reaches production. CI never builds this file.
//
//	go test -tags maildemo ./internal/appbuild -run TestDemo_ScheduledMail -v
//
// Every request the transport makes is written to /tmp/maildemo-http/ as
// numbered .json files, plus a summary.txt, so the delivered payload can be
// inspected after the run.
//
// # Why this file is in package appbuild rather than appbuild_test
//
// The endpoint of `transport: http` is a compile-time constant on purpose
// (see internal/mail/http.go): an operator-settable URL would turn a
// credential-bearing POST into a redirect primitive. The only override is
// mail.WithHTTPBaseURL, which is Go-only and has no YAML counterpart —
// exactly the seam this demo needs. Services.mail is unexported, so swapping
// in a sender built with that option requires being inside the package. That
// is the same trade internal/mail/export_test.go makes: test-only code in the
// production package, in a _test.go file, so it can never reach a binary.
//
// No production code is modified to make this run.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/mail"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/scheduler"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// demoOutDir is where the captured payloads land for inspection.
const demoOutDir = "/tmp/maildemo-http"

// The project the demo builds.
//
// Two people, two roles, deliberately asymmetric:
//
//   - alice is a `manager`: she may read `task` AND `salary_review`, and she
//     may see every property of a task including `budget`.
//   - bob is a `worker`: he may read `task` only, and `visible:` narrows him to
//     title + due, so `budget` is redacted out of his copy.
//
// Both are selected by the same for_each, both get the same template. The only
// thing separating their two messages is the ACL, which is the point.
const demoMetamodel = `version: "1.0"
entities:
  person:
    label: Person
    plural: people
    id_prefix: "PERS-"
    id_type: sequential
    properties:
      name: {type: string}
      email: {type: string, unique: true}
      active: {type: string}
  task:
    label: Task
    plural: tasks
    id_prefix: "TASK-"
    id_type: sequential
    properties:
      title: {type: string}
      due: {type: string}
      budget: {type: string}
  salary_review:
    label: Salary review
    plural: salary_reviews
    id_prefix: "SAL-"
    id_type: sequential
    properties:
      title: {type: string}
      amount: {type: string}
relations: {}
`

const demoACL = `user_entity_type: person
principal_property: email
roles:
  # The identity the EXPANSION step runs as. It selects recipients and nothing
  # else — the per-recipient render happens under the recipient's own role.
  recipient_selector:
    read: [person]
  manager:
    read: ["*"]
  worker:
    read: [person, task]
    visible:
      task:
        - field: title
        - field: due
assignments:
  system:scheduler: recipient_selector
  PERS-1: manager
  PERS-2: worker
`

const demoTemplates = `mail_templates:
  daily_digest:
    subject: "Your digest"
    intro: "Everything below is scoped to what you can see."
    address_property: email
    sections:
      - title: "Open tasks"
        entity_type: task
        columns: [title, due, budget]
      - title: "Salary reviews"
        entity_type: salary_review
        columns: [title, amount]
`

const demoSchedules = `tasks:
  - name: daily-digest
    template: daily_digest
    every: day
    for_each:
      entity_type: person
      where: ["active = true"]
`

// captured is one request the stub APIv2 server received.
type captured struct {
	Method string
	Path   string
	Auth   string
	// RenderedFor is only set on the script-transport run, where the send
	// script forwards message.rendered_for as a header.
	RenderedFor string
	Raw         []byte
	Body        map[string]any
}

// newAPIv2Stub stands in for https://api.simplemailservice.eu/v2. It records
// every request verbatim and returns 202, the way the real API does.
func newAPIv2Stub(t *testing.T) (srv *httptest.Server, received func() []captured) {
	t.Helper()
	var mu sync.Mutex
	var got []captured

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := make([]byte, 0, 1<<16)
		buf := make([]byte, 4096)
		for {
			n, err := r.Body.Read(buf)
			raw = append(raw, buf[:n]...)
			if err != nil {
				break
			}
		}
		var body map[string]any
		_ = json.Unmarshal(raw, &body)

		mu.Lock()
		got = append(got, captured{
			Method: r.Method, Path: r.URL.Path,
			Auth:        r.Header.Get("Authorization"),
			RenderedFor: r.Header.Get("X-Rendered-For"),
			Raw:         raw, Body: body,
		})
		mu.Unlock()

		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"queued"}`))
	}))
	t.Cleanup(srv.Close)

	received = func() []captured {
		mu.Lock()
		defer mu.Unlock()
		out := make([]captured, len(got))
		copy(out, got)
		return out
	}
	return srv, received
}

// writeDemoProject lays down the on-disk project the demo boots from.
func writeDemoProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	}

	write("metamodel.yaml", demoMetamodel)
	write("acl.yaml", demoACL)
	write("mail-templates.yaml", demoTemplates)
	write("schedules.yaml", demoSchedules)

	// .rela/mail.yaml — the real operator-facing config for transport: http.
	// account_id and the token are read from here and from secrets.yaml; only
	// the base URL is overridden below, and only from Go.
	write(".rela/mail.yaml", `transport: http
account_id: demo-account
from: notifications@example.com
from_name: Rela Demo
base_url: https://rela.example.com
`)
	write(".rela/secrets.yaml", "mail_api_token: demo-token-do-not-log\n")

	// Two recipients and the content they are differently allowed to see.
	write("entities/people/PERS-1.md",
		"---\nid: PERS-1\ntype: person\nname: Alice\nemail: alice@example.test\nactive: \"true\"\n---\n")
	write("entities/people/PERS-2.md",
		"---\nid: PERS-2\ntype: person\nname: Bob\nemail: bob@example.test\nactive: \"true\"\n---\n")
	// A third person who must NOT be selected, so the for_each filter is
	// demonstrably doing something.
	write("entities/people/PERS-3.md",
		"---\nid: PERS-3\ntype: person\nname: Carol\nemail: carol@example.test\nactive: \"false\"\n---\n")

	write("entities/tasks/TASK-1.md",
		"---\nid: TASK-1\ntype: task\ntitle: Ship the mail feature\ndue: \"2026-09-01\"\nbudget: \"EUR 42000\"\n---\n")

	write("entities/salary_reviews/SAL-1.md",
		"---\nid: SAL-1\ntype: salary_review\ntitle: Bob annual review\namount: \"EUR 71000\"\n---\n")

	require.NoError(t, os.MkdirAll(filepath.Join(root, "relations"), 0o750))
	return root
}

func TestDemo_ScheduledMailOverHTTP(t *testing.T) {
	stub, received := newAPIv2Stub(t)
	root := writeDemoProject(t)

	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fs)
	require.NoError(t, err)

	svc, err := New(Config{
		FS: fs, Paths: paths, ScriptEngine: script.NewEngine(), Audit: audit.Nop{},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	// The project's own .rela/mail.yaml already selected transport: http and
	// supplied account_id + token; startMailRuntime built a real HTTPSender
	// from it. Rebuild that same sender with the Go-only base-URL seam so the
	// POST lands on the local stub instead of the real provider. Config,
	// credential resolution, request shape and error handling are untouched.
	require.NotNil(t, svc.mail, "mail runtime did not start; check .rela/mail.yaml")
	require.Equal(t, mail.TransportHTTP, svc.mail.config.Transport)
	httpSender, err := mail.NewHTTPSender(svc.mail.config, mail.WithHTTPBaseURL(stub.URL))
	require.NoError(t, err)
	svc.mail.sender = httpSender

	// Boot a real scheduler over the real job queue and let it run the due task.
	data, err := svc.Config().Load(context.Background(), scheduler.ConfigFile)
	require.NoError(t, err)
	cfg, err := scheduler.ParseConfig(data)
	require.NoError(t, err)

	sch, err := scheduler.NewWithQueue(cfg, script.NewEngine(), svc,
		slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- sch.Run(ctx) }()

	require.Eventually(t, func() bool { return len(received()) >= 2 },
		30*time.Second, 50*time.Millisecond,
		"the scheduled fan-out never delivered two messages")

	// Give a would-be third message a chance to arrive, so "exactly two" means
	// something rather than "we stopped looking after two".
	time.Sleep(500 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("scheduler did not stop")
	}

	got := received()
	dumpTo(t, demoOutDir, got)
	assertDemoProperties(t, got)
}

// assertDemoProperties checks the four things this demo exists to show.
func assertDemoProperties(t *testing.T, got []captured) {
	t.Helper()

	fmt.Println("\n================ PROPERTY 1: fan-out is one job per recipient ================")
	require.Len(t, got, 2,
		"for_each must produce one POST per selected recipient — not one broadcast, "+
			"and not one for the inactive person the filter excludes")

	byEmail := map[string]captured{}
	for _, c := range got {
		rcpts, _ := c.Body["recipients"].([]any)
		require.Len(t, rcpts, 1,
			"each message must address exactly one recipient; more than one would be a broadcast")
		first, _ := rcpts[0].(map[string]any)
		email, _ := first["email"].(string)
		byEmail[email] = c
		fmt.Printf("  POST %s  ->  recipients=[%s]  (%d bytes)\n", c.Path, email, len(c.Raw))
	}
	emails := make([]string, 0, len(byEmail))
	for e := range byEmail {
		emails = append(emails, e)
	}
	sort.Strings(emails)
	require.Equal(t, []string{"alice@example.test", "bob@example.test"}, emails,
		"the inactive person must not be selected by where: [active = true]")

	fmt.Println("\n================ PROPERTY 2: ACL scoping changes the content ================")
	alice := textOf(t, byEmail["alice@example.test"])
	bob := textOf(t, byEmail["bob@example.test"])

	require.Contains(t, alice, "EUR 42000",
		"alice is a manager: task.budget must be visible to her")
	require.NotContains(t, bob, "EUR 42000",
		"bob is a worker: acl.yaml `visible:` narrows task to title+due, so budget "+
			"must be redacted out of HIS copy of the SAME template")

	require.Contains(t, alice, "EUR 71000",
		"alice may read salary_review rows")
	require.NotContains(t, bob, "EUR 71000",
		"bob's role has no read on salary_review, so the whole ROW must be absent")

	require.NotEqual(t, alice, bob,
		"two recipients of one template must not receive identical bodies")
	fmt.Printf("  alice (manager) body: %d bytes, contains budget=%v salary_review=%v\n",
		len(alice), strings.Contains(alice, "EUR 42000"), strings.Contains(alice, "EUR 71000"))
	fmt.Printf("  bob   (worker)  body: %d bytes, contains budget=%v salary_review=%v\n",
		len(bob), strings.Contains(bob, "EUR 42000"), strings.Contains(bob, "EUR 71000"))
	fmt.Println("\n  --- alice's text part ---")
	fmt.Println(indent(alice))
	fmt.Println("  --- bob's text part ---")
	fmt.Println(indent(bob))

	fmt.Println("\n================ PROPERTY 3: the wire body is APIv2-shaped ================")
	for _, c := range got {
		require.Equal(t, http.MethodPost, c.Method)
		require.Equal(t, "/accounts/demo-account/messages", c.Path,
			"account_id from mail.yaml must appear in the path")
		require.Equal(t, "Bearer demo-token-do-not-log", c.Auth,
			"the token must come from .rela/secrets.yaml as a bearer header")

		from, ok := c.Body["from"].(map[string]any)
		require.True(t, ok, "from must be an object, not a string")
		require.Equal(t, "notifications@example.com", from["email"])
		require.Equal(t, "Rela Demo", from["name"])

		rcpts, ok := c.Body["recipients"].([]any)
		require.True(t, ok, "recipients must be an array of {email,name}")
		r0, _ := rcpts[0].(map[string]any)
		require.NotEmpty(t, r0["email"])

		require.Equal(t, "Your digest", c.Body["subject"])
		require.NotEmpty(t, c.Body["html_content"], "html_content must be populated")
		require.NotEmpty(t, c.Body["text_content"], "text_content must be populated")
	}
	keys := make([]string, 0, len(got[0].Body))
	for k := range got[0].Body {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Printf("  top-level JSON keys on the wire: %v\n", keys)
	fmt.Printf("  path: %s   auth: Bearer <redacted>\n", got[0].Path)

	fmt.Println("\n================ PROPERTY 4: payloads captured for inspection ================")
	fmt.Printf("  wrote %d requests to %s\n", len(got), demoOutDir)
}

// textOf returns the text_content of a captured request.
func textOf(t *testing.T, c captured) string {
	t.Helper()
	s, _ := c.Body["text_content"].(string)
	require.NotEmpty(t, s, "captured request had no text_content")
	return s
}

func indent(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = "    | " + l
	}
	return strings.Join(lines, "\n")
}

// TestDemo_ScheduledMailOverScript runs the SAME scheduled fan-out through
// `transport: script`, the user-facing "bring your own provider" path.
//
// The difference from the test above is the point of running both:
//
//   - transport: http has a hardcoded endpoint, so proving it needs the Go-only
//     mail.WithHTTPBaseURL seam. What that proves is rela's own request shape.
//
//   - transport: script has no endpoint of its own. The URL comes from the
//     script, out of .rela/secrets.yaml, exactly as an operator would deploy
//     it — so this half runs with NO test seam anywhere: the Services keeps the
//     sender its own .rela/mail.yaml produced, and the HTTP round-trip to the
//     local server is as real as a round-trip to a provider.
//
//     go test -tags maildemo ./internal/appbuild -run TestDemo_ScheduledMailOverScript -v
//
// Requests land in /tmp/maildemo-script/.
const demoScriptOutDir = "/tmp/maildemo-script"

// demoSendScript is the operator's mapping from a rendered message onto their
// provider's API. Modeled on examples/mail/resend.lua.
const demoSendScript = `
local base = rela.secrets.demo_base_url
if not base then error("demo: set demo_base_url in .rela/secrets.yaml") end

local to = {}
for _, rcpt in ipairs(message.to) do
  table.insert(to, { email = rcpt.email, name = rcpt.name })
end

local resp, err = http.request({
  url = base .. "/send",
  method = "POST",
  headers = {
    ["Authorization"] = "Bearer " .. rela.secrets.demo_key,
    ["Content-Type"] = "application/json",
    ["X-Rendered-For"] = message.rendered_for,
  },
  body = rela.json.encode({
    from = { email = message.from.email, name = message.from.name },
    recipients = to,
    subject = message.subject,
    html_content = message.html,
    text_content = message.text,
  }),
})
if err then error("demo: " .. tostring(err)) end
-- status_code, not status: matching examples/mail/*.lua. Getting this wrong
-- raises, which the outbox correctly treats as a failed send — the first
-- draft of this script did exactly that and only the retry ladder delivered.
if resp.status_code < 200 or resp.status_code >= 300 then
  error("demo: provider returned " .. tostring(resp.status_code))
end
`

func TestDemo_ScheduledMailOverScript(t *testing.T) {
	stub, received := newAPIv2Stub(t)
	root := writeDemoProject(t)

	// Swap the project onto transport: script. Nothing else about the project
	// changes — same schedule, same template, same ACL, same recipients.
	require.NoError(t, os.MkdirAll(filepath.Join(root, "mail"), 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "mail", "demo.lua"), []byte(demoSendScript), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".rela", "mail.yaml"), []byte(`transport: script
script: mail/demo.lua
from: notifications@example.com
from_name: Rela Demo
base_url: https://rela.example.com
capabilities:
  http: true
  secrets: [demo_key, demo_base_url]
`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".rela", "secrets.yaml"), []byte(
		"demo_key: demo-token-do-not-log\ndemo_base_url: "+stub.URL+"\n"), 0o600))

	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fs)
	require.NoError(t, err)

	svc, err := New(Config{
		FS: fs, Paths: paths, ScriptEngine: script.NewEngine(), Audit: audit.Nop{},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	// NO sender substitution. This is the sender startMailRuntime built from
	// the project's own mail.yaml, unmodified.
	require.NotNil(t, svc.mail, "mail runtime did not start")
	require.Equal(t, mail.TransportScript, svc.mail.config.Transport)

	data, err := svc.Config().Load(context.Background(), scheduler.ConfigFile)
	require.NoError(t, err)
	cfg, err := scheduler.ParseConfig(data)
	require.NoError(t, err)

	sch, err := scheduler.NewWithQueue(cfg, script.NewEngine(), svc,
		slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- sch.Run(ctx) }()

	require.Eventually(t, func() bool { return len(received()) >= 2 },
		30*time.Second, 50*time.Millisecond,
		"the scheduled fan-out never delivered two messages through the send script")

	time.Sleep(500 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("scheduler did not stop")
	}

	got := received()
	dumpTo(t, demoScriptOutDir, got)

	fmt.Println("\n============ transport: script — no test seam, real HTTP ============")
	require.Len(t, got, 2, "for_each must fan out to one send-script invocation per recipient")

	byEmail := map[string]captured{}
	for _, c := range got {
		require.Equal(t, "/send", c.Path, "the URL came from the script, not from rela")
		rcpts, _ := c.Body["recipients"].([]any)
		require.Len(t, rcpts, 1)
		m, _ := rcpts[0].(map[string]any)
		email, _ := m["email"].(string)
		byEmail[email] = c
		fmt.Printf("  POST %s  ->  %s  (%d bytes, rendered_for=%s)\n",
			c.Path, email, len(c.Raw), c.RenderedFor)
	}

	alice := textOf(t, byEmail["alice@example.test"])
	bob := textOf(t, byEmail["bob@example.test"])
	require.Contains(t, alice, "EUR 42000")
	require.NotContains(t, bob, "EUR 42000",
		"the ACL scoping must hold on the script transport too")
	require.Contains(t, alice, "EUR 71000")
	require.NotContains(t, bob, "EUR 71000")

	require.Equal(t, "PERS-1", byEmail["alice@example.test"].RenderedFor,
		"rendered_for must name the identity whose visibility bounded the content")
	require.Equal(t, "PERS-2", byEmail["bob@example.test"].RenderedFor)

	fmt.Printf("  alice contains budget=%v salary=%v | bob contains budget=%v salary=%v\n",
		strings.Contains(alice, "EUR 42000"), strings.Contains(alice, "EUR 71000"),
		strings.Contains(bob, "EUR 42000"), strings.Contains(bob, "EUR 71000"))
	fmt.Printf("  wrote %d requests to %s\n", len(got), demoScriptOutDir)
}

// dumpTo writes every request to dir so the delivered payload can be read
// after the run, the way manual_e2e_test.go writes received.eml.
func dumpTo(t *testing.T, dir string, got []captured) {
	t.Helper()
	require.NoError(t, os.RemoveAll(dir))
	require.NoError(t, os.MkdirAll(dir, 0o750))

	var summary strings.Builder
	for i, c := range got {
		rcpts, _ := c.Body["recipients"].([]any)
		who := "unknown"
		if len(rcpts) == 1 {
			if m, ok := rcpts[0].(map[string]any); ok {
				who, _ = m["email"].(string)
			}
		}
		var pretty map[string]any
		_ = json.Unmarshal(c.Raw, &pretty)
		out, _ := json.MarshalIndent(pretty, "", "  ")
		name := fmt.Sprintf("%02d-%s.json", i+1, strings.ReplaceAll(who, "@", "-at-"))
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), out, 0o600))

		if html, ok := c.Body["html_content"].(string); ok {
			require.NoError(t, os.WriteFile(
				filepath.Join(dir, fmt.Sprintf("%02d-%s.html", i+1,
					strings.ReplaceAll(who, "@", "-at-"))), []byte(html), 0o600))
		}
		fmt.Fprintf(&summary, "%s %s  recipient=%s  bytes=%d  file=%s\n",
			c.Method, c.Path, who, len(c.Raw), name)
	}
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "summary.txt"), []byte(summary.String()), 0o600))
}
