// Package mailtest provides a conformance suite for mail.Sender
// implementations, in the shape internal/store/storetest establishes: each
// implementation wires the suite through a Factory returning a fresh sender.
//
// Every clause here exists because a real caller would break without it. The
// suite is the contract; a transport that passes it can be swapped for another
// without the outbox or its callers noticing.
package mailtest

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/mail"
)

// Bounds for the concurrency clause.
const (
	concurrentSenders = 16
	subjectVariants   = 4
	concurrentTimeout = 30 * time.Second
)

// Factory returns a fresh sender plus a Sent function reporting what that
// sender received. A transport that cannot observe its own deliveries (a real
// SMTP client against a live server) is not conformance-testable; wire it
// against a local fake instead.
type Factory func(tb testing.TB) (mail.Sender, Sent)

// Sent reports the messages a sender accepted, oldest first.
type Sent func() []mail.Message

// RunAll runs every conformance test against the sender produced by newSender.
func RunAll(t *testing.T, newSender Factory) {
	t.Helper()

	for name, fn := range map[string]func(*testing.T, Factory){
		"DeliversMessage":        testDeliversMessage,
		"PreservesBothParts":     testPreservesBothParts,
		"MultipleRecipients":     testMultipleRecipients,
		"DoesNotMutateMessage":   testDoesNotMutateMessage,
		"RejectsNoRecipients":    testRejectsNoRecipients,
		"RejectsEmptyBody":       testRejectsEmptyBody,
		"RejectsCRLFInSubject":   testRejectsCRLFInSubject,
		"RejectsCRLFInRecipient": testRejectsCRLFInRecipient,
		"HonoursContextCancel":   testHonoursContextCancel,
		"UnicodeSubject":         testUnicodeSubject,
		"ConcurrentSends":        testConcurrentSends,
	} {
		t.Run(name, func(t *testing.T) { fn(t, newSender) })
	}
}

// validMessage is the baseline every case starts from.
func validMessage() mail.Message {
	return mail.Message{
		To:          []mail.Address{{Email: "to@example.com", Name: "To"}},
		Subject:     "Subject",
		HTML:        []byte("<p>hello</p>"),
		Text:        []byte("hello"),
		RenderedFor: "user:test",
	}
}

func testDeliversMessage(t *testing.T, newSender Factory) {
	t.Helper()
	t.Parallel()
	s, sent := newSender(t)

	require.NoError(t, s.Send(context.Background(), validMessage()))

	got := sent()
	require.Len(t, got, 1)
	require.Equal(t, "Subject", got[0].Subject)
	require.Equal(t, "to@example.com", got[0].To[0].Email)
}

// testPreservesBothParts pins that a transport transmits the message it was
// handed. Rendering happens upstream precisely so two transports cannot
// disagree about content; a transport that regenerated a part would break that.
func testPreservesBothParts(t *testing.T, newSender Factory) {
	t.Helper()
	t.Parallel()
	s, sent := newSender(t)

	m := validMessage()
	m.HTML = []byte("<h1>Distinct HTML</h1>")
	m.Text = []byte("Distinct text")
	require.NoError(t, s.Send(context.Background(), m))

	got := sent()
	require.Len(t, got, 1)
	require.Equal(t, "<h1>Distinct HTML</h1>", string(got[0].HTML))
	require.Equal(t, "Distinct text", string(got[0].Text))
}

func testMultipleRecipients(t *testing.T, newSender Factory) {
	t.Helper()
	t.Parallel()
	s, sent := newSender(t)

	m := validMessage()
	m.To = []mail.Address{
		{Email: "a@example.com"},
		{Email: "b@example.com", Name: "Bee"},
		{Email: "c@example.com"},
	}
	require.NoError(t, s.Send(context.Background(), m))

	got := sent()
	require.Len(t, got, 1)
	require.Len(t, got[0].To, 3)
}

// testDoesNotMutateMessage is the clause the RETRY path depends on. The outbox
// hands the same Message to a second attempt after a failure; a transport that
// consumed or rewrote it would make attempt two send something different from
// attempt one — the kind of bug that only shows up under a flaky mail server.
func testDoesNotMutateMessage(t *testing.T, newSender Factory) {
	t.Helper()
	t.Parallel()
	s, _ := newSender(t)

	m := validMessage()
	before := mail.Message{
		To:          append([]mail.Address(nil), m.To...),
		Subject:     m.Subject,
		HTML:        append([]byte(nil), m.HTML...),
		Text:        append([]byte(nil), m.Text...),
		RenderedFor: m.RenderedFor,
	}

	require.NoError(t, s.Send(context.Background(), m))

	require.Equal(t, before.Subject, m.Subject)
	require.Equal(t, before.To, m.To)
	require.Equal(t, string(before.HTML), string(m.HTML))
	require.Equal(t, string(before.Text), string(m.Text))
	require.Equal(t, before.RenderedFor, m.RenderedFor)
}

