package filechannels

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// The retention half of the store: what a tombstoned batch recorded, and
// the methods that list, confirm and purge batches. Helpers are free
// functions so the Store's method count stays the interface's.
//
// A batch's filing is a file named by its stamp, which is unique to one
// Tombstone call and short, so the name stays within a filesystem's limit
// however long the anchor is:
//
//	<root>/~filings/<stamp>.yaml
//
// The file names the anchor too; a filing whose anchor is not the
// handle's reads as absent. The purged batches of an id keep their filed
// trails, each followed by its purge records, in one file:
//
//	<root>/_audit/~purged/<b32(id)>.jsonl
//
// where neither Untombstone nor a second Purge looks.

const filingsDir = "~filings"

// filingFile is the on-disk form of a [channels.Filing].
type filingFile struct {
	ID         string                   `yaml:"id"`
	Face       entity.Face              `yaml:"face,omitempty"`
	Policy     channels.TombstonePolicy `yaml:"policy,omitempty"`
	PurgeAfter time.Time                `yaml:"purge_after,omitempty"`
	TypeAfter  map[string]time.Time     `yaml:"type_after,omitempty"`
	Pending    bool                     `yaml:"pending,omitempty"`
}

func (ff filingFile) filing() channels.Filing {
	var early map[string]time.Time
	if len(ff.TypeAfter) > 0 {
		early = make(map[string]time.Time, len(ff.TypeAfter))
		for typ, t := range ff.TypeAfter {
			early[typ] = utc(t)
		}
	}
	return channels.Filing{
		Policy: ff.Policy, PurgeAfter: utc(ff.PurgeAfter), TypeAfter: early, Pending: ff.Pending,
	}
}

// filingKey is where the batch h keeps its filing.
func filingKey(h channels.TombstoneHandle) string { return filingsDir + "/" + h.Stamp() + ".yaml" }

// purgedAuditKey is where the purged batches of id keep their trails.
func purgedAuditKey(id string) string { return auditDir + "/~purged/" + enc(id) + ".jsonl" }

func writeFiling(d *disk, h channels.TombstoneHandle, f channels.Filing) error {
	a := h.Anchor()
	data, err := yaml.Marshal(filingFile{
		ID: a.ID, Face: a.Face, Policy: f.Policy, PurgeAfter: f.PurgeAfter, TypeAfter: f.TypeAfter,
		Pending: f.Pending,
	})
	if err != nil { // coverage-ignore: invariant: plain data
		return err
	}
	if err := d.rfs.MkdirAll(filingsDir, 0o755); err != nil {
		return fmt.Errorf("filechannels: file %s: %w", h, err)
	}
	if err := d.rfs.WriteFile(filingKey(h), data, filePerm); err != nil {
		return fmt.Errorf("filechannels: file %s: %w", h, err)
	}
	return nil
}

func removeFiling(d *disk, h channels.TombstoneHandle) error {
	if err := d.rfs.Remove(filingKey(h)); err != nil && !notExist(err) {
		return fmt.Errorf("filechannels: drop filing of %s: %w", h, err)
	}
	return nil
}

// errDamagedFiling marks a filing file that does not parse.
var errDamagedFiling = errors.New("filechannels: damaged filing")

// readFiling returns the filing of the batch h, and whether it has one.
func readFiling(d *disk, h channels.TombstoneHandle) (channels.Filing, bool, error) {
	data, err := d.readFile(filingKey(h))
	if err != nil {
		return channels.Filing{}, false, fmt.Errorf("filechannels: read filing of %s: %w", h, err)
	}
	if data == nil {
		return channels.Filing{}, false, nil
	}
	var ff filingFile
	if err := yaml.Unmarshal(data, &ff); err != nil {
		return channels.Filing{}, false, fmt.Errorf("%w of %s: %w", errDamagedFiling, h, err)
	}
	if ff.ID != h.Anchor().ID || ff.Face != h.Anchor().Face {
		return channels.Filing{}, false, nil
	}
	return ff.filing(), true, nil
}

