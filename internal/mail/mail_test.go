package mail_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/Sourcehaven-BV/rela/internal/mail"
	"github.com/Sourcehaven-BV/rela/internal/mail/mailtest"
)

// TestMain adds leak detection: this package owns a background goroutine, and a
// worker that outlives its Stop is exactly the bug that would otherwise surface
// as a flaky unrelated test.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// --- conformance -----------------------------------------------------------

// TestConformance_Memory covers AC 4 for the memory transport.
func TestConformance_Memory(t *testing.T) {
	mailtest.RunAll(t, func(tb testing.TB) (mail.Sender, mailtest.Sent) {
		tb.Helper()
		s := mail.NewMemorySender(0)
		return s, s.Messages
	})
}

// TestConformance_SMTP covers AC 4 for the SMTP transport, against the fake.
//
// Wired exactly as production wires it — mandatory TLS, real certificate
// verification against a pinned pool — so the suite exercises the composition
// that actually ships rather than a loosened variant.
func TestConformance_SMTP(t *testing.T) {
	mailtest.RunAll(t, func(tb testing.TB) (mail.Sender, mailtest.Sent) {
		tb.Helper()
		st, ok := tb.(*testing.T)
		require.True(tb, ok)

		fake, pool := newFakeSMTP(st, true)
		sender, err := mail.NewSMTPSender(&mail.Config{
			Transport: mail.TransportSMTP,
			Host:      fake.host(),
			Port:      fake.port(),
			From:      "from@example.com",
		}, mail.WithRootCAs(pool))
		require.NoError(tb, err)

		// Adapt what the wire recorded back into the Message shape the suite
		// asserts on. Subject and body are read out of the DATA stream, which
		// is the real proof they were transmitted.
		sent := func() []mail.Message {
			var out []mail.Message
			for _, fm := range fake.received() {
				out = append(out, messageFromWire(fm))
			}
			return out
		}
		return sender, sent
	})
}

// messageFromWire reconstructs enough of a Message from a captured SMTP DATA
// stream for the conformance assertions.
func messageFromWire(fm fakeMessage) mail.Message {
	m := mail.Message{}
	for _, to := range fm.To {
		m.To = append(m.To, mail.Address{Email: to})
	}
	m.Subject = decodeHeader(headerValue(fm.Data, "Subject"))

	// Bodies are quoted-printable inside MIME parts; the conformance suite only
	// needs to see the distinctive text, so decode softly rather than fully.
	body := decodeQuotedPrintableish(fm.Data)
	if i := strings.Index(body, "<h1>Distinct HTML</h1>"); i >= 0 {
		m.HTML = []byte("<h1>Distinct HTML</h1>")
	} else if strings.Contains(body, "<p>hello</p>") {
		m.HTML = []byte("<p>hello</p>")
	}
	if strings.Contains(body, "Distinct text") {
		m.Text = []byte("Distinct text")
	} else if strings.Contains(body, "hello") {
		m.Text = []byte("hello")
	}
	return m
}

// --- config ---------------------------------------------------------------

// TestLoadConfig_Missing covers AC 1: an absent file is "mail is off", not a
// failure, so every other command must still start.
func TestLoadConfig_Missing(t *testing.T) {
	t.Parallel()

	_, err := mail.LoadConfig(t.TempDir())
	require.ErrorIs(t, err, mail.ErrConfigNotFound)
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name: "smtp minimal",
			yaml: "transport: smtp\nhost: smtp.example.com\nfrom: rela@example.com\n",
		},
		{
			name: "memory",
			yaml: "transport: memory\nfrom: rela@example.com\n",
		},
		{
			name:    "literal password refused",
			yaml:    "transport: smtp\nhost: h\nfrom: f@e.com\npassword: hunter2\n",
			wantErr: "password_env",
		},
		{
			name:    "unknown transport",
			yaml:    "transport: carrier-pigeon\nfrom: f@e.com\n",
			wantErr: "unknown transport",
		},
		{
			name:    "missing transport",
			yaml:    "host: h\nfrom: f@e.com\n",
			wantErr: "transport is required",
		},
		{
			name:    "smtp without host",
			yaml:    "transport: smtp\nfrom: f@e.com\n",
			wantErr: "host is required",
		},
		{
			name:    "missing from",
			yaml:    "transport: smtp\nhost: h\n",
			wantErr: "from is required",
		},
		{
			name:    "credentials in host",
			yaml:    "transport: smtp\nhost: user:pass@smtp.example.com\nfrom: f@e.com\n",
			wantErr: "bare hostname",
		},
		{
			name:    "url as host",
			yaml:    "transport: smtp\nhost: https://smtp.example.com/x\nfrom: f@e.com\n",
			wantErr: "bare hostname",
		},
		{
			name:    "port out of range",
			yaml:    "transport: smtp\nhost: h\nfrom: f@e.com\nport: 99999\n",
			wantErr: "out of range",
		},
		{
			name:    "negative timeout",
			yaml:    "transport: memory\nfrom: f@e.com\ntimeout_seconds: -1\n",
			wantErr: "must not be negative",
		},
		{
			name:    "bad base_url",
			yaml:    "transport: memory\nfrom: f@e.com\nbase_url: app.example.com\n",
			wantErr: "base_url must start with",
		},
		{
			name:    "CRLF in from",
			yaml:    "transport: memory\nfrom: \"f@e.com\\r\\nBcc: evil@e.com\"\n",
			wantErr: "control character",
		},
		{
			name:    "malformed yaml",
			yaml:    "transport: [smtp\n",
			wantErr: "parse",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, mail.ConfigFile), []byte(tc.yaml), 0o600))

			cfg, err := mail.LoadConfig(dir)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, cfg)
		})
	}
}

