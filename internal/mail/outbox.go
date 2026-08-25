package mail

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
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
//
// # One consumer, so a bad message delays the good ones
//
// The worker is a single goroutine and a retry sleeps that goroutine, so an
// undeliverable message blocks everything behind it for the length of its
// backoff ladder — roughly 30 seconds at the defaults below, and N x 30s for a
// burst of them. A typo'd recipient address in an entity property is enough to
// do it, and a long enough stall fills the buffer so [ErrOutboxFull] starts
// rejecting messages that would have delivered fine.
//
// Fixing it properly means classifying permanent failures (5xx, invalid
// recipient) separately from transient ones, or requeueing with a deadline
// instead of sleeping the consumer. Both are queue semantics that belong with
// the durable backend (IDEA-WIJ2H1) rather than bolted onto an in-memory
// buffer; until then this is a known cost, not an oversight.
type Outbox struct {
	sender Sender
	cfg    OutboxConfig

	ch chan Message

	// ctx/cancel are created in NewOutbox, NOT in Start.
	//
	// Creating them in Start would mean Stop reads a field Start writes, and
	// two different sync.Once values establish no happens-before between each
	// other — a genuine data race (the detector fires on it), and worse, Stop
	// could observe a nil cancel while a worker is running, take the
	// "never started" path, and leave the goroutine alive for the life of the
	// process. Constructing here removes the shared write entirely.
	//nolint:containedctx // deliberate: see the field comment above. This is a
	// worker LIFETIME context owned by the Outbox, not a per-call context
	// smuggled through a struct — Stop is the only thing that cancels it.
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	// started records whether Start ever ran, so Stop can distinguish "no
	// worker to wait for" from "worker running" without racing on cancel.
	started atomic.Bool

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

	// A detached lifetime context, deliberately not derived from any request
	// ctx: the worker outlives the operation that enqueued a message, and
	// tying it to a request would cancel delivery when that request finished.
	// The cancel func is stored on the Outbox and invoked by Stop, which is
	// nil-safe and idempotent; it is not dropped.
	ctx, cancel := context.WithCancel(context.Background()) //nolint:gosec // G118: cancel is called by Stop

	return &Outbox{
		sender: sender,
		cfg:    c,
		ch:     make(chan Message, c.Capacity),
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}, nil
}

// Start launches the worker. Idempotent, and safe to race against Stop.
func (o *Outbox) Start() {
	o.startOnce.Do(func() {
		o.started.Store(true)
		go o.run(o.ctx)
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
// On timeout it returns having abandoned in-flight work. Be precise about what
// that costs: the queued messages are lost (which is what best-effort means),
// AND the worker goroutine is still running, holding whatever connection its
// current send opened, until that send returns on its own. Stop does not and
// cannot kill it.
//
// This stays an edge case only because [Sender] requires implementations to
// honor ctx — a transport that ignores cancellation makes it the norm, and in
// multi-tenant deployments, where Close runs on every tenant eviction against a
// shared provider, those abandoned connections accumulate. Hanging shutdown
// instead would be worse, so the bound stays and the cost is written down.
func (o *Outbox) Stop() {
	if o == nil {
		return
	}
	o.stopOnce.Do(func() {
		// Cancel unconditionally: it is safe on a never-started outbox and
		// makes a Start racing with Stop a no-op rather than a leak (the
		// worker sees an already-cancelled context and exits immediately).
		o.cancel()

		if !o.started.Load() {
			// No worker was ever launched, so there is nothing to wait for.
			return
		}
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