// parseDetached reads the handle back from a batch directory name, as
// detachedKey writes it: <b32(id)>-<stamp> or <b32(id)>-<b32(face)>-<stamp>.
// Base32 has no hyphen and a stamp has exactly one.
func parseDetached(name string) (channels.TombstoneHandle, bool) {
	const wholeParts, faceParts = 3, 4
	parts := strings.Split(name, "-")
	if len(parts) != wholeParts && len(parts) != faceParts {
		return channels.TombstoneHandle{}, false
	}
	id, ok := dec(parts[0])
	if !ok {
		return channels.TombstoneHandle{}, false
	}
	a := channels.Anchor{ID: id}
	if len(parts) == faceParts {
		face, ok := dec(parts[1])
		if !ok || face == "" {
			return channels.TombstoneHandle{}, false
		}
		a.Face = entity.Face(face)
	}
	h := channels.NewTombstoneHandle(a, parts[len(parts)-2]+"-"+parts[len(parts)-1])
	if h.Validate() != nil || detachedKey(h) != detachedDir+"/"+name {
		return channels.TombstoneHandle{}, false
	}
	return h, true
}

// batchChannels returns the channels of the batch h, read from its
// directory. typ narrows to one type.
func batchChannels(d *disk, h channels.TombstoneHandle, typ string) ([]channels.Channel, error) {
	out := []channels.Channel{}
	collect := func(k channels.Key, m *meta) error {
		out = append(out, m.channel(k))
		return nil
	}
	a := h.Anchor()
	var err error
	if a.Face != "" {
		err = visitFaceAt(d, detachedKey(h), a, typ, collect)
	} else {
		err = visitEntityAt(d, detachedKey(h), a.ID, typ, collect)
	}
	return out, err
}

// batchChannelDir is where the channel k sits inside the batch h.
func batchChannelDir(h channels.TombstoneHandle, k channels.Key) string {
	if h.Anchor().Face != "" {
		return detachedKey(h) + "/" + k.Type + "/" + nameSeg(k.Name)
	}
	return detachedKey(h) + "/" + faceSeg(k.Face) + "/" + k.Type + "/" + nameSeg(k.Name)
}

// batches returns the handle of every batch directory, by stamp and id.
func batches(d *disk) ([]channels.TombstoneHandle, error) {
	names, err := d.readDir(detachedDir)
	if err != nil {
		return nil, fmt.Errorf("filechannels: list tombstones: %w", err)
	}
	out := make([]channels.TombstoneHandle, 0, len(names))
	for _, n := range names {
		if h, ok := parseDetached(n); ok {
			out = append(out, h)
		}
	}
	sortHandles(out)
	return out, nil
}

func sortHandles(hs []channels.TombstoneHandle) {
	slices.SortFunc(hs, func(x, y channels.TombstoneHandle) int {
		if c := strings.Compare(x.Stamp(), y.Stamp()); c != 0 {
			return c
		}
		return strings.Compare(x.Anchor().ID, y.Anchor().ID)
	})
}

// confirmFiling makes the filing of h final, when it has one.
func confirmFiling(d *disk, h channels.TombstoneHandle) (bool, error) {
	f, ok, err := readFiling(d, h)
	if err != nil || !ok {
		return ok, err
	}
	if c := f.Confirmed(); c.Pending != f.Pending || c.Policy != f.Policy {
		return true, writeFiling(d, h, c)
	}
	return true, nil
}

// ConfirmTombstone implements [channels.Store].
func (s *Store) ConfirmTombstone(ctx context.Context, h channels.TombstoneHandle) error {
	if err := h.Validate(); err != nil {
		return err
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	ok, err := confirmFiling(&s.disk, h)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: tombstone %s", channels.ErrNotFound, h)
	}
	return nil
}

