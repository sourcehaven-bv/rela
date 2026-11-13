// Package filerecords is the file [records.Store] for the fs and desktop
// tiers: one JSON line per row, in one file per owner per month, under the
// project's records/ directory, so rows are diffable and travel with the
// project in git.
//
// Layout: <root>/<type>/<owner>/<YYYY-MM>.jsonl, where the owner id is
// escaped to a safe path segment. Lines in a file are in (TS, ID) order.
//
// The fs tier is one local user (see CLAUDE.md, "Design for postgres"), so
// this backend is simple rather than fast: every operation takes a process
// lock and a file lock, reads whole month files and rewrites the ones it
// changes. An append that spans several files is not atomic across a crash;
// the conformance suite pins the behavior without crashes.
package filerecords

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// Locker serializes processes sharing one records directory.
type Locker interface {
	Lock(ctx context.Context) (unlock func(), err error)
}

// Store is the file backend.
type Store struct {
	fs     storage.FS
	root   string
	locker Locker
	mu     sync.Mutex
}

var _ records.Store = (*Store)(nil)

// New returns a store over the directory root of fsys.
//
// Nil: fsys and locker are rejected.
func New(fsys storage.FS, root string, locker Locker) (*Store, error) {
	if fsys == nil || locker == nil || root == "" {
		return nil, errors.New("filerecords: New requires a filesystem, a root and a locker")
	}
	return &Store{fs: fsys, root: root, locker: locker}, nil
}

func (s *Store) lock(ctx context.Context) (func(), error) {
	s.mu.Lock()
	unlock, err := s.locker.Lock(ctx)
	if err != nil {
		s.mu.Unlock()
		return nil, err
	}
	return func() { unlock(); s.mu.Unlock() }, nil
}

