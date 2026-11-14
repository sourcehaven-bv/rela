package apptest_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/apptest"
)

// callAutomation is added to the test schema: on create, call.lua sends
// the request the ticket's title names and writes the outcome to note.
const callAutomation = `
  - name: call
    on: {entity: [ticket], created: true}
    do:
      - lua_file: call.lua
        capabilities: {http: true, secrets: [api_key]}
`

// callScript sends one request per title and records "<status_code> <body>"
// or the error kind in note.
const callScript = `
local title = entity.properties.title
local auth = {Authorization = "Bearer " .. (rela.secrets.api_key or "none")}
local res, err
if title == "get" then
  res, err = http.get("https://api.example.com/items?b=2&a=1", {headers = auth})
elseif title == "other" then
  res, err = http.get("https://api.example.com/other")
elseif title == "post" then
  res, err = http.post("https://api.example.com/payments",
    rela.json.encode({amount = {currency = "EUR", value = "10.00"}, description = entity.id}),
    {headers = {["Content-Type"] = "application/json", Authorization = auth.Authorization}})
elseif title == "form" then
  res, err = http.request{url = "https://mail.example.com/send", method = "POST",
    form = {to = "a@example.com", subject = "hi"}}
elseif title == "encoded" then
  res, err = http.request{url = "https://mail.example.com/send", method = "POST",
    headers = {["Content-Type"] = "application/x-www-form-urlencoded"},
    body = http.encode_query({to = "b@example.com", subject = "yo"})}
end
if err then
  rela.update_entity(entity.id, {note = err.kind})
elseif res then
  rela.update_entity(entity.id, {note = res.status_code .. " " .. res.body})
end
`

// httpProject is the test project with the call automation and files on
// top. It also writes a real-looking .rela/secrets.yaml, which a case must
// never read.
func httpProject(t *testing.T, files map[string]string) string {
	t.Helper()
	schema, err := os.ReadFile(filepath.Join("testdata", "project", "schema.yaml"))
	require.NoError(t, err)
	all := map[string]string{
		"schema.yaml":        string(schema) + callAutomation,
		"scripts/call.lua":   callScript,
		".rela/secrets.yaml": "api_key: real-secret-do-not-leak\nhook_key: real-hook-secret\n",
	}
	maps.Copy(all, files)
	return project(t, all)
}

// runHTTP runs one test file on the http project and returns the report.
func runHTTP(t *testing.T, code string, files map[string]string) *apptest.Report {
	t.Helper()
	if files == nil {
		files = map[string]string{}
	}
	files["tests/a.test.lua"] = code
	return run(t, httpProject(t, files), apptest.Options{})
}

// caseByName returns the report's case named name.
func caseByName(t *testing.T, rep *apptest.Report, name string) apptest.CaseResult {
	t.Helper()
	for _, c := range rep.Cases {
		if c.Case == name {
			return c
		}
	}
	require.Failf(t, "no case", "%s", name)
	return apptest.CaseResult{}
}

