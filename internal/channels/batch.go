package channels

import (
	"fmt"
	"maps"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// MaxBatch caps the messages one [Store.AppendBatch] posts.
const MaxBatch = realtimelimits.MaxBatch

// MaxAsked caps the questions [Channel.Asked] holds: those of the current
// session span, across batches.
const MaxAsked = 20

// Asked is one question of the session's current span: a top-level message
// whose replies of AnswerType answer it. The store keeps the set on
// [Channel.Asked], keyed by the question's seq. It adds a question when a
// [NewMessage] with Asks set is stored, counts an answer when a reply of
// the answer type is stored, and uncounts it when that reply is removed.
// [ApplySession] clears the set, so a session move starts a new span. The
// store applies no configuration: the caller names the answer type.
type Asked struct {
	AnswerType string `json:"answer_type" yaml:"answer_type"`
	Answers    int    `json:"answers"     yaml:"answers"`
}

// OpenQuestions returns how many questions in c.Asked have no answer.
func (c Channel) OpenQuestions() int {
	n := 0
	for _, q := range c.Asked {
		if q.Answers == 0 {
			n++
		}
	}
	return n
}

// CloneAsked returns a copy of a, or nil for none.
func CloneAsked(a map[int64]Asked) map[int64]Asked {
	if len(a) == 0 {
		return nil
	}
	return maps.Clone(a)
}

// CheckAsked refuses a set with a seq below 1, an invalid answer type or a
// negative count, with [ErrInvalidMessage]. The stores run it on read,
// where a failure marks a damaged row. It does not check [MaxAsked]:
// [PlanBatch] holds a write to the cap, and a stored set over a later,
// lower cap stays readable.
func CheckAsked(a map[int64]Asked) error {
	for seq, q := range a {
		if seq < 1 || !ValidName(q.AnswerType) || q.Answers < 0 {
			return fmt.Errorf("%w: asked question %d", ErrInvalidMessage, seq)
		}
	}
	return nil
}

// UnaskRemoved returns the asked set after m, the stored message before its
// removal, is removed: a removed question leaves the set, and a removed
// answer to a question in it is uncounted. changed is false when m touches
// no question; a is then returned as it was. a itself is never modified.
func UnaskRemoved(a map[int64]Asked, m Message) (out map[int64]Asked, changed bool) {
	if _, ok := a[m.Seq]; ok {
		out = maps.Clone(a)
		delete(out, m.Seq)
		return CloneAsked(out), true
	}
	q, ok := a[m.ReplyTo]
	if m.ReplyTo == 0 || !ok || q.AnswerType != m.Type || q.Answers == 0 {
		return a, false
	}
	out = maps.Clone(a)
	q.Answers--
	out[m.ReplyTo] = q
	return out, true
}

// PrepareBatch runs [PrepareMessage] and [PrepareAppendStep] on every
// message of a batch and checks the batch: one to [MaxBatch] messages
// ([ErrInvalidMessage] or [ErrTooLarge]), a valid Asks on top-level
// messages only, and, for more than one message, no reply
// ([ErrInvalidReply]) and one incarnation and author.
func PrepareBatch(batch []NewMessage) ([]NewMessage, error) {
	if len(batch) == 0 {
		return nil, fmt.Errorf("%w: a batch holds no message", ErrInvalidMessage)
	}
	if len(batch) > MaxBatch {
		return nil, fmt.Errorf("%w: %d messages in one batch, at most %d", ErrTooLarge, len(batch), MaxBatch)
	}
	out := make([]NewMessage, len(batch))
	for i, raw := range batch {
		if err := PrepareAppendStep(raw.Session); err != nil {
			return nil, err
		}
		nm, _, err := PrepareMessage(raw)
		if err != nil {
			return nil, err
		}
		if nm.Asks != "" && (!ValidName(nm.Asks) || nm.ReplyTo != 0) {
			return nil, fmt.Errorf("%w: a question needs a valid answer type and cannot be a reply",
				ErrInvalidMessage)
		}
		if len(batch) > 1 {
			if nm.ReplyTo != 0 {
				return nil, fmt.Errorf("%w: a batch of several messages posts no replies", ErrInvalidReply)
			}
			if len(raw.Files) > 0 {
				return nil, fmt.Errorf("%w: a batch of several messages holds no files", ErrInvalidMessage)
			}
			if nm.Incarnation != batch[0].Incarnation || nm.Author != batch[0].Author {
				return nil, fmt.Errorf("%w: a batch has one author and one incarnation", ErrInvalidMessage)
			}
		}
		out[i] = nm
	}
	return out, nil
}

// BatchPlan is what [PlanBatch] decided for a batch.
type BatchPlan struct {
	// MoveAt is the index of the message whose post moves the session, or
	// -1 when none does.
	MoveAt int
	// Move is the move of the message at MoveAt.
	Move SessionMove
	// Seqs are the seqs of the batch's messages, in order. The notice of a
	// move takes the seq after Seqs[MoveAt], so the next message follows it.
	Seqs []int64
	// Asked is the channel's asked set after the batch (see [Asked]).
	Asked map[int64]Asked
}

// Moved reports whether the message at index i moves the session.
func (p BatchPlan) Moved(i int) bool { return i == p.MoveAt }

// Batch returns the batch id of the plan's messages: the first seq when
// the batch holds more than one message, else 0.
func (p BatchPlan) Batch() int64 {
	if len(p.Seqs) < 2 {
		return 0
	}
	return p.Seqs[0]
}

// Due is the schedule step of the batch: that of the message that moves
// the session, else that of the last message with a step. Nil when no
// message has one.
func (p BatchPlan) Due(batch []NewMessage) *SessionStep {
	if p.MoveAt >= 0 {
		return batch[p.MoveAt].Session
	}
	for i := len(batch) - 1; i >= 0; i-- {
		if batch[i].Session != nil {
			return batch[i].Session
		}
	}
	return nil
}

// PlanBatch decides a prepared batch (see [PrepareBatch]) against cur, the
// channel as committed before it. It is the session logic every backend's
// [Store.AppendBatch] runs under its lock, and the service runs it to
// pre-check a post, so both see one rule:
//
//   - Every step's Admit runs against cur. An error refuses the batch and
//     is returned unchanged.
//   - The messages are taken in order. A reply of the answer type of a
//     question in the asked set counts as an answer. Then, until one
//     message moves the session, its step's decider runs against the
//     channel as committed so far: LastSeq is the seq before the message,
//     and the asked set holds the answers counted so far. The first move
//     clears the asked set, and later messages are not decided, so a batch
//     moves the session at most once.
//   - The batch's questions then join the asked set. Over [MaxAsked]
//     questions, the oldest answered ones leave it; more than [MaxAsked]
//     unanswered questions is [ErrTooManyAsked].
func PlanBatch(cur Channel, batch []NewMessage) (BatchPlan, error) {
	for _, nm := range batch {
		if nm.Session != nil && nm.Session.Admit != nil {
			if err := nm.Session.Admit(cur); err != nil {
				return BatchPlan{}, err
			}
		}
	}
	p := BatchPlan{MoveAt: -1, Seqs: make([]int64, len(batch))}
	c := cur
	asked := maps.Clone(cur.Asked)
	if asked == nil {
		asked = map[int64]Asked{}
	}
	next := cur.LastSeq + 1
	for i, nm := range batch {
		p.Seqs[i] = next
		next++
		if q, ok := asked[nm.ReplyTo]; ok && nm.ReplyTo != 0 && q.AnswerType == nm.Type {
			q.Answers++
			asked[nm.ReplyTo] = q
		}
		if p.MoveAt >= 0 || nm.Session == nil {
			continue
		}
		c.LastSeq, c.Asked = p.Seqs[i]-1, asked
		move, ok := nm.Session.Decide(c)
		if !ok {
			continue
		}
		if err := CheckSessionMove(move); err != nil {
			return BatchPlan{}, err
		}
		p.MoveAt, p.Move = i, move
		asked = map[int64]Asked{}
		next++
	}
	for i, nm := range batch {
		if nm.Asks != "" {
			asked[p.Seqs[i]] = Asked{AnswerType: nm.Asks}
		}
	}
	evictAnswered(asked)
	if len(asked) > MaxAsked {
		return BatchPlan{}, fmt.Errorf("%w: %d open questions in the session span, at most %d",
			ErrTooManyAsked, len(asked), MaxAsked)
	}
	p.Asked = CloneAsked(asked)
	return p, nil
}

// evictAnswered drops the answered questions of a with the lowest seqs
// until a holds at most [MaxAsked], or no answered one is left.
func evictAnswered(a map[int64]Asked) {
	if len(a) <= MaxAsked {
		return
	}
	var answered []int64
	for seq, q := range a {
		if q.Answers > 0 {
			answered = append(answered, seq)
		}
	}
	slices.Sort(answered)
	for _, seq := range answered {
		if len(a) <= MaxAsked {
			return
		}
		delete(a, seq)
	}
}

// FillBatchOutcome records the session after a batch in the Outcome of
// every step that asks for one: c is the channel as committed. Only the
// step of the message that moved the session reports the move and its
// notice.
func FillBatchOutcome(batch []NewMessage, p BatchPlan, c Channel) {
	for i, nm := range batch {
		step := nm.Session
		if step == nil || step.Outcome == nil {
			continue
		}
		*step.Outcome = SessionOutcome{Session: c.Session, CreatedAt: c.CreatedAt}
		if p.Moved(i) {
			step.Outcome.Moved, step.Outcome.Move, step.Outcome.Notice = true, p.Move, p.Seqs[i]+1
		}
	}
}