// ConfirmTombstones implements [channels.Store]. It reads the names of
// the batch directories, which carry the anchor, and the filings of the
// anchor's batches only.
func (s *Store) ConfirmTombstones(ctx context.Context, a channels.Anchor) error {
	if err := a.Validate(); err != nil {
		return err
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	hs, err := batches(&s.disk)
	if err != nil {
		return err
	}
	var errs []error
	for _, h := range hs {
		ha := h.Anchor()
		if ha.ID != a.ID || (a.Face != "" && ha.Face != a.Face) {
			continue
		}
		if _, err := confirmFiling(&s.disk, h); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Tombstones implements [channels.Store]. Like the other list methods it
// reads without the lock; a batch restored or purged during the walk is
// left out or listed with what was read. A batch whose filing is damaged
// is left out and logged, so one bad file does not stop retention for
// every other batch.
func (s *Store) Tombstones(_ context.Context, before time.Time) ([]channels.TombstoneBatch, error) {
	hs, err := batches(&s.disk)
	if err != nil {
		return nil, err
	}
	bound := channels.StampBefore(before)
	out := []channels.TombstoneBatch{}
	for _, h := range hs {
		if h.Stamp() >= bound {
			continue
		}
		f, _, err := readFiling(&s.disk, h)
		if errors.Is(err, errDamagedFiling) {
			slog.Warn("filechannels: skipping a tombstone with a damaged filing", "tombstone", h.String(), "error", err)
			continue
		}
		if err != nil {
			return nil, err
		}
		b, ok, err := listedBatch(&s.disk, h, f)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, b)
		}
	}
	return out, nil
}

// listedBatch reads the channels of h for a listing, and false when it
// holds none.
func listedBatch(d *disk, h channels.TombstoneHandle, f channels.Filing) (channels.TombstoneBatch, bool, error) {
	cs, err := batchChannels(d, h, "")
	if err != nil || len(cs) == 0 {
		return channels.TombstoneBatch{}, false, err
	}
	at, err := channels.StampTime(h.Stamp())
	if err != nil {
		return channels.TombstoneBatch{}, false, err
	}
	return channels.TombstoneBatch{Handle: h, At: at, Filing: f, Channels: cs}, true, nil
}

// DueTombstones implements [channels.Store]. It reads the filing files,
// which are small and one per batch, and the channels of the due batches
// only. A filing whose batch is gone, left by a crash between writing the
// filing and moving the batch, is removed under the lock. A damaged filing
// is skipped and logged.
func (s *Store) DueTombstones(ctx context.Context, at time.Time) ([]channels.TombstoneBatch, error) {
	entries, err := s.rfs.ReadDir(filingsDir)
	if err != nil && !notExist(err) {
		return nil, fmt.Errorf("filechannels: list filings: %w", err)
	}
	at = channels.NormTime(at)
	var due, leaked []channels.TombstoneHandle
	filings := map[channels.TombstoneHandle]channels.Filing{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		h, f, ok := s.filingAt(e.Name())
		if !ok {
			continue
		}
		hasDir, err := s.exists(detachedKey(h))
		if err != nil {
			return nil, err
		}
		if !hasDir {
			leaked = append(leaked, h)
			continue
		}
		if d := f.DueAt(); !f.Pending && !d.IsZero() && !d.After(at) {
			due = append(due, h)
			filings[h] = f
		}
	}
	sortHandles(due)
	out := []channels.TombstoneBatch{}
	for _, h := range due {
		b, ok, err := listedBatch(&s.disk, h, filings[h])
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, b)
		}
	}
	if len(leaked) > 0 {
		if err := s.dropLeakedFilings(ctx, leaked); err != nil {
			return out, err
		}
	}
	return out, nil
}

// filingAt reads the filing file named name, and the handle it belongs to.
func (s *Store) filingAt(name string) (channels.TombstoneHandle, channels.Filing, bool) {
	stamp, ok := strings.CutSuffix(name, ".yaml")
	if !ok {
		return channels.TombstoneHandle{}, channels.Filing{}, false
	}
	data, err := s.readFile(filingsDir + "/" + name)
	if err != nil || data == nil {
		return channels.TombstoneHandle{}, channels.Filing{}, false
	}
	var ff filingFile
	if err := yaml.Unmarshal(data, &ff); err != nil {
		slog.Warn("filechannels: skipping a damaged filing", "file", name, "error", err)
		return channels.TombstoneHandle{}, channels.Filing{}, false
	}
	h := channels.NewTombstoneHandle(channels.Anchor{ID: ff.ID, Face: ff.Face}, stamp)
	if h.Validate() != nil {
		slog.Warn("filechannels: skipping a damaged filing", "file", name)
		return channels.TombstoneHandle{}, channels.Filing{}, false
	}
	return h, ff.filing(), true
}