func TestHTTPStub_Answers(t *testing.T) {
	rep := runHTTP(t, `
local t = require("rela.test")
local function note(title)
  local carol = t.as("carol")
  return carol:get(carol:create("ticket", {title = title})).properties.note
end

t.case("exact url ignores query order and checks headers", function()
  t.secrets{api_key = "fake"}
  local s = t.http.stub{method = "get", url = "https://api.example.com/items?a=1&b=2",
    headers = {authorization = "Bearer fake"},
    respond = {status = 200, json = {items = {1, 2}}}}
  t.expect(note("get")).to_be('200 {"items":[1,2]}')
  t.expect(s:count()).to_be(1)
  local c = s:calls()[1]
  t.expect(c.method).to_be("GET")
  t.expect(c.headers.Authorization).to_be("Bearer fake")
  t.expect(c.stub).to_be(s.name)
end)

t.case("prefix and pattern", function()
  t.http.stub{url_prefix = "https://api.example.com/items", respond = {body = "p"}}
  t.http.stub{url_pattern = "https://api\\.example\\.com/o.*", respond = {status = 202, body = "q"}}
  t.expect(note("get")).to_be("200 p")
  t.expect(note("other")).to_be("202 q")
end)

t.case("error kinds", function()
  t.http.stub{url_prefix = "https://api.example.com/items", respond = {error = "timeout"}}
  t.http.stub{url_prefix = "https://api.example.com/other", respond = {error = "network"}}
  t.expect(note("get")).to_be("timeout")
  t.expect(note("other")).to_be("network")
end)

t.case("a sequence answers in order", function()
  local s = t.http.stub{name = "flaky", url_prefix = "https://api.example.com/",
    responses = {{status = 503, headers = {["Retry-After"] = "5"}}, {status = 200, body = "ok"}}}
  t.expect(note("get")).to_be("503 ")
  t.expect(note("other")).to_be("200 ok")
  local calls = t.http.calls()
  t.expect(#calls).to_be(2)
  t.expect(calls[1].url).to_be("https://api.example.com/items?b=2&a=1")
  t.expect(calls[2].url).to_be("https://api.example.com/other")
  t.expect(calls[2].stub).to_be("flaky")
end)

t.case("json subset picks the stub", function()
  t.secrets{api_key = "k"}
  t.http.stub{name = "usd", method = "POST", url = "https://api.example.com/payments",
    json = {amount = {currency = "USD"}}, respond = {status = 400}}
  local eur = t.http.stub{name = "eur", method = "POST", url = "https://api.example.com/payments",
    json = {amount = {currency = "EUR"}}, respond = {status = 201, file = "tests/fixtures/payment.json"}}
  t.expect(note("post")).to_be('201 {"id": "tr_test"}' .. "\n")
  local c = eur:calls()[1]
  t.expect(c.json.amount.value).to_be("10.00")
  t.expect(c.headers["Content-Type"]).to_be("application/json")
end)

t.case("form subset, multipart and urlencoded", function()
  local s = t.http.stub{url = "https://mail.example.com/send", form = {to = "a@example.com"}, respond = {body = "m"}}
  t.http.stub{url = "https://mail.example.com/send", form = {subject = "yo"}, respond = {body = "u"}}
  t.expect(note("form")).to_be("200 m")
  t.expect(note("encoded")).to_be("200 u")
  t.expect(s:count()).to_be(1)
end)

t.case("exact url ignores host case and default port", function()
  t.http.stub{url = "https://API.example.com:443/other", respond = {body = "h"}}
  t.expect(note("other")).to_be("200 h")
end)

t.case("a pattern matches the whole url", function()
  t.http.stub{url_pattern = "api", respond = {body = "no"}}
  t.http.stub{url_pattern = ".*/other", respond = {body = "yes"}}
  t.expect(note("other")).to_be("200 yes")
end)

t.case("an error in a sequence", function()
  t.http.stub{url_prefix = "https://api.example.com/", responses = {{error = "timeout"}, {status = 200, body = "ok"}}}
  t.expect(note("other")).to_be("timeout")
  t.expect(note("other")).to_be("200 ok")
end)

t.case("no real secrets", function()
  local s = t.http.stub{url_prefix = "https://api.example.com/"}
  note("get")
  t.expect(s:calls()[1].headers.Authorization).to_be("Bearer none")
end)
`, map[string]string{"tests/fixtures/payment.json": `{"id": "tr_test"}` + "\n"})
	requireOK(t, rep)
}