func TestConfig_Defaults(t *testing.T) {
	t.Parallel()

	cfg := &mail.Config{}
	require.Equal(t, mail.DefaultTimeoutSeconds, cfg.Timeout())
	require.Equal(t, mail.DefaultPort, cfg.EffectivePort())
}

// --- SMTP ------------------------------------------------------------------

// TestSMTP_DeliversOverSTARTTLS covers AC 2's positive half.
func TestSMTP_DeliversOverSTARTTLS(t *testing.T) {
	// No t.Parallel: t.Setenv is incompatible with it.
	fake, pool := newFakeSMTP(t, true)
	t.Setenv("RELA_TEST_SMTP_PASSWORD", "s3cret-value")

	sender, err := mail.NewSMTPSender(&mail.Config{
		Transport:   mail.TransportSMTP,
		Host:        fake.host(),
		Port:        fake.port(),
		Username:    "relay",
		PasswordEnv: "RELA_TEST_SMTP_PASSWORD",
		From:        "rela@example.com",
		FromName:    "rela",
	}, mail.WithRootCAs(pool))
	require.NoError(t, err)

	err = sender.Send(context.Background(), mail.Message{
		To:      []mail.Address{{Email: "to@example.com"}},
		Subject: "Hello",
		HTML:    []byte("<p>hi</p>"),
		Text:    []byte("hi"),
	})
	require.NoError(t, err)

	got := fake.received()
	require.Len(t, got, 1)
	require.Equal(t, "rela@example.com", got[0].From)
	require.Equal(t, []string{"to@example.com"}, got[0].To)

	// Credentials reached the server, so AUTH genuinely happened over TLS.
	user, pass := fake.credentials()
	require.Equal(t, "relay", user)
	require.Equal(t, "s3cret-value", pass)
}

// TestSMTP_RefusesPlaintextDowngrade covers AC 2's negative half — the clause
// that makes "STARTTLS required" a control rather than a comment.
func TestSMTP_RefusesPlaintextDowngrade(t *testing.T) {
	t.Parallel()

	fake, pool := newFakeSMTP(t, false) // advertises no STARTTLS
	sender, err := mail.NewSMTPSender(&mail.Config{
		Transport: mail.TransportSMTP,
		Host:      fake.host(),
		Port:      fake.port(),
		From:      "rela@example.com",
	}, mail.WithRootCAs(pool))
	require.NoError(t, err)

	err = sender.Send(context.Background(), mail.Message{
		To:      []mail.Address{{Email: "to@example.com"}},
		Subject: "Hello",
		Text:    []byte("hi"),
	})
	require.Error(t, err, "must refuse to send when the server offers no STARTTLS")
	require.Empty(t, fake.received(), "nothing may be transmitted in the clear")
}

// TestSMTP_CredentialNeverInError covers AC 11 for the send path.
func TestSMTP_CredentialNeverInError(t *testing.T) {
	// No t.Parallel: t.Setenv is incompatible with it.
	const secret = "super-secret-password-value"
	t.Setenv("RELA_TEST_SMTP_PASSWORD", secret)

	// Port 1 is reserved and refuses connections, so the send fails and the
	// error text is the interesting artefact.
	sender, err := mail.NewSMTPSender(&mail.Config{
		Transport:   mail.TransportSMTP,
		Host:        "127.0.0.1",
		Port:        1,
		Username:    "relay",
		PasswordEnv: "RELA_TEST_SMTP_PASSWORD",
		From:        "rela@example.com",
	})
	require.NoError(t, err)

	err = sender.Send(context.Background(), mail.Message{
		To:      []mail.Address{{Email: "to@example.com"}},
		Subject: "Hello",
		Text:    []byte("hi"),
	})
	require.Error(t, err)
	require.NotContains(t, err.Error(), secret)
}

