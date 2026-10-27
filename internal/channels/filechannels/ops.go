package filechannels

import (
	"bytes"
	"container/heap"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Append implements [channels.Store]. With a session step that moves the
// session, the notice follows the message in the log, and the session
// audit record is written last (see the package doc).
func (s *Store) Append(ctx context.Context, k channels.Key, nm channels.NewMessage) (channels.Message, error) {
	if err := k.Validate(); err != nil {
		return channels.Message{}, err
	}
	if err := channels.PrepareAppendStep(nm.Session); err != nil {
		return channels.Message{}, err
	}
	nm, _, err := channels.PrepareMessage(nm)
	if err != nil {
		return channels.Message{}, err
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return channels.Message{}, err
	}
	defer unlock()

	m, created, err := s.appendTarget(k, nm)
	if err != nil {
		return channels.Message{}, err
	}
	last, err := s.logEnd(k, m)
	if err != nil {
		return channels.Message{}, err
	}
	parent, parentSeg, err := s.replyParent(k, nm.ReplyTo)
	if err != nil {
		return channels.Message{}, err
	}
	dropDangling(m, last)
	committed := m.channel(k)
	committed.LastSeq = last
	move, moved, err := channels.DecideSession(nm.Session, committed)
	if err != nil {
		return channels.Message{}, err
	}

	seq := last + 1
	idx := s.idx(seq)
	msg := channels.Message{
		Seq: seq, Rev: m.bump(idx), Type: nm.Type, Author: nm.Author, At: nm.At, Props: nm.Props,
		ReplyTo: nm.ReplyTo, SchemaHash: nm.SchemaHash, TriggeredBy: nm.TriggeredBy, MovedSession: moved,
	}
	recs := []segRecord{{idx: idx, rec: encodeLine(line{K: "m", Message: msg})}}
	if nm.ReplyTo > 0 {
		count := parent.ReplyCount + 1
		recs = append(recs, segRecord{idx: s.idx(parent.Seq), seg: parentSeg, rec: encodeLine(line{K: "p",
			Message: channels.Message{Seq: parent.Seq, Rev: m.bump(s.idx(parent.Seq))}, ReplyCount: &count})})
	}
	m.advance(seq, msg.At)
	var after channels.Channel
	if moved {
		var notice channels.Message
		after, notice, err = moveSession(m, k, nm, msg, move, s.idx(seq), s.idx(seq+1))
		if err != nil {
			return channels.Message{}, err
		}
		recs = append(recs, segRecord{idx: s.idx(notice.Seq), rec: encodeLine(line{K: "m", Message: notice})})
	}
	if schedErr := m.scheduleStep(k, nm.Session); schedErr != nil {
		return channels.Message{}, schedErr
	}
	if created {
		// The create record goes before meta.yaml: a crash in between
		// leaves a create record for a channel the next append creates
		// again, so the trail may repeat a create but never misses one.
		err = appendCreateAudit(&s.disk, k, nm.Author, nm.At)
		if err != nil {
			return channels.Message{}, err
		}
	}
	err = s.writeMeta(k, m)
	if err != nil {
		return channels.Message{}, err
	}
	if err := s.appendRecords(k, recs); err != nil {
		return channels.Message{}, err
	}
	if moved {
		rec := channels.SessionAudit(k, nm.Session, msg, move, after.Session.Generation)
		if err := s.appendAudit(k.ID, rec); err != nil {
			return channels.Message{}, err
		}
	}
	channels.FillSessionOutcome(nm.Session, m.channel(k), move, moved)
	if nm.MarkRead {
		markAfterWrite(&s.disk, k, channels.ReaderOf(nm.Author), m.LastSeq, nm.At)
	}
	return msg, nil
}

// appendCreateAudit appends the create record of channel k, by by at at.
func appendCreateAudit(d *disk, k channels.Key, by channels.Author, at time.Time) error {
	rec := channels.KeyAudit(k, channels.AuditCreate, at)
	rec.By = by
	return d.appendAudit(k.ID, rec)
}

// segRecord is a log record of an append and the segment it goes to. seg,
// when set, is the segment as the caller already read it.
type segRecord struct {
	idx int64
	seg []byte
	rec []byte
}

// appendRecords appends recs to their segments, in order. Records for one
// segment go in one appendLog call, so the segment is read once.
func (d *disk) appendRecords(k channels.Key, recs []segRecord) error {
	groups := map[int64][]byte{}
	seen := map[int64][]byte{}
	var order []int64
	for _, r := range recs {
		if _, ok := groups[r.idx]; !ok {
			order = append(order, r.idx)
		}
		groups[r.idx] = append(groups[r.idx], r.rec...)
		if r.seg != nil {
			seen[r.idx] = r.seg
		}
	}
	for _, idx := range order {
		cur, ok := seen[idx]
		if !ok {
			var err error
			if cur, err = d.segment(k, idx); err != nil {
				return err
			}
		}
		if err := d.appendLog(k, idx, cur, groups[idx]); err != nil {
			return err
		}
	}
	return nil
}

// moveSession applies move, caused by cause at causeIdx, to m and returns
// the channel after it and the notice at idx. It records the notice in m:
// the log's last seq, the notice's pending action and, for a job move, the
// log records of the cause and the notice (see stashJob).
func moveSession(
	m *meta, k channels.Key, nm channels.NewMessage, cause channels.Message, move channels.SessionMove,
	causeIdx, idx int64,
) (channels.Channel, channels.Message, error) {
	after := m.channel(k)
	channels.ApplySession(&after, move, nm.At)
	m.setSession(after.Session)
	notice, _, err := channels.SessionNotice(nm.Session, cause, move, after.Session.Generation, m.bump(idx))
	if err != nil {
		return after, notice, err
	}
	m.advance(notice.Seq, notice.At)
	if notice.Action != nil {
		setPending(m, notice.Seq, *notice.Action)
	}
	// A trigger is never a reply, so a job move has no parent record.
	if nm.ReplyTo == 0 && isJobNotice(notice) {
		stashJob(m, notice, []segRecord{
			{idx: causeIdx, rec: encodeLine(line{K: "m", Message: cause})},
			{idx: idx, rec: encodeLine(line{K: "m", Message: notice})},
		}, []int64{cause.Seq, notice.Seq})
	}
	return after, notice, nil
}

// appendTarget returns the head of the channel an append goes to, or a new
// head for an implicit channel's first message. created reports the latter.
// A channel recorded with another incarnation is [channels.ErrStaleAnchor].
func (s *Store) appendTarget(k channels.Key, nm channels.NewMessage) (m *meta, created bool, err error) {
	m, err = s.readMeta(k)
	if errors.Is(err, channels.ErrNotFound) && k.Name == "" {
		return &meta{
			Incarnation: nm.Incarnation, State: channels.StateOpen, CreatedBy: nm.Author, CreatedAt: nm.At,
		}, true, nil
	}
	if err == nil {
		err = channels.CheckAppendIncarnation(k, m.Incarnation, nm.Incarnation)
	}
	if err == nil {
		err = channels.CheckAppendState(m.State, nm)
	}
	return m, false, err
}

// replyParent returns the message a reply answers and the segment holding
// it. It refuses a missing, removed or reply parent. A replyTo of zero is
// not a reply and returns nothing.
func (s *Store) replyParent(k channels.Key, replyTo int64) (channels.Message, []byte, error) {
	if replyTo == 0 {
		return channels.Message{}, nil, nil
	}
	msgs, i, err := s.loadMessage(k, replyTo)
	if errors.Is(err, channels.ErrNotFound) {
		return channels.Message{}, nil, fmt.Errorf("%w: message %d does not exist", channels.ErrInvalidReply, replyTo)
	}
	if err != nil {
		return channels.Message{}, nil, err
	}
	parent := msgs[i]
	if parent.Removed != nil || parent.ReplyTo != 0 {
		return channels.Message{}, nil, fmt.Errorf("%w: message %d is removed or a reply",
			channels.ErrInvalidReply, replyTo)
	}
	seg, err := s.segment(k, s.idx(parent.Seq))
	if err != nil {
		return channels.Message{}, nil, err
	}
	return parent, seg, nil
}

// mutate loads message seq of k under the lock, lets change edit it and
// reserve its rev, writes meta.yaml and then rewrites the segment. A
// removed message and a message with a pending action are refused.
func (s *Store) mutate(
	ctx context.Context, k channels.Key, seq int64,
	change func(m *meta, msg *channels.Message, idx int64) error,
) (channels.Message, error) {
	unlock, err := s.lock(ctx)
	if err != nil {
		return channels.Message{}, err
	}
	defer unlock()
	m, err := s.readMeta(k)
	if err != nil {
		return channels.Message{}, err
	}
	msgs, i, err := s.loadMessage(k, seq)
	if err != nil {
		return channels.Message{}, err
	}
	if msgs[i].Removed != nil {
		return channels.Message{}, fmt.Errorf("%w: message %d in %s", channels.ErrRemoved, seq, k)
	}
	if err := channels.CheckNotPending(&msgs[i]); err != nil {
		return channels.Message{}, err
	}
	idx := s.idx(seq)
	if err := change(m, &msgs[i], idx); err != nil {
		return channels.Message{}, err
	}
	if err := s.writeMeta(k, m); err != nil {
		return channels.Message{}, err
	}
	if err := s.rewrite(k, idx, msgs); err != nil {
		return channels.Message{}, err
	}
	return msgs[i], nil
}

// Edit implements [channels.Store].
func (s *Store) Edit(
	ctx context.Context, k channels.Key, seq int64, props map[string]any, at time.Time,
) (channels.Message, error) {
	if err := k.Validate(); err != nil {
		return channels.Message{}, err
	}
	props, _, err := channels.PrepareProps(props)
	if err != nil {
		return channels.Message{}, err
	}
	at = channels.NormTime(at)
	return s.mutate(ctx, k, seq, func(m *meta, msg *channels.Message, idx int64) error {
		msg.Props, msg.EditedAt, msg.Rev = props, at, m.bump(idx)
		return nil
	})
}

// Remove implements [channels.Store].
func (s *Store) Remove(ctx context.Context, k channels.Key, seq int64, r channels.Removal) (channels.Message, error) {
	if err := k.Validate(); err != nil {
		return channels.Message{}, err
	}
	r, err := channels.PrepareRemoval(r)
	if err != nil {
		return channels.Message{}, err
	}
	msg, err := s.mutateRemove(ctx, k, seq, r)
	if err != nil {
		return channels.Message{}, err
	}
	return msg, nil
}

func (s *Store) mutateRemove(
	ctx context.Context, k channels.Key, seq int64, r channels.Removal,
) (channels.Message, error) {
	var out channels.Message
	var auditErr error
	_, err := s.mutate(ctx, k, seq, func(m *meta, msg *channels.Message, idx int64) error {
		*msg = channels.Message{
			Seq: msg.Seq, Rev: m.bump(idx), ReplyTo: msg.ReplyTo, ReplyCount: msg.ReplyCount, Removed: &r,
		}
		delete(m.Pending, seq)
		out = *msg
		rec := channels.KeyAudit(k, channels.AuditRemove, r.At)
		rec.Seq, rec.By = seq, r.By
		// The audit record goes first: a removal that is not audited must
		// not happen, and the record holds nothing the removal would erase.
		auditErr = s.appendAudit(k.ID, rec)
		return auditErr
	})
	return out, err
}

// SetAction implements [channels.Store].
func (s *Store) SetAction(
	ctx context.Context, k channels.Key, seq int64, name string, expect channels.ActionStatus, next channels.Action,
) (channels.Message, error) {
	if err := k.Validate(); err != nil {
		return channels.Message{}, err
	}
	next, err := channels.PrepareAction(name, next)
	if err != nil {
		return channels.Message{}, err
	}
	if relErr := channels.CheckRelease(expect, next); relErr != nil {
		return channels.Message{}, relErr
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return channels.Message{}, err
	}
	defer unlock()
	m, err := s.readMeta(k)
	if err != nil {
		return channels.Message{}, err
	}
	msgs, i, err := s.loadMessage(k, seq)
	if err != nil {
		return channels.Message{}, err
	}
	msg := msgs[i]
	if msg.Removed != nil {
		return channels.Message{}, fmt.Errorf("%w: message %d in %s", channels.ErrRemoved, seq, k)
	}
	err = channels.CheckActionCAS(msg.Action, name, expect)
	if err != nil {
		return channels.Message{}, err
	}
	if outErr := channels.CheckOutputStep(msg.Action, name, expect, next); outErr != nil {
		return channels.Message{}, outErr
	}
	if stateErr := channels.CheckActionState(m.State, expect, next); stateErr != nil {
		return channels.Message{}, stateErr
	}
	idx := s.idx(seq)
	msg.Rev = m.bump(idx)
	l := line{K: "p", Message: channels.Message{Seq: seq, Rev: msg.Rev}}
	if next.Status == channels.ActionNone {
		msg.Action, l.Clear = nil, true
		delete(m.Pending, seq)
	} else {
		a := next
		msg.Action, l.Action = &a, &a
		if next.Status == channels.ActionPending {
			setPending(m, seq, next)
		} else {
			delete(m.Pending, seq)
		}
	}
	err = s.writeMeta(k, m)
	if err != nil {
		return channels.Message{}, err
	}
	cur, err := s.segment(k, idx)
	if err != nil {
		return channels.Message{}, err
	}
	if err := s.appendLog(k, idx, cur, encodeLine(l)); err != nil {
		return channels.Message{}, err
	}
	if rec, ok := channels.ActionAudit(k, seq, name, expect, next); ok {
		if err := s.appendAudit(k.ID, rec); err != nil {
			return channels.Message{}, err
		}
	}
	return msg, nil
}

// CreateMany implements [channels.Store].
func (s *Store) CreateMany(
	ctx context.Context, a channels.Anchor, nc channels.NewChannel,
) (channels.Channel, error) {
	nc, err := channels.PrepareChannel(a, nc)
	if err != nil {
		return channels.Channel{}, err
	}
	typ := nc.Type
	unlock, err := s.lock(ctx)
	if err != nil {
		return channels.Channel{}, err
	}
	defer unlock()
	names, err := s.readDir(faceDir(a) + "/" + typ)
	if err != nil {
		return channels.Channel{}, fmt.Errorf("filechannels: list %s: %w", a.ID, err)
	}
	taken := map[string]bool{}
	for _, n := range names {
		if n != none {
			taken[n] = true
		}
	}
	if len(taken) >= nc.Cap() {
		return channels.Channel{}, fmt.Errorf("%w: %s holds %d %s channels",
			channels.ErrTooManyChannels, a.ID, len(taken), typ)
	}
	k := channels.Key{Anchor: a, Type: typ, Name: channels.NewName()}
	for taken[k.Name] { // coverage-ignore: a 64-bit collision is not reproducible
		k.Name = channels.NewName()
	}
	m := &meta{
		Incarnation: nc.Incarnation, State: channels.StateOpen, Title: nc.Title, CreatedBy: nc.By, CreatedAt: nc.At,
	}
	if err := m.schedule(k, nc.SessionDue); err != nil {
		return channels.Channel{}, err
	}
	// The create record goes first, for the reason Append writes it first.
	if err := appendCreateAudit(&s.disk, k, nc.By, nc.At); err != nil {
		return channels.Channel{}, err
	}
	if err := s.writeMeta(k, m); err != nil {
		return channels.Channel{}, err
	}
	if nc.MarkRead {
		markAfterWrite(&s.disk, k, channels.ReaderOf(nc.By), 0, nc.At)
	}
	return m.channel(k), nil
}

// SetLifecycle implements [channels.Store].
func (s *Store) SetLifecycle(
	ctx context.Context, k channels.Key, expect channels.State, t channels.Transition,
) (channels.Channel, error) {
	if err := k.Validate(); err != nil {
		return channels.Channel{}, err
	}
	t, err := channels.PrepareTransition(expect, t)
	if err != nil {
		return channels.Channel{}, err
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return channels.Channel{}, err
	}
	defer unlock()
	m, err := s.readMeta(k)
	if err != nil {
		return channels.Channel{}, err
	}
	if m.State != expect {
		return channels.Channel{}, fmt.Errorf("%w: %s is %s, expected %s", channels.ErrConflict, k, m.State, expect)
	}
	c := m.channel(k)
	channels.ApplyTransition(&c, t)
	m.State, m.ClosedAt, m.ArchivedAt, m.ReopenedAt = c.State, c.ClosedAt, c.ArchivedAt, c.ReopenedAt
	m.Rev++
	if err := m.schedule(k, t.SessionDue); err != nil {
		return channels.Channel{}, err
	}
	if err := s.writeMeta(k, m); err != nil {
		return channels.Channel{}, err
	}
	rec := channels.KeyAudit(k, channels.AuditLifecycle, t.At)
	rec.By, rec.From, rec.To, rec.TriggeredBy = t.By, string(expect), string(t.To), t.TriggeredBy
	if err := s.appendAudit(k.ID, rec); err != nil {
		return channels.Channel{}, err
	}
	return s.head(k, m)
}

// Delete implements [channels.Store].
func (s *Store) Delete(ctx context.Context, k channels.Key, d channels.Deletion) error {
	if err := k.Validate(); err != nil {
		return err
	}
	d, err := channels.PrepareDeletion(d)
	if err != nil {
		return err
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	m, err := s.readMeta(k)
	if err != nil {
		return err
	}
	if err := channels.CheckDeletion(k, m.State, m.Rev, d); err != nil {
		return err
	}
	rec := channels.KeyAudit(k, channels.AuditDelete, d.At)
	rec.By = d.By
	if err := s.appendAudit(k.ID, rec); err != nil {
		return err
	}
	// Moving the directory aside first takes the channel out of reach in
	// one rename; the removal that follows may be slow or interrupted.
	// Removing all of the trash, not just this channel, finishes any
	// earlier delete that was interrupted, so deleted content does not
	// outlive the next delete.
	trash := trashDir + "/" + channels.NewName()
	if err := s.move(chanDir(k), trash); err != nil {
		return fmt.Errorf("filechannels: delete %s: %w", k, err)
	}
	s.pruneEmpty(k)
	if err := s.removeAll(trashDir); err != nil {
		return fmt.Errorf("filechannels: delete %s: %w", k, err)
	}
	return nil
}

// Rekey implements [channels.Store].
func (s *Store) Rekey(ctx context.Context, oldID, newID string, at time.Time) error {
	if err := channels.CheckRekey(oldID, newID); err != nil {
		return err
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	occupied := false
	err = s.visitEntity(newID, "", func(channels.Key, *meta) error {
		occupied = true
		return nil
	})
	if err != nil {
		return err
	}
	if occupied {
		return fmt.Errorf("%w: %s", channels.ErrRekeyConflict, newID)
	}
	hasDir, err := s.exists(enc(oldID))
	if err != nil {
		return fmt.Errorf("filechannels: rekey %s: %w", oldID, err)
	}
	oldAudit, err := s.readFile(auditKey(oldID))
	if err != nil {
		return fmt.Errorf("filechannels: rekey %s: %w", oldID, err)
	}
	if !hasDir && oldAudit == nil {
		return nil
	}
	if hasDir {
		// A directory left with no channel in it would make the rename
		// merge into it on some filesystems; it holds nothing to keep.
		if err = s.removeAll(enc(newID)); err != nil {
			return fmt.Errorf("filechannels: rekey %s: %w", newID, err)
		}
		if err = s.move(enc(oldID), enc(newID)); err != nil {
			return fmt.Errorf("filechannels: rekey %s to %s: %w", oldID, newID, err)
		}
	}
	newAudit, err := s.readFile(auditKey(newID))
	if err != nil {
		return fmt.Errorf("filechannels: rekey %s: %w", newID, err)
	}
	b, err := json.Marshal(channels.AuditRecord{
		At: channels.NormTime(at), Op: channels.AuditRekey, From: oldID, To: newID,
	})
	if err != nil { // coverage-ignore: invariant: plain data
		return err
	}
	merged := append(append(complete(oldAudit), complete(newAudit)...), append(b, '\n')...)
	if err := s.rfs.WriteFile(auditKey(newID), merged, filePerm); err != nil {
		return fmt.Errorf("filechannels: rekey audit of %s: %w", oldID, err)
	}
	if oldAudit != nil {
		if err := s.rfs.Remove(auditKey(oldID)); err != nil && !notExist(err) {
			return fmt.Errorf("filechannels: rekey audit of %s: %w", oldID, err)
		}
	}
	return nil
}

// Tombstone implements [channels.Store]. A face batch goes to
// ~detached/<id>-<face>-<stamp>, a whole-entity batch to
// ~detached/<id>-<stamp> with its audit trail at _audit/<id>-<stamp>.jsonl.
// The filing is written before the batch moves (see filingKey), so a
// crash between the two leaves a filing without a batch,
// never a batch without its filing.
func (s *Store) Tombstone(
	ctx context.Context, id string, face entity.Face, at time.Time, file channels.Filer,
) (channels.TombstoneHandle, error) {
	a := channels.Anchor{ID: id, Face: face}
	if err := a.Validate(); err != nil {
		return channels.TombstoneHandle{}, err
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return channels.TombstoneHandle{}, err
	}
	defer unlock()
	h := channels.NewTombstoneHandle(a, channels.TombstoneStamp(at))
	rec := channels.AuditRecord{At: channels.NormTime(at), Op: channels.AuditTombstone, Face: face}
	var f channels.Filing

	if face != "" {
		ok, existsErr := s.exists(faceDir(a))
		if existsErr != nil || !ok {
			return channels.TombstoneHandle{}, existsErr
		}
		f, err = fileBatch(&s.disk, id, face, file)
		if err != nil {
			return channels.TombstoneHandle{}, err
		}
		if fileErr := writeFiling(&s.disk, h, f); fileErr != nil {
			return channels.TombstoneHandle{}, fileErr
		}
		if moveErr := s.move(faceDir(a), detachedKey(h)); moveErr != nil {
			return channels.TombstoneHandle{}, fmt.Errorf("filechannels: tombstone %s@%s: %w", id, face, moveErr)
		}
		s.pruneEmpty(channels.Key{Anchor: a, Type: none})
		return h, s.appendAudit(id, rec)
	}

	moved, err := s.exists(enc(id))
	if err != nil {
		return channels.TombstoneHandle{}, err
	}
	if moved {
		f, err = fileBatch(&s.disk, id, "", file)
		if err != nil {
			return channels.TombstoneHandle{}, err
		}
		if fileErr := writeFiling(&s.disk, h, f); fileErr != nil {
			return channels.TombstoneHandle{}, fileErr
		}
		if err = s.move(enc(id), detachedKey(h)); err != nil {
			return channels.TombstoneHandle{}, fmt.Errorf("filechannels: tombstone %s: %w", id, err)
		}
	}
	hasAudit, err := s.exists(auditKey(id))
	if err != nil {
		return channels.TombstoneHandle{}, err
	}
	if !hasAudit {
		if !moved {
			return channels.TombstoneHandle{}, nil
		}
		return h, nil
	}
	if err := s.appendAudit(id, rec); err != nil {
		return h, err
	}
	if err := s.move(auditKey(id), detachedAuditKey(h)); err != nil {
		return h, fmt.Errorf("filechannels: tombstone audit of %s: %w", id, err)
	}
	return h, nil
}

// fileBatch reads the channels a tombstone of id@face moves and runs file
// over them. The caller holds the store lock, so the list is the batch.
func fileBatch(d *disk, id string, face entity.Face, file channels.Filer) (channels.Filing, error) {
	if file == nil {
		return channels.Filing{}, nil
	}
	var moved []channels.Channel
	err := d.visitEntity(id, "", func(k channels.Key, m *meta) error {
		if face == "" || k.Face == face {
			moved = append(moved, m.channel(k))
		}
		return nil
	})
	if err != nil {
		return channels.Filing{}, err
	}
	return channels.FileBatch(file, moved)
}

// detachedKey is where the batch h was filed.
func detachedKey(h channels.TombstoneHandle) string {
	a := h.Anchor()
	if a.Face != "" {
		return detachedDir + "/" + enc(a.ID) + "-" + faceSeg(a.Face) + "-" + h.Stamp()
	}
	return detachedDir + "/" + enc(a.ID) + "-" + h.Stamp()
}

// detachedAuditKey is where a whole-entity batch filed its audit trail.
func detachedAuditKey(h channels.TombstoneHandle) string {
	return auditDir + "/" + enc(h.Anchor().ID) + "-" + h.Stamp() + ".jsonl"
}

// Untombstone implements [channels.Store].
//
// resume runs under the same lock before the batch goes back in place:
// each open channel's meta.yaml is rewritten with its result while it is
// still under ~detached, and the move then restores the channel with its
// resumed schedule in one rename. A crash before the move leaves the batch
// tombstoned with the restart anchors of this attempt, which a later
// restore applies its own resume over; a crash after it leaves the
// channel restored and resumed. No crash restores a channel without its
// restart anchors, which would fire a timer that came due while it was
// tombstoned at once.
func (s *Store) Untombstone(
	ctx context.Context, h channels.TombstoneHandle, at time.Time, resume channels.ScheduleFunc,
) error {
	if h.IsZero() {
		return nil
	}
	if err := h.Validate(); err != nil {
		return err
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	a := h.Anchor()
	hasDir, err := s.exists(detachedKey(h))
	if err != nil {
		return fmt.Errorf("filechannels: untombstone %s: %w", h, err)
	}
	var tombAudit []byte
	if a.Face == "" {
		if tombAudit, err = s.readFile(detachedAuditKey(h)); err != nil {
			return fmt.Errorf("filechannels: untombstone %s: %w", h, err)
		}
	}
	if !hasDir && tombAudit == nil {
		return fmt.Errorf("%w: tombstone %s", channels.ErrNotFound, h)
	}
	f, hasFiling, err := readFiling(&s.disk, h)
	if err != nil {
		return err
	}
	if hasFiling && !f.Restorable() {
		return fmt.Errorf("%w: tombstone %s is final", channels.ErrConflict, h)
	}
	occupied, err := occupied(&s.disk, a)
	if err != nil {
		return err
	}
	if occupied {
		return &channels.OccupiedError{Anchor: a}
	}
	if hasDir {
		if err := resumeDetached(&s.disk, h, resume); err != nil {
			return err
		}
		live := enc(a.ID)
		if a.Face != "" {
			live = faceDir(a)
		}
		// A directory left with no channel in it would make the rename
		// merge into it on some filesystems; it holds nothing to keep.
		if err := s.removeAll(live); err != nil {
			return fmt.Errorf("filechannels: untombstone %s: %w", h, err)
		}
		if err := s.move(detachedKey(h), live); err != nil {
			return fmt.Errorf("filechannels: untombstone %s: %w", h, err)
		}
	}
	if err := removeFiling(&s.disk, h); err != nil {
		return err
	}
	return s.restoreAudit(h, tombAudit, at)
}

// resumeDetached applies resume to every open channel of the batch h
// while it is still filed under ~detached, so the move that restores it
// carries the resumed schedule. Nil resume changes nothing.
func resumeDetached(d *disk, h channels.TombstoneHandle, resume channels.ScheduleFunc) error {
	if resume == nil {
		return nil
	}
	a, dir := h.Anchor(), detachedKey(h)
	// live is the prefix of a channel's live directory that dir stands in
	// for: the entity directory for a whole-entity batch, the face
	// directory for a face batch.
	live := enc(a.ID)
	if a.Face != "" {
		live = faceDir(a)
	}
	visit := func(k channels.Key, m *meta) error {
		if m.State != channels.StateOpen {
			return nil
		}
		if err := m.schedule(k, resume); err != nil {
			return err
		}
		return writeMetaAt(d, dir+strings.TrimPrefix(chanDir(k), live), k, m)
	}
	if a.Face == "" {
		return visitEntityAt(d, dir, a.ID, "", visit)
	}
	return visitFaceAt(d, dir, a, "", visit)
}

// occupied reports whether a live channel sits on a: any channel of the id
// for an empty face, any channel of the face otherwise.
func occupied(d *disk, a channels.Anchor) (bool, error) {
	found := false
	visit := func(channels.Key, *meta) error {
		found = true
		return nil
	}
	var err error
	if a.Face == "" {
		err = d.visitEntity(a.ID, "", visit)
	} else {
		err = d.visitFace(a, "", visit)
	}
	return found, err
}

// restoreAudit puts a whole-entity batch's filed audit trail back in front
// of whatever the live trail recorded since, and appends the restore.
func (s *Store) restoreAudit(h channels.TombstoneHandle, tombAudit []byte, at time.Time) error {
	id := h.Anchor().ID
	rec := channels.AuditRecord{At: channels.NormTime(at), Op: channels.AuditUntombstone, Face: h.Anchor().Face}
	if tombAudit == nil {
		return s.appendAudit(id, rec)
	}
	live, err := s.readFile(auditKey(id))
	if err != nil {
		return fmt.Errorf("filechannels: untombstone audit of %s: %w", id, err)
	}
	b, err := json.Marshal(rec)
	if err != nil { // coverage-ignore: invariant: plain data
		return err
	}
	merged := append(append(complete(tombAudit), complete(live)...), append(b, '\n')...)
	if err := s.rfs.WriteFile(auditKey(id), merged, filePerm); err != nil {
		return fmt.Errorf("filechannels: untombstone audit of %s: %w", id, err)
	}
	if err := s.rfs.Remove(detachedAuditKey(h)); err != nil && !notExist(err) {
		return fmt.Errorf("filechannels: untombstone audit of %s: %w", id, err)
	}
	return nil
}

// head returns the channel with its last seq read from the log.
func (s *Store) head(k channels.Key, m *meta) (channels.Channel, error) {
	last, err := s.lastSeq(k)
	if err != nil {
		return channels.Channel{}, err
	}
	c := m.channel(k)
	c.LastSeq = last
	return c, nil
}

// Head implements [channels.Store]. It takes the lock so it never reports
// a rev whose records are still being written.
func (s *Store) Head(ctx context.Context, k channels.Key) (channels.Channel, error) {
	if err := k.Validate(); err != nil {
		return channels.Channel{}, err
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return channels.Channel{}, err
	}
	defer unlock()
	m, err := s.readMeta(k)
	if err != nil {
		return channels.Channel{}, err
	}
	return s.head(k, m)
}

// Get implements [channels.Store].
func (s *Store) Get(_ context.Context, k channels.Key, seq int64) (channels.Message, error) {
	if err := k.Validate(); err != nil {
		return channels.Message{}, err
	}
	if _, err := s.readMeta(k); err != nil {
		return channels.Message{}, err
	}
	msgs, i, err := s.loadMessage(k, seq)
	if err != nil {
		return channels.Message{}, err
	}
	return msgs[i], nil
}

// Messages implements [channels.Store].
func (s *Store) Messages(_ context.Context, k channels.Key, p channels.Page) ([]channels.Message, error) {
	if err := k.Validate(); err != nil {
		return nil, err
	}
	p, err := p.Normalize()
	if err != nil {
		return nil, err
	}
	m, err := s.readMeta(k)
	if err != nil {
		return nil, err
	}
	idxs, err := s.segments(k)
	if err != nil {
		return nil, err
	}
	switch {
	case p.RevOrder():
		return s.byRev(k, m, idxs, p)
	case p.AfterSeq > 0:
		return s.afterSeq(k, idxs, p)
	default:
		return s.beforeSeq(k, idxs, p)
	}
}

// afterSeq reads up to p.Limit messages after p.AfterSeq, oldest first.
func (s *Store) afterSeq(k channels.Key, idxs []int64, p channels.Page) ([]channels.Message, error) {
	out := []channels.Message{}
	for _, idx := range idxs {
		if len(out) == p.Limit {
			break
		}
		if idx < s.idx(p.AfterSeq+1) {
			continue
		}
		data, err := s.segment(k, idx)
		if err != nil {
			return nil, err
		}
		for _, msg := range fold(data, segKey(k, idx)) {
			if msg.Seq > p.AfterSeq && len(out) < p.Limit {
				out = append(out, msg)
			}
		}
	}
	return out, nil
}

// beforeSeq reads the newest p.Limit messages before p.BeforeSeq, or the
// newest of all without it, and returns them oldest first.
func (s *Store) beforeSeq(k channels.Key, idxs []int64, p channels.Page) ([]channels.Message, error) {
	upper := int64(1<<62 - 1)
	if p.BeforeSeq > 0 {
		upper = p.BeforeSeq - 1
	}
	var out []channels.Message
	for i := len(idxs) - 1; i >= 0 && len(out) < p.Limit; i-- {
		if upper < 1 || idxs[i] > s.idx(upper) {
			continue
		}
		data, err := s.segment(k, idxs[i])
		if err != nil {
			return nil, err
		}
		msgs := fold(data, segKey(k, idxs[i]))
		for j := len(msgs) - 1; j >= 0 && len(out) < p.Limit; j-- {
			if msgs[j].Seq <= upper {
				out = append(out, msgs[j])
			}
		}
	}
	slices.Reverse(out)
	if out == nil {
		out = []channels.Message{}
	}
	return out, nil
}

// byRev reads the p.Limit messages with the lowest revs above p.AfterRev,
// ordered by rev. A rev cursor cannot walk the log in order, because a patch
// gives an old message a new rev, so it reads every segment whose highest
// rev is above the cursor. It keeps only the best p.Limit candidates in a
// heap, so memory and sorting stay bounded by the page, not the channel.
func (s *Store) byRev(k channels.Key, m *meta, idxs []int64, p channels.Page) ([]channels.Message, error) {
	best := &revHeap{}
	for _, idx := range idxs {
		if idx < int64(len(m.SegRev)) && m.SegRev[idx] <= p.AfterRev {
			continue
		}
		data, err := s.segment(k, idx)
		if err != nil {
			return nil, err
		}
		for _, msg := range fold(data, segKey(k, idx)) {
			if msg.Rev <= p.AfterRev {
				continue
			}
			if best.Len() < p.Limit {
				heap.Push(best, msg)
			} else if msg.Rev < (*best)[0].Rev {
				(*best)[0] = msg
				heap.Fix(best, 0)
			}
		}
	}
	out := make([]channels.Message, best.Len())
	for i := len(out) - 1; i >= 0; i-- {
		out[i], _ = heap.Pop(best).(channels.Message) // revHeap holds only messages
	}
	return out, nil
}

// revHeap is a max-heap of messages by rev: its root is the worst of the
// candidates byRev keeps.
type revHeap []channels.Message

func (h revHeap) Len() int           { return len(h) }
func (h revHeap) Less(i, j int) bool { return h[i].Rev > h[j].Rev }
func (h revHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *revHeap) Push(x any) {
	msg, _ := x.(channels.Message) // byRev pushes only messages
	*h = append(*h, msg)
}

func (h *revHeap) Pop() any {
	old := *h
	msg := old[len(old)-1]
	*h = old[:len(old)-1]
	return msg
}

// List implements [channels.Store].
func (s *Store) List(
	_ context.Context, a channels.Anchor, typ string, state channels.State,
) ([]channels.Channel, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	out := []channels.Channel{}
	err := s.visitFace(a, typ, func(k channels.Key, m *meta) error {
		if state == "" || m.State == state {
			out = append(out, m.channel(k))
		}
		return nil
	})
	return out, err
}

// ListEntity implements [channels.Store].
func (s *Store) ListEntity(_ context.Context, id string) ([]channels.Channel, error) {
	if err := (channels.Anchor{ID: id}).Validate(); err != nil {
		return nil, err
	}
	out := []channels.Channel{}
	err := s.visitEntity(id, "", func(k channels.Key, m *meta) error {
		out = append(out, m.channel(k))
		return nil
	})
	return out, err
}

// ListByLifecycle implements [channels.Store]. It walks every entity
// directory but reads only the metadata of channels of typ.
func (s *Store) ListByLifecycle(
	_ context.Context, typ string, state channels.State, before time.Time,
) ([]channels.Channel, error) {
	if !channels.ValidName(typ) {
		return nil, fmt.Errorf("%w: channel type %q", channels.ErrInvalidKey, typ)
	}
	if !state.Valid() {
		return nil, fmt.Errorf("%w: state %q", channels.ErrInvalidMessage, state)
	}
	before = channels.NormTime(before)
	out := []channels.Channel{}
	err := s.visitAll(typ, func(k channels.Key, m *meta) error {
		c := m.channel(k)
		if c.State == state && c.StateSince().Before(before) {
			out = append(out, c)
		}
		return nil
	})
	channels.SortChannels(out)
	return out, err
}

// ListSessionsDue implements [channels.Store]. It walks the tree once for
// all of types (see visitTypes) and takes no lock: a meta.yaml is replaced
// whole, so a read sees one version or the other, and the caller re-derives
// every answer under the lock before it acts on it.
func (s *Store) ListSessionsDue(
	_ context.Context, types []string, until time.Time, limit int,
) ([]channels.Channel, error) {
	if err := channels.CheckDueQuery(types, limit); err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(types))
	for _, t := range types {
		set[t] = true
	}
	until = channels.NormTime(until)
	out := []channels.Channel{}
	err := s.visitTypes(set, true, func(k channels.Key, m *meta) error {
		if m.State != channels.StateOpen || m.SessionNextDueAt.IsZero() {
			return nil
		}
		c := m.channel(k)
		if !c.Session.NextDueAt.After(until) {
			out = append(out, c)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	channels.SortDue(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ListWaiting implements [channels.Store]. It reads the channels of each
// listed anchor's face directory, and for the creator branch walks the
// tree once for the creator types (see visitTypes). Like ListSessionsDue
// it takes no lock and skips a damaged meta.yaml with a warning.
func (s *Store) ListWaiting(_ context.Context, q channels.WaitingQuery) ([]channels.Channel, error) {
	if err := channels.CheckWaitingQuery(q); err != nil {
		return nil, err
	}
	types := make(map[string]bool, len(q.States))
	for t := range q.States {
		types[t] = true
	}
	anchors := make(map[channels.Anchor]bool, len(q.Anchors))
	for _, a := range q.Anchors {
		anchors[a] = true
	}
	found := map[channels.Key]channels.Channel{}
	ceiling := q.RowCeiling()
	keep := func(k channels.Key, m *meta) error {
		c := m.channel(k)
		if !q.Matches(c, anchors) {
			return nil
		}
		if _, dup := found[k]; !dup && len(found) >= ceiling {
			return channels.TooManyWaiting(q)
		}
		found[k] = c
		return nil
	}
	for a := range anchors {
		if err := s.visitTypesAt(faceDir(a), a, types, true, keep); err != nil {
			return nil, err
		}
	}
	if q.CreatorBranch() {
		creator := map[string]bool{}
		for _, t := range q.CreatorTypes {
			creator[t] = types[t]
		}
		if err := s.visitTypes(creator, true, keep); err != nil {
			return nil, err
		}
	}
	out := make([]channels.Channel, 0, len(found))
	for _, c := range found {
		out = append(out, c)
	}
	channels.SortChannels(out)
	return out, nil
}

// SetSession implements [channels.Store]. It follows Append's order under
// the lock: meta.yaml, then the notice, then the audit record (see the
// package doc).
func (s *Store) SetSession(
	ctx context.Context, k channels.Key, at time.Time, step channels.SessionStep,
) (channels.Channel, error) {
	if err := k.Validate(); err != nil {
		return channels.Channel{}, err
	}
	if err := channels.PrepareTimerStep(step); err != nil {
		return channels.Channel{}, err
	}
	at = channels.NormTime(at)
	unlock, err := s.lock(ctx)
	if err != nil {
		return channels.Channel{}, err
	}
	defer unlock()
	m, err := s.readMeta(k)
	if err != nil {
		return channels.Channel{}, err
	}
	last, err := s.logEnd(k, m)
	if err != nil {
		return channels.Channel{}, err
	}
	dropDangling(m, last)
	committed := m.channel(k)
	committed.LastSeq = last
	w, err := applySessionStep(m, s.idx, &step, committed, at)
	if err != nil {
		return channels.Channel{}, err
	}
	written, move, moved, reminded := w.written, w.move, w.moved, w.reminded
	var rec []segRecord
	if moved || reminded {
		rec = []segRecord{{idx: s.idx(written.Seq), rec: encodeLine(line{K: "m", Message: written})}}
	}
	if moved {
		stashJob(m, written, rec, []int64{written.Seq})
	}
	if err := m.schedule(k, step.Due); err != nil {
		return channels.Channel{}, err
	}
	if err := s.writeMeta(k, m); err != nil {
		return channels.Channel{}, err
	}
	if moved || reminded {
		if err := s.appendRecords(k, rec); err != nil {
			return channels.Channel{}, err
		}
	}
	if moved {
		audit := channels.TimerAudit(k, &step, written, move, m.SessionGen)
		if err := s.appendAudit(k.ID, audit); err != nil {
			return channels.Channel{}, err
		}
	}
	out := m.channel(k)
	out.LastSeq = m.LastSeq
	if !moved && !reminded {
		out.LastSeq = last
	}
	if reminded {
		channels.FillReminderOutcome(&step, out)
	} else {
		channels.FillSessionOutcome(&step, out, move, moved)
	}
	return out, nil
}

// repairJob replays the stashed records of k's last job move that the log
// lacks, drops the pending refs the log does not hold, and clears the stash
// in meta.yaml. It returns the meta.yaml it wrote. It is a function, not a
// method, to keep Store under its method load line.
func repairJob(ctx context.Context, s *Store, k channels.Key) (*meta, error) {
	unlock, err := s.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	m, err := s.readMeta(k)
	if err != nil {
		return nil, err
	}
	if len(m.JobRecords) == 0 {
		return m, nil
	}
	last, err := s.logEnd(k, m)
	if err != nil {
		return nil, err
	}
	dropDangling(m, last)
	if err := s.writeMeta(k, m); err != nil {
		return nil, err
	}
	return m, nil
}

// sessionWrite is what [applySessionStep] decided and applied to m.
type sessionWrite struct {
	written  channels.Message
	move     channels.SessionMove
	moved    bool
	reminded bool
}

// applySessionStep decides step against committed, whose LastSeq is set,
// and applies the move or the reminder to m: the session, the rev and the
// last sequence number. idx maps a sequence number to its segment. written
// is the notice or reminder message to append. A timer move wins over a
// reminder.
func applySessionStep(
	m *meta, idx func(int64) int64, step *channels.SessionStep, committed channels.Channel, at time.Time,
) (sessionWrite, error) {
	var w sessionWrite
	var err error
	w.move, w.moved, err = channels.DecideTimer(step, committed)
	if err != nil {
		return w, err
	}
	seq := committed.LastSeq + 1
	after := committed
	if w.moved {
		channels.ApplySession(&after, w.move, at)
		m.setSession(after.Session)
		w.written, _, err = channels.TimerNotice(step, seq, at, w.move, after.Session.Generation, m.bump(idx(seq)))
		if err != nil {
			return w, err
		}
		if w.written.Action != nil {
			setPending(m, seq, *w.written.Action)
		}
		m.advance(seq, at)
		return w, nil
	}
	post, reminded, err := channels.DecideReminder(step, committed, at)
	if err != nil || !reminded {
		return w, err
	}
	channels.ApplyReminder(&after, post)
	if w.written, _, err = channels.ReminderMessage(step, post, seq, at, m.Rev+1); err != nil {
		return w, err
	}
	w.reminded = true
	m.setSession(after.Session)
	m.bump(idx(seq))
	m.advance(seq, at)
	return w, nil
}

// StalePending implements [channels.Store]. A channel whose meta.yaml
// still holds the records of a job move is repaired first (see repairJob),
// so a job mark a crash kept out of the log is listed.
func (s *Store) StalePending(ctx context.Context, before time.Time) ([]channels.PendingAction, error) {
	before = channels.NormTime(before)
	out := []channels.PendingAction{}
	err := s.visitAll("", func(k channels.Key, m *meta) error {
		if len(m.JobRecords) > 0 {
			var err error
			if m, err = repairJob(ctx, s, k); err != nil {
				return err
			}
		}
		for seq, p := range m.Pending {
			if p.At.Before(before) {
				out = append(out, channels.PendingAction{Key: k, Seq: seq, Name: p.Name, PendingAt: p.At.UTC()})
			}
		}
		return nil
	})
	channels.SortPending(out)
	return out, err
}

// Audit implements [channels.Store].
func (s *Store) Audit(_ context.Context, id string) ([]channels.AuditRecord, error) {
	if err := (channels.Anchor{ID: id}).Validate(); err != nil {
		return nil, err
	}
	data, err := s.readFile(auditKey(id))
	if err != nil {
		return nil, fmt.Errorf("filechannels: read audit of %s: %w", id, err)
	}
	out := []channels.AuditRecord{}
	for raw := range bytes.SplitSeq(complete(data), []byte{'\n'}) {
		if len(raw) == 0 {
			continue
		}
		var r channels.AuditRecord
		if err := json.Unmarshal(raw, &r); err != nil {
			continue
		}
		r.At = r.At.UTC()
		out = append(out, r)
	}
	return out, nil
}

// Keys implements [channels.Store].
func (s *Store) Keys(_ context.Context) ([]channels.Key, error) {
	out := []channels.Key{}
	err := s.visitAll("", func(k channels.Key, _ *meta) error {
		out = append(out, k)
		return nil
	})
	channels.SortKeys(out)
	return out, err
}

// TypeRecords implements [channels.Store].
func (s *Store) TypeRecords(ctx context.Context) (map[string]channels.TypeRecord, error) {
	unlock, err := s.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	return readTypes(&s.disk)
}

// RecordTypes implements [channels.Store].
func (s *Store) RecordTypes(ctx context.Context, records map[string]channels.TypeRecord) error {
	records, err := channels.PrepareTypeRecords(records)
	if err != nil {
		return err
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	cur, err := readTypes(&s.disk)
	if err != nil {
		return err
	}
	changed := false
	for _, typ := range slices.Sorted(maps.Keys(records)) {
		given := records[typ]
		held, ok := cur[typ]
		added, fill, mergeErr := channels.MergeTypeRecord(typ, held, ok, given)
		if mergeErr != nil {
			return mergeErr
		}
		if ok && !fill && len(added) == 0 {
			continue
		}
		if !ok || fill {
			held.Anchor, held.Cardinality = given.Anchor, given.Cardinality
		}
		held.Refs = slices.Sorted(slices.Values(slices.Concat(held.Refs, added)))
		cur[typ], changed = held, true
	}
	if !changed {
		return nil
	}
	data, err := yaml.Marshal(cur)
	if err != nil { // coverage-ignore: a map of plain structs always encodes
		return fmt.Errorf("filechannels: encode type records: %w", err)
	}
	err = s.rfs.WriteFile(typesFile, data, filePerm)
	if err != nil {
		return fmt.Errorf("filechannels: write type records: %w", err)
	}
	return nil
}

// readTypes reads the type records. A missing file is an empty record; a
// damaged one is an error, because guessing would let a changed anchor or
// cardinality through.
//
// A type found only in legacyTypesFile reads with
// [channels.CardinalityUnknown]. RecordTypes writes every record it read to
// typesFile, so after that write typesFile holds each legacy type too and
// takes precedence over the legacy file.
func readTypes(d *disk) (map[string]channels.TypeRecord, error) {
	legacy := map[string]channels.AnchorMode{}
	data, err := d.readFile(legacyTypesFile)
	if err != nil {
		return nil, fmt.Errorf("filechannels: read legacy anchor modes: %w", err)
	}
	if err = yaml.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("filechannels: read legacy anchor modes: %w", err)
	}
	cur := map[string]channels.TypeRecord{}
	data, err = d.readFile(typesFile)
	if err != nil {
		return nil, fmt.Errorf("filechannels: read type records: %w", err)
	}
	if err = yaml.Unmarshal(data, &cur); err != nil {
		return nil, fmt.Errorf("filechannels: read type records: %w", err)
	}
	out := make(map[string]channels.TypeRecord, len(legacy)+len(cur))
	for typ, mode := range legacy {
		out[typ] = channels.TypeRecord{Anchor: mode, Cardinality: channels.CardinalityUnknown}
	}
	maps.Copy(out, cur)
	return out, nil
}