func TestHTTPStub_Failures(t *testing.T) {
	rep := runHTTP(t, `
local t = require("rela.test")
local function make(title) t.as("carol"):create("ticket", {title = title}) end

t.case("unmatched lists the stubs", function()
  t.http.stub{name = "items", method = "GET", url_prefix = "https://api.example.com/items"}
  t.http.stub{method = "POST", url = "https://api.example.com/payments"}
  make("other")
end)

t.case("no stubs", function() make("http") end)

t.case("swallowed", function() make("http-quiet") end)

t.case("spent", function()
  t.http.stub{name = "once", url_prefix = "https://api.example.com/", responses = {{status = 200}}}
  make("get")
  make("other")
end)

t.case("header mismatch", function()
  t.http.stub{name = "auth", url_prefix = "https://api.example.com/", headers = {Authorization = "Bearer x"}}
  make("get")
end)

t.case("outside", function() t.http.stub{url_prefix = "https://x/", respond = {file = "../x.json"}} end)
t.case("cache dir upper case", function() t.http.stub{url_prefix = "https://x/", respond = {file = ".RELA/secrets.yaml"}} end)
t.case("body not a string", function() t.http.stub{url = "https://x/", body = 123} end)
t.case("an earlier assertion wins", function()
  t.expect(1).to_be(2)
  make("other")
end)
t.case("cache dir", function() t.http.stub{url_prefix = "https://x/", respond = {file = "./.rela/secrets.yaml"}} end)
t.case("two urls", function() t.http.stub{url = "https://x/", url_prefix = "https://x/"} end)
t.case("unknown key", function() t.http.stub{url = "https://x/", respnd = {}} end)
t.case("bad error", function() t.http.stub{url = "https://x/", respond = {error = "boom"}} end)
t.case("duplicate name", function()
  t.http.stub{name = "a", url = "https://x/"}
  t.http.stub{name = "a", url = "https://y/"}
end)
`, nil)
	require.Equal(t, len(rep.Cases), rep.Failed)
	want := map[string][]string{
		"unmatched lists the stubs": {"unexpected HTTP call GET https://api.example.com/other",
			"registered stubs:", `"items": GET https://api.example.com/items*`,
			"\n  - POST https://api.example.com/payments at "},
		"no stubs":                  {"unexpected HTTP call GET http://127.0.0.1:9/", "no stubs are registered"},
		"swallowed":                 {"unexpected HTTP call GET http://127.0.0.1:9/"},
		"spent":                     {`stub "once" matches it but has given all its 1 responses`},
		"header mismatch":           {`"auth"`, "headers Authorization"},
		"outside":                   {"outside the project"},
		"cache dir":                 {"is in .rela, which may hold secrets"},
		"cache dir upper case":      {"which may hold secrets"},
		"body not a string":         {"body must be a string, not a number"},
		"an earlier assertion wins": {"expected"},
		"two urls":                  {"exactly one of url, url_prefix and url_pattern"},
		"unknown key":               {"unknown option respnd"},
		"bad error":                 {`must be "timeout" or "network"`},
		"duplicate name":            {`a stub named "a" is already registered`},
	}
	for name, parts := range want {
		c := caseByName(t, rep, name)
		for _, p := range parts {
			assert.Contains(t, c.Message, p, name)
		}
	}
	for _, c := range rep.Cases {
		assert.NotContains(t, c.Message, "real-secret", c.Case)
	}
	assert.NotContains(t, caseByName(t, rep, "an earlier assertion wins").Message, "unexpected HTTP call")
	assert.Equal(t, "tests/a.test.lua:3", caseByName(t, rep, "unmatched lists the stubs").Location,
		"an unexpected call names the test line that made it")
}

func TestHTTPStub_FixtureSymlinkRefused(t *testing.T) {
	root := httpProject(t, map[string]string{"tests/a.test.lua": `
local t = require("rela.test")
t.case("symlink", function() t.http.stub{url = "https://x/", respond = {file = "tests/link.json"}} end)
`})
	outside := filepath.Join(t.TempDir(), "secret.json")
	require.NoError(t, os.WriteFile(outside, []byte("{}"), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "tests", "link.json")))
	rep := run(t, root, apptest.Options{})
	require.Equal(t, 1, rep.Failed)
	assert.Contains(t, rep.Cases[0].Message, "symlink")
}

// flowFiles add flows to the http project: pay runs charge.lua, which
// creates a payment and retries on a 5xx; notify posts a signed webhook.
var flowFiles = map[string]string{
	"acl.yaml": `user_entity_type: user
roles:
  viewer:
    read: [ticket, user]
  editor:
    read: [ticket, user]
    create: [ticket]
    update: [ticket]
    permissions: [payments.start]
  payments-bot:
    read: [ticket]
    update: [ticket]
assignments:
  carol: editor
  "system:automation": viewer
  "system:payments": payments-bot
`,
	"flows.yaml": `flows:
  pay:
    subject: ticket
    run_as: system:payments
    per_subject: many
    start: [{ on: manual, permission: payments.start }]
    initial: charge
    states:
      charge:
        job:
          script: flows/charge.lua
          capabilities: { http: true, secrets: [api_key] }
          retry: { attempts: 3, backoff: { initial: 30s } }
        on:
          done: { to: paid }
      paid: { final: true }
  notify:
    subject: ticket
    run_as: system:payments
    per_subject: many
    start: [{ on: manual, permission: payments.start }]
    initial: send
    states:
      send:
        job:
          kind: webhook
          url: https://hooks.example.com/paid
          secret: hook_key
          retry: { attempts: 2, backoff: { initial: 10s } }
        on:
          done: { to: sent }
      sent: { final: true }
`,
	"scripts/flows/charge.lua": `
local res, err = http.post("https://api.mollie.example/v2/payments",
  rela.json.encode({amount = {currency = "EUR", value = "10.00"}, description = entity.id}),
  {headers = {["Content-Type"] = "application/json", Authorization = "Bearer " .. rela.secrets.api_key}})
if err then error("payment provider: " .. err.kind) end
if res.status_code >= 500 then error("payment provider answered " .. res.status_code) end
if res.status_code ~= 201 then error({message = "refused: " .. res.status_code, retry = false}) end
local p = rela.json.decode(res.body)
rela.update_entity(entity.id, {note = p.id})
`,
	"tests/fixtures/payment.json": `{"id": "tr_test"}`,
}

