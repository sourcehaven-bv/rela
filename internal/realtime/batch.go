package realtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// BatchInput is a batch of log messages to post as one step (see
// [Service.PostBatch]).
type BatchInput struct {
	Messages []BatchMessage
	// ExpectGeneration is the session generation the batch was decided
	// against, as for [PostInput.ExpectGeneration]: one for the batch.
	ExpectGeneration *int64
}

// BatchMessage is one message of a [BatchInput]: a top-level log message.
type BatchMessage struct {
	Type  string
	Props map[string]any
}

// BatchResult is a posted batch: its messages in seq order, and the
// session and outputs as for [PostResult].
type BatchResult struct {
	Messages []MessageView
	Session  *SessionView
	Outputs  *OutputsView
}

// PostBatch posts the messages of in as one store step (see
// [channels.Store.AppendBatch]): all of them or none. Each message passes
// the checks of [Service.Post], in order, and is charged at the same point:
// after the session generation and before the properties, one token per
// message in one step. A batch refused before that point spends nothing; a
// batch with an invalid message spends its tokens, as an invalid post does.
// The batch counts as one post for the session: every `in:` and
// the expect_generation are judged against the session before the batch,
// and the session moves at most once, on the first message whose
// transition fires. A batch holds 1 to [channels.MaxBatch] top-level log
// messages, and its mentions together stay within [channels.MaxMentions].
// A message of a type with `choices:` on a channel type with a session is
// a question of the session's span (see [channels.Asked]).
func (s *Service) PostBatch(ctx context.Context, addr RawAddress, in BatchInput) (BatchResult, error) {
	c, openErr := s.open(ctx, addr)
	if openErr != nil {
		return BatchResult{}, openErr
	}
	if err := c.requireCaller(); err != nil {
		return BatchResult{}, err
	}
	if n := len(in.Messages); n == 0 || n > channels.MaxBatch {
		return BatchResult{}, invalid("a batch holds 1 to %d messages, not %d", channels.MaxBatch, n)
	}
	sess := c.ct.Session()
	mts := make([]*realtimecfg.MessageType, len(in.Messages))
	for i, bm := range in.Messages {
		mt := c.ct.MessageTypes[bm.Type]
		if mt == nil {
			return BatchResult{}, invalid("message %d: channel type %s has no message type %q", i, c.ct.Name, bm.Type)
		}
		if mt.Persist == realtimecfg.PersistEphemeral {
			return BatchResult{}, invalid("message %d: a batch posts log messages; %s is a signal", i, mt.Name)
		}
		if err := expectGenerationShape(sess, mt, in.ExpectGeneration); err != nil {
			return BatchResult{}, numbered(i, err)
		}
		if !c.mayPost(mt) {
			return BatchResult{}, numbered(i, postDenied(c.ct, mt))
		}
		mts[i] = mt
	}
	if err := c.requireOpen(); err != nil {
		return BatchResult{}, err
	}
	before := sessionChannel(c.head, c.now)
	for i, mt := range mts {
		if e := sessionRefusal(sess, mt, before); e != nil {
			return BatchResult{}, numbered(i, sessionWrongState(c.ct, mt, e))
		}
	}
	if e := generationRefusal(sess, before, in.ExpectGeneration); e != nil {
		return BatchResult{}, sessionMoved(e)
	}
	if err := chargeBatch(s.local.limiter, c, mts); err != nil {
		return BatchResult{}, err
	}
	posts := make([]logPost, len(mts))
	mentions := 0
	for i, mt := range mts {
		p, err := s.checkLogProps(c, mt, in.Messages[i].Props, 0, nil)
		if err != nil {
			return BatchResult{}, numbered(i, err)
		}
		mentions += len(p.targets)
		posts[i] = p
	}
	if mentions > channels.MaxMentions {
		return BatchResult{}, tooManyMentions()
	}
	if err := s.record(c); err != nil {
		return BatchResult{}, err
	}
	for i := range posts {
		//nolint:contextcheck // c.ctx is ctx with the acl.Request attached; see .golangci.yml on realtime.
		posts[i].sp = newSessionPost(c.ctx, c.ct, posts[i].mt, posts[i].props, posts[i].counts, c.now,
			poster{roles: c.transitionRoles(posts[i].mt)}, "", in.ExpectGeneration)
	}
	ms, sv, outs, err := s.appendLog(c, posts)
	if err != nil {
		return BatchResult{}, err
	}
	out := BatchResult{Messages: make([]MessageView, len(ms)), Session: sv, Outputs: outs}
	for i, m := range ms {
		out.Messages[i] = rendererWith(c, posts[i].refs).message(m)
	}
	return out, nil
}

// numbered prefixes the title of a denial of the message at index i of a
// batch with its index; another error is returned as it is.
func numbered(i int, err error) error {
	if d, ok := errors.AsType[*DenialError](err); ok {
		cp := *d
		cp.Title = fmt.Sprintf("message %d: %s", i, d.Title)
		return &cp
	}
	return err
}

// logPost is one log message of a post or a batch, after its checks: the
// validated properties, the refs they resolve, the mention targets, the
// files per property and the checked files, and the session part. A batch
// message holds no files.
type logPost struct {
	mt      *realtimecfg.MessageType
	props   map[string]any
	refs    map[string]RefView
	targets []channels.Mention
	replyTo int64
	counts  map[string]int
	files   []channels.NewFile
	sp      sessionPost
}

