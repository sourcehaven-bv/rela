package filechannels

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// tailRecorder records what a hintTail delivers.
type tailRecorder struct {
	mu    sync.Mutex
	lines []string
	gaps  int
}

func (r *tailRecorder) deliver(b []byte) {
	r.mu.Lock()
	r.lines = append(r.lines, string(b))
	r.mu.Unlock()
}

func (r *tailRecorder) gap() {
	r.mu.Lock()
	r.gaps++
	r.mu.Unlock()
}

func (r *tailRecorder) take() (lines []string, gaps int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	lines, gaps = r.lines, r.gaps
	r.lines, r.gaps = nil, 0
	return lines, gaps
}

func newHintTestFeed(t *testing.T) (f *HintFeed, dir string) {
	t.Helper()
	dir = filepath.Join(t.TempDir(), "channels")
	f, err := NewHintFeed(newDiskStore(t, dir))
	require.NoError(t, err)
	return f, dir
}

func newTail(f *HintFeed) (*hintTail, *tailRecorder) {
	r := &tailRecorder{}
	t := &hintTail{feed: f, deliver: r.deliver, gap: r.gap}
	t.start()
	return t, r
}

// appendRaw appends b to the journal as another process would, bypassing
// SendHint's checks.
func appendRaw(t *testing.T, dir string, b []byte) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(hintFile))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	fh, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	_, err = fh.Write(b)
	require.NoError(t, err)
	require.NoError(t, fh.Close())
}

func journalSize(t *testing.T, dir string) int64 {
	t.Helper()
	info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(hintFile)))
	require.NoError(t, err)
	return info.Size()
}

func TestNewHintFeed(t *testing.T) {
	_, err := NewHintFeed(nil)
	require.Error(t, err)
	mem, err := newStore(storage.NewMemFS(), "/channels", filelock.ProcessPrivate{}, SegmentSize)
	require.NoError(t, err)
	_, err = NewHintFeed(mem)
	require.Error(t, err, "an in-memory store has no other process to tell")
}

func TestHintFeed_RoundTripAndStartOffset(t *testing.T) {
	ctx := context.Background()
	f, dir := newHintTestFeed(t)
	require.NoError(t, f.SendHint(ctx, []byte(`before`)))
	tail, rec := newTail(f)
	require.NoError(t, f.SendHint(ctx, []byte(`one`)))
	require.NoError(t, f.SendHint(ctx, []byte(`two`)))
	tail.poll()
	lines, gaps := rec.take()
	require.Equal(t, []string{"one", "two"}, lines, "only lines written after the start")
	require.Zero(t, gaps)

	info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(hintFile)))
	require.NoError(t, err)
	if os.PathSeparator == '/' {
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
	require.Error(t, f.SendHint(ctx, []byte("a\nb")), "a newline would split the hint")
}

func TestHintFeed_PartialLineWaits(t *testing.T) {
	f, dir := newHintTestFeed(t)
	tail, rec := newTail(f)
	appendRaw(t, dir, []byte(`par`))
	tail.poll()
	lines, _ := rec.take()
	require.Empty(t, lines)
	appendRaw(t, dir, []byte("tial\nnext\n"))
	tail.poll()
	lines, gaps := rec.take()
	require.Equal(t, []string{"partial", "next"}, lines)
	require.Zero(t, gaps)
}

func TestHintFeed_MissingThenCreated(t *testing.T) {
	f, _ := newHintTestFeed(t)
	tail, rec := newTail(f)
	tail.poll()
	require.NoError(t, f.SendHint(context.Background(), []byte(`first`)))
	tail.poll()
	lines, gaps := rec.take()
	require.Equal(t, []string{"first"}, lines)
	require.Zero(t, gaps, "a journal that did not exist yet loses nothing")
}

func TestHintFeed_Rotation(t *testing.T) {
	ctx := context.Background()
	f, dir := newHintTestFeed(t)
	appendRaw(t, dir, append(bytes.Repeat([]byte("x"), hintRotateBytes), '\n'))
	tail, rec := newTail(f)
	require.NoError(t, f.SendHint(ctx, []byte(`after`)))
	_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(hintOldFile)))
	require.NoError(t, err, "the journal was renamed away")
	require.Equal(t, int64(len("after\n")), journalSize(t, dir))
	tail.poll()
	lines, gaps := rec.take()
	require.Equal(t, []string{"after"}, lines)
	require.Equal(t, 1, gaps, "another file is a gap")
}