func TestHTTPStub_FlowJobRetry(t *testing.T) {
	files := maps.Clone(flowFiles)
	files["tests/a.test.lua"] = `
local t = require("rela.test")

t.case("a 503 then a 201 retries the job", function()
  t.secrets{api_key = "test_key"}
  local mollie = t.http.stub{name = "create payment", method = "POST",
    url = "https://api.mollie.example/v2/payments",
    headers = {Authorization = "Bearer test_key"},
    json = {amount = {currency = "EUR"}},
    responses = {{status = 503}, {status = 201, file = "tests/fixtures/payment.json"}}}
  local carol = t.as("carol")
  local tk = carol:create("ticket", {title = "pay"})
  local id = carol:start_flow("pay", tk)

  t.expect(t.flows.work()).to_be(1)
  local run = t.flows.run(id)
  t.expect(run.state).to_be("charge")
  t.expect(run.job.phase).to_be("waiting")
  t.expect(run.job.failed).to_be(1)

  t.advance(t.seconds(29))
  t.expect(t.flows.work()).to_be(0)
  t.advance(t.seconds(1))
  t.expect(t.flows.work()).to_be(1)

  run = t.flows.run(id)
  t.expect(run.state).to_be("paid")
  t.expect(run.job).to_be_nil()
  t.expect(mollie:count()).to_be(2)
  t.expect(mollie:calls()[2].json.description).to_be(tk.id)
  t.expect(carol:get(tk).properties.note).to_be("tr_test")
end)

t.case("a 4xx fails the job at once", function()
  t.secrets{api_key = "test_key"}
  t.http.stub{url = "https://api.mollie.example/v2/payments", respond = {status = 422}}
  local carol = t.as("carol")
  local id = carol:start_flow("pay", carol:create("ticket", {title = "pay"}))
  t.flows.work()
  local run = t.flows.run(id)
  t.expect(run.state).to_be("charge")
  t.expect(run.job.phase).to_be("failed")
  t.expect(run.job.fail_kind).to_be("permanent")
end)

t.case("webhook retries a network error", function()
  t.secrets{hook_key = "s3cret"}
  local hook = t.http.stub{method = "POST", url = "https://hooks.example.com/paid",
    responses = {{error = "network"}, {status = 204}}}
  local carol = t.as("carol")
  local tk = carol:create("ticket", {title = "hook"})
  local id = carol:start_flow("notify", tk)
  t.expect(t.flows.work()).to_be(1)
  t.advance(t.seconds(10))
  t.expect(t.flows.work()).to_be(1)
  t.expect(t.flows.run(id).state).to_be("sent")
  local c = hook:calls()[2]
  t.expect(c.json.subject).to_be(tk.id)
  t.expect(c.json.attempt).to_be(2)
  t.expect(c.headers["X-Rela-Step"]).not_to_be_nil()
  t.expect(c.headers["X-Rela-Signature"]).to_contain("sha256=")
  t.expect(t.flows.run("NOPE")).to_be_nil()
end)
`
	requireOK(t, run(t, httpProject(t, files), apptest.Options{}))
}

// TestHTTPStub_WebhookSignedWithFakeSecret checks the signature the case's
// stub records is made with the t.secrets value, not the real one.
func TestHTTPStub_WebhookSignedWithFakeSecret(t *testing.T) {
	files := maps.Clone(flowFiles)
	files["tests/a.test.lua"] = `
local t = require("rela.test")
t.case("sig", function()
  t.secrets{hook_key = "s3cret"}
  local hook = t.http.stub{url = "https://hooks.example.com/paid", respond = {status = 204}}
  local carol = t.as("carol")
  carol:start_flow("notify", carol:create("ticket", {title = "hook"}))
  t.flows.work()
  local c = hook:calls()[1]
  t.fail(c.headers["X-Rela-Timestamp"] .. "|" .. c.headers["X-Rela-Signature"] .. "|" .. c.body)
end)
`
	rep := run(t, httpProject(t, files), apptest.Options{})
	require.Len(t, rep.Cases, 1)
	parts := strings.SplitN(rep.Cases[0].Message, "|", 3)
	require.Len(t, parts, 3, rep.Cases[0].Message)
	mac := hmac.New(sha256.New, []byte("s3cret"))
	mac.Write([]byte(parts[0] + "." + parts[2]))
	assert.Equal(t, "sha256="+hex.EncodeToString(mac.Sum(nil)), parts[1])
}
