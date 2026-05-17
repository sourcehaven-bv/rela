package audit_test

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

func TestNewFilesystem_RejectsEmptyDir(t *testing.T) {
	_, err := audit.NewFilesystem("")
	if err == nil {
		t.Fatal("expected error for empty dir")
	}
}

func TestFilesystem_RecordWritesJSONL(t *testing.T) {
	dir := t.TempDir()
	auditDir := filepath.Join(dir, "audit")
	clock := fixedClock(time.Date(2026, 5, 17, 8, 0, 0, 0, time.UTC))
	a, err := audit.NewFilesystem(auditDir, audit.WithClock(clock.now))
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}

	a.Record(audit.Record{
		Op:        audit.OpCreateEntity,
		Subject:   &audit.Subject{Kind: "entity", Type: "ticket", ID: "TKT-1"},
		Principal: principal.Principal{User: "alice", Tool: principal.ToolCLI},
	})

	lines := readLines(t, filepath.Join(auditDir, "2026-05-17.jsonl"))
	if len(lines) != 1 {
		t.Fatalf("want 1 line, got %d", len(lines))
	}
	var got audit.Record
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Op != audit.OpCreateEntity {
		t.Errorf("Op: got %q", got.Op)
	}
	if got.Subject.ID != "TKT-1" {
		t.Errorf("Subject.ID: got %q", got.Subject.ID)
	}
}

func TestFilesystem_DailyRotation(t *testing.T) {
	dir := t.TempDir()
	auditDir := filepath.Join(dir, "audit")
	clock := fixedClock(time.Date(2026, 5, 17, 23, 59, 0, 0, time.UTC))
	a, err := audit.NewFilesystem(auditDir, audit.WithClock(clock.now))
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}

	a.Record(audit.Record{Op: audit.OpCreateEntity, Subject: &audit.Subject{Kind: "entity", ID: "first"}})

	// Cross midnight UTC.
	clock.set(time.Date(2026, 5, 18, 0, 1, 0, 0, time.UTC))
	a.Record(audit.Record{Op: audit.OpCreateEntity, Subject: &audit.Subject{Kind: "entity", ID: "second"}})

	day1 := readLines(t, filepath.Join(auditDir, "2026-05-17.jsonl"))
	day2 := readLines(t, filepath.Join(auditDir, "2026-05-18.jsonl"))

	if len(day1) != 1 || !strings.Contains(day1[0], `"id":"first"`) {
		t.Errorf("day 1 file wrong: %v", day1)
	}
	if len(day2) != 1 || !strings.Contains(day2[0], `"id":"second"`) {
		t.Errorf("day 2 file wrong: %v", day2)
	}
}

func TestFilesystem_ConcurrentRecordWithRotation(t *testing.T) {
	dir := t.TempDir()
	auditDir := filepath.Join(dir, "audit")
	clock := fixedClock(time.Date(2026, 5, 17, 23, 59, 0, 0, time.UTC))
	a, err := audit.NewFilesystem(auditDir, audit.WithClock(clock.now))
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}

	const writers = 50
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range writers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			// Half the writers see "today", the rest see "tomorrow".
			if i%2 == 0 {
				clock.set(time.Date(2026, 5, 17, 23, 59, 0, 0, time.UTC))
			} else {
				clock.set(time.Date(2026, 5, 18, 0, 1, 0, 0, time.UTC))
			}
			a.Record(audit.Record{Op: audit.OpCreateEntity})
		}(i)
	}
	close(start)
	wg.Wait()

	day1 := readLines(t, filepath.Join(auditDir, "2026-05-17.jsonl"))
	day2 := readLines(t, filepath.Join(auditDir, "2026-05-18.jsonl"))
	if got := len(day1) + len(day2); got != writers {
		t.Errorf("expected %d total lines, got %d (day1=%d day2=%d)", writers, got, len(day1), len(day2))
	}
	// Every line must be valid JSON (no torn writes).
	for _, line := range append(day1, day2...) {
		var rec audit.Record
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Errorf("torn line: %q: %v", line, err)
		}
	}
}

func TestFilesystem_FileMode0o600AndDir0o700(t *testing.T) {
	if runtimeIsWindows() {
		t.Skip("file modes not applicable on windows")
	}
	dir := t.TempDir()
	auditDir := filepath.Join(dir, "audit")
	a, err := audit.NewFilesystem(auditDir)
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}
	a.Record(audit.Record{Op: audit.OpCreateEntity})

	dirInfo, err := os.Stat(auditDir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("dir mode = %o, want 0700", perm)
	}

	matches, err := filepath.Glob(filepath.Join(auditDir, "*.jsonl"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("glob: %v matches=%v", err, matches)
	}
	fileInfo, err := os.Stat(matches[0])
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if perm := fileInfo.Mode().Perm(); perm != 0o600 {
		t.Errorf("file mode = %o, want 0600", perm)
	}
}

func TestFilesystem_SymlinkedDirRefused(t *testing.T) {
	if runtimeIsWindows() {
		t.Skip("symlinks behave differently on windows")
	}
	tempBase := t.TempDir()
	realDir := filepath.Join(tempBase, "real")
	if err := os.MkdirAll(realDir, 0o700); err != nil {
		t.Fatalf("mkdir real: %v", err)
	}
	auditDir := filepath.Join(tempBase, "audit")
	if err := os.Symlink(realDir, auditDir); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	a, err := audit.NewFilesystem(auditDir)
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}
	// First Record must NOT panic and must NOT write into the symlink target.
	a.Record(audit.Record{Op: audit.OpCreateEntity})

	matches, _ := filepath.Glob(filepath.Join(realDir, "*.jsonl"))
	if len(matches) != 0 {
		t.Errorf("audit wrote into symlink target despite refusal: %v", matches)
	}
	// Subsequent records also skip (backend in retry cool-down).
	a.Record(audit.Record{Op: audit.OpDeleteEntity})
	matches, _ = filepath.Glob(filepath.Join(realDir, "*.jsonl"))
	if len(matches) != 0 {
		t.Errorf("audit kept writing after symlink refusal: %v", matches)
	}
}