// chargeBatch charges the caller one token per message of mts, from the
// bucket of each message type, as one step: a refused charge spends nothing.
// A batch holding more messages of one type than that type's rate allows
// at all is refused with a 400, since no wait would let it through.
func chargeBatch(l *limiter, c *call, mts []*realtimecfg.MessageType) error {
	var cs []charge
	at := map[string]int{}
	for _, mt := range mts {
		i, ok := at[mt.Name]
		if !ok {
			i = len(cs)
			at[mt.Name] = i
			cs = append(cs, charge{op: opPostPfx + mt.Name, rate: postRate(c.snap.Config.Limits(), mt)})
		}
		cs[i].n++
		if cs[i].n > cs[i].rate.Count {
			return invalid("a batch holds at most %d messages of type %s, its rate limit", cs[i].rate.Count, mt.Name)
		}
	}
	return l.takeAll(c.now, bucketIdentity(c.p), c.key, cs)
}

// checkLogPost charges the caller the tokens of a post of mt (see
// takePostTokens), checks it (see Service.checkLogProps), then runs each
// of its files through the file checker.
func (s *Service) checkLogPost(
	c *call, mt *realtimecfg.MessageType, props map[string]any, replyTo int64, files []FileInput,
) (logPost, error) {
	if err := takePostTokens(s.local.limiter, c, mt, len(files) > 0); err != nil {
		return logPost{}, err
	}
	p, err := s.checkLogProps(c, mt, props, replyTo, files)
	if err != nil {
		return logPost{}, err
	}
	p.files, err = checkFiles(c, s.files, mt, files)
	if err != nil {
		return logPost{}, err
	}
	if len(p.files) > 0 {
		// A scan or transform may take a while; the message is stamped
		// when its files are checked, not when the request arrived.
		c.now = s.clock()
	}
	return p, nil
}

// checkLogProps checks the shape of the files of a post of mt (see
// checkFileShape), its properties (see Service.checkMessageKeeping) and
// its mentions. A file property counts as its number of files.
//
// Nil: files is accepted and means none.
func (s *Service) checkLogProps(
	c *call, mt *realtimecfg.MessageType, props map[string]any, replyTo int64, files []FileInput,
) (logPost, error) {
	if err := checkFileShape(mt, files); err != nil {
		return logPost{}, err
	}
	counts := inputCounts(files)
	mplan := &mentionPlan{}
	vp, refs, err := s.checkMessageKeeping(c, mt, props, counts, c.snap.Config.Limits().MessageBytes, nil, mplan)
	if err != nil {
		return logPost{}, err
	}
	targets := mplan.targets(refs, c.snap.Config.MentionUserType(), c.p.User)
	if len(targets) > channels.MaxMentions {
		return logPost{}, tooManyMentions()
	}
	return logPost{mt: mt, props: vp.props, refs: refs, targets: targets, replyTo: replyTo, counts: counts}, nil
}

// asks is the answer type a post of p on ct asks for: the `choices:`
// answer of a top-level message on a channel type with a session, else "".
// Without a session no move would ever end the span, so nothing is asked.
func asks(ct realtimecfg.ChannelType, p logPost) string {
	if ct.Session() == nil || p.mt.Choices == nil || p.replyTo != 0 {
		return ""
	}
	return p.mt.Choices.Answer
}

// newMessages is what the store appends for posts of c: the caller's
// messages, the last of which moves the caller's read marker.
func newMessages(c *call, posts []logPost) []channels.NewMessage {
	out := make([]channels.NewMessage, len(posts))
	for i, p := range posts {
		out[i] = channels.NewMessage{
			Type: p.mt.Name, Author: authorOf(c.p), At: c.now, Props: p.props, ReplyTo: p.replyTo,
			Session: p.sp.step, MarkRead: i == len(posts)-1, Mentions: p.targets, NoUnread: p.mt.NoUnread,
			Asks: asks(c.ct, p), Files: p.files, InlineUnder: inlineUnder(&c.ct, p.mt.Name),
		}
	}
	return out
}

// appendLog pre-checks the outputs of posts, appends them as one store
// step and tells the process-local state. It returns the stored messages,
// the session after them, and the outputs of the move they made after the
// inline run.
func (s *Service) appendLog(
	c *call, posts []logPost,
) ([]channels.Message, *SessionView, *OutputsView, error) {
	plan, err := s.precheckOutputs(c, posts)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, p := range posts {
		if p.sp.step != nil {
			p.sp.step.Mark = personMark(c.ct.Session(), p.mt.Name, authorOf(c.p), c.p.Roles(), c.now, plan)
		}
	}
	ms, err := s.store.AppendBatch(c.ctx, c.key, newMessages(c, posts))
	for _, p := range posts {
		p.sp.logGuardFailure(c.ctx)
	}
	if err != nil {
		return nil, nil, nil, appendRefusal(c, posts, err)
	}
	var last *channels.SessionStep
	var mover *channels.SessionStep
	for _, p := range posts {
		if p.sp.step == nil {
			continue
		}
		last = p.sp.step
		if p.sp.step.Outcome.Moved {
			mover = p.sp.step
		}
	}
	noteDue(s.dueWake, last)
	for _, m := range ms {
		s.local.notePosted(c, m)
	}
	sv := postedSession(c, last)
	var outs *OutputsView
	if plan.marked && mover != nil {
		outs = s.runInline(c, mover.Outcome.Notice, plan)
		if outs != nil && outs.Status != OutputsPending {
			// The on_final close may have closed the channel, and a set
			// may have handed a role to someone else.
			sv = s.sessionAfterOutputs(c, sv, wroteRoleProperty(c.ct, outs))
		}
	}
	return ms, sv, outs, nil
}