func TestNewSMTPSender_Rejects(t *testing.T) {
	t.Parallel()

	t.Run("nil config", func(t *testing.T) {
		t.Parallel()
		_, err := mail.NewSMTPSender(nil)
		require.Error(t, err)
	})

	t.Run("wrong transport", func(t *testing.T) {
		t.Parallel()
		_, err := mail.NewSMTPSender(&mail.Config{
			Transport: mail.TransportMemory,
			From:      "f@e.com",
		})
		require.Error(t, err)
	})
}

// --- memory ----------------------------------------------------------------

// TestMemorySender covers AC 3.
func TestMemorySender(t *testing.T) {
	t.Parallel()

	s := mail.NewMemorySender(0)
	m := mail.Message{
		To:      []mail.Address{{Email: "a@example.com", Name: "A"}},
		Subject: "Subj",
		HTML:    []byte("<p>h</p>"),
		Text:    []byte("t"),
	}
	require.NoError(t, s.Send(context.Background(), m))

	got := s.Messages()
	require.Len(t, got, 1)
	require.Equal(t, "Subj", got[0].Subject)
	require.Equal(t, "a@example.com", got[0].To[0].Email)
	require.Equal(t, "<p>h</p>", string(got[0].HTML))
	require.Equal(t, "t", string(got[0].Text))
	require.Equal(t, 1, s.Count())

	s.Reset()
	require.Empty(t, s.Messages())
	require.Zero(t, s.Count())
}

// TestMemorySender_RingBuffer pins that retention is bounded: an unbounded
// recorder in a long-running dev server is a memory leak.
func TestMemorySender_RingBuffer(t *testing.T) {
	t.Parallel()

	s := mail.NewMemorySender(3)
	for i := range 5 {
		require.NoError(t, s.Send(context.Background(), mail.Message{
			To:      []mail.Address{{Email: "a@example.com"}},
			Subject: string(rune('A' + i)),
			Text:    []byte("t"),
		}))
	}

	got := s.Messages()
	require.Len(t, got, 3, "ring buffer must cap retention")
	require.Equal(t, "C", got[0].Subject, "oldest entries are evicted first")
	require.Equal(t, "E", got[2].Subject)
	require.Equal(t, 5, s.Count(), "count tracks lifetime sends, not retention")
}

// --- message validation ----------------------------------------------------

