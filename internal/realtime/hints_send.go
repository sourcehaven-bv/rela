package realtime

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// Sender tuning. A mark only enqueues; the sender goroutine merges what
// arrives within hintMergeWindow into one send, and gives each send
// hintSendTimeout.
const (
	hintQueueSize   = 256
	hintMergeWindow = 20 * time.Millisecond
	hintSendTimeout = 2 * time.Second
	hintErrorLogGap = time.Minute
)

// hintEntry is one queued durable mark, or a flush request.
type hintEntry struct {
	kind    markKind
	keys    []channels.Key
	targets []string
	read    bool
	reader  channels.Reader
	resync  bool
	// flush, when set, is closed once every entry queued before it is sent.
	flush chan struct{}
}

// hintOut sends the hints of one [Local]. Marks enqueue without waiting;
// one goroutine merges and sends, so no write waits on the network.
type hintOut struct {
	origin string
	sender HintSender
	queue  chan hintEntry
	// overflow is set when an entry did not fit the queue. The next send
	// is then one resync hint.
	overflow atomic.Bool
	stop     chan struct{}
	done     chan struct{}
	once     sync.Once
	// failures counts send errors since the last logged one; lastLog is
	// when that was. Only the sender goroutine uses them.
	failures int
	lastLog  time.Time
	now      func() time.Time
}

// errHintsClosed is returned by Flush after Close.
var errHintsClosed = errors.New("realtime: channel hints closed")

func newHintOut(origin string, sender HintSender) *hintOut {
	o := &hintOut{
		origin: origin, sender: sender, queue: make(chan hintEntry, hintQueueSize),
		stop: make(chan struct{}), done: make(chan struct{}), now: time.Now,
	}
	go o.run()
	return o
}

// enqueue queues e without waiting. A full queue drops e and asks for a
// resync instead.
func (o *hintOut) enqueue(e hintEntry) {
	select {
	case <-o.stop:
		return
	default:
	}
	select {
	case o.queue <- e:
	default:
		o.overflow.Store(true)
	}
}

// flush waits until every entry queued before it was sent, or ctx ends.
func (o *hintOut) flush(ctx context.Context) error {
	ch := make(chan struct{})
	select {
	case o.queue <- hintEntry{flush: ch}:
	case <-o.done:
		return errHintsClosed
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-ch:
		return nil
	case <-o.done:
		return errHintsClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// close flushes, then stops the goroutine. Safe to call more than once.
func (o *hintOut) close(ctx context.Context) error {
	err := o.flush(ctx)
	if errors.Is(err, errHintsClosed) {
		err = nil
	}
	o.once.Do(func() { close(o.stop) })
	select {
	case <-o.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	return err
}

// run is the sender goroutine.
func (o *hintOut) run() {
	defer close(o.done)
	for {
		var first hintEntry
		select {
		case <-o.stop:
			return
		case first = <-o.queue:
		}
		batch, flushes := o.collect(first)
		o.send(batch)
		for _, ch := range flushes {
			close(ch)
		}
	}
}

// collect merges first and what arrives within the merge window. A flush
// request ends the window at once.
func (o *hintOut) collect(first hintEntry) (batch *hintBatch, flushes []chan struct{}) {
	batch = newHintBatch()
	take := func(e hintEntry) bool {
		if e.flush != nil {
			flushes = append(flushes, e.flush)
			return false
		}
		batch.add(e)
		return true
	}
	if take(first) {
		timer := time.NewTimer(hintMergeWindow)
		defer timer.Stop()
	window:
		for {
			select {
			case e := <-o.queue:
				if !take(e) {
					break window
				}
			case <-timer.C:
				break window
			case <-o.stop:
				break window
			}
		}
	}
	if o.overflow.Swap(false) {
		batch.resync = true
	}
	return batch, flushes
}

// send encodes the batch and sends each payload. An error is logged at most
// once per hintErrorLogGap and never returned.
func (o *hintOut) send(b *hintBatch) {
	hs := b.hints()
	if len(hs) == 0 {
		return
	}
	for _, p := range encodeHints(o.origin, hs) {
		ctx, cancel := context.WithTimeout(context.Background(), hintSendTimeout)
		err := o.sender.SendHint(ctx, p)
		cancel()
		if err != nil {
			o.failed(err)
		}
	}
}

// failed records a send error, logging it when the last log was long ago.
func (o *hintOut) failed(err error) {
	o.failures++
	now := o.now()
	if !o.lastLog.IsZero() && now.Sub(o.lastLog) < hintErrorLogGap {
		return
	}
	slog.Warn("realtime: cannot send a channel hint; other processes see the change at their next refetch",
		"error", err, "failures", o.failures)
	o.failures, o.lastLog = 0, now
}

// queueMark queues a durable mark of keys with kind. Ephemeral marks never
// leave the process.
func (l *Local) queueMark(kind markKind, keys []channels.Key) {
	if l.out == nil || kind == markKindEphemeral || len(keys) == 0 {
		return
	}
	l.out.enqueue(hintEntry{kind: kind, keys: slices.Clone(keys)})
}

// Flush waits until every hint queued so far was sent, or ctx ends. A
// short-lived process calls it (through [Local.Close]) before it exits, so
// its last write reaches the others. A Local without hints returns nil.
func (l *Local) Flush(ctx context.Context) error {
	if l.out == nil {
		return nil
	}
	return l.out.flush(ctx)
}

// Close flushes the hints and stops the sender. Marks after Close are not
// sent. A Local without hints returns nil.
func (l *Local) Close(ctx context.Context) error {
	if l.out == nil {
		return nil
	}
	return l.out.close(ctx)
}
