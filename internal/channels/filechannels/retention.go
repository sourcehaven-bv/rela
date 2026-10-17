package filechannels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
// trails, each followed by its purge record, in one file:
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
	Pending    bool                     `yaml:"pending,omitempty"`
}

// filingKey is where the batch h keeps its filing.
func filingKey(h channels.TombstoneHandle) string { return filingsDir + "/" + h.Stamp() + ".yaml" }

// purgedAuditKey is where the purged batches of id keep their trails.
func purgedAuditKey(id string) string { return auditDir + "/~purged/" + enc(id) + ".jsonl" }

func writeFiling(d *disk, h channels.TombstoneHandle, f channels.Filing) error {
	a := h.Anchor()
	data, err := yaml.Marshal(filingFile{
		ID: a.ID, Face: a.Face, Policy: f.Policy, PurgeAfter: f.PurgeAfter, Pending: f.Pending,
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
		return channels.Filing{}, false, fmt.Errorf("filechannels: filing of %s is damaged: %w", h, err)
	}
	if ff.ID != h.Anchor().ID || ff.Face != h.Anchor().Face {
		return channels.Filing{}, false, nil
	}
	return channels.Filing{Policy: ff.Policy, PurgeAfter: utc(ff.PurgeAfter), Pending: ff.Pending}, true, nil
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
	slices.SortFunc(out, func(x, y channels.TombstoneHandle) int {
		if c := strings.Compare(x.Stamp(), y.Stamp()); c != 0 {
			return c
		}
		return strings.Compare(x.Anchor().ID, y.Anchor().ID)
	})
	return out, nil
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
	f, ok, err := readFiling(&s.disk, h)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: tombstone %s", channels.ErrNotFound, h)
	}
	if !f.Pending {
		return nil
	}
	f.Pending = false
	return writeFiling(&s.disk, h, f)
}

// Tombstones implements [channels.Store]. Like the other list methods it
// reads without the lock; a batch restored or purged during the walk is
// left out or listed with what was read.
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
		cs, err := batchChannels(&s.disk, h, "")
		if err != nil {
			return nil, err
		}
		if len(cs) == 0 {
			continue
		}
		at, err := channels.StampTime(h.Stamp())
		if err != nil {
			return nil, err
		}
		f, _, err := readFiling(&s.disk, h)
		if err != nil {
			return nil, err
		}
		out = append(out, channels.TombstoneBatch{Handle: h, At: at, Filing: f, Channels: cs})
	}
	return out, nil
}

// Purge implements [channels.Store]. The batch directory moves to the
// trash in one rename, which takes it out of reach; the filed trail then
// moves to purgedAuditKey with the purge record appended, and the trash is
// emptied last.
func (s *Store) Purge(ctx context.Context, h channels.TombstoneHandle, by channels.Author, at time.Time) error {
	if h.IsZero() {
		return nil
	}
	if err := h.Validate(); err != nil {
		return err
	}
	if err := channels.CheckPurge(by, at); err != nil {
		return err
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	f, _, err := readFiling(&s.disk, h)
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
		return fmt.Errorf("%w: tombstone %s", channels.ErrNotFound, h)
	}
	if hasDir {
		if err = s.move(detachedKey(h), trashDir+"/"+channels.NewName()); err != nil {
			return fmt.Errorf("filechannels: purge %s: %w", h, err)
		}
	}
	rec, err := json.Marshal(channels.PurgeAudit(h, by, at))
	if err != nil { // coverage-ignore: invariant: plain data
		return err
	}
	if err := s.rfs.MkdirAll(auditDir+"/~purged", 0o755); err != nil {
		return fmt.Errorf("filechannels: purge audit of %s: %w", h, err)
	}
	if err := s.appendTo(purgedAuditKey(h.Anchor().ID), append(complete(filed), append(rec, '\n')...)); err != nil {
		return fmt.Errorf("filechannels: purge audit of %s: %w", h, err)
	}
	if filed != nil {
		if err := s.rfs.Remove(detachedAuditKey(h)); err != nil && !notExist(err) {
			return fmt.Errorf("filechannels: purge audit of %s: %w", h, err)
		}
	}
	if err := removeFiling(&s.disk, h); err != nil {
		return err
	}
	if err := s.removeAll(trashDir); err != nil {
		return fmt.Errorf("filechannels: purge %s: %w", h, err)
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