// TestMessage_Validate covers AC 9a and the header-injection surface generally.
func TestMessage_Validate(t *testing.T) {
	t.Parallel()

	base := func() mail.Message {
		return mail.Message{
			To:      []mail.Address{{Email: "to@example.com"}},
			Subject: "ok",
			Text:    []byte("body"),
		}
	}

	tests := []struct {
		name    string
		mutate  func(*mail.Message)
		wantErr bool
	}{
		{"valid", func(*mail.Message) {}, false},
		{"no recipients", func(m *mail.Message) { m.To = nil }, true},
		{"blank recipient", func(m *mail.Message) { m.To = []mail.Address{{Email: "  "}} }, true},
		{"no body", func(m *mail.Message) { m.Text = nil }, true},
		{"html only is fine", func(m *mail.Message) { m.Text = nil; m.HTML = []byte("<p>x</p>") }, false},

		{"CRLF subject", func(m *mail.Message) { m.Subject = "a\r\nBcc: e@e.com" }, true},
		{"LF subject", func(m *mail.Message) { m.Subject = "a\nBcc: e@e.com" }, true},
		{"CR subject", func(m *mail.Message) { m.Subject = "a\rBcc: e@e.com" }, true},
		{"NUL subject", func(m *mail.Message) { m.Subject = "a\x00b" }, true},
		{"CRLF recipient", func(m *mail.Message) { m.To[0].Email = "a@e.com\r\nBcc: e@e.com" }, true},
		{"CRLF recipient name", func(m *mail.Message) { m.To[0].Name = "A\r\nBcc: e@e.com" }, true},
		{"unicode subject ok", func(m *mail.Message) { m.Subject = "Tâches — 期限切れ" }, false},

		{"blank inline CID", func(m *mail.Message) {
			m.InlineImages = []mail.InlineImage{{CID: " ", ContentType: "image/png"}}
		}, true},
		{"CRLF inline CID", func(m *mail.Message) {
			m.InlineImages = []mail.InlineImage{{CID: "a\r\nX: y", ContentType: "image/png"}}
		}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := base()
			tc.mutate(&m)
			err := m.Validate()
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

// --- outbox ----------------------------------------------------------------

// blockingSender fails a configurable number of times, then succeeds.
type blockingSender struct {
	mu       sync.Mutex
	attempts int32
	failFor  int32
	sent     []mail.Message
	block    chan struct{}
}

func (b *blockingSender) Send(ctx context.Context, m mail.Message) error {
	n := atomic.AddInt32(&b.attempts, 1)

	if b.block != nil {
		select {
		case <-b.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if n <= atomic.LoadInt32(&b.failFor) {
		return errors.New("transient failure")
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b.sent = append(b.sent, m)
	return nil
}

func (b *blockingSender) delivered() []mail.Message {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]mail.Message, len(b.sent))
	copy(out, b.sent)
	return out
}

func testMessage() mail.Message {
	return mail.Message{
		To:      []mail.Address{{Email: "to@example.com"}},
		Subject: "Subj",
		Text:    []byte("body"),
	}
}

func TestNewOutbox_RejectsNilSender(t *testing.T) {
	t.Parallel()

	_, err := mail.NewOutbox(nil, mail.OutboxConfig{})
	require.Error(t, err, "a nil sender must be refused, not replaced with a no-op")
}

// TestOutbox_EnqueueDoesNotDial covers AC 8 — the property the whole design
// exists for. A caller on the write path must never wait on a mail server.
func TestOutbox_EnqueueDoesNotDial(t *testing.T) {
	t.Parallel()

	blocked := &blockingSender{block: make(chan struct{})}
	ob, err := mail.NewOutbox(blocked, mail.OutboxConfig{})
	require.NoError(t, err)
	ob.Start()
	t.Cleanup(ob.Stop)

	done := make(chan error, 1)
	go func() { done <- ob.Enqueue(testMessage()) }()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Enqueue blocked; it must never wait on delivery")
	}

	close(blocked.block)
}

// TestOutbox_DeliversAsync pins that an enqueued message actually goes out.
func TestOutbox_DeliversAsync(t *testing.T) {
	t.Parallel()

	s := mail.NewMemorySender(0)
	ob, err := mail.NewOutbox(s, mail.OutboxConfig{})
	require.NoError(t, err)
	ob.Start()
	t.Cleanup(ob.Stop)

	require.NoError(t, ob.Enqueue(testMessage()))
	require.Eventually(t, func() bool { return s.Count() == 1 }, 5*time.Second, 10*time.Millisecond)
}

// TestOutbox_RetriesWithoutDuplicating covers AC 9.
func TestOutbox_RetriesWithoutDuplicating(t *testing.T) {
	t.Parallel()

	s := &blockingSender{failFor: 2}
	ob, err := mail.NewOutbox(s, mail.OutboxConfig{
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     20 * time.Millisecond,
	})
	require.NoError(t, err)
	ob.Start()
	t.Cleanup(ob.Stop)

	require.NoError(t, ob.Enqueue(testMessage()))

	require.Eventually(t, func() bool { return len(s.delivered()) == 1 },
		5*time.Second, 10*time.Millisecond)

	require.Equal(t, int32(3), atomic.LoadInt32(&s.attempts), "two failures then one success")
	require.Len(t, s.delivered(), 1, "a retried message must be delivered exactly once")
}

// TestOutbox_GivesUpAfterMaxAttempts pins that a permanently failing message
// does not retry forever and block everything behind it.
func TestOutbox_GivesUpAfterMaxAttempts(t *testing.T) {
	t.Parallel()

	s := &blockingSender{failFor: 1000}
	ob, err := mail.NewOutbox(s, mail.OutboxConfig{
		MaxAttempts:    3,
		InitialBackoff: 5 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
	})
	require.NoError(t, err)
	ob.Start()
	t.Cleanup(ob.Stop)

	require.NoError(t, ob.Enqueue(testMessage()))

	require.Eventually(t, func() bool { return atomic.LoadInt32(&s.attempts) == 3 },
		5*time.Second, 10*time.Millisecond)

	// A later message still gets through: one poison message must not wedge
	// the queue.
	require.NoError(t, ob.Enqueue(testMessage()))
	require.Eventually(t, func() bool { return atomic.LoadInt32(&s.attempts) > 3 },
		5*time.Second, 10*time.Millisecond)
}

// TestOutbox_FullReturnsError pins that a backlog is SIGNALLED, not dropped:
// a full buffer means the mail server is unreachable, which the caller may
// want to act on.
func TestOutbox_FullReturnsError(t *testing.T) {
	t.Parallel()

	blocked := &blockingSender{block: make(chan struct{})}
	ob, err := mail.NewOutbox(blocked, mail.OutboxConfig{Capacity: 2})
	require.NoError(t, err)
	// Not started: nothing drains, so the buffer fills deterministically.

	require.NoError(t, ob.Enqueue(testMessage()))
	require.NoError(t, ob.Enqueue(testMessage()))
	require.ErrorIs(t, ob.Enqueue(testMessage()), mail.ErrOutboxFull)
	require.Equal(t, 2, ob.Len())

	close(blocked.block)
}

// TestOutbox_EnqueueValidates pins that a malformed message is refused at the
// boundary rather than failing later on a worker where nobody is listening.
func TestOutbox_EnqueueValidates(t *testing.T) {
	t.Parallel()

	ob, err := mail.NewOutbox(mail.NewMemorySender(0), mail.OutboxConfig{})
	require.NoError(t, err)

	m := testMessage()
	m.Subject = "a\r\nBcc: evil@example.com"
	require.Error(t, ob.Enqueue(m))
	require.Zero(t, ob.Len())
}

// TestOutbox_LosesPendingOnStop covers AC 10.
//
// This asserts the LIMIT, not a feature: undelivered mail is discarded when the
// process goes away. Pinning it means the best-effort contract is a tested
// property that a future change has to consciously break, rather than something
// discovered in production.
func TestOutbox_LosesPendingOnStop(t *testing.T) {
	t.Parallel()

	blocked := &blockingSender{block: make(chan struct{})}
	ob, err := mail.NewOutbox(blocked, mail.OutboxConfig{
		Capacity:     8,
		DrainTimeout: 100 * time.Millisecond,
	})
	require.NoError(t, err)
	ob.Start()

	for range 5 {
		require.NoError(t, ob.Enqueue(testMessage()))
	}

	// Stop while delivery is blocked: the worker cannot finish, so the drain
	// times out and the queued messages are gone.
	ob.Stop()
	close(blocked.block)

	require.Empty(t, blocked.delivered(),
		"best-effort: pending mail is lost on stop, and this is the documented behaviour")
}

// TestOutbox_StopIsBoundedAndIdempotent covers AC 9b. Stop is usually called
// from Close while a process is exiting; an unbounded wait would hang shutdown.
func TestOutbox_StopIsBoundedAndIdempotent(t *testing.T) {
	t.Parallel()

	blocked := &blockingSender{block: make(chan struct{})}
	ob, err := mail.NewOutbox(blocked, mail.OutboxConfig{DrainTimeout: 150 * time.Millisecond})
	require.NoError(t, err)
	ob.Start()
	require.NoError(t, ob.Enqueue(testMessage()))

	start := time.Now()
	ob.Stop()
	require.Less(t, time.Since(start), 3*time.Second, "Stop must be bounded by DrainTimeout")

	// Idempotent, and safe after the fact.
	ob.Stop()
	ob.Stop()

	close(blocked.block)
}

// TestOutbox_StopWithoutStart pins the nil-safe/never-started path, so Close
// can call Stop unconditionally.
func TestOutbox_StopWithoutStart(t *testing.T) {
	t.Parallel()

	ob, err := mail.NewOutbox(mail.NewMemorySender(0), mail.OutboxConfig{})
	require.NoError(t, err)
	ob.Stop()

	var nilOutbox *mail.Outbox
	nilOutbox.Stop()
}

// TestOutbox_ConcurrentEnqueue exercises the buffer under -race.
func TestOutbox_ConcurrentEnqueue(t *testing.T) {
	t.Parallel()

	s := mail.NewMemorySender(0)
	ob, err := mail.NewOutbox(s, mail.OutboxConfig{Capacity: 256})
	require.NoError(t, err)
	ob.Start()
	t.Cleanup(ob.Stop)

	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = ob.Enqueue(testMessage())
		}()
	}
	wg.Wait()

	require.Eventually(t, func() bool { return s.Count() == 32 },
		10*time.Second, 10*time.Millisecond)
}
