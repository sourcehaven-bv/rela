package mail

import (
	"context"
	"sync"
)

// DefaultMemoryCapacity bounds how many messages a MemorySender keeps.
const DefaultMemoryCapacity = 256

// MemorySender records messages instead of sending them.
//
// This is a real transport selected by `transport: memory`, not a test double
// bolted onto the package — the same posture memstore and LinearSearch take. It
// has two genuine uses:
//
//   - Local development: `just dev` exercises digests and notifications with no
//     mail server configured and no risk of mailing real people, and the
//     messages stay inspectable in process.
//   - Tests: assert on what WOULD have been sent — recipients, subject, both
//     rendered parts — without standing up an SMTP fake for every case.
//
// It also keeps the [Sender] interface honest. With a second implementation
// from the start, the abstraction is exercised rather than asserted.
//
// Messages are held in a ring buffer: an unbounded recorder in a long-running
// dev server is a memory leak, and the recent messages are the interesting ones.
type MemorySender struct {
	mu       sync.Mutex
	messages []Message
	capacity int

	// sent counts every accepted message, including ones the ring has since
	// evicted, so a test can assert "20 were sent" while retaining 10.
	sent int
}

// NewMemorySender returns a MemorySender holding up to capacity messages.
// A capacity <= 0 uses DefaultMemoryCapacity.
func NewMemorySender(capacity int) *MemorySender {
	if capacity <= 0 {
		capacity = DefaultMemoryCapacity
	}
	return &MemorySender{
		capacity: capacity,
		messages: make([]Message, 0, capacity),
	}
}

// Send records m.
//
// It honors ctx cancellation and applies the same [Message.Validate] checks a
// wire transport does, so the two transports agree on what a valid message is —
// the property the mailtest conformance suite exists to hold.
func (s *MemorySender) Send(ctx context.Context, m Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Validate exactly as a wire transport does. A recorder that accepted
	// messages SMTP would reject would bless malformed mail in tests and local
	// dev, and the divergence would only surface in production.
	if err := m.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.sent++
	if len(s.messages) == s.capacity {
		copy(s.messages, s.messages[1:])
		s.messages[len(s.messages)-1] = m
		return nil
	}
	s.messages = append(s.messages, m)
	return nil
}

// Messages returns a copy of the retained messages, oldest first.
//
// A copy, so a caller iterating results cannot race a concurrent Send, and
// cannot mutate what a later assertion reads.
func (s *MemorySender) Messages() []Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Message, len(s.messages))
	copy(out, s.messages)
	return out
}

// Count returns the number of messages accepted over the sender's lifetime,
// including any the ring buffer has evicted.
func (s *MemorySender) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sent
}

// Reset discards retained messages and zeroes the count.
func (s *MemorySender) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = s.messages[:0]
	s.sent = 0
}