// TestFilesystem_TransientFailureRecovers verifies the retry behavior:
// a rotate failure puts the backend into cool-down; once the
// failure-condition is removed and the cool-down expires, the next
// Record succeeds. Replaces the previous "permanent disable" bug.
func TestFilesystem_TransientFailureRecovers(t *testing.T) {
	if runtimeIsWindows() {
		t.Skip("symlinks behave differently on windows")
	}
	tempBase := t.TempDir()
	auditDir := filepath.Join(tempBase, "audit")
	realDir := filepath.Join(tempBase, "real")
	if err := os.MkdirAll(realDir, 0o700); err != nil {
		t.Fatalf("mkdir real: %v", err)
	}
	if err := os.Symlink(realDir, auditDir); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	// Inject a clock so we can advance past the cool-down without
	// time.Sleep.
	clock := fixedClock(time.Date(2026, 5, 17, 8, 0, 0, 0, time.UTC))
	a, err := audit.NewFilesystem(auditDir, audit.WithClock(clock.now))
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}

	// First record fails (symlinked dir) — cool-down begins.
	a.Record(audit.Record{Op: audit.OpCreateEntity, Subject: &audit.Subject{Kind: "entity", ID: "first"}})

	// Remove the symlink and create a real directory in its place.
	if err := os.Remove(auditDir); err != nil {
		t.Fatalf("remove symlink: %v", err)
	}

	// Within cool-down: still skipped even though the underlying
	// problem is fixed.
	a.Record(audit.Record{Op: audit.OpCreateEntity, Subject: &audit.Subject{Kind: "entity", ID: "skipped"}})
	if _, err := os.Stat(auditDir); !os.IsNotExist(err) {
		// auditDir created during the skipped Record is a regression.
		t.Errorf("backend tried to write during cool-down")
	}

	// Advance past cool-down — next Record retries and succeeds.
	clock.set(time.Date(2026, 5, 17, 8, 2, 0, 0, time.UTC))
	a.Record(audit.Record{Op: audit.OpCreateEntity, Subject: &audit.Subject{Kind: "entity", ID: "recovered"}})

	lines := readLines(t, filepath.Join(auditDir, "2026-05-17.jsonl"))
	if len(lines) != 1 {
		t.Fatalf("expected 1 line after recovery, got %d", len(lines))
	}
	if !strings.Contains(lines[0], `"id":"recovered"`) {
		t.Errorf("recovered line missing expected id: %q", lines[0])
	}
}

func TestFilesystem_SanitizationStripsControlChars(t *testing.T) {
	dir := t.TempDir()
	auditDir := filepath.Join(dir, "audit")
	a, err := audit.NewFilesystem(auditDir)
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}

	a.Record(audit.Record{
		Op:        audit.OpCreateEntity,
		Subject:   &audit.Subject{Kind: "entity", Type: "tick\net", ID: "TKT-\x001"},
		Principal: principal.Principal{User: "al\x1bice", Tool: principal.ToolCLI},
		Summary:   "created\nwith newline",
	})

	lines := readLines(t, mustGlobOne(t, filepath.Join(auditDir, "*.jsonl")))
	if len(lines) != 1 {
		t.Fatalf("want 1 line, got %d", len(lines))
	}
	line := lines[0]

	// No raw control bytes survive into the JSONL stream (encoding/json
	// would escape them, but our sanitizer replaces them entirely).
	for _, bad := range []string{"\n", "\x00", "\x1b"} {
		if strings.Contains(line, bad) {
			t.Errorf("expected control char %q to be stripped, line: %q", bad, line)
		}
	}
	// Test still produces parseable JSON.
	var rec audit.Record
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !strings.Contains(rec.Summary, "created") {
		t.Errorf("Summary lost its content: %q", rec.Summary)
	}
}

func TestFilesystem_SanitizationTruncates(t *testing.T) {
	dir := t.TempDir()
	auditDir := filepath.Join(dir, "audit")
	a, err := audit.NewFilesystem(auditDir)
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}

	long := strings.Repeat("x", 2000)
	a.Record(audit.Record{
		Op:      audit.OpCreateEntity,
		Subject: &audit.Subject{Kind: "entity", ID: long},
	})

	lines := readLines(t, mustGlobOne(t, filepath.Join(auditDir, "*.jsonl")))
	var rec audit.Record
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(rec.Subject.ID) != 1024 {
		t.Errorf("Subject.ID len = %d, want 1024 (truncation)", len(rec.Subject.ID))
	}
}

// --- helpers ---

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func fixedClock(t time.Time) *clock { return &clock{t: t} }

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = t
}

// atomicBool guards against unused-import warnings if tests evolve.
var _ atomic.Bool

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var lines []string
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for s.Scan() {
		lines = append(lines, s.Text())
	}
	if err := s.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	return lines
}

func mustGlobOne(t *testing.T, pattern string) string {
	t.Helper()
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob %s: %v", pattern, err)
	}
	if len(matches) != 1 {
		t.Fatalf("glob %s: want 1 match, got %d", pattern, len(matches))
	}
	return matches[0]
}

func runtimeIsWindows() bool {
	return os.PathSeparator == '\\'
}