// escape maps an id to a path segment: lower-case letters, digits, - and _
// stay, and every other byte becomes %XX, so no id can name . or .. or a
// separator. Upper-case letters are escaped too, so ids that differ only in
// case stay apart on a case-folding filesystem.
func escape(id string) string {
	var b strings.Builder
	for i := range len(id) {
		c := id[i]
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

func unescape(seg string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(seg); i++ {
		if seg[i] != '%' {
			b.WriteByte(seg[i])
			continue
		}
		if i+2 >= len(seg) {
			return "", fmt.Errorf("filerecords: bad segment %q", seg)
		}
		var c byte
		if _, err := fmt.Sscanf(seg[i+1:i+3], "%02X", &c); err != nil {
			return "", fmt.Errorf("filerecords: bad segment %q", seg)
		}
		b.WriteByte(c)
		i += 2
	}
	return b.String(), nil
}

func (s *Store) typeDir(typ string) string { return path.Join(s.root, escape(typ)) }

func (s *Store) ownerDir(typ, owner string) string { return path.Join(s.typeDir(typ), escape(owner)) }

func monthFile(ts time.Time) string { return ts.UTC().Format("2006-01") + ".jsonl" }

// readFile returns the rows of one month file; a missing file is empty.
func (s *Store) readFile(p string) ([]records.Record, error) {
	data, err := s.fs.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []records.Record
	for line := range bytes.SplitSeq(data, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		rec, err := records.DecodeRecord(line)
		if err != nil {
			return nil, fmt.Errorf("filerecords: %s: %w", p, err)
		}
		out = append(out, rec)
	}
	return out, nil
}

// writeFile writes rows to p in order, or removes p when rows is empty.
func (s *Store) writeFile(p string, rows []records.Record) error {
	if len(rows) == 0 {
		if err := s.fs.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	slices.SortFunc(rows, records.Compare)
	var buf bytes.Buffer
	for _, r := range rows {
		line, err := records.EncodeRecord(r)
		if err != nil {
			return err
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	if err := s.fs.MkdirAll(path.Dir(p), 0o755); err != nil {
		return err
	}
	return s.fs.WriteFile(p, buf.Bytes(), 0o644)
}

// readDir lists a directory; a missing one is empty.
func (s *Store) readDir(p string) ([]os.DirEntry, error) {
	ents, err := s.fs.ReadDir(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return ents, err
}

// ownerRows returns every row of one owner of one type.
func (s *Store) ownerRows(typ, owner string) ([]records.Record, error) {
	dir := s.ownerDir(typ, owner)
	ents, err := s.readDir(dir)
	if err != nil {
		return nil, err
	}
	var out []records.Record
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		rows, err := s.readFile(path.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if r.Owner == owner {
				out = append(out, r)
			}
		}
	}
	return out, nil
}

// owners lists the owners with a directory under typ.
func (s *Store) owners(typ string) ([]string, error) {
	ents, err := s.readDir(s.typeDir(typ))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		id, err := unescape(e.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

// typesOnDisk lists the record types with a directory.
func (s *Store) typesOnDisk() ([]string, error) {
	ents, err := s.readDir(s.root)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() {
			id, err := unescape(e.Name())
			if err != nil {
				return nil, err
			}
			out = append(out, id)
		}
	}
	return out, nil
}

// groupTaken reports whether any row of typ has group g.
func (s *Store) groupTaken(typ, g string) (bool, error) {
	owners, err := s.owners(typ)
	if err != nil {
		return false, err
	}
	for _, o := range owners {
		rows, err := s.ownerRows(typ, o)
		if err != nil {
			return false, err
		}
		for _, r := range rows {
			if r.Group == g {
				return true, nil
			}
		}
	}
	return false, nil
}

// Append implements [records.Store.Append].
func (s *Store) Append(ctx context.Context, r records.AppendRequest) error {
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	for _, g := range r.Groups {
		taken, err := s.groupTaken(g.Type, g.Group)
		if err != nil {
			return err
		}
		if taken {
			return fmt.Errorf("%w: %s", records.ErrGroupExists, g.Group)
		}
	}
	byFile := map[string][]records.Record{}
	for _, rec := range r.Records {
		p := path.Join(s.ownerDir(rec.Type, rec.Owner), monthFile(rec.TS))
		byFile[p] = append(byFile[p], rec)
	}
	files := make([]string, 0, len(byFile))
	for p := range byFile {
		files = append(files, p)
	}
	sort.Strings(files)
	// Read every file first, so a damaged one fails the append before
	// anything is written.
	merged := make(map[string][]records.Record, len(files))
	for _, p := range files {
		rows, err := s.readFile(p)
		if err != nil {
			return err
		}
		merged[p] = append(rows, byFile[p]...)
	}
	for _, p := range files {
		if err := s.writeFile(p, merged[p]); err != nil {
			return err
		}
	}
	return nil
}

// List implements [records.Store.List].
func (s *Store) List(ctx context.Context, q records.ListQuery) (records.Page, error) {
	micros, id, err := records.ParseCursor(q.After)
	if err != nil {
		return records.Page{}, err
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return records.Page{}, err
	}
	defer unlock()
	owners := []string{q.Owner}
	if q.Owner == "" {
		if owners, err = s.owners(q.Type); err != nil {
			return records.Page{}, err
		}
	}
	var rows []records.Record
	for _, o := range owners {
		got, err := s.ownerRows(q.Type, o)
		if err != nil {
			return records.Page{}, err
		}
		for _, r := range got {
			if q.Group != "" && r.Group != q.Group {
				continue
			}
			if !records.InRange(r.TS, q.From, q.To) {
				continue
			}
			if q.After != "" && !records.After(r, micros, id) {
				continue
			}
			rows = append(rows, r)
		}
	}
	slices.SortFunc(rows, records.Compare)
	limit := records.PageLimit(q)
	page := records.Page{Records: rows}
	if len(rows) > limit {
		page.Records = rows[:limit]
		page.Next = records.Cursor(rows[limit-1])
	}
	return page, nil
}

// Owners implements [records.Store.Owners].
func (s *Store) Owners(ctx context.Context, typ string) ([]string, error) {
	unlock, err := s.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	return s.owners(typ)
}

// RekeyOwner implements [records.Store.RekeyOwner].
func (s *Store) RekeyOwner(ctx context.Context, oldID, newID string) error {
	if oldID == newID {
		return nil
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	types, err := s.typesOnDisk()
	if err != nil {
		return err
	}
	for _, typ := range types {
		rows, err := s.ownerRows(typ, oldID)
		if err != nil || len(rows) == 0 {
			if err != nil {
				return err
			}
			continue
		}
		byFile := map[string][]records.Record{}
		for _, r := range rows {
			r.Owner = newID
			p := path.Join(s.ownerDir(typ, newID), monthFile(r.TS))
			byFile[p] = append(byFile[p], r)
		}
		for p, moved := range byFile {
			existing, err := s.readFile(p)
			if err != nil {
				return err
			}
			if err := s.writeFile(p, append(existing, moved...)); err != nil {
				return err
			}
		}
		if err := s.removeOwner(typ, oldID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) removeOwner(typ, owner string) error {
	dir := s.ownerDir(typ, owner)
	ents, err := s.readDir(dir)
	if err != nil {
		return err
	}
	for _, e := range ents {
		if err := s.fs.Remove(path.Join(dir, e.Name())); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	if err := s.fs.Remove(dir); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// DropOwner implements [records.Store.DropOwner].
func (s *Store) DropOwner(ctx context.Context, id string) error {
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	types, err := s.typesOnDisk()
	if err != nil {
		return err
	}
	for _, typ := range types {
		if err := s.removeOwner(typ, id); err != nil {
			return err
		}
	}
	return nil
}

// Expire implements [records.Store.Expire].
func (s *Store) Expire(ctx context.Context, typ string, before time.Time) (int64, error) {
	unlock, err := s.lock(ctx)
	if err != nil {
		return 0, err
	}
	defer unlock()
	owners, err := s.owners(typ)
	if err != nil {
		return 0, err
	}
	var n int64
	for _, o := range owners {
		dir := s.ownerDir(typ, o)
		ents, err := s.readDir(dir)
		if err != nil {
			return n, err
		}
		for _, e := range ents {
			p := path.Join(dir, e.Name())
			rows, err := s.readFile(p)
			if err != nil {
				return n, err
			}
			kept := rows[:0]
			for _, r := range rows {
				if r.TS.Before(before) {
					n++
					continue
				}
				kept = append(kept, r)
			}
			if len(kept) != len(rows) {
				if err := s.writeFile(p, kept); err != nil {
					return n, err
				}
			}
		}
		if rest, err := s.readDir(dir); err == nil && len(rest) == 0 {
			_ = s.fs.Remove(dir)
		}
	}
	return n, nil
}
