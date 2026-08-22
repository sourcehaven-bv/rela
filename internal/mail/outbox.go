package mail

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"
)

// Outbox defaults. Retry backoff doubles from Initial up to Max.
const (
	DefaultOutboxCapacity = 128
	DefaultMaxAttempts    = 5
	DefaultInitialBackoff = 2 * time.Second
	DefaultMaxBackoff     = 2 * time.Minute
	DefaultDrainTimeout   = 10 * time.Second
)

// OutboxConfig tunes the worker. The zero value uses the defaults above.
type OutboxConfig struct {
	// Capacity bounds the buffer. Enqueue returns ErrOutboxFull beyond it.
	Capacity int

	// MaxAttempts caps delivery attempts per message.
	MaxAttempts int

	// InitialBackoff is the delay before the second attempt.
	InitialBackoff time.Duration

	// MaxBackoff caps the doubling.
	MaxBackoff time.Duration

	// DrainTimeout bounds Stop.
	//
	// Without a bound, a worker blocked mid-send against an unresponsive
	// server would hang shutdown indefinitely — the caller of Stop is usually
	// Close, which runs while a process is trying to exit.
	DrainTimeout time.Duration

	// Logger receives delivery failures. Defaults to slog.Default().
	Logger *slog.Logger
}

func (c OutboxConfig) withDefaults() OutboxConfig {
	if c.Capacity <= 0 {
		c.Capacity = DefaultOutboxCapacity
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = DefaultMaxAttempts
	}
	if c.InitialBackoff <= 0 {
		c.InitialBackoff = DefaultInitialBackoff
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = DefaultMaxBackoff
	}
	if c.DrainTimeout <= 0 {
		c.DrainTimeout = DefaultDrainTimeout
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	return c
}

// Outbox accepts messages and delivers them on a background goroutine.
//
// Enqueue never dials: that is the whole point. Automations run inside the
// write path, so a synchronous send would block a user's save on a remote mail
// server, and a failure there would fail the write. Here a failing mail server
// degrades to a logged delivery failure instead.
//
// Delivery is BEST-EFFORT and messages are held only in memory — see the
// package doc for exactly what that costs on restart.
type Outbox struct {
	sender Sender
	cfg    OutboxConfig

	ch chan Message

	cancel context.CancelFunc
	done   chan struct{}

	startOnce sync.Once
	stopOnce  sync.Once
}

// NewOutbox returns an Outbox delivering through sender.
//
// Nil: a nil sender is rejected. Substituting a no-op would turn a wiring
// mistake into silently discarded mail — a downstream symptom far harder to
// diagnose than a startup error.
func NewOutbox(sender Sender, cfg OutboxConfig) (*Outbox, error) {
	if sender == nil {
		return nil, errors.New("mail: nil sender")
	}
	c := cfg.withDefaults()
	return &Outbox{
		sender: sender,
		cfg:    c,
		ch:     make(chan Message, c.Capacity),
		done:   make(chan struct{}),
	}, nil
}

// Start launches the worker. Idempotent.
//
// The worker runs on a DETACHED lifetime context, deliberately not derived from
// any request context: it outlives the operation that enqueued a message, and
// tying it to a request would cancel delivery the moment that request finished.
func (o *Outbox) Start() {
	o.startOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		o.cancel = cancel
		go o.run(ctx)
	})
}

// Enqueue accepts m for delivery.
//
// Returns ErrOutboxFull at capacity rather than blocking or dropping: blocking
// would defeat the purpose (the caller is on a write path), and dropping
// silently would hide a backlog that means the mail server is unreachable.
func (o *Outbox) Enqueue(m Message) error {
	if err := m.Validate(); err != nil {
		return err
	}
	select {
	case o.ch <- m:
		return nil
	default:
		return ErrOutboxFull
	}
}

// Len reports how many messages are waiting.
func (o *Outbox) Len() int { return len(o.ch) }

// Stop signals the worker to finish and waits for it, bounded by DrainTimeout.
//
// Idempotent and nil-safe, so Close can call it unconditionally on an Outbox
// that was never started.
//
// On timeout it returns having abandoned in-flight work — the messages are
// lost, which is what best-effort means. Better that than hanging a shutdown.
func (o *Outbox) Stop() {
	if o == nil {
		return
	}
	o.stopOnce.Do(func() {
		if o.cancel == nil {
			// Never started: nothing to wait for.
			return
		}
		o.cancel()
		select {
		case <-o.done:
		case <-time.After(o.cfg.DrainTimeout):
			o.cfg.Logger.Warn("mail: outbox drain timed out; undelivered messages discarded",
				"timeout", o.cfg.DrainTimeout, "pending", len(o.ch))
		}
	})
}

// run is the worker loop. Single goroutine, sequential delivery — which is also
// why the outbox needs no idempotency key: with one consumer and no
// persistence, a message cannot be picked up twice.
func (o *Outbox) run(ctx context.Context) {
	defer close(o.done)

	for {
		select {
		case <-ctx.Done():
			return
		case m := <-o.ch:
			o.deliver(ctx, m)
		}
	}
}

// deliver sends m, retrying with capped exponential backoff.
func (o *Outbox) deliver(ctx context.Context, m Message) {
	backoff := o.cfg.InitialBackoff

	for attempt := 1; attempt <= o.cfg.MaxAttempts; attempt++ {
		err := o.sender.Send(ctx, m)
		if err == nil {
			return
		}

		// A cancel during a send is a shutdown, not a delivery failure; logging
		// it as an error would fill the log with noise on every clean stop.
		if ctx.Err() != nil {
			return
		}

		if attempt == o.cfg.MaxAttempts {
			o.cfg.Logger.Error("mail: delivery failed, giving up",
				"attempts", attempt, "recipients", len(m.To), "error", err)
			return
		}

		o.cfg.Logger.Warn("mail: delivery failed, will retry",
			"attempt", attempt, "backoff", backoff, "error", err)

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		if backoff *= 2; backoff > o.cfg.MaxBackoff {
			backoff = o.cfg.MaxBackoff
		}
	}
}

// newByteReader adapts a byte slice for go-mail's embed API.
func newByteReader(b []byte) io.Reader { return bytes.NewReader(b) }