func testRejectsNoRecipients(t *testing.T, newSender Factory) {
	t.Helper()
	t.Parallel()
	s, sent := newSender(t)

	m := validMessage()
	m.To = nil
	require.Error(t, s.Send(context.Background(), m))
	require.Empty(t, sent(), "a rejected message must not be delivered")
}

func testRejectsEmptyBody(t *testing.T, newSender Factory) {
	t.Helper()
	t.Parallel()
	s, sent := newSender(t)

	m := validMessage()
	m.HTML, m.Text = nil, nil
	require.Error(t, s.Send(context.Background(), m))
	require.Empty(t, sent())
}

// testRejectsCRLFInSubject is the header-injection clause, and it asserts
// rejection AT THE BOUNDARY rather than that the emitted header looks escaped.
//
// The distinction matters: go-mail accepts a CRLF subject and neutralizes it
// only incidentally, via RFC 2047 encoded-word escaping. A test asserting on
// the encoded output would keep passing if the validation were deleted, because
// the escaping would still be there — until a value took a different encoding
// path. Only asserting that Send REFUSES catches the removal.
func testRejectsCRLFInSubject(t *testing.T, newSender Factory) {
	t.Helper()
	t.Parallel()

	for _, bad := range []string{
		"Hi\r\nBcc: evil@example.com",
		"Hi\nBcc: evil@example.com",
		"Hi\rBcc: evil@example.com",
		"Hi\x00Bcc: evil@example.com",
	} {
		s, sent := newSender(t)
		m := validMessage()
		m.Subject = bad

		err := s.Send(context.Background(), m)
		require.Error(t, err, "subject %q must be refused", bad)
		require.Empty(t, sent())
	}
}

func testRejectsCRLFInRecipient(t *testing.T, newSender Factory) {
	t.Helper()
	t.Parallel()
	s, sent := newSender(t)

	m := validMessage()
	m.To = []mail.Address{{Email: "ok@example.com\r\nBcc: evil@example.com"}}

	require.Error(t, s.Send(context.Background(), m))
	require.Empty(t, sent())
}

// testHonoursContextCancel pins that a cancelled context stops a send. The
// outbox cancels its workers at shutdown and relies on this to stop promptly
// rather than after a full network timeout.
func testHonoursContextCancel(t *testing.T, newSender Factory) {
	t.Helper()
	t.Parallel()
	s, _ := newSender(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.Error(t, s.Send(ctx, validMessage()))
}

// testUnicodeSubject guards the non-ASCII path. A subject needing RFC 2047
// encoded-words is the case a hand-rolled MIME implementation gets wrong, and
// it must survive to the transport intact.
func testUnicodeSubject(t *testing.T, newSender Factory) {
	t.Helper()
	t.Parallel()
	s, sent := newSender(t)

	m := validMessage()
	m.Subject = "Tâches — 期限切れ ☕"
	require.NoError(t, s.Send(context.Background(), m))

	got := sent()
	require.Len(t, got, 1)
	require.Equal(t, "Tâches — 期限切れ ☕", got[0].Subject)
}

// testConcurrentSends pins the documented "safe for concurrent use" contract.
// Run under -race this catches unsynchronized state in a transport.
func testConcurrentSends(t *testing.T, newSender Factory) {
	t.Helper()
	t.Parallel()
	s, sent := newSender(t)

	const n = concurrentSenders
	errs := make(chan error, n)
	for i := range n {
		go func(i int) {
			m := validMessage()
			m.Subject = "concurrent " + strings.Repeat("x", i%subjectVariants)
			errs <- s.Send(context.Background(), m)
		}(i)
	}

	deadline := time.After(concurrentTimeout)
	for range n {
		select {
		case err := <-errs:
			require.NoError(t, err)
		case <-deadline:
			t.Fatal("timed out waiting for concurrent sends")
		}
	}
	require.Len(t, sent(), n)
}
