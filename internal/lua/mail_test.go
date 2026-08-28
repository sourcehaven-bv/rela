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

// newMailRuntime builds a reader runtime with the given sender wired.
func newMailRuntime(t *testing.T, sender MailSender) (*Runtime, *strings.Builder) {
	t.Helper()
	var sb strings.Builder
	var opts []Option
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

// TestMailSend_NoCapabilityNeeded pins that mail.send is NOT capability-gated.
//
// It is not an ambient capability the way http and ai are: the sender is the
// project's configured transport, so a script cannot reach a destination the
// operator did not set up, and gating it would only add a knob whose "off"
// position is indistinguishable from "mail is not configured".
func TestMailSend_NoCapabilityNeeded(t *testing.T) {
	t.Parallel()

	sender := &recordingMailSender{}
	rt := NewReader(ReadDeps{}, &bytes.Buffer{}, WithMailSender(sender))
	t.Cleanup(rt.Close)

	require.NoError(t, rt.RunString(`
assert(http == nil, "http must still be absent without its capability")
assert(ai == nil, "ai must still be absent without its capability")
local ok = mail.send{to = "a@example.com", subject = "S", text = "t"}
assert(ok == true)`))

	assert.Len(t, sender.messages(), 1)
}

// TestMailSend_OnWriterRuntime pins that the binding is registered on writer
// runtimes too — automations are the main consumer, and they are writers.
func TestMailSend_OnWriterRuntime(t *testing.T) {
	t.Parallel()

	sender := &recordingMailSender{}
	rt := NewWriter(WriteDeps{EntityManager: &recordingMutator{}}, &bytes.Buffer{},
		WithMailSender(sender))
	t.Cleanup(rt.Close)

	require.NoError(t, rt.RunString(`
local ok, err = mail.send{to = "a@example.com", subject = "S", text = "t"}
assert(err == nil)
assert(ok == true)`))

	assert.Len(t, sender.messages(), 1)
}