// dropLeakedFilings removes the filings of hs whose batch is still gone
// under the lock. Tombstone writes a filing and moves its batch under the
// lock, so a filing without a batch seen under the lock belongs to no
// batch: a crash left it, or a purge or restore that stopped part way.
// A whole-entity batch that kept only its filed trail is left alone.
func (s *Store) dropLeakedFilings(ctx context.Context, hs []channels.TombstoneHandle) error {
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	var errs []error
	for _, h := range hs {
		gone, err := s.batchGone(h)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if gone {
			errs = append(errs, removeFiling(&s.disk, h))
		}
	}
	return errors.Join(errs...)
}

// batchGone reports whether neither the directory nor the filed trail of
// h exists.
func (s *Store) batchGone(h channels.TombstoneHandle) (bool, error) {
	if ok, err := s.exists(detachedKey(h)); err != nil || ok {
		return false, err
	}
	if h.Anchor().Face != "" {
		return true, nil
	}
	ok, err := s.exists(detachedAuditKey(h))
	return !ok, err
}

// Purge implements [channels.Store]. It runs in an order that a retry
// after a crash completes without loss or duplication:
//
//  1. The purge records, with the filed trail of a whole purge, are
//     appended to purgedAuditKey in one write, unless a retry finds them
//     there already.
//  2. The erased channels move to the trash, each in one rename: the
//     whole batch directory, or one channel directory per erased channel.
//  3. A whole purge removes the filed trail and the filing; a partial one
//     drops the erased types from the filing.
//  4. The trash is emptied.
//
// A crash after step 1 leaves records of a purge that the next Purge of
// the batch completes. A filing left without its batch is removed by
// the next Purge or [Store.DueTombstones] that finds it.
func (s *Store) Purge(
	ctx context.Context, h channels.TombstoneHandle, types []string, by channels.Author, at time.Time,
) error {
	if h.IsZero() {
		return nil
	}
	if err := h.Validate(); err != nil {
		return err
	}
	if err := channels.CheckPurge(types, by, at); err != nil {
		return err
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	f, hasFiling, err := readFiling(&s.disk, h)
	if err != nil {
		return err
	}
	if f.Pending {
		return fmt.Errorf("%w: tombstone %s waits for its delete to commit", channels.ErrConflict, h)
	}
	hasDir, err := s.exists(detachedKey(h))
	if err != nil {
		return fmt.Errorf("filechannels: purge %s: %w", h, err)
	}
	var filed []byte
	if h.Anchor().Face == "" {
		if filed, err = s.readFile(detachedAuditKey(h)); err != nil {
			return fmt.Errorf("filechannels: purge %s: %w", h, err)
		}
	}
	if !hasDir && filed == nil {
		if hasFiling {
			if err := removeFiling(&s.disk, h); err != nil {
				return err
			}
		}
		return fmt.Errorf("%w: tombstone %s", channels.ErrNotFound, h)
	}
	var batch []channels.Channel
	if hasDir {
		if batch, err = batchChannels(&s.disk, h, ""); err != nil {
			return err
		}
	}
	erased := channels.PurgeTypes(batch, types)
	whole := len(erased) == len(batch)
	if err := s.appendPurgeRecords(h, erased, whole, filed, by, at); err != nil {
		return err
	}
	if err := s.trashErased(h, erased, whole, hasDir); err != nil {
		return err
	}
	if whole {
		if filed != nil {
			if err := s.rfs.Remove(detachedAuditKey(h)); err != nil && !notExist(err) {
				return fmt.Errorf("filechannels: purge audit of %s: %w", h, err)
			}
		}
		if err := removeFiling(&s.disk, h); err != nil {
			return err
		}
	} else if hasFiling && len(f.TypeAfter) > 0 {
		rest := maps.Clone(f.TypeAfter)
		for _, typ := range types {
			delete(rest, typ)
		}
		f.TypeAfter = rest
		if err := writeFiling(&s.disk, h, f); err != nil {
			return err
		}
	}
	if err := s.removeAll(trashDir); err != nil {
		return fmt.Errorf("filechannels: purge %s: %w", h, err)
	}
	return nil
}

// appendPurgeRecords writes step 1 of Purge: the records of the erased
// channels, after the filed trail of a whole purge, in one append. A
// record already in purgedAuditKey, from a purge a crash cut short, is
// not written again, and neither is the trail then.
func (s *Store) appendPurgeRecords(
	h channels.TombstoneHandle, erased []channels.Channel, whole bool, filed []byte, by channels.Author, at time.Time,
) error {
	key := purgedAuditKey(h.Anchor().ID)
	prior, err := s.readFile(key)
	if err != nil {
		return fmt.Errorf("filechannels: purge audit of %s: %w", h, err)
	}
	seen := purgedRecords(prior, h.Stamp())
	var buf []byte
	for _, rec := range channels.PurgeAudit(h, erased, by, at) {
		if seen[purgeRecordKey(rec)] {
			continue
		}
		b, err := json.Marshal(rec)
		if err != nil { // coverage-ignore: invariant: plain data
			return err
		}
		buf = append(append(buf, b...), '\n')
	}
	if buf == nil {
		return nil
	}
	if whole {
		buf = append(complete(filed), buf...)
	}
	if err := s.rfs.MkdirAll(auditDir+"/~purged", 0o755); err != nil {
		return fmt.Errorf("filechannels: purge audit of %s: %w", h, err)
	}
	if err := s.appendTo(key, buf); err != nil {
		return fmt.Errorf("filechannels: purge audit of %s: %w", h, err)
	}
	return nil
}

// purgeRecordKey names what one purge record erased.
func purgeRecordKey(r channels.AuditRecord) string {
	return string(r.Face) + "\x00" + r.Type + "\x00" + r.Name
}

// purgedRecords returns the keys of the purge records of the batch with
// stamp found in data.
func purgedRecords(data []byte, stamp string) map[string]bool {
	out := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(nil, len(data)+1)
	for sc.Scan() {
		var r channels.AuditRecord
		if json.Unmarshal(sc.Bytes(), &r) != nil {
			continue
		}
		if r.Op == channels.AuditPurge && r.Tomb == stamp {
			out[purgeRecordKey(r)] = true
		}
	}
	return out
}

// trashErased writes step 2 of Purge.
func (s *Store) trashErased(h channels.TombstoneHandle, erased []channels.Channel, whole, hasDir bool) error {
	if whole {
		if !hasDir {
			return nil
		}
		if err := s.move(detachedKey(h), trashDir+"/"+channels.NewName()); err != nil {
			return fmt.Errorf("filechannels: purge %s: %w", h, err)
		}
		return nil
	}
	for _, c := range erased {
		if err := s.move(batchChannelDir(h, c.Key), trashDir+"/"+channels.NewName()); err != nil {
			return fmt.Errorf("filechannels: purge %s from %s: %w", c.Key, h, err)
		}
	}
	return nil
}

// DropTypeRecord implements [channels.Store]. A type found only in the
// legacy record file is dropped too: the remaining records are written to
// typesFile and the legacy file is removed, since typesFile then holds
// every type it held.
func (s *Store) DropTypeRecord(ctx context.Context, typ string) error {
	if err := channels.CheckDropType(typ); err != nil {
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
	if _, ok := cur[typ]; !ok {
		return nil
	}
	inUse, err := typeInUse(&s.disk, typ)
	if err != nil {
		return err
	}
	if inUse {
		return fmt.Errorf("%w: %s", channels.ErrTypeInUse, typ)
	}
	delete(cur, typ)
	data, err := yaml.Marshal(cur)
	if err != nil { // coverage-ignore: a map of plain structs always encodes
		return fmt.Errorf("filechannels: encode type records: %w", err)
	}
	if err := s.rfs.WriteFile(typesFile, data, filePerm); err != nil {
		return fmt.Errorf("filechannels: write type records: %w", err)
	}
	if err := s.rfs.Remove(legacyTypesFile); err != nil && !notExist(err) {
		return fmt.Errorf("filechannels: drop legacy anchor modes: %w", err)
	}
	return nil
}

// errFound stops a walk at the first channel.
var errFound = errors.New("found")

// typeInUse reports whether a channel of typ exists, live or tombstoned.
func typeInUse(d *disk, typ string) (bool, error) {
	stop := func(channels.Key, *meta) error { return errFound }
	if err := d.visitAll(typ, stop); err != nil {
		if errors.Is(err, errFound) {
			return true, nil
		}
		return false, err
	}
	hs, err := batches(d)
	if err != nil {
		return false, err
	}
	for _, h := range hs {
		cs, err := batchChannels(d, h, typ)
		if err != nil {
			return false, err
		}
		if len(cs) > 0 {
			return true, nil
		}
	}
	return false, nil
}