func TestHintFeed_RotatedAwayBeforePoll(t *testing.T) {
	f, dir := newHintTestFeed(t)
	require.NoError(t, f.SendHint(context.Background(), []byte(`a`)))
	tail, rec := newTail(f)
	require.NoError(t, os.Rename(filepath.Join(dir, filepath.FromSlash(hintFile)),
		filepath.Join(dir, filepath.FromSlash(hintOldFile))))
	tail.poll()
	_, gaps := rec.take()
	require.Equal(t, 1, gaps)
	require.NoError(t, f.SendHint(context.Background(), []byte(`b`)))
	tail.poll()
	lines, gaps := rec.take()
	require.Equal(t, []string{"b"}, lines)
	require.Zero(t, gaps)
}

func TestHintFeed_FailedRenameTruncatesPastHardCap(t *testing.T) {
	ctx := context.Background()
	f, dir := newHintTestFeed(t)
	renames := 0
	f.rename = func(string, string) error {
		renames++
		return errors.New("file in use")
	}

	// Below the hard cap a failed rename leaves the journal; the send
	// still lands.
	appendRaw(t, dir, append(bytes.Repeat([]byte("x"), hintRotateBytes), '\n'))
	before := journalSize(t, dir)
	require.NoError(t, f.SendHint(ctx, []byte(`kept`)))
	require.Equal(t, before+int64(len("kept\n")), journalSize(t, dir))
	require.Equal(t, 1, renames)
	// The next send does not try again until the journal grew.
	require.NoError(t, f.SendHint(ctx, []byte(`kept too`)))
	require.Equal(t, 1, renames, "a failed rename is not retried on every send")

	appendRaw(t, dir, append(bytes.Repeat([]byte("y"), hintHardCapBytes), '\n'))
	tail, rec := newTail(f)
	require.NoError(t, f.SendHint(ctx, []byte(`fresh`)))
	require.Equal(t, int64(len("fresh\n")), journalSize(t, dir))
	tail.poll()
	lines, gaps := rec.take()
	require.Equal(t, 1, gaps, "a shrunk journal is a gap")
	require.Equal(t, []string{"fresh"}, lines)
}

func TestHintFeed_TruncatedAndUnusableLines(t *testing.T) {
	f, dir := newHintTestFeed(t)
	require.NoError(t, f.SendHint(context.Background(), []byte(`one`)))
	tail, rec := newTail(f)

	// Truncated by hand: the same file, shorter.
	require.NoError(t, os.Truncate(filepath.Join(dir, filepath.FromSlash(hintFile)), 0))
	appendRaw(t, dir, []byte("\n"))
	tail.poll()
	lines, gaps := rec.take()
	require.Empty(t, lines)
	require.Equal(t, 2, gaps, "the shrink and the empty line")

	// A torn line: a writer died mid-line, and the next append follows it.
	// The feed delivers the joined line; the decoder rejects it as a gap.
	appendRaw(t, dir, []byte(`{"o":"x","h`))
	require.NoError(t, f.SendHint(context.Background(), []byte(`{"o":"y","h":[]}`)))
	tail.poll()
	lines, _ = rec.take()
	require.Equal(t, []string{`{"o":"x","h{"o":"y","h":[]}`}, lines)

	// An oversized line is skipped up to its newline, with one gap.
	appendRaw(t, dir, bytes.Repeat([]byte("z"), hintMaxLine+1))
	tail.poll()
	appendRaw(t, dir, []byte("zz\nok\n"))
	tail.poll()
	lines, gaps = rec.take()
	require.Equal(t, []string{"ok"}, lines)
	require.Equal(t, 1, gaps)
}

