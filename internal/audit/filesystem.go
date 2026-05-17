package audit

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
)

// Filesystem is the production [Audit] backend. It appends one
// JSON-object-per-line to .rela/audit/YYYY-MM-DD.jsonl, rotating
// daily on UTC midnight.
//
// Concurrency: every operation (rotation check, file open, append)
// runs under f.mu so a concurrent writer never observes a half-
// rotated state. Tested with -race (AC9).
//
// Security:
//   - Files opened with O_APPEND|O_CREATE|O_WRONLY|O_NOFOLLOW and mode [fileMode].
//   - The audit directory is Lstat'd before MkdirAll; if it is a symlink,
//     the backend disables itself for the rest of the process (slog.Error;
//     subsequent Record calls become no-ops). Refusing-and-degrading is
//     correct here per AC14 — an unwritable audit must not block legitimate
//     entity writes.
//
// Sanitization: every string field on Record is sanitized at this
// layer — truncated to 1024 chars (UTF-8 safe) and C0/DEL control
// chars replaced with non-breaking space. [Memory] retains raw bytes
// for test assertions; that asymmetry is documented (AC15).
type Filesystem struct {
	dir   string
	clock func() time.Time

	mu          sync.Mutex
	currentDate string
	file        *os.File
	disabled    bool // set when symlink detected or open keeps failing
}

// filesystemConfig is the receiver for [Option]s.
type filesystemConfig struct {
	clock func() time.Time
}

// Option configures a [Filesystem] at construction.
type Option func(*filesystemConfig)

// WithClock injects a clock for rotation testing. Production code
// omits this and gets time.Now (UTC applied internally).
func WithClock(now func() time.Time) Option {
	return func(c *filesystemConfig) { c.clock = now }
}

// NewFilesystem constructs a Filesystem backed by dir. Returns an
// error if dir is empty. The directory is *not* created here — it is
// lazily created on the first [Record] call, with a symlink check.
// This keeps construction cheap and side-effect-free for callers that
// may never actually write.
func NewFilesystem(dir string, opts ...Option) (*Filesystem, error) {
	if dir == "" {
		return nil, errors.New("audit: NewFilesystem: dir is required")
	}
	cfg := filesystemConfig{clock: func() time.Time { return time.Now().UTC() }}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &Filesystem{dir: dir, clock: cfg.clock}, nil
}

// Record sanitizes rec and appends it to the current day's JSONL
// file, rotating if the UTC date has changed since the last write.
// Errors are logged via slog and otherwise swallowed — audit must
// never block an entity write.
func (f *Filesystem) Record(rec Record) {
	rec = sanitize(rec)

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.disabled {
		return
	}

	today := f.clock().UTC().Format("2006-01-02")
	if today != f.currentDate || f.file == nil {
		if err := f.rotateLocked(today); err != nil {
			slog.Error("audit.write_failed", "stage", "rotate", "error", err)
			f.disabled = true
			return
		}
	}

	line, err := json.Marshal(rec)
	if err != nil {
		// Unreachable for well-formed Records (encoding/json on plain
		// structs with primitive fields never errors), but logged for
		// completeness.
		slog.Error("audit.write_failed", "stage", "marshal", "error", err)
		return
	}
	if _, err := f.file.Write(append(line, '\n')); err != nil {
		slog.Error("audit.write_failed", "stage", "write", "error", err)
	}
}

// rotateLocked closes the current file (if any), creates the audit
// dir (with symlink check), and opens today's file. Must be called
// with f.mu held.
func (f *Filesystem) rotateLocked(today string) error {
	if f.file != nil {
		_ = f.file.Close()
		f.file = nil
	}
	if err := ensureDirSafe(f.dir); err != nil {
		return err
	}
	path := filepath.Join(f.dir, today+".jsonl")
	file, err := os.OpenFile(
		path,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY|syscall.O_NOFOLLOW,
		fileMode,
	)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	f.file = file
	f.currentDate = today
	return nil
}

// ensureDirSafe creates dir with mode 0o700 if missing. If dir
// already exists and is a symlink, returns an error rather than
// using it (defense against attacker-planted redirects to elsewhere).
func ensureDirSafe(dir string) error {
	info, err := os.Lstat(dir)
	switch {
	case err == nil:
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to use symlinked audit dir %q", dir)
		}
		if !info.IsDir() {
			return fmt.Errorf("audit dir %q is not a directory", dir)
		}
		return nil
	case os.IsNotExist(err):
		return os.MkdirAll(dir, dirMode)
	default:
		return fmt.Errorf("lstat %s: %w", dir, err)
	}
}

// fieldLimit caps each string field at 1024 chars to keep JSONL
// lines bounded. 1024 is generous for entity IDs, automation names,
// and summaries while still preventing pathological inputs from
// blowing up the log.
const fieldLimit = 1024

// dirMode / fileMode are the security-relevant filesystem modes the
// audit backend uses. Owner-only (0o700 / 0o600) — audit content is
// effectively a forensic record of operator activity.
const (
	dirMode  os.FileMode = 0o700
	fileMode os.FileMode = 0o600
)

// sanitize returns a copy of rec with every string field truncated
// and control chars replaced. C0 (\x00-\x1f) and DEL (\x7f) become
// non-breaking space ( ); printable UTF-8 is untouched.
//
// Sanitization runs once at the JSONL boundary because that's the
// stream consumers actually see — Memory holds raw bytes for tests.
func sanitize(rec Record) Record {
	rec.Op = clean(rec.Op)
	rec.Subject = sanitizeSubject(rec.Subject)
	rec.Before = sanitizeSubject(rec.Before)
	rec.After = sanitizeSubject(rec.After)
	rec.Principal.User = clean(rec.Principal.User)
	rec.Principal.Tool = clean(rec.Principal.Tool)
	rec.TriggeredBy = clean(rec.TriggeredBy)
	rec.Summary = clean(rec.Summary)
	return rec
}

func sanitizeSubject(s Subject) Subject {
	s.Kind = clean(s.Kind)
	s.Type = clean(s.Type)
	s.ID = clean(s.ID)
	s.RelationType = clean(s.RelationType)
	s.FromID = clean(s.FromID)
	s.ToID = clean(s.ToID)
	return s
}

// clean truncates s to fieldLimit (UTF-8 safe) and replaces control
// chars with non-breaking space.
func clean(s string) string {
	if s == "" {
		return s
	}
	s = truncateRunes(s, fieldLimit)
	if !needsControlCharReplace(s) {
		return s
	}
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if isControlRune(r) {
			out = append(out, ' ')
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

func truncateRunes(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	out := make([]rune, 0, limit)
	for i, r := range []rune(s) {
		if i >= limit {
			break
		}
		out = append(out, r)
	}
	return string(out)
}

func needsControlCharReplace(s string) bool {
	for _, r := range s {
		if isControlRune(r) {
			return true
		}
	}
	return false
}

func isControlRune(r rune) bool {
	return (r >= 0 && r <= 0x1f) || r == 0x7f
}
