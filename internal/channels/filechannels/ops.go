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
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Append implements [channels.Store].
func (s *Store) Append(ctx context.Context, k channels.Key, nm channels.NewMessage) (channels.Message, error) {
	if err := k.Validate(); err != nil {
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
	last, err := s.lastSeq(k)
	if err != nil {
		return channels.Message{}, err
	}
	parent, parentSeg, err := s.replyParent(k, nm.ReplyTo)
	if err != nil {
		return channels.Message{}, err
	}

	seq := last + 1
	idx := s.idx(seq)
	msg := channels.Message{
		Seq: seq, Rev: m.bump(idx), Type: nm.Type, Author: nm.Author, At: nm.At, Props: nm.Props,
		ReplyTo: nm.ReplyTo, SchemaHash: nm.SchemaHash, TriggeredBy: nm.TriggeredBy,
	}
	var patch []byte
	if nm.ReplyTo > 0 {
		count := parent.ReplyCount + 1
		patch = encodeLine(line{K: "p", Message: channels.Message{Seq: parent.Seq, Rev: m.bump(s.idx(parent.Seq))},
			ReplyCount: &count})
	}
	m.LastSeq = seq
	if created {
		// The create record goes before meta.yaml: a crash in between
		// leaves a create record for a channel the next append creates
		// again, so the trail may repeat a create but never misses one.
		audit := channels.KeyAudit(k, channels.AuditCreate, nm.At)
		audit.By = nm.Author
		err = s.appendAudit(k.ID, audit)
		if err != nil {
			return channels.Message{}, err
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
	rec := encodeLine(line{K: "m", Message: msg})
	if patch != nil && s.idx(parent.Seq) == idx {
		rec, patch = append(rec, patch...), nil
	}
	err = s.appendLog(k, idx, cur, rec)
	if err == nil && patch != nil {
		err = s.appendLog(k, s.idx(parent.Seq), parentSeg, patch)
	}
	if err != nil {
		return channels.Message{}, err
	}
	return msg, nil
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
// reserve its rev, writes meta.yaml and then rewrites the segment.
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
	if err := channels.CheckRelease(expect, next); err != nil {
		return channels.Message{}, err
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
			if m.Pending == nil {
				m.Pending = map[int64]pendingRef{}
			}
			m.Pending[seq] = pendingRef{Name: name, At: next.PendingAt}
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
	// The create record goes first, for the reason Append writes it first.
	rec := channels.KeyAudit(k, channels.AuditCreate, nc.At)
	rec.By = nc.By
	if err := s.appendAudit(k.ID, rec); err != nil {
		return channels.Channel{}, err
	}
	if err := s.writeMeta(k, m); err != nil {
		return channels.Channel{}, err
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
	m.State, m.ClosedAt, m.ArchivedAt = c.State, c.ClosedAt, c.ArchivedAt
	m.Rev++
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
func (s *Store) Delete(ctx context.Context, k channels.Key, by channels.Author, at time.Time) error {
	if err := k.Validate(); err != nil {
		return err
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	if _, err := s.readMeta(k); err != nil {
		return err
	}
	rec := channels.KeyAudit(k, channels.AuditDelete, channels.NormTime(at))
	rec.By = by
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
	ctx context.Context, id string, face entity.Face, at time.Time, f channels.Filing,
) (channels.TombstoneHandle, error) {
	a := channels.Anchor{ID: id, Face: face}
	if err := a.Validate(); err != nil {
		return channels.TombstoneHandle{}, err
	}
	f, err := channels.PrepareFiling(f)
	if err != nil {
		return channels.TombstoneHandle{}, err
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return channels.TombstoneHandle{}, err
	}
	defer unlock()
	h := channels.NewTombstoneHandle(a, channels.TombstoneStamp(at))
	rec := channels.AuditRecord{At: channels.NormTime(at), Op: channels.AuditTombstone, Face: face}

	if face != "" {
		ok, existsErr := s.exists(faceDir(a))
		if existsErr != nil || !ok {
			return channels.TombstoneHandle{}, existsErr
		}
		if err := writeFiling(&s.disk, h, f); err != nil {
			return channels.TombstoneHandle{}, err
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
		if err := writeFiling(&s.disk, h, f); err != nil {
			return channels.TombstoneHandle{}, err
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
func (s *Store) Untombstone(ctx context.Context, h channels.TombstoneHandle, at time.Time) error {
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
	occupied, err := s.occupied(a)
	if err != nil {
		return err
	}
	if occupied {
		return &channels.OccupiedError{Anchor: a}
	}
	if hasDir {
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

// occupied reports whether a live channel sits on a: any channel of the id
// for an empty face, any channel of the face otherwise.
func (s *Store) occupied(a channels.Anchor) (bool, error) {
	found := false
	visit := func(channels.Key, *meta) error {
		found = true
		return nil
	}
	var err error
	if a.Face == "" {
		err = s.visitEntity(a.ID, "", visit)
	} else {
		err = s.visitFace(a, "", visit)
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
	case p.AfterRev > 0:
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

// StalePending implements [channels.Store].
func (s *Store) StalePending(_ context.Context, before time.Time) ([]channels.PendingAction, error) {
	before = channels.NormTime(before)
	out := []channels.PendingAction{}
	err := s.visitAll("", func(k channels.Key, m *meta) error {
		seqs := make([]int64, 0, len(m.Pending))
		for seq := range m.Pending {
			seqs = append(seqs, seq)
		}
		slices.Sort(seqs)
		for _, seq := range seqs {
			p := m.Pending[seq]
			if p.At.Before(before) {
				out = append(out, channels.PendingAction{Key: k, Seq: seq, Name: p.Name, PendingAt: p.At.UTC()})
			}
		}
		return nil
	})
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
	return s.readTypes()
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
	cur, err := s.readTypes()
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
func (s *Store) readTypes() (map[string]channels.TypeRecord, error) {
	legacy := map[string]channels.AnchorMode{}
	data, err := s.readFile(legacyTypesFile)
	if err != nil {
		return nil, fmt.Errorf("filechannels: read legacy anchor modes: %w", err)
	}
	if err = yaml.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("filechannels: read legacy anchor modes: %w", err)
	}
	cur := map[string]channels.TypeRecord{}
	data, err = s.readFile(typesFile)
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