func TestHintFeed_FollowHintsPolls(t *testing.T) {
	f, _ := newHintTestFeed(t)
	f.poll = 5 * time.Millisecond
	rec := &tailRecorder{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- f.FollowHints(ctx, rec.deliver, rec.gap) }()
	require.Eventually(t, func() bool {
		rec.mu.Lock()
		defer rec.mu.Unlock()
		return rec.gaps == 1
	}, 5*time.Second, time.Millisecond, "the follower reports its start as a gap")
	require.NoError(t, f.SendHint(context.Background(), []byte(`live`)))
	require.Eventually(t, func() bool {
		rec.mu.Lock()
		defer rec.mu.Unlock()
		return len(rec.lines) == 1
	}, 5*time.Second, 5*time.Millisecond)
	cancel()
	require.NoError(t, <-done)
}

// hintHelperEnv makes the test binary append hint lines and exit, so the
// concurrent-append test runs real processes, not goroutines on one handle.
const hintHelperEnv = "RELA_HINT_APPEND_HELPER"

func TestHintFeedHelperProcess(t *testing.T) {
	spec := os.Getenv(hintHelperEnv)
	if spec == "" {
		t.Skip("helper process only")
	}
	parts := strings.Split(spec, "|")
	require.Len(t, parts, 4)
	dir, name := parts[0], parts[1]
	n, err := strconv.Atoi(parts[2])
	require.NoError(t, err)
	rotateAt, err := strconv.ParseInt(parts[3], 10, 64)
	require.NoError(t, err)
	f, err := NewHintFeed(newDiskStore(t, dir))
	require.NoError(t, err)
	f.rotateAt = rotateAt
	pad := strings.Repeat("p", 300)
	for i := range n {
		require.NoError(t, f.SendHint(context.Background(), fmt.Appendf(nil, "%s-%d-%s", name, i, pad)))
	}
}

// Processes append at once, and rotate the journal often while others
// append. No line is torn or written twice, and no line is lost except
// with a journal that a later rotation replaced, which a follower sees as
// a gap.
func TestHintFeed_ConcurrentProcesses(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns processes")
	}
	for _, tc := range []struct {
		name     string
		rotateAt int64
	}{
		{"one journal", hintRotateBytes},
		{"rotating", 16 << 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "channels")
			const procs, each = 4, 200
			var cmds []*exec.Cmd
			for p := range procs {
				cmd := exec.Command(os.Args[0], "-test.run=^TestHintFeedHelperProcess$")
				cmd.Env = append(os.Environ(),
					fmt.Sprintf("%s=%s|p%d|%d|%d", hintHelperEnv, dir, p, each, tc.rotateAt))
				var out bytes.Buffer
				cmd.Stdout, cmd.Stderr = &out, &out
				require.NoError(t, cmd.Start())
				cmds = append(cmds, cmd)
				t.Cleanup(func() {
					if t.Failed() {
						t.Log(out.String())
					}
				})
			}
			for _, c := range cmds {
				require.NoError(t, c.Wait())
			}
			seen := map[string]bool{}
			for _, name := range []string{hintFile, hintOldFile} {
				readJournalLines(t, filepath.Join(dir, filepath.FromSlash(name)), seen)
			}
			if tc.rotateAt == hintRotateBytes {
				require.Len(t, seen, procs*each)
				return
			}
			require.NotEmpty(t, seen)
			_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(hintOldFile)))
			require.NoError(t, err, "the journal rotated")
		})
	}
}

// readJournalLines adds the lines of the journal at path to seen, and fails
// on a torn or repeated line. A missing file adds nothing.
func readJournalLines(t *testing.T, path string, seen map[string]bool) {
	t.Helper()
	fh, err := os.Open(path)
	if os.IsNotExist(err) {
		return
	}
	require.NoError(t, err)
	defer fh.Close()
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, hintMaxLine), hintMaxLine)
	pad := strings.Repeat("p", 300)
	for sc.Scan() {
		line := sc.Text()
		head, tailPad, ok := strings.Cut(line, "-"+pad)
		require.True(t, ok && tailPad == "", "torn line %q", line)
		require.False(t, seen[head], "duplicate %q", head)
		seen[head] = true
	}
	require.NoError(t, sc.Err())
}
